package storage

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReadFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	for _, name := range []string{"rules.json", "usage_history.json", "\u72b6\u6001.json"} {
		t.Run(name, func(t *testing.T) {
			want := bytes.Repeat([]byte(`{"from":"gti","to":"git"}`), 4096)
			if err := os.WriteFile(name, want, 0600); err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{name, filepath.Join(dir, name)} {
				got, err := ReadFile(path)
				if err != nil || !bytes.Equal(got, want) {
					t.Fatalf("ReadFile(%q) = %d bytes, %v; want %d bytes", path, len(got), err, len(want))
				}
			}
		})
	}
}

func TestReadFileErrors(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{filepath.Join(dir, "missing.json"), dir, "invalid\x00path"} {
		t.Run(name, func(t *testing.T) {
			_, err := ReadFile(name)
			var pathErr *os.PathError
			if !errors.As(err, &pathErr) || pathErr.Path != name {
				t.Fatalf("ReadFile(%q) error = %v, want PathError for original path", name, err)
			}
			if filepath.Base(name) == "missing.json" && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("missing file error = %v, want ErrNotExist", err)
			}
		})
	}
}
