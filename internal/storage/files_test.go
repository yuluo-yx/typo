package storage

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileAtomic(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "data.json")

	if err := WriteFileAtomic(target, []byte(`{"ok":true}`), 0600); err != nil {
		t.Fatalf("WriteFileAtomic failed: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("failed to read atomic write target: %v", err)
	}
	if string(data) != `{"ok":true}` {
		t.Fatalf("unexpected atomic write content: %q", data)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("failed to stat atomic write target: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("unexpected atomic write permission: %v", info.Mode().Perm())
	}
}

func TestWriteFileAtomic_CreateTempFailure(t *testing.T) {
	target := filepath.Join(t.TempDir(), "missing", "data.json")
	if err := WriteFileAtomic(target, []byte("x"), 0600); err == nil {
		t.Fatal("Expected WriteFileAtomic to fail when parent directory is missing")
	}
}

func TestWriteFileAtomic_RenameFailure(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "existing-dir")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatalf("failed to create target directory: %v", err)
	}

	if err := WriteFileAtomic(target, []byte("x"), 0600); err == nil {
		t.Fatal("Expected WriteFileAtomic to fail when target path is a directory")
	}
}

func TestQuarantineInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "broken.json")
	if err := os.WriteFile(target, []byte("{"), 0600); err != nil {
		t.Fatalf("failed to seed invalid json file: %v", err)
	}

	output := captureStderr(t, func() {
		QuarantineInvalidJSON(target, errors.New("invalid character"))
	})

	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("expected original invalid file to be moved away, got %v", err)
	}

	matches, err := filepath.Glob(target + ".corrupt-*")
	if err != nil {
		t.Fatalf("failed to glob quarantined files: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one quarantined file, got %v", matches)
	}
	if !strings.Contains(output, "moved invalid JSON file") {
		t.Fatalf("expected quarantine log output, got %q", output)
	}
}

func TestQuarantineInvalidJSON_RenameFailure(t *testing.T) {
	target := filepath.Join(t.TempDir(), "missing.json")
	output := captureStderr(t, func() {
		QuarantineInvalidJSON(target, errors.New("invalid character"))
	})

	if !strings.Contains(output, "ignoring invalid JSON file") {
		t.Fatalf("expected ignore log output, got %q", output)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stderr pipe: %v", err)
	}

	os.Stderr = w
	fn()
	_ = w.Close()
	os.Stderr = oldStderr

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read captured stderr: %v", err)
	}
	return string(data)
}
