// Package cmd provides CLI command implementations for typo.
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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
  typo <command> [options]

Common:
  typo fix <command>                         Print the best correction
  typo fix --select <command>                Choose from configured candidates
  typo explain <command>                     Explain why Typo chose a correction
  typo doctor                                Check install, shell, and config status
  typo version                               Print version metadata

Fix options:
  typo fix -s <file> <command>               Use stderr from the previous command
  typo fix --exit-code <n> <command>         Use the previous command exit code
  typo fix --alias-context <file> <command>  Use shell aliases and env context
  typo fix --no-history <command>            Do not record this correction

Personalization:
  typo learn <from> <to>                     Teach a personal correction
  typo rules list                            List builtin and user rules
  typo rules add <from> <to>                 Add a user rule
  typo rules remove <from>                   Remove a user rule
  typo rules enable <scope>                  Enable a builtin rule scope
  typo rules disable <scope>                 Disable a builtin rule scope
  typo config list                           List current configuration
  typo config get <key>                      Show one configuration value
  typo config set <key> <value>              Persist a configuration override
  typo config reset                          Reset configuration to defaults
  typo config gen [--force]                  Write the default config file

History and stats:
  typo history list                          List correction history
  typo history clear                         Clear correction history
  typo stats [--since <days>] [--top <n>]    Analyze accepted corrections

Shell integration:
  typo init zsh                              Print zsh integration script
  typo init bash                             Print bash integration script
  typo init fish                             Print fish integration script
  typo init powershell                       Print PowerShell integration script

Maintenance:
  typo update                                Update script installs from main
  typo update --check                        Check update support
  typo update --version <tag>                Install a Release tag, e.g. 1.1.0
  typo update --dry-run                      Simulate update actions
  typo uninstall                             Remove local config and show cleanup steps

Diagnostics:
  typo fix --debug <command>                 Print a readable debug trace
  typo fix --debug=json <command>            Print a structured debug trace
  typo fix --trace-file <file> <command>     Write structured debug trace JSON

Examples:
  typo fix "gut stattus"
  typo explain "gut stattus"
  typo learn "gut" "git"
  typo config set keyboard dvorak
  typo config set candidates.enabled true
  typo config set candidates.limit 3
  typo config set experimental.long-option-correction.enabled true
  typo rules add "mytypo" "mycommand"
  typo rules disable git
  typo stats --since 7
  typo update
  typo update --check
  eval "$(typo init zsh)"

Config keys:
  candidates.enabled                         Enable interactive candidate selection
  candidates.limit                           Number of candidates to show
  experimental.long-option-correction.enabled
                                              Enable experimental --long-option fixes

Shell setup:
  zsh:        eval "$(typo init zsh)"
  bash:       eval "$(typo init bash)"
  fish:       typo init fish | source
  PowerShell: Invoke-Expression (& typo init powershell)

  Restart the shell, then press <Esc><Esc> to fix the current command.`)
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
// The loader receives a context so it can cancel work early when the timeout fires.
func discoverCommandsWithinTimeout(loader func(ctx context.Context) []string, timeout time.Duration) []string {
	if loader == nil {
		return nil
	}
	if timeout <= 0 {
		return loader(context.Background())
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resultCh := make(chan []string, 1)
	go func() {
		resultCh <- loader(ctx)
	}()

	select {
	case result := <-resultCh:
		return result
	case <-ctx.Done():
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
			return discoverCommandsWithinTimeout(commands.DiscoverContext, CommandDiscoveryTimeout)
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
