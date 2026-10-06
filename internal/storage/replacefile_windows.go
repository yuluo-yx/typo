//go:build windows

package storage

import (
	"os"
	"path/filepath"
)

func (osAtomicFileOps) rename(oldpath, newpath string) error {
	// Atomic writes create their temporary file in the target directory.
	if filepath.Dir(oldpath) != filepath.Dir(newpath) {
		return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: os.ErrInvalid}
	}
	root, err := os.OpenRoot(filepath.Dir(newpath))
	if err == nil {
		defer func() { _ = root.Close() }()
		// Root.Rename uses Windows POSIX replacement semantics, unlike os.Rename's
		// MoveFileEx. This allows existing delete-sharing readers to keep snapshots.
		err = root.Rename(filepath.Base(oldpath), filepath.Base(newpath))
	}
	if err != nil {
		return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: err}
	}
	return nil
}
