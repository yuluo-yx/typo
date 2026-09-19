package engine

import (
	"context"
	"path/filepath"

	"github.com/yuluo-yx/typo/internal/storage"
)

// withPersistentState reloads snapshots after acquiring the shared writer lock.
// All CLI rule/history mutations, including auto-learn rollback, use this boundary.
// Correction and terminal candidate selection run outside it.
func (e *Engine) withPersistentState(ctx context.Context, transaction func() error) error {
	dir := e.rules.configDir
	if dir == "" && e.history != nil {
		dir = e.history.configDir
	}
	if dir == "" {
		return transaction()
	}
	return storage.WithFileLock(ctx, filepath.Join(dir, ".state.lock"), func() error {
		if err := e.rules.reload(); err != nil {
			return err
		}
		if e.history != nil {
			if err := e.history.reload(); err != nil {
				return err
			}
		}
		return transaction()
	})
}

func (r *Rules) reload() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.loadUserRules(); err != nil {
		return err
	}
	r.rebuildTargets()
	return nil
}

func (h *History) reload() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.load(); err != nil {
		return err
	}
	h.rebuildTargets()
	return nil
}

// ClearHistory clears the latest persisted history under the shared writer lock.
func (e *Engine) ClearHistory() error {
	return e.withPersistentState(context.Background(), e.history.Clear)
}
