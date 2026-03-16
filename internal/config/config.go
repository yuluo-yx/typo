package config

import (
	"os"
	"path/filepath"
)

// Config holds the application configuration.
type Config struct {
	ConfigDir string
	Debug     bool
}

// DefaultConfigDir returns the default configuration directory.
func DefaultConfigDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".typo")
}

// Load loads the configuration.
func Load() *Config {
	return &Config{
		ConfigDir: DefaultConfigDir(),
	}
}

// EnsureConfigDir ensures the configuration directory exists.
func (c *Config) EnsureConfigDir() error {
	if c.ConfigDir == "" {
		return nil
	}
	return os.MkdirAll(c.ConfigDir, 0755)
}
