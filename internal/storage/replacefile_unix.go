//go:build !windows

package storage

import "os"

func (osAtomicFileOps) rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}
