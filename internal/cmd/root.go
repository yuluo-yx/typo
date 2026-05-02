// Package cmd provides CLI command implementations for typo.
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/yuluo-yx/typo/install"
	"github.com/yuluo-yx/typo/internal/commands"
	"github.com/yuluo-yx/typo/internal/config"
	"github.com/yuluo-yx/typo/internal/engine"
	"github.com/yuluo-yx/typo/internal/parser"
)

// Version information injected at build time.
var (
	version = "dev"
	commit  = "none"
	date    = UnknownValue

	// Injectable system functions for testing.
	readBuildInfo = debug.ReadBuildInfo
	lookPath      = exec.LookPath
	userHomeDir   = os.UserHomeDir
	executable    = os.Executable
	removeAll     = os.RemoveAll
	statPath      = os.Stat
	evalSymlinks  = filepath.EvalSymlinks

	// Shell integration scripts.
	zshIntegrationScript        = install.ZshScript
	bashIntegrationScript       = install.BashScript
	fishIntegrationScript       = install.FishScript
	powerShellIntegrationScript = install.PowerShellScript
)

// Constants.
const (
	CommandDiscoveryTimeout = 150 * time.Millisecond
	UnknownValue            = "unknown"
)

// ruleScopeDisabledCommands maps rule scopes to commands that should be filtered.
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

// Run is the main entry point for the CLI.
func Run() int {
	if len(os.Args) < 2 {
		printUsage()
		return 1
	}

	switch os.Args[1] {
	case "fix":
		return cmdFix(os.Args[2:])
	case "explain":
		return cmdExplain(os.Args[2:])
	case "learn":
		return cmdLearn(os.Args[2:])
	case "config":
		return cmdConfig(os.Args[2:])
	case "rules":
		return cmdRules(os.Args[2:])
	case "history":
		return cmdHistory(os.Args[2:])
	case "stats":
		return cmdStats(os.Args[2:])
	case "init":
		return cmdInit(os.Args[2:])
	case "version":
		cmdVersion()
		return 0
	case "doctor":
		return cmdDoctor()
	case "uninstall":
		return cmdUninstall()
	case "update", "upgrade":
		return cmdUpdate(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		return 1
	}
}

// printUsage prints the usage information.
func printUsage() {
	fmt.Println(`typo - Command auto-correction tool

Usage:
  typo fix <command>                       Fix a command
  typo fix -s <file> <command>            Fix command with stderr from file
  typo fix --exit-code <n> <command>      Fix command with previous exit code
  typo fix --alias-context <file> <command>
                                           Fix command with shell correction context
  typo fix --debug <command>              Print fix debug trace to stderr
  typo fix --debug=json <command>         Print structured debug trace to stderr
  typo fix --trace-file <file> <command>  Write structured debug trace to a file
  typo explain <command>                  Explain why Typo chose a correction
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
  typo stats [--since <days>] [--top <n>] Analyze accepted correction history
  typo update                             Update typo to the latest version
  typo update --check                     Only check for updates
  typo update --version <tag>             Install a specific version
  typo update --dry-run                   Simulate without making changes
  typo init zsh                           Print zsh integration script
  typo init bash                          Print bash integration script
  typo init fish                          Print fish integration script
  typo init powershell                    Print PowerShell integration script
  typo doctor                             Check configuration status
  typo uninstall                          Remove local config and show remaining cleanup steps
  typo version                            Print version

Examples:
  typo fix "gut stattus"
  typo explain "gut stattus"
  typo learn "gut" "git"
  typo config set keyboard dvorak
  typo config set experimental.long-option-correction.enabled true
  typo rules add "mytypo" "mycommand"
  typo rules disable git
  typo stats --since 7
  typo update
  typo update --check
  eval "$(typo init zsh)"

Experimental:
  experimental.long-option-correction.enabled
                                           Enable experimental --long-option typo correction

Zsh Integration:
  After running 'eval "$(typo init zsh)"', press <Esc><Esc> to fix the current command.

Bash Integration:
  After running 'eval "$(typo init bash)"', press <Esc><Esc> to fix the current command.

Fish Integration:
  Add 'typo init fish | source' to ~/.config/fish/config.fish, then press <Esc><Esc> to fix the current command.

PowerShell Integration:
  Add 'Invoke-Expression (& typo init powershell)' to $PROFILE.CurrentUserCurrentHost, then press <Esc><Esc> to fix the current command.`)
}

// disabledCommandsFromConfig extracts disabled commands from config rule scopes.
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

		cmds, ok := DisabledCommandsForRuleScope(scope)
		if !ok {
			unknownScopes = append(unknownScopes, scope)
			continue
		}
		disabled = append(disabled, cmds...)
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

// DisabledCommandsForRuleScope returns commands for a given rule scope.
func DisabledCommandsForRuleScope(scope string) ([]string, bool) {
	cmds, ok := ruleScopeDisabledCommands[scope]
	return cmds, ok
}

// discoverCommandsWithinTimeout discovers commands with a timeout.
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

// createEngine creates a new engine with the given config.
func createEngine(cfg *config.Config) *engine.Engine {
	seedCommands := append([]string{"typo"}, commands.DiscoverCommon()...)
	seedCommands = append(seedCommands, commands.ShellBuiltins()...)
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

	toolTreeRegistry := commands.NewToolTreeRegistry(cfg.ConfigDir)
	commandTreeRegistry := commands.NewCommandTreeRegistry()

	return engine.NewEngine(
		engine.WithKeyboard(keyboard),
		engine.WithSimilarityThreshold(cfg.User.SimilarityThreshold),
		engine.WithMaxEditDistance(cfg.User.MaxEditDistance),
		engine.WithMaxFixPasses(cfg.User.MaxFixPasses),
		engine.WithAutoLearnThreshold(cfg.User.AutoLearnThreshold),
		engine.WithExperimentalLongOptionFix(cfg.User.Experimental.LongOptionCorrection.Enabled),
		engine.WithDisabledCommands(disabledCommands),
		engine.WithRules(rules),
		engine.WithHistory(engine.NewHistory(cfg.ConfigDir)),
		engine.WithParser(parser.NewRegistry()),
		engine.WithCommands(seedCommands),
		engine.WithCommandLoader(func() []string {
			return discoverCommandsWithinTimeout(commands.Discover, CommandDiscoveryTimeout)
		}),
		engine.WithToolTrees(toolTreeRegistry),
		engine.WithCommandTrees(commandTreeRegistry),
	)
}

// SortedConfigRuleScopes returns sorted rule scopes from config.
func SortedConfigRuleScopes(cfg *config.Config) []string {
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
