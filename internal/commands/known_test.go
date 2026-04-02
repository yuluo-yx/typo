package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscover(t *testing.T) {
	cmds := Discover()
	if len(cmds) == 0 {
		t.Error("Expected to discover some commands from PATH")
	}

	// Should include common commands
	common := []string{"ls", "cat", "echo"}
	for _, c := range common {
		found := false
		for _, cmd := range cmds {
			if cmd == c {
				found = true
				break
			}
		}
		if !found {
			t.Logf("Warning: common command %q not found in PATH", c)
		}
	}
}

func TestDiscoverCommon(t *testing.T) {
	cmds := DiscoverCommon()
	if len(cmds) == 0 {
		t.Error("Expected some common commands")
	}

	for _, expected := range []string{"git", "xargs", "aws", "gcloud", "az"} {
		found := false
		for _, cmd := range cmds {
			if cmd == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected %q in common commands", expected)
		}
	}
}

func TestFilter(t *testing.T) {
	cmds := []string{"git", "grep", "go", "cat"}

	filtered := Filter(cmds, "g")
	if len(filtered) != 3 { // git, grep, go
		t.Errorf("Expected 3 commands starting with 'g', got %d", len(filtered))
	}

	filtered = Filter(cmds, "git")
	if len(filtered) != 1 {
		t.Errorf("Expected 1 command starting with 'git', got %d", len(filtered))
	}

	filtered = Filter(cmds, "")
	if len(filtered) != len(cmds) {
		t.Errorf("Expected all commands with empty prefix, got %d", len(filtered))
	}
}

func TestIsExecutable(t *testing.T) {
	// Create a temp file
	tmpDir := t.TempDir()

	// Create non-executable file
	nonExecFile := filepath.Join(tmpDir, "nonexec")
	if err := os.WriteFile(nonExecFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if IsExecutable(nonExecFile) {
		t.Error("Expected non-executable file to return false")
	}

	// Create executable file
	execFile := filepath.Join(tmpDir, "exec")
	if err := os.WriteFile(execFile, []byte("test"), 0755); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if !IsExecutable(execFile) {
		t.Error("Expected executable file to return true")
	}

	// Non-existent file
	if IsExecutable("/nonexistent/file") {
		t.Error("Expected non-existent file to return false")
	}
}

func TestGetPath(t *testing.T) {
	// Test with a common command
	path := GetPath("ls")
	if path == "" {
		t.Log("Warning: 'ls' not found in PATH")
	} else if !filepath.IsAbs(path) {
		t.Errorf("Expected absolute path, got %s", path)
	}

	// Test with non-existent command
	path = GetPath("nonexistentcommand12345")
	if path != "" {
		t.Errorf("Expected empty path for non-existent command, got %s", path)
	}
}

func TestAddFileExtension(t *testing.T) {
	result := AddFileExtension("test")
	if result != "test" {
		t.Errorf("Expected 'test', got %s", result)
	}
}

func TestIsCommonCommand(t *testing.T) {
	if !IsCommonCommand("docker") {
		t.Fatal("Expected docker to be a common command")
	}
	for _, cmd := range []string{"aws", "gcloud", "az"} {
		if !IsCommonCommand(cmd) {
			t.Fatalf("Expected %s to be a common command", cmd)
		}
	}
	if IsCommonCommand("not-a-real-common-command") {
		t.Fatal("Expected unknown command to not be common")
	}
}

func TestIsShellBuiltin(t *testing.T) {
	if !IsShellBuiltin("source") {
		t.Fatal("Expected source to be a shell builtin")
	}
	if IsShellBuiltin("docker") {
		t.Fatal("Expected docker to not be a shell builtin")
	}
}

func TestDiscoverInDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create executable files
	execFile := filepath.Join(tmpDir, "mycmd")
	if err := os.WriteFile(execFile, []byte("#!/bin/sh"), 0755); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create hidden file (should be ignored)
	hiddenFile := filepath.Join(tmpDir, ".hidden")
	if err := os.WriteFile(hiddenFile, []byte("#!/bin/sh"), 0755); err != nil {
		t.Fatalf("Failed to create hidden file: %v", err)
	}

	// Create non-executable file
	nonExecFile := filepath.Join(tmpDir, "nonexec")
	if err := os.WriteFile(nonExecFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create non-exec file: %v", err)
	}

	// Create subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	commands := make(map[string]bool)
	discoverInDir(tmpDir, commands)

	if !commands["mycmd"] {
		t.Error("Expected 'mycmd' to be discovered")
	}
	if commands[".hidden"] {
		t.Error("Expected hidden file to be ignored")
	}
	if commands["nonexec"] {
		t.Error("Expected non-executable file to be ignored")
	}
	if commands["subdir"] {
		t.Error("Expected directory to be ignored")
	}
}

func TestDiscover_EmptyPath(t *testing.T) {
	oldPath := os.Getenv("PATH")
	defer func() {
		if err := os.Setenv("PATH", oldPath); err != nil {
			t.Fatalf("Restore PATH failed: %v", err)
		}
	}()

	if err := os.Setenv("PATH", ""); err != nil {
		t.Fatalf("Setenv PATH failed: %v", err)
	}
	cmds := Discover()
	if len(cmds) != 0 {
		t.Errorf("Expected empty slice with empty PATH, got %d", len(cmds))
	}
}

func TestDiscoverInDir_NonexistentDir(t *testing.T) {
	commands := make(map[string]bool)
	// Should not panic or error
	discoverInDir("/nonexistent/dir/12345", commands)
	if len(commands) != 0 {
		t.Error("Expected no commands from nonexistent dir")
	}
}

func TestGetPath_EmptyPath(t *testing.T) {
	oldPath := os.Getenv("PATH")
	defer func() {
		if err := os.Setenv("PATH", oldPath); err != nil {
			t.Fatalf("Restore PATH failed: %v", err)
		}
	}()

	if err := os.Setenv("PATH", ""); err != nil {
		t.Fatalf("Setenv PATH failed: %v", err)
	}
	path := GetPath("ls")
	if path != "" {
		t.Errorf("Expected empty path with empty PATH, got %s", path)
	}
}

func TestDiscoverInDir_FileInfoError(t *testing.T) {
	// Test case where entry.Info() would fail
	// This is hard to trigger directly, but we can at least verify it doesn't panic
	tmpDir := t.TempDir()

	// Create a valid executable to ensure function runs
	execFile := filepath.Join(tmpDir, "testcmd")
	if err := os.WriteFile(execFile, []byte("#!/bin/sh"), 0755); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	commands := make(map[string]bool)
	discoverInDir(tmpDir, commands)

	if !commands["testcmd"] {
		t.Error("Expected 'testcmd' to be discovered")
	}
}
