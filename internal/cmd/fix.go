package cmd

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/yuluo-yx/typo/internal/config"
	itypes "github.com/yuluo-yx/typo/internal/types"
)

func cmdFix(args []string) int {
	fs := flag.NewFlagSet("fix", flag.ExitOnError)
	stderrFile := fs.String("s", "", "file containing stderr from previous command")
	exitCode := fs.Int("exit-code", -1, "exit code from previous command")
	noHistory := fs.Bool("no-history", false, "do not persist correction history")
	aliasContextFile := fs.String("alias-context", "", "file containing shell correction context")

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

	result := eng.FixWithContext(itypes.ParserContext{
		Command:      cmd,
		Stderr:       stderr,
		ExitCode:     *exitCode,
		AliasContext: loadAliasContext(*aliasContextFile),
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

func shouldRecordHistory(original string, result itypes.FixResult) bool {
	if !result.Fixed || result.Command == original {
		return false
	}

	return result.Kind != itypes.FixKindPermissionSudo && !result.UsedParser
}
