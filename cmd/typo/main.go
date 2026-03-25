package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
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
)

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
  typo fix <command>           Fix a command
  typo fix -s <file> <command> Fix command with stderr from file
  typo learn <from> <to>       Learn a correction
  typo rules list              List all rules
  typo rules add <from> <to>   Add a user rule
  typo rules remove <from>     Remove a user rule
  typo history list            List correction history
  typo history clear           Clear correction history
  typo init zsh                Print zsh integration script
  typo doctor                  Check configuration status
  typo uninstall               Uninstall typo completely
  typo version                 Print version

Examples:
  typo fix "gut stattus"
  typo learn "gut" "git"
  typo rules add "mytypo" "mycommand"
  eval "$(typo init zsh)"

Zsh Integration:
  After running 'eval "$(typo init zsh)"', press <Esc><Esc> to fix the current command.`)
}

func cmdFix(args []string) int {
	fs := flag.NewFlagSet("fix", flag.ExitOnError)
	stderrFile := fs.String("s", "", "file containing stderr from previous command")

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

	result := eng.Fix(cmd, stderr)

	if result.Fixed {
		if result.Command != cmd {
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

func cmdRules(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Error: subcommand required (list, add, remove)")
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
		// Need to access rules directly for removal
		r := engine.NewRules(cfg.ConfigDir)
		if err := r.RemoveUserRule(args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		fmt.Printf("Removed rule: %s\n", args[1])
		return 0
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n", args[0])
		return 1
	}
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
		fmt.Fprintln(os.Stderr, "Error: shell required (zsh)")
		return 1
	}

	switch args[0] {
	case "zsh":
		printZshIntegration()
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

	// Check if typo is in PATH
	fmt.Print("[1/4] typo command: ")
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
			fmt.Println("  Add the following to your ~/.zshrc:")
			fmt.Printf("    export PATH=\"$PATH:%s\"\n", goBinPath)
			fmt.Println()
		}
		hasError = true
	}

	// Check config directory
	fmt.Print("[2/4] config directory: ")
	cfg := config.Load()
	if info, err := os.Stat(cfg.ConfigDir); err == nil && info.IsDir() {
		fmt.Printf("✓ %s\n", cfg.ConfigDir)
	} else {
		fmt.Printf("⊘ %s (will be created on first use)\n", cfg.ConfigDir)
	}

	// Check shell integration
	fmt.Print("[3/4] shell integration: ")
	shellIntegration := os.Getenv("TYPO_SHELL_INTEGRATION")
	if shellIntegration == "1" {
		fmt.Println("✓ loaded")
	} else {
		fmt.Println("✗ not loaded")
		fmt.Println()
		fmt.Println("To enable shell integration, add to your ~/.zshrc:")
		fmt.Println("  eval \"$(typo init zsh)\"")
		fmt.Println()
		fmt.Println("Then restart your shell or run: source ~/.zshrc")
		hasError = true
	}

	// Check Go bin in PATH
	fmt.Print("[4/4] Go bin PATH: ")
	goBinDir := getGoBinDir()

	// Check if typo was installed via go install (in Go bin directory)
	typoInGoBin := false
	if typoPath != "" {
		typoInGoBin = sameDir(filepath.Dir(typoPath), goBinDir)
	}

	if !typoInGoBin {
		fmt.Println("⊘ skipped (installed from release)")
	} else if goBinDir == "" {
		fmt.Println("⊘ Go not installed or GOPATH not set")
	} else {
		if pathContainsDir(os.Getenv("PATH"), goBinDir) {
			fmt.Println("✓ configured")
		} else {
			fmt.Printf("✗ %s not in PATH\n", goBinDir)
			fmt.Println()
			fmt.Println("  If you installed typo with 'go install', add to your shell config:")
			fmt.Printf("    export PATH=\"$PATH:%s\"\n", goBinDir)
			hasError = true
		}
	}

	fmt.Println()
	if hasError {
		fmt.Println("Some checks failed. Please fix the issues above.")
		return 1
	}

	fmt.Println("All checks passed!")
	return 0
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
		homeDir, err := os.UserHomeDir()
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
	if _, err := os.Stat(typoPath); err == nil {
		return goBinDir
	}
	return ""
}

func cmdUninstall() int {
	fmt.Println("Uninstalling typo...")
	fmt.Println()

	cfg := config.Load()
	hasError := false

	// Remove ~/.typo directory
	fmt.Print("[1/3] Removing config directory: ")
	if cfg.ConfigDir != "" {
		if err := os.RemoveAll(cfg.ConfigDir); err != nil {
			fmt.Printf("✗ failed: %v\n", err)
			hasError = true
		} else {
			fmt.Printf("✓ removed %s\n", cfg.ConfigDir)
		}
	} else {
		fmt.Println("⊘ not found")
	}

	// Print zsh config cleanup instructions
	fmt.Print("[2/3] Zsh integration: ")
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("✗ cannot determine home directory")
		hasError = true
	} else {
		zshrc := homeDir + "/.zshrc"
		if _, statErr := os.Stat(zshrc); statErr == nil {
			fmt.Printf("please remove the following line from ~/.zshrc:\n")
			fmt.Println()
			fmt.Println("    eval \"$(typo init zsh)\"")
			fmt.Println()
		} else {
			fmt.Println("✓ no .zshrc found")
		}
	}

	// Print binary removal instructions
	fmt.Print("[3/3] Binary: ")
	execPath, err := os.Executable()
	if err != nil {
		fmt.Println("✗ cannot determine binary location")
		hasError = true
	} else {
		fmt.Printf("please remove the binary manually:\n")
		fmt.Println()
		fmt.Printf("    rm %s\n", execPath)
		fmt.Println()
	}

	fmt.Println("Uninstallation complete.")
	if hasError {
		return 1
	}
	return 0
}

func printZshIntegration() {
	fmt.Print(install.ZshScript)
	if !strings.HasSuffix(install.ZshScript, "\n") {
		fmt.Println()
	}
}

func createEngine(cfg *config.Config) *engine.Engine {
	// Discover commands from PATH and merge with builtins
	cmds := commands.Discover()
	cmds = append(cmds, commands.ShellBuiltins()...)

	// Create subcommand registry
	subcmdRegistry := commands.NewSubcommandRegistry(cfg.ConfigDir)

	return engine.NewEngine(
		engine.WithRules(engine.NewRules(cfg.ConfigDir)),
		engine.WithHistory(engine.NewHistory(cfg.ConfigDir)),
		engine.WithParser(parser.NewRegistry()),
		engine.WithCommands(cmds),
		engine.WithSubcommands(subcmdRegistry),
	)
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
