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

func TestWriteFileAtomic_OverwritesExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	target := filepath.Join(tmpDir, "data.json")
	if err := os.WriteFile(target, []byte("old"), 0644); err != nil {
		t.Fatalf("failed to seed target file: %v", err)
	}

	if err := WriteFileAtomic(target, []byte("new"), 0600); err != nil {
		t.Fatalf("WriteFileAtomic overwrite failed: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("failed to read overwritten target: %v", err)
	}
	if string(data) != "new" {
		t.Fatalf("unexpected overwritten content: %q", data)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("failed to stat overwritten target: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("unexpected overwritten permission: %v", info.Mode().Perm())
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

func TestWriteFileAtomic_OperationFailuresCleanTempFile(t *testing.T) {
	target := filepath.Join(t.TempDir(), "data.json")

	tests := []struct {
		name      string
		configure func(*fakeAtomicFileOps)
	}{
		{
			name: "chmod",
			configure: func(ops *fakeAtomicFileOps) {
				ops.file.chmodErr = errors.New("chmod failed")
			},
		},
		{
			name: "write",
			configure: func(ops *fakeAtomicFileOps) {
				ops.file.writeErr = errors.New("write failed")
			},
		},
		{
			name: "sync",
			configure: func(ops *fakeAtomicFileOps) {
				ops.file.syncErr = errors.New("sync failed")
			},
		},
		{
			name: "close",
			configure: func(ops *fakeAtomicFileOps) {
				ops.file.closeErr = errors.New("close failed")
			},
		},
		{
			name: "rename",
			configure: func(ops *fakeAtomicFileOps) {
				ops.renameErr = errors.New("rename failed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := &fakeAtomicFileOps{file: &fakeAtomicFile{name: "temp-file"}}
			tt.configure(ops)

			err := writeFileAtomicWithOps(target, []byte("x"), 0600, ops)
			if err == nil {
				t.Fatal("expected WriteFileAtomic to return operation failure")
			}
			if !strings.Contains(err.Error(), tt.name) {
				t.Fatalf("expected %s error, got %v", tt.name, err)
			}
			if len(ops.removed) != 1 || ops.removed[0] != "temp-file" {
				t.Fatalf("expected temp file cleanup, got %v", ops.removed)
			}
		})
	}
}

func TestWriteFileAtomic_SyncsParentDirectoryAfterRename(t *testing.T) {
	target := filepath.Join(t.TempDir(), "nested", "data.json")
	ops := &fakeAtomicFileOps{file: &fakeAtomicFile{name: "temp-file"}}

	if err := writeFileAtomicWithOps(target, []byte("x"), 0600, ops); err != nil {
		t.Fatalf("WriteFileAtomic failed: %v", err)
	}

	if len(ops.syncedDirs) != 1 || ops.syncedDirs[0] != filepath.Dir(target) {
		t.Fatalf("expected parent directory sync for %q, got %v", filepath.Dir(target), ops.syncedDirs)
	}
	if len(ops.removed) != 0 {
		t.Fatalf("expected no temp cleanup after successful rename, got %v", ops.removed)
	}
}

func TestWriteFileAtomic_ParentDirectorySyncFailure(t *testing.T) {
	target := filepath.Join(t.TempDir(), "data.json")
	ops := &fakeAtomicFileOps{
		file:       &fakeAtomicFile{name: "temp-file"},
		syncDirErr: errors.New("sync dir failed"),
	}

	err := writeFileAtomicWithOps(target, []byte("x"), 0600, ops)
	if err == nil {
		t.Fatal("expected WriteFileAtomic to fail when parent directory sync fails")
	}
	if !strings.Contains(err.Error(), "sync dir") {
		t.Fatalf("expected parent directory sync error, got %v", err)
	}
	if len(ops.removed) != 0 {
		t.Fatalf("expected no temp cleanup after successful rename, got %v", ops.removed)
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

type fakeAtomicFileOps struct {
	file       *fakeAtomicFile
	renameErr  error
	syncDirErr error
	removed    []string
	syncedDirs []string
}

func (ops *fakeAtomicFileOps) createTemp(string, string) (atomicFile, error) {
	return ops.file, nil
}

func (ops *fakeAtomicFileOps) rename(string, string) error {
	return ops.renameErr
}

func (ops *fakeAtomicFileOps) remove(name string) error {
	ops.removed = append(ops.removed, name)
	return nil
}

func (ops *fakeAtomicFileOps) syncDir(dir string) error {
	ops.syncedDirs = append(ops.syncedDirs, dir)
	return ops.syncDirErr
}

type fakeAtomicFile struct {
	name     string
	chmodErr error
	writeErr error
	syncErr  error
	closeErr error
}

func (f *fakeAtomicFile) Name() string {
	return f.name
}

func (f *fakeAtomicFile) Chmod(os.FileMode) error {
	return f.chmodErr
}

func (f *fakeAtomicFile) Write(data []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return len(data), nil
}

func (f *fakeAtomicFile) Sync() error {
	return f.syncErr
}

func (f *fakeAtomicFile) Close() error {
	return f.closeErr
}
