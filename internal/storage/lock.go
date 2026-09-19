package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// WithFileLock serializes a read-modify-write transaction across processes.
// The lock file must remain in place: unlinking it would allow concurrent locks
// on different inodes. Closing the descriptor releases the operating-system lock.
func WithFileLock(ctx context.Context, path string, transaction func() error) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	// #nosec G703 -- the engine supplies a local config-directory lock path, not command input.
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	// #nosec G703 -- this is the same config-directory lock path created above.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	if err := waitForFileLock(ctx, file); err != nil {
		return fmt.Errorf("lock %s: %w", path, err)
	}
	return transaction()
}

func waitForFileLock(ctx context.Context, file *os.File) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		locked, err := tryFileLock(file)
		if err != nil || locked {
			return err
		}
		timer := time.NewTimer(5 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
