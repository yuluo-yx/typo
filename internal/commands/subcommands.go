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
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yuluo-yx/typo/internal/storage"
	itypes "github.com/yuluo-yx/typo/internal/types"
	"github.com/yuluo-yx/typo/internal/utils"
)

// CacheHeader identifies the subcommand cache schema version.
type CacheHeader struct {
	SchemaVersion int `json:"schema_version"`
}

// ToolTreeCache stores one tool's tree-shaped subcommand cache.
type ToolTreeCache struct {
	Tool      string    `json:"tool"`
	Tree      *TreeNode `json:"tree"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TreeNode is the JSON tree node used by the v2 subcommand cache.
type TreeNode struct {
	Children    map[string]*TreeNode `json:"children,omitempty"`
	Terminal    bool                 `json:"terminal,omitempty"`
	Passthrough bool                 `json:"passthrough,omitempty"`
	Alias       string               `json:"alias,omitempty"`
}

// ToCommandTreeNode converts a cached JSON tree node into an engine command tree node.
func (n *TreeNode) ToCommandTreeNode() *itypes.CommandTreeNode {
	if n == nil {
		return nil
	}

	node := &itypes.CommandTreeNode{
		StopAfterMatch: n.Terminal && !n.Passthrough,
		Alias:          n.Alias,
	}
	if len(n.Children) > 0 {
		node.Children = make(map[string]*itypes.CommandTreeNode, len(n.Children))
		for name, child := range n.Children {
			node.Children[name] = child.ToCommandTreeNode()
		}
	}
	return node
}

func (n *TreeNode) clone() *TreeNode {
	if n == nil {
		return nil
	}

	cloned := &TreeNode{
		Terminal:    n.Terminal,
		Passthrough: n.Passthrough,
		Alias:       n.Alias,
	}
	if len(n.Children) > 0 {
		cloned.Children = make(map[string]*TreeNode, len(n.Children))
		for name, child := range n.Children {
			cloned.Children[name] = child.clone()
		}
	}
	return cloned
}

func (n *TreeNode) childTokens() []string {
	if n == nil || len(n.Children) == 0 {
		return nil
	}

	tokens := make([]string, 0, len(n.Children))
	for token := range n.Children {
		tokens = append(tokens, token)
	}
	sort.Strings(tokens)
	return tokens
}

// ToolTreeRegistry manages tree-shaped subcommands for external tools.
type ToolTreeRegistry struct {
	mu          sync.RWMutex
	saveMu      sync.Mutex
	trees       map[string]*ToolTreeCache
	cacheDir    string
	cacheExpiry time.Duration
	helpTimeout time.Duration
}

const (
	subcommandCacheSchemaVersion   = 2
	defaultHelpTimeout             = 1000 * time.Millisecond
	maxHierarchicalSubcommandDepth = 3
)

// NewToolTreeRegistry creates and loads the v2 tree-shaped subcommand cache.
func NewToolTreeRegistry(cacheDir string) *ToolTreeRegistry {
	r := &ToolTreeRegistry{
		trees:       make(map[string]*ToolTreeCache),
		cacheDir:    cacheDir,
		cacheExpiry: 7 * 24 * time.Hour, // 7 days
		helpTimeout: defaultHelpTimeout,
	}
	r.loadCache()
	return r
}

// Get returns a tool's root-level subcommands, running dynamic discovery when needed.
func (r *ToolTreeRegistry) Get(tool string) []string {
	return r.GetChildren(tool, nil)
}

// GetChildren returns subcommands under the given prefix path.
func (r *ToolTreeRegistry) GetChildren(tool string, prefix []string) []string {
	if tool == "" {
		return nil
	}

	if r.pathStopsAtTerminal(tool, prefix) {
		return nil
	}

	if cached := r.cachedChildren(tool, prefix); len(cached) > 0 {
		return cached
	}

	fetched := r.fetchSubcommands(tool, prefix...)
	children := utils.MergeUniqueStrings(fetched, builtinSubcommandsForPath(tool, prefix)...)
	if len(children) == 0 {
		return nil
	}

	r.storeChildren(tool, prefix, children)
	return append([]string(nil), children...)
}

// ResolveChild returns the canonical name for an exact matching child node.
func (r *ToolTreeRegistry) ResolveChild(tool string, prefix []string, token string) (string, bool) {
	if tool == "" || token == "" {
		return "", false
	}

	if node := r.cachedNode(tool, prefix); node != nil {
		if canonical, ok := resolveTreeChild(node, token); ok {
			return canonical, true
		}
	}
	if node := builtinNodeForPath(tool, prefix); node != nil {
		if canonical, ok := resolveTreeChild(node, token); ok {
			return canonical, true
		}
	}
	return "", false
}

func resolveTreeChild(node *TreeNode, token string) (string, bool) {
	if node == nil || len(node.Children) == 0 {
		return "", false
	}

	child, ok := node.Children[token]
	if !ok {
		return "", false
	}
	if child != nil && child.Alias != "" {
		return child.Alias, true
	}
	return token, true
}

func (r *ToolTreeRegistry) ensureTrees() {
	if r.trees != nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.trees == nil {
		r.trees = make(map[string]*ToolTreeCache)
	}
}

func (r *ToolTreeRegistry) expiry() time.Duration {
	if r == nil || r.cacheExpiry <= 0 {
		return 7 * 24 * time.Hour
	}
	return r.cacheExpiry
}

func (r *ToolTreeRegistry) cachedChildren(tool string, prefix []string) []string {
	node := r.cachedNode(tool, prefix)
	if node == nil || len(node.Children) == 0 {
		return nil
	}
	return utils.MergeUniqueStrings(node.childTokens(), builtinSubcommandsForPath(tool, prefix)...)
}

func (r *ToolTreeRegistry) cachedNode(tool string, prefix []string) *TreeNode {
	if r == nil {
		return nil
	}

	r.ensureTrees()
	r.mu.RLock()
	defer r.mu.RUnlock()

	cache := r.trees[tool]
	if cache == nil || cache.Tree == nil {
		return nil
	}
	if !cache.UpdatedAt.IsZero() && time.Since(cache.UpdatedAt) >= r.expiry() {
		return nil
	}

	node, ok := treeNodeForPath(cache.Tree, prefix)
	if !ok {
		return nil
	}
	return node.clone()
}

func (r *ToolTreeRegistry) pathStopsAtTerminal(tool string, prefix []string) bool {
	if len(prefix) == 0 {
		return false
	}

	if node := r.cachedNode(tool, prefix); isTerminalLeaf(node) {
		return true
	}
	return isTerminalLeaf(builtinNodeForPath(tool, prefix))
}

func isTerminalLeaf(node *TreeNode) bool {
	return node != nil && len(node.Children) == 0 && (node.Terminal || node.Passthrough)
}

func (r *ToolTreeRegistry) storeChildren(tool string, prefix []string, children []string) {
	if r == nil || tool == "" || len(children) == 0 {
		return
	}

	r.ensureTrees()
	now := time.Now()

	r.mu.Lock()
	cache := r.trees[tool]
	if cache == nil {
		cache = &ToolTreeCache{
			Tool: tool,
			Tree: builtinTreeForTool(tool).clone(),
		}
		if cache.Tree == nil {
			cache.Tree = &TreeNode{}
		}
		r.trees[tool] = cache
	}
	if cache.Tree == nil {
		cache.Tree = &TreeNode{}
	}

	node := ensureTreePath(cache.Tree, tool, prefix)
	if node.Children == nil {
		node.Children = make(map[string]*TreeNode, len(children))
	}
	for _, child := range children {
		if child == "" {
			continue
		}
		if node.Children[child] != nil {
			continue
		}
		if builtin := builtinNodeForPath(tool, appendPath(prefix, child)); builtin != nil {
			node.Children[child] = builtin.clone()
			continue
		}
		node.Children[child] = &TreeNode{}
	}
	cache.UpdatedAt = now
	r.mu.Unlock()

	r.saveCache()
}

func ensureTreePath(root *TreeNode, tool string, prefix []string) *TreeNode {
	node := root
	for i, token := range prefix {
		if node.Children == nil {
			node.Children = make(map[string]*TreeNode)
		}
		child := node.Children[token]
		if child == nil {
			if builtin := builtinNodeForPath(tool, prefix[:i+1]); builtin != nil {
				child = builtin.clone()
			} else {
				child = &TreeNode{}
			}
			node.Children[token] = child
		}
		node = child
	}
	return node
}

// fetchSubcommands dynamically fetches tool subcommands and caches results in ~/.typo/subcommands.json.
func (r *ToolTreeRegistry) fetchSubcommands(tool string, prefix ...string) []string {
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
		if len(prefix) > 0 {
			return parseGitNestedHelp(helpOutput)
		}
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

func (r *ToolTreeRegistry) getHelpOutputAtPath(tool string, prefix ...string) (string, error) {
	if len(prefix) > 0 {
		return r.getNestedHelpOutput(tool, prefix)
	}
	return r.getHelpOutput(tool)
}

func (r *ToolTreeRegistry) getHelpOutput(tool string) (string, error) {
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

func (r *ToolTreeRegistry) getNestedHelpOutput(tool string, prefix []string) (string, error) {
	if len(prefix) == 0 || len(prefix) > maxHierarchicalSubcommandDepth || !supportsHierarchicalDiscovery(tool) {
		return "", nil
	}

	switch tool {
	case "git":
		if len(prefix) != 1 {
			return "", nil
		}
		return r.runHelpCommand(tool, append(append([]string(nil), prefix...), "-h")...)
	case "docker":
		if len(prefix) != 1 {
			return "", nil
		}
		return r.runHelpCommand(tool, append(append([]string(nil), prefix...), "--help")...)
	case "aws":
		return r.runHelpCommand(tool, append(append([]string(nil), prefix...), "help")...)
	case "gcloud", "az":
		return r.runHelpCommand(tool, append(append([]string(nil), prefix...), "--help")...)
	default:
		return "", nil
	}
}

func (r *ToolTreeRegistry) runHelpCommand(tool string, args ...string) (string, error) {
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

func (r *ToolTreeRegistry) loadCache() {
	if r.cacheDir == "" {
		return
	}

	cacheFile := filepath.Join(r.cacheDir, "subcommands.json")
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return
	}

	var header CacheHeader
	if err := json.Unmarshal(data, &header); err != nil {
		storage.QuarantineInvalidJSON(cacheFile, err)
		return
	}
	if header.SchemaVersion != subcommandCacheSchemaVersion {
		storage.QuarantineInvalidJSON(cacheFile, errors.New("unsupported subcommands cache schema version"))
		return
	}

	var wrapper struct {
		SchemaVersion int              `json:"schema_version"`
		Tools         []*ToolTreeCache `json:"tools"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		storage.QuarantineInvalidJSON(cacheFile, err)
		return
	}
	if wrapper.SchemaVersion != subcommandCacheSchemaVersion {
		storage.QuarantineInvalidJSON(cacheFile, errors.New("unsupported subcommands cache schema version"))
		return
	}

	r.ensureTrees()
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, cache := range wrapper.Tools {
		if cache == nil || cache.Tool == "" || cache.Tree == nil {
			continue
		}
		r.trees[cache.Tool] = cache
	}
}

func (r *ToolTreeRegistry) saveCache() {
	if r.cacheDir == "" {
		return
	}

	r.saveMu.Lock()
	defer r.saveMu.Unlock()

	r.ensureTrees()
	r.mu.RLock()
	tools := make([]*ToolTreeCache, 0, len(r.trees))
	toolNames := make([]string, 0, len(r.trees))
	for tool := range r.trees {
		toolNames = append(toolNames, tool)
	}
	sort.Strings(toolNames)
	for _, tool := range toolNames {
		cache := r.trees[tool]
		if cache == nil || cache.Tool == "" || cache.Tree == nil {
			continue
		}
		tools = append(tools, &ToolTreeCache{
			Tool:      cache.Tool,
			Tree:      cache.Tree.clone(),
			UpdatedAt: cache.UpdatedAt,
		})
	}
	r.mu.RUnlock()

	wrapper := struct {
		SchemaVersion int              `json:"schema_version"`
		Tools         []*ToolTreeCache `json:"tools"`
	}{
		SchemaVersion: subcommandCacheSchemaVersion,
		Tools:         tools,
	}

	data, err := json.MarshalIndent(wrapper, "", "  ")
	if err != nil {
		return
	}

	_ = os.MkdirAll(r.cacheDir, 0755)
	cacheFile := filepath.Join(r.cacheDir, "subcommands.json")
	_ = storage.WriteFileAtomic(cacheFile, data, 0600)
}

// Tool help output parsers.

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

func parseGitNestedHelp(output string) []string {
	subcommands := parseGitHelp(output)
	if len(subcommands) > 0 {
		return subcommands
	}

	seen := make(map[string]bool)
	repeatedUsage := regexp.MustCompile(`\bgit\s+[\w-]+\s+([a-z][a-z0-9-]+)\b`)
	groupedUsage := regexp.MustCompile(`\bgit\s+[\w-]+\s+\(([^)]+)\)`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		for _, matches := range groupedUsage.FindAllStringSubmatch(line, -1) {
			if len(matches) <= 1 {
				continue
			}
			for _, part := range strings.Split(matches[1], "|") {
				cmd := strings.TrimSpace(part)
				if cmd != "" && !seen[cmd] {
					seen[cmd] = true
					subcommands = append(subcommands, cmd)
				}
			}
		}
		for _, matches := range repeatedUsage.FindAllStringSubmatch(line, -1) {
			if len(matches) <= 1 || seen[matches[1]] {
				continue
			}
			seen[matches[1]] = true
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

// HasSubcommands checks whether the tool is known to have subcommands.
func (r *ToolTreeRegistry) HasSubcommands(tool string) bool {
	return knownSubcommandTools[tool]
}

var knownSubcommandTools = map[string]bool{
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

var builtinSubcommands = map[string][]string{
	"git":       {"add", "branch", "checkout", "clone", "commit", "diff", "fetch", "init", "log", "merge", "pull", "push", "rebase", "remote", "restore", "status", "stash", "submodule", "switch"},
	"docker":    {"build", "compose", "container", "exec", "image", "images", "inspect", "logs", "network", "ps", "pull", "push", "rm", "run", "start", "stop", "volume"},
	"npm":       {"ci", "install", "list", "login", "publish", "run", "test", "uninstall", "update"},
	"yarn":      {"add", "build", "cache", "create", "exec", "info", "init", "install", "remove", "run", "test", "upgrade"},
	"kubectl":   {"api-resources", "apply", "config", "create", "delete", "describe", "edit", "exec", "get", "logs", "patch", "rollout"},
	"cargo":     {"bench", "build", "check", "clean", "doc", "fmt", "help", "run", "test", "update"},
	"go":        {"build", "clean", "env", "fmt", "generate", "get", "install", "list", "mod", "run", "test", "tool"},
	"brew":      {"cleanup", "doctor", "info", "install", "list", "search", "tap", "uninstall", "update", "upgrade"},
	"terraform": {"apply", "destroy", "fmt", "import", "init", "output", "plan", "show", "state", "validate"},
	"helm":      {"dependency", "get", "install", "lint", "list", "package", "pull", "repo", "search", "template", "upgrade"},
	"aws":       {"cloudwatch", "configure", "dynamodb", "ec2", "iam", "lambda", "rds", "s3", "sns", "sqs", "sts"},
	"gcloud":    {"bigquery", "compute", "config", "functions", "iam", "kubernetes", "pubsub", "services", "storage"},
	"az":        {"account", "aks", "functionapp", "group", "network", "storage", "vm", "webapp"},
}

var (
	builtinSubcommandSet = buildBuiltinSubcommandSet()
)

// HasBuiltinSubcommand reports whether a tool's builtin root-level subcommands contain the given subcommand.
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

func appendPath(prefix []string, token string) []string {
	path := make([]string, 0, len(prefix)+1)
	path = append(path, prefix...)
	path = append(path, token)
	return path
}

func supportsHierarchicalDiscovery(tool string) bool {
	switch tool {
	case "git", "docker", "aws", "gcloud", "az":
		return true
	default:
		return false
	}
}

// PreFetch prefetches common tool subcommands.
func (r *ToolTreeRegistry) PreFetch() {
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
