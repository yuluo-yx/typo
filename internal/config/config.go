package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/yuluo-yx/typo/internal/storage"
)

const (
	configFileName             = "config.json"
	defaultSimilarityThreshold = 0.6
	defaultMaxEditDistance     = 2
	defaultMaxFixPasses        = 32
	defaultKeyboardLayout      = "qwerty"
	minSimilarityThreshold     = 0.0
	maxSimilarityThreshold     = 1.0
	minMaxEditDistance         = 0
	minMaxFixPasses            = 1
)

var (
	defaultRuleScopes = []string{
		"git",
		"docker",
		"npm",
		"yarn",
		"kubectl",
		"cargo",
		"brew",
		"helm",
		"terraform",
		"python",
		"pip",
		"go",
		"java",
		"system",
	}
	supportedKeyboardLayouts = map[string]bool{
		"qwerty":  true,
		"dvorak":  true,
		"colemak": true,
	}
)

// Config represents Typo runtime settings and the local config directory.
type Config struct {
	ConfigDir string
	Debug     bool
	User      UserConfig
}

// UserConfig represents settings persisted in the user config file.
type UserConfig struct {
	SimilarityThreshold float64                  `json:"similarity_threshold"`
	MaxEditDistance     int                      `json:"max_edit_distance"`
	MaxFixPasses        int                      `json:"max_fix_passes"`
	Keyboard            string                   `json:"keyboard"`
	History             HistoryConfig            `json:"history"`
	Rules               map[string]RuleSetConfig `json:"rules"`
}

// HistoryConfig controls persistence for correction history.
type HistoryConfig struct {
	Enabled bool `json:"enabled"`
}

// RuleSetConfig controls whether a single rule set is enabled.
type RuleSetConfig struct {
	Enabled bool `json:"enabled"`
}

// Setting represents one config item displayed by the CLI.
type Setting struct {
	Key   string
	Value string
}

type fileUserConfig struct {
	SimilarityThreshold *float64                  `json:"similarity_threshold,omitempty"`
	MaxEditDistance     *int                      `json:"max_edit_distance,omitempty"`
	MaxFixPasses        *int                      `json:"max_fix_passes,omitempty"`
	Keyboard            *string                   `json:"keyboard,omitempty"`
	History             *fileHistoryConfig        `json:"history,omitempty"`
	Rules               map[string]fileRuleConfig `json:"rules,omitempty"`
}

type fileHistoryConfig struct {
	Enabled *bool `json:"enabled,omitempty"`
}

type fileRuleConfig struct {
	Enabled *bool `json:"enabled,omitempty"`
}

// DefaultConfigDir returns the default config directory path.
func DefaultConfigDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".typo")
}

// DefaultUserConfig returns the built-in default user config.
func DefaultUserConfig() UserConfig {
	rules := make(map[string]RuleSetConfig, len(defaultRuleScopes))
	for _, scope := range defaultRuleScopes {
		rules[scope] = RuleSetConfig{Enabled: true}
	}

	return UserConfig{
		SimilarityThreshold: defaultSimilarityThreshold,
		MaxEditDistance:     defaultMaxEditDistance,
		MaxFixPasses:        defaultMaxFixPasses,
		Keyboard:            defaultKeyboardLayout,
		History:             HistoryConfig{Enabled: true},
		Rules:               rules,
	}
}

// Load loads and merges the default config with local user config.
func Load() *Config {
	cfg := &Config{
		ConfigDir: DefaultConfigDir(),
		User:      DefaultUserConfig(),
	}
	cfg.loadUserConfig()
	return cfg
}

// EnsureConfigDir makes sure the config directory exists.
func (c *Config) EnsureConfigDir() error {
	if c.ConfigDir == "" {
		return nil
	}
	return os.MkdirAll(c.ConfigDir, 0755)
}

// ConfigFilePath returns the absolute path to the config file.
func (c *Config) ConfigFilePath() string {
	if c.ConfigDir == "" {
		return ""
	}
	return filepath.Join(c.ConfigDir, configFileName)
}

// Save validates and writes the current user config to disk.
func (c *Config) Save() error {
	if err := c.User.Validate(); err != nil {
		return err
	}
	if err := c.EnsureConfigDir(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c.User, "", "  ")
	if err != nil {
		return err
	}

	configFile := c.ConfigFilePath()
	if configFile == "" {
		return nil
	}
	return storage.WriteFileAtomic(configFile, data, 0600)
}

// Reset restores the user config to defaults and writes it back to disk.
func (c *Config) Reset() error {
	c.User = DefaultUserConfig()
	return c.Save()
}

// Generate creates a default config file at the target location.
func (c *Config) Generate(force bool) error {
	configFile := c.ConfigFilePath()
	if configFile == "" {
		return nil
	}

	if !force {
		if _, err := os.Stat(configFile); err == nil {
			return fmt.Errorf("config already exists: %s (use --force to overwrite)", configFile)
		} else if !os.IsNotExist(err) {
			return err
		}
	}

	c.User = DefaultUserConfig()
	return c.Save()
}

// ListSettings returns config items for `typo config list`.
func (c *Config) ListSettings() []Setting {
	settings := make([]Setting, 0, 5+len(c.User.Rules))
	settings = append(settings,
		Setting{Key: "similarity-threshold", Value: formatFloat(c.User.SimilarityThreshold)},
		Setting{Key: "max-edit-distance", Value: strconv.Itoa(c.User.MaxEditDistance)},
		Setting{Key: "max-fix-passes", Value: strconv.Itoa(c.User.MaxFixPasses)},
		Setting{Key: "keyboard", Value: c.User.Keyboard},
		Setting{Key: "history.enabled", Value: strconv.FormatBool(c.User.History.Enabled)},
	)

	for _, scope := range sortedRuleScopes(c.User.Rules) {
		settings = append(settings, Setting{
			Key:   fmt.Sprintf("rules.%s.enabled", scope),
			Value: strconv.FormatBool(c.User.Rules[scope].Enabled),
		})
	}

	return settings
}

// Get reads the string value for the given config key.
func (c *Config) Get(key string) (string, error) {
	switch key {
	case "similarity-threshold":
		return formatFloat(c.User.SimilarityThreshold), nil
	case "max-edit-distance":
		return strconv.Itoa(c.User.MaxEditDistance), nil
	case "max-fix-passes":
		return strconv.Itoa(c.User.MaxFixPasses), nil
	case "keyboard":
		return c.User.Keyboard, nil
	case "history.enabled":
		return strconv.FormatBool(c.User.History.Enabled), nil
	default:
		scope, ok := parseRuleScopeKey(key)
		if !ok {
			return "", fmt.Errorf("unknown config key: %s", key)
		}
		rule, exists := c.User.Rules[scope]
		if !exists {
			return "", fmt.Errorf("unknown rule scope: %s", scope)
		}
		return strconv.FormatBool(rule.Enabled), nil
	}
}

// Set updates the given config key and persists it to disk.
func (c *Config) Set(key, value string) error {
	switch key {
	case "similarity-threshold":
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid float value %q for %s", value, key)
		}
		c.User.SimilarityThreshold = parsed
	case "max-edit-distance":
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid int value %q for %s", value, key)
		}
		c.User.MaxEditDistance = parsed
	case "max-fix-passes":
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid int value %q for %s", value, key)
		}
		c.User.MaxFixPasses = parsed
	case "keyboard":
		normalized := strings.ToLower(strings.TrimSpace(value))
		if !supportedKeyboardLayouts[normalized] {
			return fmt.Errorf("unsupported keyboard layout: %s", normalized)
		}
		c.User.Keyboard = normalized
	case "history.enabled":
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid bool value %q for %s", value, key)
		}
		c.User.History.Enabled = parsed
	default:
		scope, ok := parseRuleScopeKey(key)
		if !ok {
			return fmt.Errorf("unknown config key: %s", key)
		}
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid bool value %q for %s", value, key)
		}
		if _, exists := c.User.Rules[scope]; !exists {
			return fmt.Errorf("unknown rule scope: %s", scope)
		}
		c.User.Rules[scope] = RuleSetConfig{Enabled: parsed}
	}

	return c.Save()
}

// Validate checks whether the user config matches allowed ranges and known enums.
func (u UserConfig) Validate() error {
	if u.SimilarityThreshold < minSimilarityThreshold || u.SimilarityThreshold > maxSimilarityThreshold {
		return fmt.Errorf("similarity_threshold must be between %.1f and %.1f", minSimilarityThreshold, maxSimilarityThreshold)
	}
	if u.MaxEditDistance < minMaxEditDistance {
		return fmt.Errorf("max_edit_distance must be >= %d", minMaxEditDistance)
	}
	if u.MaxFixPasses < minMaxFixPasses {
		return fmt.Errorf("max_fix_passes must be >= %d", minMaxFixPasses)
	}

	keyboard := strings.ToLower(strings.TrimSpace(u.Keyboard))
	if !supportedKeyboardLayouts[keyboard] {
		return fmt.Errorf("keyboard must be one of: %s", strings.Join(sortedKeyboardLayouts(), ", "))
	}

	for scope := range u.Rules {
		if !isKnownRuleScope(scope) {
			return fmt.Errorf("unknown rule scope: %s", scope)
		}
	}

	return nil
}

func (c *Config) loadUserConfig() {
	configFile := c.ConfigFilePath()
	if configFile == "" {
		return
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return
	}

	var fileCfg fileUserConfig
	if err := json.Unmarshal(data, &fileCfg); err != nil {
		storage.QuarantineInvalidJSON(configFile, err)
		return
	}

	userCfg := DefaultUserConfig()
	applyFileConfig(&userCfg, fileCfg)
	if err := userCfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "typo: invalid config file %s: %v\n", configFile, err)
		return
	}

	c.User = userCfg
}

func applyFileConfig(dst *UserConfig, src fileUserConfig) {
	if src.SimilarityThreshold != nil {
		dst.SimilarityThreshold = *src.SimilarityThreshold
	}
	if src.MaxEditDistance != nil {
		dst.MaxEditDistance = *src.MaxEditDistance
	}
	if src.MaxFixPasses != nil {
		dst.MaxFixPasses = *src.MaxFixPasses
	}
	if src.Keyboard != nil {
		dst.Keyboard = strings.ToLower(strings.TrimSpace(*src.Keyboard))
	}
	if src.History != nil && src.History.Enabled != nil {
		dst.History.Enabled = *src.History.Enabled
	}
	for scope, rule := range src.Rules {
		if rule.Enabled == nil {
			continue
		}
		dst.Rules[scope] = RuleSetConfig{Enabled: *rule.Enabled}
	}
}

func parseRuleScopeKey(key string) (string, bool) {
	if !strings.HasPrefix(key, "rules.") || !strings.HasSuffix(key, ".enabled") {
		return "", false
	}
	scope := strings.TrimSuffix(strings.TrimPrefix(key, "rules."), ".enabled")
	if scope == "" {
		return "", false
	}
	return scope, true
}

func isKnownRuleScope(scope string) bool {
	for _, known := range defaultRuleScopes {
		if known == scope {
			return true
		}
	}
	return false
}

func sortedRuleScopes(rules map[string]RuleSetConfig) []string {
	scopes := make([]string, 0, len(rules))
	for scope := range rules {
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)
	return scopes
}

func sortedKeyboardLayouts() []string {
	layouts := make([]string, 0, len(supportedKeyboardLayouts))
	for name := range supportedKeyboardLayouts {
		layouts = append(layouts, name)
	}
	sort.Strings(layouts)
	return layouts
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
