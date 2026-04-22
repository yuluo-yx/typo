package engine

import (
	"context"
	"errors"
	"strings"

	itypes "github.com/yuluo-yx/typo/internal/types"
)

type autoLearnResult struct {
	Triggered bool
	Persisted bool
	TimedOut  bool
	Err       error
	Reason    string
}

// MaybeAutoLearnFromHistory silently promotes a repeated history pair into a user rule.
func (e *Engine) MaybeAutoLearnFromHistory(ctx context.Context, from, to string) itypes.AutoLearnDebugInfo {
	return toAutoLearnDebugInfo(e.maybeAutoLearnFromHistory(ctx, from, to))
}

func (e *Engine) maybeAutoLearnFromHistory(ctx context.Context, from, to string) autoLearnResult {
	result := autoLearnResult{}
	if !autoLearnEnabled(e) {
		result.Reason = "auto-learn disabled"
		return result
	}
	ctx = autoLearnContext(ctx)
	from, to, ok := normalizeAutoLearnPair(from, to)
	if !ok {
		result.Reason = "empty correction pair"
		return result
	}

	if err := autoLearnContextErr(ctx); err != nil {
		return autoLearnResultWithErr(result, err)
	}

	entry, ok := e.history.Lookup(from)
	if !ok {
		result.Reason = "history pair not found"
		return result
	}
	if entry.To != to {
		result.Reason = "history pair points to a different target"
		return result
	}
	if entry.RuleApplied {
		result.Reason = "history pair already marked as rule-applied"
		return result
	}

	if existingResult, handled := e.handleExistingAutoLearnRule(from, to); handled {
		return existingResult
	}

	if entry.Count < e.autoLearnThreshold {
		result.Reason = "threshold not reached"
		return result
	}

	return e.promoteAutoLearnHistory(ctx, from, to)
}

func autoLearnEnabled(e *Engine) bool {
	return e != nil && e.autoLearnThreshold > 0
}

func autoLearnContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func normalizeAutoLearnPair(from, to string) (string, string, bool) {
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	if from == "" || to == "" {
		return "", "", false
	}
	return from, to, true
}

func autoLearnResultWithErr(result autoLearnResult, err error) autoLearnResult {
	result.TimedOut = errors.Is(err, context.DeadlineExceeded)
	result.Err = err
	if result.Reason == "" {
		result.Reason = err.Error()
	}
	return result
}

func (e *Engine) handleExistingAutoLearnRule(from, to string) (autoLearnResult, bool) {
	rule, ok := e.rules.MatchUser(from)
	if !ok {
		return autoLearnResult{}, false
	}

	result := autoLearnResult{}
	if rule.To != to {
		result.Reason = "existing user rule points elsewhere"
		return result, true
	}

	result.Triggered = true
	result.Reason = "user rule already exists"
	return e.markAutoLearnRuleApplied(result, from, to), true
}

func (e *Engine) promoteAutoLearnHistory(ctx context.Context, from, to string) autoLearnResult {
	result := autoLearnResult{}
	if err := autoLearnContextErr(ctx); err != nil {
		return autoLearnResultWithErr(result, err)
	}

	result.Triggered = true
	result.Reason = "promoted history pair into user rule"
	if err := e.rules.AddUserRule(itypes.Rule{From: from, To: to}); err != nil {
		result.Err = err
		return result
	}

	if err := autoLearnContextErr(ctx); err != nil {
		return autoLearnResultWithErr(result, err)
	}

	return e.markAutoLearnRuleApplied(result, from, to)
}

func (e *Engine) markAutoLearnRuleApplied(result autoLearnResult, from, to string) autoLearnResult {
	persisted, err := e.history.MarkRuleApplied(from, to)
	result.Persisted = persisted
	result.Err = err
	if err != nil && result.Reason == "" {
		result.Reason = err.Error()
	}
	return result
}

func toAutoLearnDebugInfo(result autoLearnResult) itypes.AutoLearnDebugInfo {
	info := itypes.AutoLearnDebugInfo{
		Triggered: result.Triggered,
		Persisted: result.Persisted,
		TimedOut:  result.TimedOut,
		Reason:    result.Reason,
	}
	if result.Err != nil {
		info.Error = result.Err.Error()
	}
	return info
}

func autoLearnContextErr(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
