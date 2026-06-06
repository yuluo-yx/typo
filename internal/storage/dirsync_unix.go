//go:build !windows

package storage

import "os"

func (osAtomicFileOps) syncDir(dir string) error {
	dirFile, err := os.Open(dir)
	if err != nil {
		return err
	}
	if err := dirFile.Sync(); err != nil {
		_ = dirFile.Close()
		return err
	}
	return dirFile.Close()
}
