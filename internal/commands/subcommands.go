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
	Tool        string    `json:"tool"`
	Subcommands []string  `json:"subcommands"`
	UpdatedAt   time.Time `json:"updated_at"`
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

const defaultHelpTimeout = 1000 * time.Millisecond

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
	builtin := builtinSubcommandsForTool(tool)

	r.mu.RLock()
	if cached, ok := r.cache[tool]; ok {
		if time.Since(cached.UpdatedAt) < r.cacheExpiry {
			r.mu.RUnlock()
			return mergeUniqueStrings(cached.Subcommands, builtin...)
		}
	}
	r.mu.RUnlock()

	// Need to fetch
	subcommands := mergeUniqueStrings(r.fetchSubcommands(tool), builtin...)
	if len(subcommands) > 0 {
		r.mu.Lock()
		r.cache[tool] = &SubcommandCache{
			Tool:        tool,
			Subcommands: subcommands,
			UpdatedAt:   time.Now(),
		}
		r.mu.Unlock()
		r.saveCache()
	}

	return append([]string(nil), subcommands...)
}

// fetchSubcommands dynamically fetches subcommands for a tool.
// Results are cached to ~/.typo/subcommands.json
func (r *SubcommandRegistry) fetchSubcommands(tool string) []string {
	// Check if tool exists in PATH
	if GetPath(tool) == "" {
		return nil
	}

	// Try to get help output
	helpOutput, err := r.getHelpOutput(tool)
	if err != nil || helpOutput == "" {
		return nil
	}

	// Parse subcommands based on tool type
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
	default:
		return parseGenericHelp(helpOutput)
	}
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
		caches = append(caches, SubcommandCache{
			Tool:        c.Tool,
			Subcommands: append([]string(nil), c.Subcommands...),
			UpdatedAt:   c.UpdatedAt,
		})
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

// PreFetch prefetches subcommands for common tools.
func (r *SubcommandRegistry) PreFetch() {
	tools := []string{"git", "docker", "npm", "yarn", "kubectl", "cargo", "go"}

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
