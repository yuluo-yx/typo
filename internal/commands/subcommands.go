package commands

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
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
	cache       map[string]*SubcommandCache
	cacheDir    string
	cacheExpiry time.Duration
}

// NewSubcommandRegistry creates a new subcommand registry.
func NewSubcommandRegistry(cacheDir string) *SubcommandRegistry {
	r := &SubcommandRegistry{
		cache:       make(map[string]*SubcommandCache),
		cacheDir:    cacheDir,
		cacheExpiry: 7 * 24 * time.Hour, // 7 days
	}
	r.loadCache()
	return r
}

// Get returns subcommands for a tool, fetching from cache or dynamically.
func (r *SubcommandRegistry) Get(tool string) []string {
	r.mu.RLock()
	if cached, ok := r.cache[tool]; ok {
		if time.Since(cached.UpdatedAt) < r.cacheExpiry {
			r.mu.RUnlock()
			return cached.Subcommands
		}
	}
	r.mu.RUnlock()

	// Need to fetch
	subcommands := r.fetchSubcommands(tool)
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

	return subcommands
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
	default:
		return parseGenericHelp(helpOutput)
	}
}

func (r *SubcommandRegistry) getHelpOutput(tool string) (string, error) {
	// Special handling for git - use 'help -a' for all commands
	if tool == "git" {
		cmd := exec.Command("git", "help", "-a")
		output, err := cmd.CombinedOutput()
		if err == nil {
			return string(output), nil
		}
	}

	// Try --help first
	cmd := exec.Command(tool, "--help")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try help subcommand
		cmd = exec.Command(tool, "help")
		output, err = cmd.CombinedOutput()
		if err != nil {
			return "", err
		}
	}
	return string(output), nil
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
		return
	}

	for i := range caches {
		r.cache[caches[i].Tool] = &caches[i]
	}
}

func (r *SubcommandRegistry) saveCache() {
	if r.cacheDir == "" {
		return
	}

	caches := make([]SubcommandCache, 0, len(r.cache))
	for _, c := range r.cache {
		caches = append(caches, *c)
	}

	data, err := json.MarshalIndent(caches, "", "  ")
	if err != nil {
		return
	}

	_ = os.MkdirAll(r.cacheDir, 0755)
	cacheFile := filepath.Join(r.cacheDir, "subcommands.json")
	_ = os.WriteFile(cacheFile, data, 0644)
}

// Parser functions for different tools

func parseGitHelp(output string) []string {
	// Git help format:
	//   add                  Add file contents to the index
	//   commit               Record changes to the repository
	subcommands := []string{}
	re := regexp.MustCompile(`^\s{2,}(\w+)\s{2,}`)

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
	re := regexp.MustCompile(`^\s{2,}(\w+)\s+`)

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
	// install, i, add      Install a package
	// run, run-script      Run arbitrary package scripts
	subcommands := []string{}
	re := regexp.MustCompile(`^\s{2,}(\w+)`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if matches := re.FindStringSubmatch(line); len(matches) > 1 {
			// Only take the first command, not aliases
			cmd := matches[1]
			if !strings.Contains(line, ",") || strings.Index(line, cmd) < strings.Index(line, ",") {
				subcommands = append(subcommands, cmd)
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
	re := regexp.MustCompile(`^\s{2,}(\w+)\s+`)

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
	re := regexp.MustCompile(`^\s{2,}(\w+)\s+`)

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
	re := regexp.MustCompile(`^\s{2,}(\w+)`)

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
	re := regexp.MustCompile(`^\s{2,}(\w+)\s+`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if matches := re.FindStringSubmatch(line); len(matches) > 1 {
			subcommands = append(subcommands, matches[1])
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
