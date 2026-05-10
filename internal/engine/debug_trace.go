package engine

import (
	"time"

	itypes "github.com/yuluo-yx/typo/internal/types"
)

// EnableDebug enables per-fix debug tracing.
func (e *Engine) EnableDebug() {
	if e != nil {
		e.debugEnabled = true
	}
}

// DisableDebug disables per-fix debug tracing.

// DisableDebug disables per-fix debug tracing.
func (e *Engine) DisableDebug() {
	if e != nil {
		e.debugEnabled = false
		e.currentDebug = nil
	}
}

// Fix attempts to fix the given command.
// stderr is optional and used for error parsing.

func (e *Engine) beginDebugTrace(input itypes.ParserContext) *itypes.FixDebugInfo {
	if e == nil || !e.debugEnabled {
		return nil
	}

	debugInfo := &itypes.FixDebugInfo{
		InputCommand:         input.Command,
		AliasContextProvided: len(input.AliasContext) > 0,
		AliasContextEntries:  len(input.AliasContext),
	}
	e.currentDebug = debugInfo
	return debugInfo
}

func (e *Engine) clearDebugTrace() {
	if e != nil {
		e.currentDebug = nil
	}
}

func (e *Engine) attachDebug(result itypes.FixResult, debugInfo *itypes.FixDebugInfo, startedAt time.Time) itypes.FixResult {
	if debugInfo != nil {
		debugInfo.EngineDuration = time.Since(startedAt)
		result.Debug = debugInfo
	}
	return result
}

func (e *Engine) debugTrace() *itypes.FixDebugInfo {
	if e == nil {
		return nil
	}
	return e.currentDebug
}

func (e *Engine) markDebugFeature(stage string) {
	debug := e.debugTrace()
	if debug == nil {
		return
	}

	switch stage {
	case "alias":
		debug.UsedAlias = true
		debug.AliasContextUsed = true
	case fixSourceParser:
		debug.UsedParser = true
	case fixSourceHistory:
		debug.UsedHistory = true
	case "rule":
		debug.UsedRule = true
	case "tree":
		debug.UsedCommandTree = true
	case "subcommand":
		debug.UsedSubcommand = true
	case fixSourceDistance:
		debug.UsedDistance = true
	case "env":
		debug.UsedEnv = true
	case "option":
		debug.UsedOption = true
	}
}

func (e *Engine) recordAcceptedFix(pass int, before string, result itypes.FixResult) {
	debug := e.debugTrace()
	if debug == nil || !result.Fixed {
		return
	}

	e.markDebugFeature(result.Source)
	if result.UsedParser {
		debug.UsedParser = true
	}

	debug.Events = append(debug.Events, itypes.FixDebugEvent{
		Pass:    pass,
		Stage:   result.Source,
		Before:  before,
		After:   result.Command,
		Message: result.Message,
	})
}

func (e *Engine) recordRejectedCandidate(stage, input, candidate string, distance int, similarity float64, reason string) {
	debug := e.debugTrace()
	if debug == nil || input == "" || candidate == "" || reason == "" {
		return
	}

	for _, existing := range debug.RejectedCandidates {
		if existing.Stage == stage && existing.Input == input && existing.Candidate == candidate {
			return
		}
	}
	if len(debug.RejectedCandidates) >= 5 {
		return
	}

	debug.RejectedCandidates = append(debug.RejectedCandidates, itypes.FixDebugCandidate{
		Stage:      stage,
		Input:      input,
		Candidate:  candidate,
		Distance:   distance,
		Similarity: similarity,
		Reason:     reason,
	})
}
