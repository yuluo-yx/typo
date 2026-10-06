//go:build windows

package storage

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFileAllowsReplacementWithOpenReaders(t *testing.T) {
	for _, filename := range []string{"rules.json", "usage_history.json"} {
		t.Run(filename, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), filename)
			if err := WriteFileAtomic(target, []byte("old snapshot"), 0600); err != nil {
				t.Fatal(err)
			}
			// Keep multiple startup-style readers open throughout replacement.
			var readers []*os.File
			for range 24 {
				reader, err := openSharedReadFile(target)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = reader.Close() })
				readers = append(readers, reader)
			}
			if err := WriteFileAtomic(target, []byte("new snapshot"), 0600); err != nil {
				t.Fatalf("replacement with open readers failed: %v", err)
			}
			for _, reader := range readers {
				got, err := io.ReadAll(reader)
				if err != nil || string(got) != "old snapshot" {
					t.Fatalf("open reader = %q, %v; want old snapshot", got, err)
				}
			}
			got, err := ReadFile(target)
			if err != nil || string(got) != "new snapshot" {
				t.Fatalf("fresh reader = %q, %v; want new snapshot", got, err)
			}
		})
	}
}

func TestReadFileLongPath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), strings.Repeat("long-segment-", 10), strings.Repeat("segment-", 15))
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "rules.json")
	if err := WriteFileAtomic(target, []byte("snapshot"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFile(target)
	if err != nil || string(got) != "snapshot" {
		t.Fatalf("long-path reader = %q, %v; want snapshot", got, err)
	}
}
