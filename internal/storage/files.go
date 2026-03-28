package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// WriteFileAtomic writes the target file atomically by renaming a temp file in the same directory.
func WriteFileAtomic(filename string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(filename)

	tmpFile, err := os.CreateTemp(dir, filepath.Base(filename)+".tmp-*")
	if err != nil {
		return err
	}

	tmpName := tmpFile.Name()
	keepTemp := false
	defer func() {
		if !keepTemp {
			_ = os.Remove(tmpName)
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
	if err := os.Rename(tmpName, filename); err != nil {
		return err
	}

	keepTemp = true
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
