package utils

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSameDir(t *testing.T) {
	dir := filepath.Join("tmp", "dir")

	tests := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{
			name: "same directory with trailing separator",
			a:    dir,
			b:    dir + string(os.PathSeparator),
			want: true,
		},
		{
			name: "empty left path",
			a:    "",
			b:    dir,
			want: false,
		},
		{
			name: "different directories",
			a:    dir,
			b:    filepath.Join("tmp", "other"),
			want: false,
		},
		{
			name: "case sensitivity follows platform",
			a:    filepath.Join("tmp", "dir"),
			b:    filepath.Join("tmp", "DIR"),
			want: runtime.GOOS == "windows",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SameDir(tt.a, tt.b); got != tt.want {
				t.Fatalf("SameDir(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestPathWithinDir(t *testing.T) {
	dir := filepath.Join("tmp", "typo")

	tests := []struct {
		name string
		path string
		dir  string
		want bool
	}{
		{
			name: "same directory",
			path: dir,
			dir:  dir,
			want: true,
		},
		{
			name: "child path",
			path: filepath.Join(dir, "bin", "typo"),
			dir:  dir,
			want: true,
		},
		{
			name: "sibling path",
			path: filepath.Join("tmp", "typo-other", "bin"),
			dir:  dir,
			want: false,
		},
		{
			name: "parent escape",
			path: filepath.Join("tmp"),
			dir:  dir,
			want: false,
		},
		{
			name: "empty path",
			path: "",
			dir:  dir,
			want: false,
		},
		{
			name: "empty dir",
			path: filepath.Join(dir, "bin"),
			dir:  "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PathWithinDir(tt.path, tt.dir); got != tt.want {
				t.Fatalf("PathWithinDir(%q, %q) = %v, want %v", tt.path, tt.dir, got, tt.want)
			}
		})
	}
}

func TestPathContainsDir(t *testing.T) {
	target := filepath.Join("tmp", "dir")
	pathValue := filepath.Join("usr", "bin") + string(os.PathListSeparator) +
		target + string(os.PathListSeparator) +
		filepath.Join("bin")

	tests := []struct {
		name      string
		pathValue string
		dir       string
		want      bool
	}{
		{
			name:      "path list contains directory",
			pathValue: pathValue,
			dir:       target,
			want:      true,
		},
		{
			name:      "path list does not contain directory",
			pathValue: filepath.Join("usr", "bin") + string(os.PathListSeparator) + filepath.Join("bin"),
			dir:       target,
			want:      false,
		},
		{
			name:      "empty path list",
			pathValue: "",
			dir:       target,
			want:      false,
		},
		{
			name:      "empty target directory",
			pathValue: pathValue,
			dir:       "",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PathContainsDir(tt.pathValue, tt.dir); got != tt.want {
				t.Fatalf("PathContainsDir(%q, %q) = %v, want %v", tt.pathValue, tt.dir, got, tt.want)
			}
		})
	}
}
