package cmd

import (
	"fmt"
	"os"

	"github.com/yuluo-yx/typo/internal/config"
	"github.com/yuluo-yx/typo/internal/engine"
)

func cmdHistory(args []string) int {
	if message := historyArgsError(args); message != "" {
		fmt.Fprintln(os.Stderr, message)
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
		if err := requireConfigDirectory(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
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

func historyArgsError(args []string) string {
	if len(args) == 0 {
		return "Error: subcommand required (list, clear)"
	}

	switch args[0] {
	case "list":
		if len(args) != 1 {
			return "Error: history list does not accept arguments"
		}
	case "clear":
		if len(args) != 1 {
			return "Error: history clear does not accept arguments"
		}
	default:
		return fmt.Sprintf("Unknown subcommand: %s", args[0])
	}

	return ""
}
