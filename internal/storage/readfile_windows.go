//go:build windows

package storage

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// ReadFile reads a state snapshot without blocking its replacement by a writer.
func ReadFile(filename string) ([]byte, error) {
	file, err := openSharedReadFile(filename)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	return io.ReadAll(file)
}

func openSharedReadFile(filename string) (*os.File, error) {
	path, err := filepath.Abs(filename)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: filename, Err: err}
	}
	// Extended paths preserve support for long config-directory names and UNC shares.
	if len(path) >= 248 && !strings.HasPrefix(path, `\\?\`) && !strings.HasPrefix(path, `\\.\`) {
		if strings.HasPrefix(path, `\\`) {
			path = `\\?\UNC\` + path[2:]
		} else {
			path = `\\?\` + path
		}
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: filename, Err: err}
	}
	// Startup readers run outside the writer lock. Ordinary os.Open handles omit
	// FILE_SHARE_DELETE and can make a concurrent atomic rename fail on Windows.
	handle, err := windows.CreateFile(name, windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: filename, Err: err}
	}
	return os.NewFile(uintptr(handle), filename), nil
}
