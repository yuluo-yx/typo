package commands

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/yuluo-yx/typo/internal/storage"
)

// SubcommandCache represents cached subcommands for a tool.
type SubcommandCache struct {
	Tool        string              `json:"tool"`
	Subcommands []string            `json:"subcommands"`
	Children    map[string][]string `json:"children,omitempty"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

// SubcommandRegistry manages subcommands for various tools.
type SubcommandRegistry struct {
	mu          sync.RWMutex
	saveMu      sync.Mutex
	cache       map[string]*SubcommandCache
	cacheDir    string
	cacheExpiry time.Duration
	helpTimeout time.Duration
}

const (
	defaultHelpTimeout             = 1000 * time.Millisecond
	maxHierarchicalSubcommandDepth = 3
	rootSubcommandPath             = ""
)

// NewSubcommandRegistry creates a new subcommand registry.
func NewSubcommandRegistry(cacheDir string) *SubcommandRegistry {
	r := &SubcommandRegistry{
		cache:       make(map[string]*SubcommandCache),
		cacheDir:    cacheDir,
		cacheExpiry: 7 * 24 * time.Hour, // 7 days
		helpTimeout: defaultHelpTimeout,
	}
	r.loadCache()
	return r
}

// Get returns subcommands for a tool, fetching from cache or dynamically.
func (r *SubcommandRegistry) Get(tool string) []string {
	return r.GetChildren(tool, nil)
}

// GetChildren returns subcommands for a tool under the given prefix path.
func (r *SubcommandRegistry) GetChildren(tool string, prefix []string) []string {
	pathKey := subcommandPathKey(prefix)
	builtin := builtinSubcommandsForPath(tool, prefix)

	r.mu.RLock()
	if cached, ok := r.cache[tool]; ok && time.Since(cached.UpdatedAt) < r.cacheExpiry {
		if children, found := cached.childrenFor(pathKey); found {
			r.mu.RUnlock()
			return mergeUniqueStrings(children, builtin...)
		}
	}
	r.mu.RUnlock()

	subcommands := mergeUniqueStrings(r.fetchSubcommands(tool, prefix...), builtin...)
	if len(subcommands) == 0 {
		return nil
	}

	r.mu.Lock()
	cache := r.cache[tool]
	if cache == nil {
		cache = &SubcommandCache{Tool: tool}
		r.cache[tool] = cache
	}
	cache.setChildren(pathKey, subcommands)
	cache.UpdatedAt = time.Now()
	r.mu.Unlock()
	r.saveCache()

	return append([]string(nil), subcommands...)
}

// fetchSubcommands dynamically fetches subcommands for a tool.
// Results are cached to ~/.typo/subcommands.json
func (r *SubcommandRegistry) fetchSubcommands(tool string, prefix ...string) []string {
	// Check if tool exists in PATH
	if GetPath(tool) == "" {
		return nil
	}

	// Try to get help output
	helpOutput, err := r.getHelpOutputAtPath(tool, prefix...)
	if err != nil || helpOutput == "" {
		return nil
	}

	return parseToolHelp(tool, prefix, helpOutput)
}

func parseToolHelp(tool string, prefix []string, helpOutput string) []string {
	switch tool {
	case "git":
		return parseGitHelp(helpOutput)
	case "docker":
		return parseDockerHelp(helpOutput)
	case "npm":
		return parseNpmHelp(helpOutput)
	case "yarn":
		return parseYarnHelp(helpOutput)
	case "kubectl":
		return parseKubectlHelp(helpOutput)
	case "cargo":
		return parseCargoHelp(helpOutput)
	case "go":
		return parseGoHelp(helpOutput)
	case "brew":
		return parseBrewHelp(helpOutput)
	case "aws":
		return parseAWSHelp(helpOutput)
	case "gcloud":
		return parseGCloudHelp(helpOutput)
	case "az":
		return parseAzureHelp(helpOutput)
	default:
		return parseGenericHelp(helpOutput)
	}
}

func (r *SubcommandRegistry) getHelpOutputAtPath(tool string, prefix ...string) (string, error) {
	if len(prefix) > 0 {
		return r.getNestedHelpOutput(tool, prefix)
	}
	return r.getHelpOutput(tool)
}

func (r *SubcommandRegistry) getHelpOutput(tool string) (string, error) {
	// Special handling for git - use 'help -a' for all commands
	if tool == "git" {
		output, err := r.runHelpCommand("git", "help", "-a")
		if err == nil {
			return output, nil
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return "", err
		}
	}

	// Special handling for brew - use 'commands' to get command list
	if tool == "brew" {
		output, err := r.runHelpCommand("brew", "commands")
		if err == nil {
			return output, nil
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return "", err
		}
	}

	// Try --help first
	output, err := r.runHelpCommand(tool, "--help")
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", err
		}
		// Try help subcommand
		output, err = r.runHelpCommand(tool, "help")
		if err != nil {
			return "", err
		}
	}
	return output, nil
}

func (r *SubcommandRegistry) getNestedHelpOutput(tool string, prefix []string) (string, error) {
	if len(prefix) == 0 || len(prefix) > maxHierarchicalSubcommandDepth || !supportsHierarchicalDiscovery(tool) {
		return "", nil
	}

	switch tool {
	case "aws":
		return r.runHelpCommand(tool, append(append([]string(nil), prefix...), "help")...)
	case "gcloud", "az":
		return r.runHelpCommand(tool, append(append([]string(nil), prefix...), "--help")...)
	default:
		return "", nil
	}
}

func (r *SubcommandRegistry) runHelpCommand(tool string, args ...string) (string, error) {
	timeout := r.helpTimeout
	if timeout <= 0 {
		timeout = defaultHelpTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.Command(tool, args...)
	configureHelpCommand(cmd)

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := cmd.Start(); err != nil {
		return "", err
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	select {
	case err := <-waitCh:
		return output.String(), err
	case <-ctx.Done():
		_ = killHelpCommand(cmd)
		<-waitCh
		return "", ctx.Err()
	}
}

func (r *SubcommandRegistry) loadCache() {
	if r.cacheDir == "" {
		return
	}

	cacheFile := filepath.Join(r.cacheDir, "subcommands.json")
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return
	}

	var caches []SubcommandCache
	if err := json.Unmarshal(data, &caches); err != nil {
		storage.QuarantineInvalidJSON(cacheFile, err)
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range caches {
		r.cache[caches[i].Tool] = &caches[i]
	}
}

func (r *SubcommandRegistry) saveCache() {
	if r.cacheDir == "" {
		return
	}

	r.saveMu.Lock()
	defer r.saveMu.Unlock()

	r.mu.RLock()
	caches := make([]SubcommandCache, 0, len(r.cache))
	for _, c := range r.cache {
		cache := SubcommandCache{
			Tool:        c.Tool,
			Subcommands: append([]string(nil), c.Subcommands...),
			UpdatedAt:   c.UpdatedAt,
		}
		if len(c.Children) > 0 {
			cache.Children = make(map[string][]string, len(c.Children))
			for key, values := range c.Children {
				cache.Children[key] = append([]string(nil), values...)
			}
		}
		caches = append(caches, cache)
	}
	r.mu.RUnlock()

	data, err := json.MarshalIndent(caches, "", "  ")
	if err != nil {
		return
	}

	_ = os.MkdirAll(r.cacheDir, 0755)
	cacheFile := filepath.Join(r.cacheDir, "subcommands.json")
	_ = storage.WriteFileAtomic(cacheFile, data, 0600)
}

// Parser functions for different tools

func parseGitHelp(output string) []string {
	// Git help format:
	//   add                  Add file contents to the index
	//   commit               Record changes to the repository
	subcommands := []string{}
	re := regexp.MustCompile(`^\s{2,}([\w-]+)\s{2,}`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if matches := re.FindStringSubmatch(line); len(matches) > 1 {
			subcommands = append(subcommands, matches[1])
		}
	}

	return subcommands
}

func parseDockerHelp(output string) []string {
	// Docker help format:
	//   builder     Manage builds
	//   build       Build an image from a Dockerfile
	subcommands := []string{}
	re := regexp.MustCompile(`^\s{2,}([\w-]+)\s+`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	inCommands := false

	for scanner.Scan() {
		line := scanner.Text()

		// Look for "Commands:" section
		if strings.Contains(line, "Commands:") || strings.Contains(line, "Management Commands:") {
			inCommands = true
			continue
		}

		if inCommands {
			if matches := re.FindStringSubmatch(line); len(matches) > 1 {
				subcommands = append(subcommands, matches[1])
			} else if line == "" && len(subcommands) > 0 {
				// Empty line after commands section
				break
			}
		}
	}

	return subcommands
}

func parseNpmHelp(output string) []string {
	// npm help format:
	// All commands:
	// 	   access, ...
	// 	   config, ...
	// 	   edit, ....
	subcommands := []string{}
	scanner := bufio.NewScanner(strings.NewReader(output))

	inCommandsSection := false

	for scanner.Scan() {
		line := scanner.Text()

		// Compatible with with npm v7+ (All commands) and v6- (where <command> is one of:)
		if strings.HasPrefix(line, "All commands:") || strings.Contains(line, "where <command> is one of:") {
			inCommandsSection = true
			continue
		}
		if inCommandsSection {
			if strings.TrimSpace(line) == "" {
				continue
			}
			// End of commands block
			if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
				inCommandsSection = false
				// Don't break to prevent future versions with multiple command sections from being missed
				continue
			}
			// Extract commands from the line
			parts := strings.Split(line, ",")
			for _, part := range parts {
				cmd := strings.TrimSpace(part)
				if cmd != "" {
					subcommands = append(subcommands, cmd)
				}
			}
		}
	}

	return subcommands
}

func parseYarnHelp(output string) []string {
	// yarn help format:
	//   add       Installs a package and any packages that it depends on.
	//   init      Interactively creates or updates a package.json file.
	subcommands := []string{}
	re := regexp.MustCompile(`^\s{2,}([\w-]+)\s+`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if matches := re.FindStringSubmatch(line); len(matches) > 1 {
			subcommands = append(subcommands, matches[1])
		}
	}

	return subcommands
}

func parseKubectlHelp(output string) []string {
	// kubectl help format:
	//   get           Display one or many resources
	//   describe      Show details of a specific resource or group of resources
	subcommands := []string{}
	re := regexp.MustCompile(`^\s{2,}([\w-]+)\s+`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if matches := re.FindStringSubmatch(line); len(matches) > 1 {
			subcommands = append(subcommands, matches[1])
		}
	}

	return subcommands
}

func parseCargoHelp(output string) []string {
	// cargo help format:
	//    build, b    Compile the current package
	//    check, c    Analyze the current package and report errors
	subcommands := []string{}
	re := regexp.MustCompile(`^\s{2,}([\w-]+)`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if matches := re.FindStringSubmatch(line); len(matches) > 1 {
			cmd := matches[1]
			// Skip aliases (after comma)
			if !strings.Contains(line, ",") || strings.Index(line, cmd) < strings.Index(line, ",") {
				subcommands = append(subcommands, cmd)
			}
		}
	}

	return subcommands
}

func parseGoHelp(output string) []string {
	// go help format:
	// build       compile packages and dependencies
	// clean       remove object files and cached files
	subcommands := []string{}
	re := regexp.MustCompile(`^\s+([\w-]+)\s+`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	inCommands := false
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if !inCommands {
			if trimmed == "The commands are:" {
				inCommands = true
			}
			continue
		}

		if trimmed == "" {
			if len(subcommands) > 0 {
				break
			}
			continue
		}

		if matches := re.FindStringSubmatch(line); len(matches) > 1 {
			subcommands = append(subcommands, matches[1])
			continue
		}

		if len(subcommands) > 0 {
			break
		}
	}

	return subcommands
}

func parseBrewHelp(output string) []string {
	// brew commands format:
	// ==> Built-in commands
	// --cache
	// --caskroom
	// install
	// list
	subcommands := []string{}
	re := regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and section headers (==> ...)
		if line == "" || strings.HasPrefix(line, "==>") {
			continue
		}
		// Skip internal commands starting with --
		if strings.HasPrefix(line, "--") {
			continue
		}
		// Only accept valid command names (letters, numbers, hyphens)
		if re.MatchString(line) {
			subcommands = append(subcommands, line)
		}
	}

	return subcommands
}

func parseAWSHelp(output string) []string {
	return parseSectionedHelp(output, "SERVICES", "AVAILABLE SERVICES", "COMMANDS", "AVAILABLE COMMANDS")
}

func parseGCloudHelp(output string) []string {
	return parseSectionedHelp(output, "GROUPS", "COMMANDS")
}

func parseAzureHelp(output string) []string {
	return parseSectionedHelp(output, "GROUPS", "SUBGROUPS", "COMMANDS")
}

func parseGenericHelp(output string) []string {
	// Generic help format - try to find lines that look like commands
	// Pattern: whitespace + word + whitespace + description
	subcommands := []string{}
	re := regexp.MustCompile(`^\s{2,}([a-z][a-z0-9-]*)\s{2,}`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if matches := re.FindStringSubmatch(line); len(matches) > 1 {
			subcommands = append(subcommands, matches[1])
		}
	}

	return subcommands
}

func parseSectionedHelp(output string, headers ...string) []string {
	allowed := make(map[string]bool, len(headers))
	for _, header := range headers {
		allowed[normalizeHelpSection(header)] = true
	}

	subcommands := make([]string, 0)
	seen := make(map[string]bool)
	re := regexp.MustCompile(`^[\t ]{2,}([a-z][a-z0-9-]*)\b`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	inSection := false
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		normalized := normalizeHelpSection(trimmed)
		if allowed[normalized] {
			inSection = true
			continue
		}

		if !inSection {
			continue
		}

		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			inSection = false
			continue
		}

		matches := re.FindStringSubmatch(line)
		if len(matches) <= 1 || seen[matches[1]] {
			continue
		}

		seen[matches[1]] = true
		subcommands = append(subcommands, matches[1])
	}

	return subcommands
}

func normalizeHelpSection(value string) string {
	trimmed := strings.TrimSpace(strings.TrimSuffix(value, ":"))
	return strings.ToUpper(trimmed)
}

// HasSubcommands checks if a tool is known to have subcommands.
func (r *SubcommandRegistry) HasSubcommands(tool string) bool {
	knownTools := map[string]bool{
		"git":       true,
		"docker":    true,
		"npm":       true,
		"yarn":      true,
		"kubectl":   true,
		"cargo":     true,
		"go":        true,
		"pip":       true,
		"pip3":      true,
		"composer":  true,
		"ansible":   true,
		"terraform": true,
		"helm":      true,
		"brew":      true,
		"aws":       true,
		"gcloud":    true,
		"az":        true,
	}
	return knownTools[tool]
}

var builtinSubcommands = map[string][]string{
	"git":       {"add", "branch", "checkout", "clone", "commit", "diff", "fetch", "init", "log", "merge", "pull", "push", "rebase", "remote", "restore", "status", "switch"},
	"docker":    {"build", "compose", "exec", "images", "logs", "ps", "pull", "push", "rm", "run", "start", "stop"},
	"npm":       {"ci", "install", "list", "login", "publish", "run", "test", "uninstall", "update"},
	"yarn":      {"add", "build", "cache", "create", "exec", "info", "init", "install", "remove", "run", "test", "upgrade"},
	"kubectl":   {"api-resources", "apply", "config", "create", "delete", "describe", "exec", "get", "logs", "patch", "rollout"},
	"cargo":     {"bench", "build", "check", "clean", "doc", "fmt", "help", "run", "test", "update"},
	"go":        {"build", "clean", "env", "fmt", "generate", "get", "install", "list", "mod", "run", "test", "tool"},
	"brew":      {"cleanup", "doctor", "info", "install", "list", "search", "tap", "uninstall", "update", "upgrade"},
	"terraform": {"apply", "destroy", "fmt", "import", "init", "output", "plan", "show", "state", "validate"},
	"helm":      {"dependency", "get", "install", "lint", "list", "package", "pull", "repo", "search", "template", "upgrade"},
	"aws":       {"cloudwatch", "dynamodb", "ec2", "iam", "lambda", "rds", "s3", "sns", "sqs", "sts"},
	"gcloud":    {"bigquery", "compute", "functions", "iam", "kubernetes", "pubsub", "services", "storage"},
	"az":        {"account", "aks", "functionapp", "group", "network", "storage", "vm", "webapp"},
}

var builtinSubcommandSet = buildBuiltinSubcommandSet()

// HasBuiltinSubcommand reports whether a tool's builtin subcommand set contains the given subcommand.
func HasBuiltinSubcommand(tool, subcommand string) bool {
	if tool == "" || subcommand == "" {
		return false
	}

	toolSet, ok := builtinSubcommandSet[tool]
	return ok && toolSet[subcommand]
}

func buildBuiltinSubcommandSet() map[string]map[string]bool {
	result := make(map[string]map[string]bool, len(builtinSubcommands))
	for tool, subcommands := range builtinSubcommands {
		set := make(map[string]bool, len(subcommands))
		for _, subcommand := range subcommands {
			set[subcommand] = true
		}
		result[tool] = set
	}
	return result
}

func builtinSubcommandsForTool(tool string) []string {
	return append([]string(nil), builtinSubcommands[tool]...)
}

func builtinSubcommandsForPath(tool string, prefix []string) []string {
	if len(prefix) > 0 {
		return nil
	}

	return builtinSubcommandsForTool(tool)
}

func mergeUniqueStrings(base []string, extra ...string) []string {
	result := append([]string(nil), base...)
	seen := make(map[string]bool, len(result)+len(extra))
	for _, item := range result {
		seen[item] = true
	}

	for _, item := range extra {
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}

	return result
}

func supportsHierarchicalDiscovery(tool string) bool {
	switch tool {
	case "aws", "gcloud", "az":
		return true
	default:
		return false
	}
}

func subcommandPathKey(prefix []string) string {
	if len(prefix) == 0 {
		return rootSubcommandPath
	}
	return strings.Join(prefix, " ")
}

func (c *SubcommandCache) childrenFor(pathKey string) ([]string, bool) {
	if pathKey == rootSubcommandPath {
		if len(c.Subcommands) == 0 {
			return nil, false
		}
		return append([]string(nil), c.Subcommands...), true
	}

	if len(c.Children) == 0 {
		return nil, false
	}

	children, ok := c.Children[pathKey]
	if !ok {
		return nil, false
	}

	return append([]string(nil), children...), true
}

func (c *SubcommandCache) setChildren(pathKey string, children []string) {
	cloned := append([]string(nil), children...)
	if pathKey == rootSubcommandPath {
		c.Subcommands = cloned
		return
	}

	if c.Children == nil {
		c.Children = make(map[string][]string)
	}
	c.Children[pathKey] = cloned
}

// PreFetch prefetches subcommands for common tools.
func (r *SubcommandRegistry) PreFetch() {
	tools := []string{"git", "docker", "npm", "yarn", "kubectl", "cargo", "go", "aws", "gcloud", "az"}

	var wg sync.WaitGroup
	for _, tool := range tools {
		wg.Add(1)
		go func(t string) {
			defer wg.Done()
			r.Get(t)
		}(tool)
	}
	wg.Wait()
}
