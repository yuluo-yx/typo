package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigDir(t *testing.T) {
	dir := DefaultConfigDir()
	if dir == "" {
		t.Error("Expected non-empty config dir")
	}

	// Should contain .typo
	if !filepath.IsAbs(dir) {
		t.Errorf("Expected absolute path, got %s", dir)
	}
}

func TestLoad(t *testing.T) {
	cfg := Load()
	if cfg == nil {
		t.Fatal("Load returned nil")
	}
	if cfg.ConfigDir == "" {
		t.Error("Expected non-empty config dir")
	}
}

func TestEnsureConfigDir(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{ConfigDir: filepath.Join(tmpDir, ".typo")}

	if err := cfg.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir failed: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(cfg.ConfigDir); os.IsNotExist(err) {
		t.Error("Config directory was not created")
	}
}

func TestEnsureConfigDir_Empty(t *testing.T) {
	cfg := &Config{ConfigDir: ""}

	// Should not error with empty config dir
	if err := cfg.EnsureConfigDir(); err != nil {
		t.Errorf("EnsureConfigDir should not error with empty dir: %v", err)
	}
}

func TestConfig_Debug(t *testing.T) {
	cfg := &Config{Debug: true}
	if !cfg.Debug {
		t.Error("Expected Debug to be true")
	}
}

func TestDefaultConfigDir_NoHome(t *testing.T) {
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)

	os.Unsetenv("HOME")
	dir := DefaultConfigDir()
	if dir != "" {
		t.Errorf("Expected empty dir when no home, got %s", dir)
	}
}
