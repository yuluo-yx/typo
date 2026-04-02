package config

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func captureConfigStderr(t *testing.T, fn func()) string {
	t.Helper()

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() failed: %v", err)
	}
	os.Stderr = w
	defer func() {
		os.Stderr = oldStderr
		_ = w.Close()
		_ = r.Close()
	}()

	fn()

	_ = w.Close()

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	return buf.String()
}

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

func TestConfigFilePath(t *testing.T) {
	cfg := &Config{ConfigDir: filepath.Join(t.TempDir(), ".typo")}
	if got := cfg.ConfigFilePath(); !strings.HasSuffix(got, "/config.json") {
		t.Fatalf("ConfigFilePath() = %q", got)
	}

	empty := &Config{}
	if got := empty.ConfigFilePath(); got != "" {
		t.Fatalf("ConfigFilePath() with empty dir = %q, want empty", got)
	}
}

func TestDefaultUserConfig(t *testing.T) {
	cfg := DefaultUserConfig()

	if cfg.SimilarityThreshold != 0.6 {
		t.Fatalf("SimilarityThreshold = %v, want 0.6", cfg.SimilarityThreshold)
	}
	if cfg.MaxEditDistance != 2 {
		t.Fatalf("MaxEditDistance = %d, want 2", cfg.MaxEditDistance)
	}
	if cfg.MaxFixPasses != 32 {
		t.Fatalf("MaxFixPasses = %d, want 32", cfg.MaxFixPasses)
	}
	if cfg.Keyboard != "qwerty" {
		t.Fatalf("Keyboard = %q, want qwerty", cfg.Keyboard)
	}
	if !cfg.History.Enabled {
		t.Fatal("History.Enabled should default to true")
	}
	if !cfg.Rules["git"].Enabled {
		t.Fatal("rules.git.enabled should default to true")
	}
}

func TestLoad_MergesPartialConfigFile(t *testing.T) {
	tmpHome := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)

	if err := os.Setenv("HOME", tmpHome); err != nil {
		t.Fatalf("Setenv HOME failed: %v", err)
	}

	configDir := filepath.Join(tmpHome, ".typo")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	configFile := filepath.Join(configDir, configFileName)
	data := []byte(`{
  "similarity_threshold": 0.7,
  "keyboard": "dvorak",
  "history": {
    "enabled": false
  },
  "rules": {
    "docker": { "enabled": false }
  }
}`)
	if err := os.WriteFile(configFile, data, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfg := Load()
	if cfg.User.SimilarityThreshold != 0.7 {
		t.Fatalf("SimilarityThreshold = %v, want 0.7", cfg.User.SimilarityThreshold)
	}
	if cfg.User.Keyboard != "dvorak" {
		t.Fatalf("Keyboard = %q, want dvorak", cfg.User.Keyboard)
	}
	if cfg.User.History.Enabled {
		t.Fatal("History.Enabled should be false")
	}
	if cfg.User.MaxEditDistance != 2 {
		t.Fatalf("MaxEditDistance = %d, want default 2", cfg.User.MaxEditDistance)
	}
	if cfg.User.MaxFixPasses != 32 {
		t.Fatalf("MaxFixPasses = %d, want default 32", cfg.User.MaxFixPasses)
	}
	if cfg.User.Rules["docker"].Enabled {
		t.Fatal("rules.docker.enabled should be false")
	}
	if !cfg.User.Rules["git"].Enabled {
		t.Fatal("rules.git.enabled should stay true by default")
	}
}

func TestConfigGenerateRequiresForceForExistingFile(t *testing.T) {
	cfg := &Config{ConfigDir: t.TempDir(), User: DefaultUserConfig()}

	if err := cfg.Generate(false); err != nil {
		t.Fatalf("Generate(false) failed: %v", err)
	}

	err := cfg.Generate(false)
	if err == nil {
		t.Fatal("Generate(false) should fail when config already exists")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Fatalf("Generate(false) error = %v, want message containing --force", err)
	}

	if err := cfg.Generate(true); err != nil {
		t.Fatalf("Generate(true) failed: %v", err)
	}
}

func TestConfigGenerateAndSave_EmptyConfigDir(t *testing.T) {
	cfg := &Config{ConfigDir: "", User: DefaultUserConfig()}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() with empty config dir failed: %v", err)
	}
	if err := cfg.Generate(false); err != nil {
		t.Fatalf("Generate() with empty config dir failed: %v", err)
	}
	cfg.loadUserConfig()
}

func TestConfigSaveFailsForInvalidConfigAndBadDir(t *testing.T) {
	cfg := &Config{ConfigDir: t.TempDir(), User: DefaultUserConfig()}
	cfg.User.Keyboard = "invalid"
	if err := cfg.Save(); err == nil {
		t.Fatal("Save() should fail for invalid config")
	}

	tmpFile, err := os.CreateTemp("", "typo-config-dir-*")
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	_ = tmpFile.Close()

	cfg = &Config{ConfigDir: tmpFile.Name(), User: DefaultUserConfig()}
	if err := cfg.Save(); err == nil {
		t.Fatal("Save() should fail when config dir is a file")
	}
	if err := cfg.Generate(true); err == nil {
		t.Fatal("Generate() should fail when config dir is a file")
	}
}

func TestConfigSetGetAndReset(t *testing.T) {
	cfg := &Config{ConfigDir: t.TempDir(), User: DefaultUserConfig()}

	mustSetConfigValues(t, cfg, map[string]string{
		"similarity-threshold": "0.7",
		"max-edit-distance":    "3",
		"max-fix-passes":       "40",
		"keyboard":             "colemak",
		"rules.git.enabled":    "false",
		"history.enabled":      "false",
	})

	assertConfigValue(t, cfg, "keyboard", "colemak")
	assertConfigValue(t, cfg, "rules.git.enabled", "false")
	assertConfigValue(t, cfg, "similarity-threshold", "0.7")
	assertConfigValue(t, cfg, "max-edit-distance", "3")
	assertConfigValue(t, cfg, "max-fix-passes", "40")
	assertConfigValue(t, cfg, "history.enabled", "false")
	assertListSetting(t, cfg.ListSettings(), "rules.git.enabled", "false")
	assertResetConfig(t, cfg)
}

func mustSetConfigValues(t *testing.T, cfg *Config, values map[string]string) {
	t.Helper()

	for key, value := range values {
		if err := cfg.SetValue(key, value); err != nil {
			t.Fatalf("SetValue(%q, %q) failed: %v", key, value, err)
		}
		if err := cfg.Save(); err != nil {
			t.Fatalf("Save() after SetValue(%q, %q) failed: %v", key, value, err)
		}
	}
}

func assertConfigValue(t *testing.T, cfg *Config, key, want string) {
	t.Helper()

	value, err := cfg.Get(key)
	if err != nil {
		t.Fatalf("Get(%q) failed: %v", key, err)
	}
	if value != want {
		t.Fatalf("Get(%q) = %q, want %q", key, value, want)
	}
}

func assertListSetting(t *testing.T, settings []Setting, key, want string) {
	t.Helper()

	for _, setting := range settings {
		if setting.Key == key && setting.Value == want {
			return
		}
	}

	t.Fatalf("ListSettings() should include %s=%s", key, want)
}

func assertResetConfig(t *testing.T, cfg *Config) {
	t.Helper()

	if err := cfg.Reset(); err != nil {
		t.Fatalf("Reset failed: %v", err)
	}
	if cfg.User.Keyboard != "qwerty" {
		t.Fatalf("Keyboard after reset = %q, want qwerty", cfg.User.Keyboard)
	}
	if !cfg.User.Rules["git"].Enabled {
		t.Fatal("rules.git.enabled should reset to true")
	}
}

func TestUserConfigValidate(t *testing.T) {
	cfg := DefaultUserConfig()
	cfg.Keyboard = "unknown"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate should reject unknown keyboard")
	}

	cfg = DefaultUserConfig()
	cfg.Rules["unknown"] = RuleSetConfig{Enabled: true}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate should allow unknown rule scope for forward compatibility: %v", err)
	}

	cfg = DefaultUserConfig()
	cfg.SimilarityThreshold = -0.1
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate should reject low threshold")
	}

	cfg = DefaultUserConfig()
	cfg.SimilarityThreshold = 1.1
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate should reject high threshold")
	}

	cfg = DefaultUserConfig()
	cfg.MaxEditDistance = -1
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate should reject negative max edit distance")
	}

	cfg = DefaultUserConfig()
	cfg.MaxFixPasses = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate should reject zero max fix passes")
	}
}

func TestConfigGetAndSetErrors(t *testing.T) {
	cfg := &Config{ConfigDir: t.TempDir(), User: DefaultUserConfig()}

	if _, err := cfg.Get("unknown"); err == nil {
		t.Fatal("Get() should reject unknown key")
	}
	if _, err := cfg.Get("rules.unknown.enabled"); err == nil {
		t.Fatal("Get() should reject unknown rule scope")
	}

	tests := []struct {
		key   string
		value string
	}{
		{key: "similarity-threshold", value: "abc"},
		{key: "max-edit-distance", value: "abc"},
		{key: "max-fix-passes", value: "abc"},
		{key: "history.enabled", value: "maybe"},
		{key: "rules.git.enabled", value: "maybe"},
		{key: "unknown", value: "1"},
		{key: "rules.unknown.enabled", value: "true"},
	}

	for _, tt := range tests {
		if err := cfg.Set(tt.key, tt.value); err == nil {
			t.Fatalf("Set(%q, %q) should fail", tt.key, tt.value)
		}
	}
}

func TestConfigSetValueMutatesInMemoryWithoutSaving(t *testing.T) {
	cfg := &Config{ConfigDir: t.TempDir(), User: DefaultUserConfig()}

	if err := cfg.SetValue("keyboard", "colemak"); err != nil {
		t.Fatalf("SetValue() failed: %v", err)
	}
	if cfg.User.Keyboard != "colemak" {
		t.Fatalf("Keyboard after SetValue() = %q, want colemak", cfg.User.Keyboard)
	}
	if _, err := os.Stat(cfg.ConfigFilePath()); !os.IsNotExist(err) {
		t.Fatalf("expected config file to stay absent before Save(), got %v", err)
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}
	if _, err := os.Stat(cfg.ConfigFilePath()); err != nil {
		t.Fatalf("expected config file after Save(), got %v", err)
	}
}

func TestConfigSetValueLeavesConfigUnchangedOnFailure(t *testing.T) {
	cfg := &Config{ConfigDir: t.TempDir(), User: DefaultUserConfig()}
	original := cloneUserConfig(cfg.User)

	if err := cfg.SetValue("max-fix-passes", "0"); err == nil {
		t.Fatal("SetValue() should fail validation for zero max fix passes")
	}
	if !reflect.DeepEqual(cfg.User, original) {
		t.Fatalf("SetValue() mutated config on validation failure: got %+v want %+v", cfg.User, original)
	}

	if err := cfg.SetValue("history.enabled", "maybe"); err == nil {
		t.Fatal("SetValue() should fail parsing for invalid bool")
	}
	if !reflect.DeepEqual(cfg.User, original) {
		t.Fatalf("SetValue() mutated config on parse failure: got %+v want %+v", cfg.User, original)
	}
}

func TestConfigSetLeavesConfigUnchangedWhenSaveFails(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "typo-config-dir-*")
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	_ = tmpFile.Close()

	cfg := &Config{ConfigDir: tmpFile.Name(), User: DefaultUserConfig()}
	original := cloneUserConfig(cfg.User)

	if err := cfg.Set("keyboard", "colemak"); err == nil {
		t.Fatal("Set() should fail when config dir is a file")
	}
	if !reflect.DeepEqual(cfg.User, original) {
		t.Fatalf("Set() mutated config on save failure: got %+v want %+v", cfg.User, original)
	}
}

func TestConfigResetLeavesConfigUnchangedWhenSaveFails(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "typo-config-dir-*")
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	_ = tmpFile.Close()

	cfg := &Config{ConfigDir: tmpFile.Name(), User: DefaultUserConfig()}
	cfg.User.Keyboard = "colemak"
	original := cloneUserConfig(cfg.User)

	if err := cfg.Reset(); err == nil {
		t.Fatal("Reset() should fail when config dir is a file")
	}
	if !reflect.DeepEqual(cfg.User, original) {
		t.Fatalf("Reset() mutated config on save failure: got %+v want %+v", cfg.User, original)
	}
}

func TestConfigGenerateLeavesConfigUnchangedWhenSaveFails(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "typo-config-dir-*")
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	_ = tmpFile.Close()

	cfg := &Config{ConfigDir: tmpFile.Name(), User: DefaultUserConfig()}
	cfg.User.Keyboard = "colemak"
	original := cloneUserConfig(cfg.User)

	if err := cfg.Generate(true); err == nil {
		t.Fatal("Generate(true) should fail when config dir is a file")
	}
	if !reflect.DeepEqual(cfg.User, original) {
		t.Fatalf("Generate() mutated config on save failure: got %+v want %+v", cfg.User, original)
	}
}

func TestLoad_InvalidJSONAndInvalidConfigFallBackToDefaults(t *testing.T) {
	t.Run("invalid json", func(t *testing.T) {
		tmpHome := t.TempDir()
		oldHome := os.Getenv("HOME")
		defer os.Setenv("HOME", oldHome)
		if err := os.Setenv("HOME", tmpHome); err != nil {
			t.Fatalf("Setenv HOME failed: %v", err)
		}

		configDir := filepath.Join(tmpHome, ".typo")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("MkdirAll failed: %v", err)
		}

		configFile := filepath.Join(configDir, configFileName)
		if err := os.WriteFile(configFile, []byte("{"), 0644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		cfg := Load()
		if cfg.User.Keyboard != "qwerty" {
			t.Fatalf("Keyboard = %q, want default qwerty", cfg.User.Keyboard)
		}
		matches, err := filepath.Glob(configFile + ".corrupt-*")
		if err != nil {
			t.Fatalf("Glob failed: %v", err)
		}
		if len(matches) != 1 {
			t.Fatalf("expected quarantined invalid config file, got %v", matches)
		}
	})

	t.Run("invalid semantic config", func(t *testing.T) {
		tmpHome := t.TempDir()
		oldHome := os.Getenv("HOME")
		defer os.Setenv("HOME", oldHome)
		if err := os.Setenv("HOME", tmpHome); err != nil {
			t.Fatalf("Setenv HOME failed: %v", err)
		}

		configDir := filepath.Join(tmpHome, ".typo")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("MkdirAll failed: %v", err)
		}

		configFile := filepath.Join(configDir, configFileName)
		data := []byte(`{"keyboard":"invalid","similarity_threshold":0.8}`)
		if err := os.WriteFile(configFile, data, 0644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		cfg := Load()
		if cfg.User.Keyboard != "qwerty" {
			t.Fatalf("Keyboard = %q, want default qwerty after invalid config", cfg.User.Keyboard)
		}
		if cfg.User.SimilarityThreshold != 0.6 {
			t.Fatalf("SimilarityThreshold = %v, want default 0.6 after invalid config", cfg.User.SimilarityThreshold)
		}
	})
}

func TestLoad_AllowsUnknownRuleScopesWithWarning(t *testing.T) {
	tmpHome := t.TempDir()
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)
	if err := os.Setenv("HOME", tmpHome); err != nil {
		t.Fatalf("Setenv HOME failed: %v", err)
	}

	configDir := filepath.Join(tmpHome, ".typo")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	configFile := filepath.Join(configDir, configFileName)
	data := []byte(`{
  "rules": {
    "rust": { "enabled": false },
    "docker": { "enabled": false }
  }
}`)
	if err := os.WriteFile(configFile, data, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	var cfg *Config
	output := captureConfigStderr(t, func() {
		cfg = Load()
	})

	if cfg == nil {
		t.Fatal("Load() returned nil")
	}
	if cfg.User.Rules["rust"].Enabled {
		t.Fatal("rules.rust.enabled should be false after loading unknown scope")
	}
	if cfg.User.Rules["docker"].Enabled {
		t.Fatal("rules.docker.enabled should stay false after loading config")
	}
	if !strings.Contains(output, "unknown rule scopes") || !strings.Contains(output, "rust") {
		t.Fatalf("expected unknown scope warning, got %q", output)
	}
}

func TestConfigSavePreservesUnknownRuleScopes(t *testing.T) {
	cfg := &Config{ConfigDir: t.TempDir(), User: DefaultUserConfig()}
	cfg.User.Rules["rust"] = RuleSetConfig{Enabled: false}

	if err := cfg.SetValue("keyboard", "colemak"); err != nil {
		t.Fatalf("SetValue() failed: %v", err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	loaded := &Config{ConfigDir: cfg.ConfigDir, User: DefaultUserConfig()}
	output := captureConfigStderr(t, func() {
		loaded.loadUserConfig()
	})

	if loaded.User.Keyboard != "colemak" {
		t.Fatalf("Keyboard after reload = %q, want colemak", loaded.User.Keyboard)
	}
	rule, ok := loaded.User.Rules["rust"]
	if !ok {
		t.Fatal("expected unknown rust scope to be preserved after save/load")
	}
	if rule.Enabled {
		t.Fatal("expected rust scope to remain disabled after save/load")
	}
	if !strings.Contains(output, "rust") {
		t.Fatalf("expected warning mentioning rust scope, got %q", output)
	}
}

func TestConfigGetAndSetValueSupportUnknownPresentRuleScope(t *testing.T) {
	cfg := &Config{ConfigDir: t.TempDir(), User: DefaultUserConfig()}
	cfg.User.Rules["rust"] = RuleSetConfig{Enabled: false}

	value, err := cfg.Get("rules.rust.enabled")
	if err != nil {
		t.Fatalf("Get(rules.rust.enabled) failed: %v", err)
	}
	if value != "false" {
		t.Fatalf("Get(rules.rust.enabled) = %q, want false", value)
	}

	if err := cfg.SetValue("rules.rust.enabled", "true"); err != nil {
		t.Fatalf("SetValue(rules.rust.enabled) failed: %v", err)
	}
	if cfg.User.Rules["rust"] != (RuleSetConfig{Enabled: true}) {
		t.Fatalf("rules.rust.enabled after SetValue = %+v, want enabled", cfg.User.Rules["rust"])
	}
}

func TestParseRuleScopeKey(t *testing.T) {
	if scope, ok := parseRuleScopeKey("rules.git.enabled"); !ok || scope != "git" {
		t.Fatalf("parseRuleScopeKey(valid) = (%q, %v)", scope, ok)
	}

	invalidKeys := []string{
		"rules.git",
		"git.enabled",
		"rules..enabled",
		"rules.git.disabled",
	}
	for _, key := range invalidKeys {
		if scope, ok := parseRuleScopeKey(key); ok || scope != "" {
			t.Fatalf("parseRuleScopeKey(%q) = (%q, %v), want invalid", key, scope, ok)
		}
	}
}

func TestApplyFileConfig(t *testing.T) {
	cfg := DefaultUserConfig()
	threshold := 0.9
	maxEdit := 4
	maxPasses := 50
	keyboard := "colemak"
	historyEnabled := false
	dockerEnabled := false

	applyFileConfig(&cfg, fileUserConfig{
		SimilarityThreshold: &threshold,
		MaxEditDistance:     &maxEdit,
		MaxFixPasses:        &maxPasses,
		Keyboard:            &keyboard,
		History:             &fileHistoryConfig{Enabled: &historyEnabled},
		Rules: map[string]fileRuleConfig{
			"docker": {Enabled: &dockerEnabled},
			"git":    {},
		},
	})

	if cfg.SimilarityThreshold != 0.9 || cfg.MaxEditDistance != 4 || cfg.MaxFixPasses != 50 {
		t.Fatalf("applyFileConfig() numeric fields not applied: %+v", cfg)
	}
	if cfg.Keyboard != "colemak" {
		t.Fatalf("applyFileConfig() keyboard = %q, want colemak", cfg.Keyboard)
	}
	if cfg.History.Enabled {
		t.Fatal("applyFileConfig() should disable history")
	}
	if cfg.Rules["docker"].Enabled {
		t.Fatal("applyFileConfig() should disable docker rule scope")
	}
	if !cfg.Rules["git"].Enabled {
		t.Fatal("applyFileConfig() should leave git rule scope unchanged when enabled is nil")
	}
}
