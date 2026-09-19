//go:build unix

package storage

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func tryFileLock(file *os.File) (bool, error) {
	// #nosec G115 -- Unix descriptors are ints; a closed file's all-ones sentinel maps to -1.
	err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EINTR) {
		return false, nil
	}
	return err == nil, err
}
