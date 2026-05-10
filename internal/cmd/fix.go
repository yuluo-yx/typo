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

type fixOptions struct {
	command          string
	stderrFile       string
	exitCode         int
	noHistory        bool
	aliasContextFile string
	debugMode        fixDebugMode
	traceFile        string
	selectMode       bool
}

func cmdFix(args []string) int {
	startedAt := time.Now()
	opts, ok := parseFixOptions(args)
	if !ok {
		return 1
	}

	stderr := readFixStderr(opts.stderrFile)
	cfg := config.Load()
	eng := createEngine(cfg)
	if opts.debugMode.enabled() || opts.traceFile != "" {
		eng.EnableDebug()
	}

	input := itypes.ParserContext{
		Command:      opts.command,
		Stderr:       stderr,
		ExitCode:     opts.exitCode,
		AliasContext: loadAliasContext(opts.aliasContextFile),
	}

	result := eng.FixWithContext(input)
	if opts.selectMode && cfg.User.Candidates.Enabled {
		selected, ok := selectFixResult(eng, input, cfg.User.Candidates.Limit)
		if !ok {
			return 1
		}
		result = selected
	}

	if result.Fixed {
		return finishFixedCommand(startedAt, cfg, eng, opts, result)
	}
	return finishUnfixedCommand(startedAt, cfg, opts, result)
}

func parseFixOptions(args []string) (fixOptions, bool) {
	fs := flag.NewFlagSet("fix", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	stderrFile := fs.String("s", "", "file containing stderr from previous command")
	exitCode := fs.Int("exit-code", -1, "exit code from previous command")
	noHistory := fs.Bool("no-history", false, "do not persist correction history")
	aliasContextFile := fs.String("alias-context", "", "file containing shell correction context")
	debugMode := fixDebugModeOff
	fs.Var(&debugMode, "debug", "print debug trace to stderr; use --debug=json for structured output")
	traceFile := fs.String("trace-file", "", "write structured debug trace to a JSON file")
	selectMode := fs.Bool("select", false, "select from configured correction candidates")

	if err := fs.Parse(args); err != nil {
		return fixOptions{}, false
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Error: command required")
		return fixOptions{}, false
	}
	if *selectMode && (debugMode.enabled() || *traceFile != "") {
		fmt.Fprintln(os.Stderr, "Error: --select cannot be combined with --debug or --trace-file")
		return fixOptions{}, false
	}

	return fixOptions{
		command:          strings.Join(fs.Args(), " "),
		stderrFile:       *stderrFile,
		exitCode:         *exitCode,
		noHistory:        *noHistory,
		aliasContextFile: *aliasContextFile,
		debugMode:        debugMode,
		traceFile:        *traceFile,
		selectMode:       *selectMode,
	}, true
}

func readFixStderr(stderrFile string) string {
	if stderrFile == "" {
		return ""
	}
	data, err := os.ReadFile(stderrFile)
	if err != nil {
		return ""
	}
	return string(data)
}

func finishFixedCommand(startedAt time.Time, cfg *config.Config, eng *engine.Engine, opts fixOptions, result itypes.FixResult) int {
	autoLearnDebug := skippedAutoLearnDebugInfo(cfg.User.History.Enabled, opts.noHistory, opts.command, result)
	if cfg.User.History.Enabled && !opts.noHistory && shouldRecordHistory(opts.command, result) {
		if err := eng.RecordHistory(opts.command, result.Command); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		autoLearnDebug = runAutoLearnWithinTimeout(eng, opts.command, result.Command)
	}
	attachAutoLearnDebug(&result, autoLearnDebug)
	attachTotalTimingDebug(&result, time.Since(startedAt))
	fmt.Println(result.Command)
	if result.Message != "" {
		fmt.Fprintf(os.Stderr, "typo: %s\n", result.Message)
	}
	if err := emitFixDebug(os.Stderr, result, opts.debugMode, opts.traceFile); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

func finishUnfixedCommand(startedAt time.Time, cfg *config.Config, opts fixOptions, result itypes.FixResult) int {
	fmt.Fprintln(os.Stderr, "typo: no correction found")
	attachAutoLearnDebug(&result, skippedAutoLearnDebugInfo(cfg.User.History.Enabled, opts.noHistory, opts.command, result))
	attachTotalTimingDebug(&result, time.Since(startedAt))
	if err := emitFixDebug(os.Stderr, result, opts.debugMode, opts.traceFile); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
	return 1
}

func selectFixResult(eng *engine.Engine, input itypes.ParserContext, limit int) (itypes.FixResult, bool) {
	candidates := eng.FixCandidatesWithContext(input, limit)
	switch len(candidates) {
	case 0:
		return itypes.FixResult{Fixed: false}, true
	case 1:
		return fixResultFromCandidate(candidates[0]), true
	default:
		selected, ok, err := chooseFixCandidateFromTerminalFunc(candidates)
		if err != nil {
			return fixResultFromCandidate(candidates[0]), true
		}
		if !ok {
			return itypes.FixResult{Fixed: false}, false
		}
		return fixResultFromCandidate(selected), true
	}
}

func fixResultFromCandidate(candidate itypes.FixCandidate) itypes.FixResult {
	return itypes.FixResult{
		Fixed:   true,
		Command: candidate.Command,
		Source:  candidate.Source,
		Message: candidate.Message,
	}
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
