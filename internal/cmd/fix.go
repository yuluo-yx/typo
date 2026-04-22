package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yuluo-yx/typo/internal/config"
	"github.com/yuluo-yx/typo/internal/engine"
	itypes "github.com/yuluo-yx/typo/internal/types"
)

var (
	autoLearnFixTimeout  = 150 * time.Millisecond
	autoLearnFromHistory = func(ctx context.Context, eng *engine.Engine, from, to string) itypes.AutoLearnDebugInfo {
		return eng.MaybeAutoLearnFromHistory(ctx, from, to)
	}
)

func cmdFix(args []string) int {
	startedAt := time.Now()
	fs := flag.NewFlagSet("fix", flag.ExitOnError)
	stderrFile := fs.String("s", "", "file containing stderr from previous command")
	exitCode := fs.Int("exit-code", -1, "exit code from previous command")
	noHistory := fs.Bool("no-history", false, "do not persist correction history")
	aliasContextFile := fs.String("alias-context", "", "file containing shell correction context")
	debugEnabled := fs.Bool("debug", false, "print debug trace to stderr")

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
	if *debugEnabled {
		eng.EnableDebug()
	}

	result := eng.FixWithContext(itypes.ParserContext{
		Command:      cmd,
		Stderr:       stderr,
		ExitCode:     *exitCode,
		AliasContext: loadAliasContext(*aliasContextFile),
	})

	if result.Fixed {
		autoLearnDebug := skippedAutoLearnDebugInfo(cfg.User.History.Enabled, *noHistory, cmd, result)
		if cfg.User.History.Enabled && !*noHistory && shouldRecordHistory(cmd, result) {
			if err := eng.RecordHistory(cmd, result.Command); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return 1
			}
			autoLearnDebug = runAutoLearnWithinTimeout(eng, cmd, result.Command)
		}
		attachAutoLearnDebug(&result, autoLearnDebug)
		attachTotalTimingDebug(&result, time.Since(startedAt))
		fmt.Println(result.Command)
		if result.Message != "" {
			fmt.Fprintf(os.Stderr, "typo: %s\n", result.Message)
		}
		if *debugEnabled {
			printFixDebug(os.Stderr, result)
		}
		return 0
	}

	fmt.Fprintln(os.Stderr, "typo: no correction found")
	attachAutoLearnDebug(&result, skippedAutoLearnDebugInfo(cfg.User.History.Enabled, *noHistory, cmd, result))
	attachTotalTimingDebug(&result, time.Since(startedAt))
	if *debugEnabled {
		printFixDebug(os.Stderr, result)
	}
	return 1
}

func shouldRecordHistory(original string, result itypes.FixResult) bool {
	if !result.Fixed || result.Command == original {
		return false
	}

	return result.Kind != itypes.FixKindPermissionSudo && !result.UsedParser
}

func runAutoLearnWithinTimeout(eng *engine.Engine, from, to string) itypes.AutoLearnDebugInfo {
	startedAt := time.Now()
	if eng == nil || autoLearnFixTimeout <= 0 {
		if eng == nil {
			return itypes.AutoLearnDebugInfo{Reason: "engine unavailable", Duration: time.Since(startedAt)}
		}
		return itypes.AutoLearnDebugInfo{Reason: "auto-learn timeout disabled", Duration: time.Since(startedAt)}
	}

	ctx, cancel := context.WithTimeout(context.Background(), autoLearnFixTimeout)
	defer cancel()

	done := make(chan itypes.AutoLearnDebugInfo, 1)
	go func() {
		info := autoLearnFromHistory(ctx, eng, from, to)
		info.Attempted = true
		done <- info
	}()

	select {
	case info := <-done:
		info.Duration = time.Since(startedAt)
		return info
	case <-ctx.Done():
		return itypes.AutoLearnDebugInfo{
			Attempted: true,
			TimedOut:  true,
			Duration:  time.Since(startedAt),
			Error:     ctx.Err().Error(),
			Reason:    ctx.Err().Error(),
		}
	}
}

func attachAutoLearnDebug(result *itypes.FixResult, info itypes.AutoLearnDebugInfo) {
	if result == nil || result.Debug == nil {
		return
	}
	result.Debug.AutoLearn = info
}

func attachTotalTimingDebug(result *itypes.FixResult, elapsed time.Duration) {
	if result == nil || result.Debug == nil {
		return
	}
	result.Debug.TotalDuration = elapsed
}

func skippedAutoLearnDebugInfo(historyEnabled bool, noHistory bool, original string, result itypes.FixResult) itypes.AutoLearnDebugInfo {
	switch {
	case !result.Fixed:
		return itypes.AutoLearnDebugInfo{Reason: "no accepted fix"}
	case !historyEnabled:
		return itypes.AutoLearnDebugInfo{Reason: "history disabled by config"}
	case noHistory:
		return itypes.AutoLearnDebugInfo{Reason: "history disabled by --no-history"}
	case result.Command == original:
		return itypes.AutoLearnDebugInfo{Reason: "unchanged command is not recorded"}
	case result.Kind == itypes.FixKindPermissionSudo:
		return itypes.AutoLearnDebugInfo{Reason: "permission sudo fixes are not auto-learned"}
	case result.UsedParser:
		return itypes.AutoLearnDebugInfo{Reason: "parser-assisted fixes are not auto-learned"}
	default:
		return itypes.AutoLearnDebugInfo{Reason: "history recording allowed"}
	}
}
