package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yuluo-yx/typo/install"
	"github.com/yuluo-yx/typo/internal/commands"
	"github.com/yuluo-yx/typo/internal/config"
	"github.com/yuluo-yx/typo/internal/engine"
	"github.com/yuluo-yx/typo/internal/parser"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"

	readBuildInfo = debug.ReadBuildInfo
	lookPath      = exec.LookPath
	userHomeDir   = os.UserHomeDir
	executable    = os.Executable
	removeAll     = os.RemoveAll
	statPath      = os.Stat
)

const commandDiscoveryTimeout = 150 * time.Millisecond

var ruleScopeDisabledCommands = map[string][]string{
	"git":       {"git"},
	"docker":    {"docker"},
	"npm":       {"npm"},
	"yarn":      {"yarn"},
	"kubectl":   {"kubectl"},
	"cargo":     {"cargo"},
	"brew":      {"brew"},
	"helm":      {"helm"},
	"terraform": {"terraform"},
	"python":    {"python", "python3"},
	"pip":       {"pip"},
	"go":        {"go"},
	"java":      {"java"},
	// The system scope groups many shell builtins and common utilities. Disabling the
	// rules must not remove those commands from the known-command pool.
	"system": nil,
}

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) < 2 {
		printUsage()
		return 1
	}

	switch os.Args[1] {
	case "fix":
		return cmdFix(os.Args[2:])
	case "learn":
		return cmdLearn(os.Args[2:])
	case "config":
		return cmdConfig(os.Args[2:])
	case "rules":
		return cmdRules(os.Args[2:])
	case "history":
		return cmdHistory(os.Args[2:])
	case "init":
		return cmdInit(os.Args[2:])
	case "version":
		cmdVersion()
		return 0
	case "doctor":
		return cmdDoctor()
	case "uninstall":
		return cmdUninstall()
	case "help", "-h", "--help":
		printUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		return 1
	}
}

func printUsage() {
	fmt.Println(`typo - Command auto-correction tool

Usage:
  typo fix <command>                       Fix a command
  typo fix -s <file> <command>            Fix command with stderr from file
  typo fix --exit-code <n> <command>      Fix command with previous exit code
  typo learn <from> <to>                  Learn a correction
  typo config list                        List current configuration values
  typo config get <key>                   Show a single configuration value
  typo config set <key> <value>           Persist a configuration override
  typo config reset                       Reset configuration to defaults
  typo config gen [--force]               Generate the default config file
  typo rules list                         List all rules
  typo rules add <from> <to>              Add a user rule
  typo rules remove <from>                Remove a user rule
  typo rules enable <scope>               Enable a builtin rule scope
  typo rules disable <scope>              Disable a builtin rule scope
  typo history list                       List correction history
  typo history clear                      Clear correction history
  typo init zsh                           Print zsh integration script
	typo init bash                          Print bash integration script
  typo doctor                             Check configuration status
  typo uninstall                          Remove local config and show remaining cleanup steps
  typo version                            Print version

Examples:
  typo fix "gut stattus"
  typo learn "gut" "git"
  typo config set keyboard dvorak
  typo rules add "mytypo" "mycommand"
  typo rules disable git
  eval "$(typo init zsh)"

Zsh Integration:
  After running 'eval "$(typo init zsh)"', press <Esc><Esc> to fix the current command.

Bash Integration:
  After running 'eval "$(typo init bash)"', press <Esc><Esc> to fix the current command.`)
}

func cmdFix(args []string) int {
	fs := flag.NewFlagSet("fix", flag.ExitOnError)
	stderrFile := fs.String("s", "", "file containing stderr from previous command")
	exitCode := fs.Int("exit-code", -1, "exit code from previous command")
	noHistory := fs.Bool("no-history", false, "do not persist correction history")

	_ = fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Error: command required")
		return 1
	}

	cmd := strings.Join(fs.Args(), " ")
	stderr := ""

	if *stderrFile != "" {
		data, err := os.ReadFile(*stderrFile)
		if err == nil {
			stderr = string(data)
		}
	}

	cfg := config.Load()
	eng := createEngine(cfg)

	result := eng.FixWithContext(parser.Context{
		Command:  cmd,
		Stderr:   stderr,
		ExitCode: *exitCode,
	})

	if result.Fixed {
		if cfg.User.History.Enabled && !*noHistory && shouldRecordHistory(cmd, result) {
			if err := eng.RecordHistory(cmd, result.Command); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return 1
			}
		}
		fmt.Println(result.Command)
		if result.Message != "" {
			fmt.Fprintf(os.Stderr, "typo: %s\n", result.Message)
		}
		return 0
	}

	fmt.Fprintln(os.Stderr, "typo: no correction found")
	return 1
}

func shouldRecordHistory(original string, result engine.FixResult) bool {
	if !result.Fixed || result.Command == original {
		return false
	}

	return result.Kind != parser.ResultKindPermissionSudo && !result.UsedParser
}

func cmdLearn(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Error: <from> and <to> required")
		return 1
	}

	cfg := config.Load()
	eng := createEngine(cfg)

	if err := eng.Learn(args[0], args[1]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	fmt.Printf("Learned: %s -> %s\n", args[0], args[1])
	return 0
}

func cmdConfig(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Error: subcommand required (list, get, set, reset, gen)")
		return 1
	}

	cfg := config.Load()
	return runConfigSubcommand(cfg, args)
}

func runConfigSubcommand(cfg *config.Config, args []string) int {
	switch args[0] {
	case "list":
		return cmdConfigList(cfg)
	case "get":
		return cmdConfigGet(cfg, args)
	case "set":
		return cmdConfigSet(cfg, args)
	case "reset":
		return cmdConfigReset(cfg)
	case "gen":
		return cmdConfigGen(cfg, args)
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n", args[0])
		return 1
	}
}

func cmdConfigList(cfg *config.Config) int {
	for _, setting := range cfg.ListSettings() {
		fmt.Printf("%s=%s\n", setting.Key, setting.Value)
	}
	return 0
}

func cmdConfigGet(cfg *config.Config, args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Error: <key> required")
		return 1
	}

	value, err := cfg.Get(args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	fmt.Println(value)
	return 0
}

func cmdConfigSet(cfg *config.Config, args []string) int {
	if len(args) < 3 {
		fmt.Fprintln(os.Stderr, "Error: <key> and <value> required")
		return 1
	}

	if err := cfg.SetValue(args[1], args[2]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	if err := cfg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	fmt.Printf("Set %s=%s\n", args[1], args[2])
	return 0
}

func cmdConfigReset(cfg *config.Config) int {
	if err := cfg.Reset(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	fmt.Printf("Reset configuration: %s\n", cfg.ConfigFilePath())
	return 0
}

func cmdConfigGen(cfg *config.Config, args []string) int {
	fs := flag.NewFlagSet("config gen", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	force := fs.Bool("force", false, "overwrite existing config file")
	if err := fs.Parse(args[1:]); err != nil {
		return 1
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "Error: config gen does not accept positional arguments")
		return 1
	}
	if err := cfg.Generate(*force); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	fmt.Printf("Generated configuration: %s\n", cfg.ConfigFilePath())
	return 0
}

func cmdRules(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Error: subcommand required (list, add, remove, enable, disable)")
		return 1
	}

	cfg := config.Load()
	eng := createEngine(cfg)

	switch args[0] {
	case "list":
		rules := eng.ListRules()
		for _, r := range rules {
			status := "enabled"
			if !r.Enable {
				status = "disabled"
			}
			fmt.Printf("%s -> %s [%s] (%s)\n", r.From, r.To, r.Scope, status)
		}
		return 0
	case "add":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "Error: <from> and <to> required")
			return 1
		}
		if err := eng.AddRule(args[1], args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		fmt.Printf("Added rule: %s -> %s\n", args[1], args[2])
		return 0
	case "remove":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Error: <from> required")
			return 1
		}
		r := engine.NewRules(cfg.ConfigDir)
		if err := r.RemoveUserRule(args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		h := engine.NewHistory(cfg.ConfigDir)
		if err := h.RemoveEntriesForCommandWord(args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: removed rule but failed to clear related history: %v\n", err)
			return 1
		}
		fmt.Printf("Removed rule: %s\n", args[1])
		return 0
	case "enable":
		return cmdRulesSetScopeEnabled(cfg, args, true)
	case "disable":
		return cmdRulesSetScopeEnabled(cfg, args, false)
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n", args[0])
		return 1
	}
}

func cmdRulesSetScopeEnabled(cfg *config.Config, args []string, enabled bool) int {
	action := args[0]
	if len(args) != 2 {
		fmt.Fprintf(os.Stderr, "Error: %s requires exactly one <scope>\n", action)
		return 1
	}

	scope := strings.TrimSpace(args[1])
	if scope == "" {
		fmt.Fprintf(os.Stderr, "Error: %s requires exactly one <scope>\n", action)
		return 1
	}

	if _, exists := cfg.User.Rules[scope]; !exists {
		fmt.Fprintf(
			os.Stderr,
			"Error: unknown rule scope: %s (valid options: %s)\n",
			scope,
			strings.Join(sortedConfigRuleScopes(cfg), ", "),
		)
		return 1
	}

	key := fmt.Sprintf("rules.%s.enabled", scope)
	if err := cfg.Set(key, strconv.FormatBool(enabled)); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if enabled {
		fmt.Printf("Enabled rule scope: %s\n", scope)
		return 0
	}

	fmt.Printf("Disabled rule scope: %s\n", scope)
	return 0
}

func sortedConfigRuleScopes(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}

	scopes := make([]string, 0, len(cfg.User.Rules))
	for scope := range cfg.User.Rules {
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)
	return scopes
}

func cmdHistory(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Error: subcommand required (list, clear)")
		return 1
	}

	cfg := config.Load()
	h := engine.NewHistory(cfg.ConfigDir)

	switch args[0] {
	case "list":
		entries := h.List()
		for _, e := range entries {
			fmt.Printf("%s -> %s (used %d times)\n", e.From, e.To, e.Count)
		}
		return 0
	case "clear":
		if err := h.Clear(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		fmt.Println("History cleared")
		return 0
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n", args[0])
		return 1
	}
}

func cmdInit(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Error: shell required (zsh, bash)")
		return 1
	}

	switch args[0] {
	case "zsh":
		printIntegrationScript("zsh")
		return 0
	case "bash":
		printIntegrationScript("bash")
		return 0
	default:
		fmt.Fprintf(os.Stderr, "Unsupported shell: %s\n", args[0])
		return 1
	}
}

func cmdVersion() {
	resolvedVersion, resolvedCommit, resolvedDate := resolveVersionInfo()
	fmt.Printf("typo %s (commit: %s, built: %s)\n", resolvedVersion, resolvedCommit, resolvedDate)
}

// Prefer release metadata injected by the build pipeline; fall back to VCS metadata embedded in the Go binary when needed.
func resolveVersionInfo() (string, string, string) {
	resolvedVersion := version
	resolvedCommit := commit
	resolvedDate := date

	info, ok := readBuildInfo()
	if !ok || info == nil {
		return resolvedVersion, resolvedCommit, resolvedDate
	}

	if (resolvedVersion == "" || resolvedVersion == "dev") && info.Main.Version != "" && info.Main.Version != "(devel)" {
		resolvedVersion = info.Main.Version
	}

	settings := make(map[string]string, len(info.Settings))
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}

	if resolvedCommit == "" || resolvedCommit == "none" {
		if revision := settings["vcs.revision"]; revision != "" {
			resolvedCommit = shortRevision(revision)
		}
	}

	if resolvedDate == "" || resolvedDate == "unknown" {
		if vcsTime := settings["vcs.time"]; vcsTime != "" {
			resolvedDate = formatBuildDate(vcsTime)
		}
	}

	return resolvedVersion, resolvedCommit, resolvedDate
}

func shortRevision(revision string) string {
	if len(revision) <= 7 {
		return revision
	}

	return revision[:7]
}

func formatBuildDate(raw string) string {
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return raw
	}

	return parsed.UTC().Format("2006-01-02")
}

func cmdDoctor() int {
	fmt.Println("Checking typo configuration...")
	fmt.Println()

	hasError := false
	cfg := config.Load()
	shellName, shellRC := detectShellIntegrationTarget()

	// Check if typo is in PATH
	fmt.Print("[1/5] typo command: ")
	typoPath, err := lookPath("typo")
	if err == nil {
		fmt.Printf("✓ available in PATH (%s)\n", typoPath)
	} else {
		fmt.Println("✗ not found in PATH")
		// Check if typo exists in Go bin
		goBinPath := checkGoBinTypo()
		if goBinPath != "" {
			fmt.Println()
			fmt.Println("  Found typo in Go bin directory but not in PATH.")
			fmt.Printf("  Add the following to your %s:\n", shellRC)
			fmt.Printf("    export PATH=\"$PATH:%s\"\n", goBinPath)
			fmt.Println()
		}
		hasError = true
	}

	// Check config directory
	fmt.Print("[2/5] config directory: ")
	if info, err := statPath(cfg.ConfigDir); err == nil && info.IsDir() {
		fmt.Printf("✓ %s\n", cfg.ConfigDir)
	} else {
		fmt.Printf("⊘ %s (will be created on first use)\n", cfg.ConfigDir)
	}

	// Check config file and print effective settings
	fmt.Print("[3/5] config file: ")
	if configFile := cfg.ConfigFilePath(); configFile != "" {
		if info, err := statPath(configFile); err == nil && !info.IsDir() {
			fmt.Printf("✓ %s\n", configFile)
		} else {
			fmt.Printf("⊘ %s (using defaults; run 'typo config gen' to create it)\n", configFile)
		}
	} else {
		fmt.Println("⊘ unavailable")
	}

	printDoctorEffectiveConfig(cfg, shellName)
	hasError = checkDoctorShellIntegration(shellName, shellRC) || hasError
	hasError = checkDoctorGoBinPath(typoPath) || hasError

	fmt.Println()
	if hasError {
		fmt.Println("Some checks failed. Please fix the issues above.")
		return 1
	}

	fmt.Println("All checks passed!")
	return 0
}

func printDoctorEffectiveConfig(cfg *config.Config, shellName string) {
	fmt.Println()
	fmt.Println("effective config:")
	fmt.Printf("  shell: %s\n", shellName)
	for _, setting := range cfg.ListSettings() {
		fmt.Printf("  %s=%s\n", setting.Key, setting.Value)
	}
	fmt.Println()
}

func checkDoctorShellIntegration(shellName, shellRC string) bool {
	fmt.Print("[4/5] shell integration: ")
	if os.Getenv("TYPO_SHELL_INTEGRATION") == "1" {
		fmt.Println("✓ loaded")
		return false
	}

	fmt.Println("✗ not loaded")
	fmt.Println()
	if shellName != "" {
		fmt.Printf("To enable shell integration, add to your %s:\n", shellRC)
		fmt.Printf("  eval \"$(typo init %s)\"\n", shellName)
		fmt.Println()
		fmt.Printf("Then restart your shell or run: source %s\n", shellRC)
		return true
	}

	fmt.Println("To enable shell integration, add one of the following:")
	fmt.Println("  # Zsh (~/.zshrc)")
	fmt.Println("  eval \"$(typo init zsh)\"")
	fmt.Println("  # Bash (~/.bashrc)")
	fmt.Println("  eval \"$(typo init bash)\"")
	fmt.Println()
	fmt.Println("Then restart your shell or source the matching rc file.")
	return true
}

func checkDoctorGoBinPath(typoPath string) bool {
	fmt.Print("[5/5] Go bin PATH: ")
	goBinDir := getGoBinDir()
	typoInGoBin := false
	if typoPath != "" {
		typoInGoBin = sameDir(filepath.Dir(typoPath), goBinDir)
	}

	if !typoInGoBin {
		fmt.Println("⊘ skipped (installed from release)")
		return false
	}
	if goBinDir == "" {
		fmt.Println("⊘ Go not installed or GOPATH not set")
		return false
	}
	if pathContainsDir(os.Getenv("PATH"), goBinDir) {
		fmt.Println("✓ configured")
		return false
	}

	fmt.Printf("✗ %s not in PATH\n", goBinDir)
	fmt.Println()
	fmt.Println("  If you installed typo with 'go install', add to your shell config:")
	fmt.Printf("    export PATH=\"$PATH:%s\"\n", goBinDir)
	return true
}

func currentShellName() string {
	shellPath := strings.TrimSpace(os.Getenv("SHELL"))
	if shellPath == "" {
		return "unknown"
	}

	shellBase := strings.ToLower(filepath.Base(shellPath))
	if shellBase == "" || shellBase == "." {
		return "unknown"
	}

	return shellBase
}

func detectShellIntegrationTarget() (string, string) {
	switch currentShellName() {
	case "bash":
		return "bash", "~/.bashrc"
	case "zsh":
		return "zsh", "~/.zshrc"
	default:
		return "", "~/.zshrc or ~/.bashrc"
	}
}

func getGoBinDir() string {
	// Try GOPATH first
	goPath := os.Getenv("GOPATH")
	goBin := os.Getenv("GOBIN")
	if goBin != "" {
		return filepath.Clean(goBin)
	}
	if goPath == "" {
		// Try default GOPATH
		homeDir, err := userHomeDir()
		if err != nil {
			return ""
		}
		goPath = homeDir + "/go"
	}
	return filepath.Join(goPath, "bin")
}

func checkGoBinTypo() string {
	goBinDir := getGoBinDir()
	if goBinDir == "" {
		return ""
	}
	typoPath := filepath.Join(goBinDir, "typo")
	if _, err := statPath(typoPath); err == nil {
		return goBinDir
	}
	return ""
}

func cmdUninstall() int {
	fmt.Println("Cleaning up typo...")
	fmt.Println()

	cfg := config.Load()
	hasError := false

	// Remove ~/.typo directory
	fmt.Print("[1/3] Removing config directory: ")
	if cfg.ConfigDir != "" {
		if err := removeAll(cfg.ConfigDir); err != nil {
			fmt.Printf("✗ failed: %v\n", err)
			hasError = true
		} else {
			fmt.Printf("✓ removed %s\n", cfg.ConfigDir)
		}
	} else {
		fmt.Println("⊘ not found")
	}

	// Show manual cleanup instructions for shell configuration leftovers.
	fmt.Print("[2/3] Shell integration: ")
	homeDir, err := userHomeDir()
	if err != nil {
		fmt.Println("✗ cannot determine home directory")
		hasError = true
	} else {
		foundShellConfig := false
		for _, target := range []struct {
			shell string
			rc    string
		}{
			{shell: "zsh", rc: ".zshrc"},
			{shell: "bash", rc: ".bashrc"},
		} {
			rcPath := filepath.Join(homeDir, target.rc)
			if _, statErr := statPath(rcPath); statErr == nil {
				foundShellConfig = true
				fmt.Printf("manual cleanup required in ~/%s:\n", target.rc)
				fmt.Println()
				fmt.Printf("    eval \"$(typo init %s)\"\n", target.shell)
				fmt.Println()
			}
		}
		if !foundShellConfig {
			fmt.Println("✓ no .zshrc or .bashrc found")
		}
	}

	// Show manual cleanup instructions for the installed binary.
	fmt.Print("[3/3] Binary: ")
	execPath, err := executable()
	if err != nil {
		fmt.Println("✗ cannot determine binary location")
		hasError = true
	} else {
		fmt.Printf("manual cleanup required for the binary:\n")
		fmt.Println()
		fmt.Printf("    rm %s\n", execPath)
		fmt.Println()
	}

	fmt.Println("Local cleanup complete. Manual steps above may still be required.")
	if hasError {
		return 1
	}
	return 0
}

func printIntegrationScript(shell string) {
	var script string
	switch shell {
	case "zsh":
		script = install.ZshScript
	case "bash":
		script = install.BashScript
	default:
		fmt.Fprintf(os.Stderr, "Unsupported shell: %s\n", shell)
		os.Exit(1)
	}

	fmt.Print(script)
	if !strings.HasSuffix(script, "\n") {
		fmt.Println()
	}
}

func createEngine(cfg *config.Config) *engine.Engine {
	seedCommands := append(commands.DiscoverCommon(), commands.ShellBuiltins()...)
	disabledCommands := disabledCommandsFromConfig(cfg)

	rules := engine.NewRules(cfg.ConfigDir)
	for scope, ruleCfg := range cfg.User.Rules {
		if !ruleCfg.Enabled {
			_ = rules.EnableRuleSet(scope, false)
		}
	}

	keyboard, err := engine.KeyboardByName(cfg.User.Keyboard)
	if err != nil {
		keyboard = engine.DefaultKeyboard
	}

	subcmdRegistry := commands.NewSubcommandRegistry(cfg.ConfigDir)

	return engine.NewEngine(
		engine.WithKeyboard(keyboard),
		engine.WithSimilarityThreshold(cfg.User.SimilarityThreshold),
		engine.WithMaxEditDistance(cfg.User.MaxEditDistance),
		engine.WithMaxFixPasses(cfg.User.MaxFixPasses),
		engine.WithDisabledCommands(disabledCommands),
		engine.WithRules(rules),
		engine.WithHistory(engine.NewHistory(cfg.ConfigDir)),
		engine.WithParser(parser.NewRegistry()),
		engine.WithCommands(seedCommands),
		engine.WithCommandLoader(func() []string {
			return discoverCommandsWithinTimeout(commands.Discover, commandDiscoveryTimeout)
		}),
		engine.WithSubcommands(subcmdRegistry),
	)
}

func disabledCommandsFromConfig(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}

	disabled := make([]string, 0)
	unknownScopes := make([]string, 0)
	for scope, ruleCfg := range cfg.User.Rules {
		if ruleCfg.Enabled {
			continue
		}

		commands, ok := disabledCommandsForRuleScope(scope)
		if !ok {
			unknownScopes = append(unknownScopes, scope)
			continue
		}
		disabled = append(disabled, commands...)
	}

	if len(unknownScopes) > 0 {
		sort.Strings(unknownScopes)
		fmt.Fprintf(
			os.Stderr,
			"typo: ignoring unknown disabled rule scopes for command filtering: %s\n",
			strings.Join(unknownScopes, ", "),
		)
	}

	return disabled
}

func disabledCommandsForRuleScope(scope string) ([]string, bool) {
	commands, ok := ruleScopeDisabledCommands[scope]
	return commands, ok
}

func discoverCommandsWithinTimeout(loader func() []string, timeout time.Duration) []string {
	if loader == nil {
		return nil
	}
	if timeout <= 0 {
		return loader()
	}

	resultCh := make(chan []string, 1)
	go func() {
		resultCh <- loader()
	}()

	select {
	case result := <-resultCh:
		return result
	case <-time.After(timeout):
		return nil
	}
}

func sameDir(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func pathContainsDir(pathValue, dir string) bool {
	for _, item := range filepath.SplitList(pathValue) {
		if sameDir(item, dir) {
			return true
		}
	}
	return false
}
