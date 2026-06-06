//go:build windows

package storage

func (osAtomicFileOps) syncDir(string) error {
	return nil
}
