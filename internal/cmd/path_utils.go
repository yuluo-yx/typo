package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func sameDir(a, b string) bool {
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

func pathWithinDir(path, dir string) bool {
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

func pathContainsDir(pathValue, dir string) bool {
	for _, item := range filepath.SplitList(pathValue) {
		if sameDir(item, dir) {
			return true
		}
	}
	return false
}
