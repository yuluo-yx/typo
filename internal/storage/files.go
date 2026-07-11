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
// 父目录同步在 rename 提交后尽力执行，不会把已可见的写入报告为未提交。
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
	// rename 已经提交目标文件；目录同步只用于增强崩溃持久性，失败后无法安全回滚。
	_ = ops.syncDir(dir)
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
