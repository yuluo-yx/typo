package utils

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// SameDir reports whether two paths point to the same directory.
func SameDir(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	cleanA := filepath.Clean(a)
	cleanB := filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(cleanA, cleanB)
	}
	return cleanA == cleanB
}

// PathWithinDir reports whether path is inside dir or equals dir.
func PathWithinDir(path, dir string) bool {
	if path == "" || dir == "" {
		return false
	}

	rel, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(path))
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

// PathContainsDir reports whether a PATH-style list contains dir.
func PathContainsDir(pathValue, dir string) bool {
	for _, item := range filepath.SplitList(pathValue) {
		if SameDir(item, dir) {
			return true
		}
	}
	return false
}
