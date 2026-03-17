package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/yuluo-yx/typo/internal/commands"
	"github.com/yuluo-yx/typo/internal/config"
	"github.com/yuluo-yx/typo/internal/engine"
	"github.com/yuluo-yx/typo/internal/parser"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
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
  typo version                 Print version

Examples:
  typo fix "gut stattus"
  typo learn "gut" "git"
  typo rules add "mytypo" "mycommand"
  eval "$(typo init zsh)"

Zsh Integration:
  After running 'eval "$(typo init zsh)"', press 'ff' to fix the current command.`)
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
	fmt.Printf("typo %s (commit: %s, built: %s)\n", version, commit, date)
}

func printZshIntegration() {
	fmt.Println(`# typo - Command auto-correction for zsh
#
# Installation:
#   Add to ~/.zshrc:
#     source /path/to/typo/install/typo.zsh
#
#   Or use:
#     eval "$(typo init zsh)"
#
# Usage:
#   1. Type a wrong command, press 'ff' to fix before executing
#   2. After executing a failed command, press 'ff' to fix last command
#
# Example:
#   $ gut stattus<ff>  →  git status
#   $ gut stattus      →  command not found
#   $ <ff>             →  git status

_typo_fix_command() {
    local cmd="${BUFFER}"
    local stderr_file="${TYPO_STDERR_CACHE:-/tmp/typo-stderr-$$}"

    # If buffer is empty, get last command from history
    if [[ -z "$cmd" ]]; then
        cmd=$(fc -ln -1 | sed 's/^[[:space:]]*//')
    fi

    [[ -z "$cmd" ]] && return

    local fixed
    if [[ -f "$stderr_file" && -s "$stderr_file" ]]; then
        fixed=$(typo fix -s "$stderr_file" "$cmd" 2>/dev/null)
    else
        fixed=$(typo fix "$cmd" 2>/dev/null)
    fi

    if [[ -n "$fixed" && "$fixed" != "$cmd" ]]; then
        BUFFER="$fixed"
        CURSOR=${#BUFFER}
        zle reset-prompt
    fi
}

zle -N _typo_fix_command

# ff to fix command
bindkey 'ff' _typo_fix_command

# stderr capture hook
_typo_preexec() {
    TYPO_STDERR_CACHE="/tmp/typo-stderr-$$"
    exec 2> >(tee "$TYPO_STDERR_CACHE" >&2)
}

autoload -Uz add-zsh-hook
add-zsh-hook preexec _typo_preexec`)
}

func createEngine(cfg *config.Config) *engine.Engine {
	// Discover commands from PATH
	cmds := commands.Discover()
	if len(cmds) == 0 {
		cmds = commands.DiscoverCommon()
	}

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
