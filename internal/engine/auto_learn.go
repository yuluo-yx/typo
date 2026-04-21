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
}

// MaybeAutoLearnFromHistory silently promotes a repeated history pair into a user rule.
func (e *Engine) MaybeAutoLearnFromHistory(ctx context.Context, from, to string) {
	_ = e.maybeAutoLearnFromHistory(ctx, from, to)
}

func (e *Engine) maybeAutoLearnFromHistory(ctx context.Context, from, to string) autoLearnResult {
	result := autoLearnResult{}
	if e == nil || e.autoLearnThreshold <= 0 {
		return result
	}
	if ctx == nil {
		ctx = context.Background()
	}

	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	if from == "" || to == "" {
		return result
	}

	if err := autoLearnContextErr(ctx); err != nil {
		result.TimedOut = errors.Is(err, context.DeadlineExceeded)
		result.Err = err
		return result
	}

	entry, ok := e.history.Lookup(from)
	if !ok || entry.To != to || entry.RuleApplied {
		return result
	}

	if rule, ok := e.rules.MatchUser(from); ok {
		if rule.To == to {
			result.Triggered = true
			persisted, err := e.history.MarkRuleApplied(from, to)
			result.Persisted = persisted
			result.Err = err
		}
		return result
	}

	if entry.Count < e.autoLearnThreshold {
		return result
	}

	if err := autoLearnContextErr(ctx); err != nil {
		result.TimedOut = errors.Is(err, context.DeadlineExceeded)
		result.Err = err
		return result
	}

	result.Triggered = true
	if err := e.rules.AddUserRule(itypes.Rule{From: from, To: to}); err != nil {
		result.Err = err
		return result
	}

	if err := autoLearnContextErr(ctx); err != nil {
		result.TimedOut = errors.Is(err, context.DeadlineExceeded)
		result.Err = err
		return result
	}

	persisted, err := e.history.MarkRuleApplied(from, to)
	result.Persisted = persisted
	result.Err = err
	return result
}

func autoLearnContextErr(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
