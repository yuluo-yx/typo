//go:build !windows

package storage

import "os"

// ReadFile reads a state snapshot without blocking its replacement by a writer.
func ReadFile(filename string) ([]byte, error) {
	return os.ReadFile(filename)
}
