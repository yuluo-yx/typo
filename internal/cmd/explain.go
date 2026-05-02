package cmd

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yuluo-yx/typo/internal/config"
	itypes "github.com/yuluo-yx/typo/internal/types"
)

func cmdExplain(args []string) int {
	startedAt := time.Now()
	fs := flag.NewFlagSet("explain", flag.ExitOnError)
	stderrFile := fs.String("s", "", "file containing stderr from previous command")
	exitCode := fs.Int("exit-code", -1, "exit code from previous command")
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

	eng := createEngine(config.Load())
	eng.EnableDebug()
	result := eng.FixWithContext(itypes.ParserContext{
		Command:      cmd,
		Stderr:       stderr,
		ExitCode:     *exitCode,
		AliasContext: loadAliasContext(*aliasContextFile),
	})
	attachAutoLearnDebug(&result, itypes.AutoLearnDebugInfo{Reason: "explain does not record history"})
	attachTotalTimingDebug(&result, time.Since(startedAt))
	printFixExplanation(os.Stdout, result)
	if result.Fixed {
		return 0
	}
	return 1
}
