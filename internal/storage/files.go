package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type atomicFile interface {
	Name() string
	Chmod(os.FileMode) error
	Write([]byte) (int, error)
	Sync() error
	Close() error
}

type atomicFileOps interface {
	createTemp(dir, pattern string) (atomicFile, error)
	rename(oldpath, newpath string) error
	remove(name string) error
	syncDir(dir string) error
}

type osAtomicFileOps struct{}

func (osAtomicFileOps) createTemp(dir, pattern string) (atomicFile, error) {
	return os.CreateTemp(dir, pattern)
}

func (osAtomicFileOps) rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

func (osAtomicFileOps) remove(name string) error {
	return os.Remove(name)
}

// WriteFileAtomic writes the target file atomically by renaming a temp file in the same directory.
func WriteFileAtomic(filename string, data []byte, perm os.FileMode) error {
	return writeFileAtomicWithOps(filename, data, perm, osAtomicFileOps{})
}

func writeFileAtomicWithOps(filename string, data []byte, perm os.FileMode, ops atomicFileOps) error {
	dir := filepath.Dir(filename)

	tmpFile, err := ops.createTemp(dir, filepath.Base(filename)+".tmp-*")
	if err != nil {
		return err
	}

	tmpName := tmpFile.Name()
	keepTemp := false
	defer func() {
		if !keepTemp {
			_ = ops.remove(tmpName)
		}
	}()

	if err := tmpFile.Chmod(perm); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if err := ops.rename(tmpName, filename); err != nil {
		return err
	}

	keepTemp = true
	if err := ops.syncDir(dir); err != nil {
		return err
	}
	return nil
}

// QuarantineInvalidJSON moves a corrupted JSON file aside so later writes do not silently overwrite it.
func QuarantineInvalidJSON(path string, parseErr error) {
	backupPath := fmt.Sprintf("%s.corrupt-%s", path, time.Now().UTC().Format("20060102T150405"))
	if err := os.Rename(path, backupPath); err != nil {
		fmt.Fprintf(os.Stderr, "typo: ignoring invalid JSON file %s: %v\n", path, parseErr)
		return
	}

	fmt.Fprintf(os.Stderr, "typo: moved invalid JSON file %s to %s: %v\n", path, backupPath, parseErr)
}
