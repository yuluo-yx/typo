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
	itypes "github.com/yuluo-yx/typo/internal/types"
)

const (
	configFileName             = "config.json"
	defaultSimilarityThreshold = 0.6
	defaultMaxEditDistance     = 2
	defaultMaxFixPasses        = 32
	defaultAutoLearnThreshold  = 3
	defaultKeyboardLayout      = "qwerty"
	minSimilarityThreshold     = 0.0
	maxSimilarityThreshold     = 1.0
	minMaxEditDistance         = 0
	minMaxFixPasses            = 1
	minAutoLearnThreshold      = 0
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
	User      itypes.UserConfig
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
	AutoLearnThreshold  *int                      `json:"auto_learn_threshold,omitempty"`
	Keyboard            *string                   `json:"keyboard,omitempty"`
	History             *fileHistoryConfig        `json:"history,omitempty"`
	Experimental        *fileExperimentalConfig   `json:"experimental,omitempty"`
	Rules               map[string]fileRuleConfig `json:"rules,omitempty"`
}

type fileHistoryConfig struct {
	Enabled *bool `json:"enabled,omitempty"`
}

type fileExperimentalConfig struct {
	LongOptionCorrection *fileLongOptionCorrectionConfig `json:"long_option_correction,omitempty"`
}

type fileLongOptionCorrectionConfig struct {
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
func DefaultUserConfig() itypes.UserConfig {
	rules := make(map[string]itypes.RuleSetConfig, len(defaultRuleScopes))
	for _, scope := range defaultRuleScopes {
		rules[scope] = itypes.RuleSetConfig{Enabled: true}
	}

	return itypes.UserConfig{
		SimilarityThreshold: defaultSimilarityThreshold,
		MaxEditDistance:     defaultMaxEditDistance,
		MaxFixPasses:        defaultMaxFixPasses,
		AutoLearnThreshold:  defaultAutoLearnThreshold,
		Keyboard:            defaultKeyboardLayout,
		History:             itypes.HistoryConfig{Enabled: true},
		Experimental: itypes.ExperimentalConfig{
			LongOptionCorrection: itypes.LongOptionCorrectionConfig{Enabled: false},
		},
		Rules: rules,
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
	return c.saveUserConfig(c.User)
}

// Reset restores the user config to defaults and writes it back to disk.
func (c *Config) Reset() error {
	next := DefaultUserConfig()
	if err := c.saveUserConfig(next); err != nil {
		return err
	}
	c.User = next
	return nil
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

	next := DefaultUserConfig()
	if err := c.saveUserConfig(next); err != nil {
		return err
	}
	c.User = next
	return nil
}

// ListSettings returns config items for `typo config list`.
func (c *Config) ListSettings() []Setting {
	settings := make([]Setting, 0, 7+len(c.User.Rules))
	settings = append(settings,
		Setting{Key: "similarity-threshold", Value: formatFloat(c.User.SimilarityThreshold)},
		Setting{Key: "max-edit-distance", Value: strconv.Itoa(c.User.MaxEditDistance)},
		Setting{Key: "max-fix-passes", Value: strconv.Itoa(c.User.MaxFixPasses)},
		Setting{Key: "auto-learn-threshold", Value: strconv.Itoa(c.User.AutoLearnThreshold)},
		Setting{Key: "keyboard", Value: c.User.Keyboard},
		Setting{Key: "history.enabled", Value: strconv.FormatBool(c.User.History.Enabled)},
		Setting{
			Key:   "experimental.long-option-correction.enabled",
			Value: strconv.FormatBool(c.User.Experimental.LongOptionCorrection.Enabled),
		},
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
	case "auto-learn-threshold":
		return strconv.Itoa(c.User.AutoLearnThreshold), nil
	case "keyboard":
		return c.User.Keyboard, nil
	case "history.enabled":
		return strconv.FormatBool(c.User.History.Enabled), nil
	case "experimental.long-option-correction.enabled":
		return strconv.FormatBool(c.User.Experimental.LongOptionCorrection.Enabled), nil
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
	next, err := c.updatedUserConfig(key, value)
	if err != nil {
		return err
	}
	if err := c.saveUserConfig(next); err != nil {
		return err
	}
	c.User = next
	return nil
}

// SetValue updates the given config key in memory without persisting it.
func (c *Config) SetValue(key, value string) error {
	next, err := c.updatedUserConfig(key, value)
	if err != nil {
		return err
	}
	c.User = next
	return nil
}

func (c *Config) updatedUserConfig(key, value string) (itypes.UserConfig, error) {
	next := cloneUserConfig(c.User)
	if err := applyUserConfigValue(&next, key, value); err != nil {
		return itypes.UserConfig{}, err
	}

	if err := ValidateUserConfig(next); err != nil {
		return itypes.UserConfig{}, err
	}

	return next, nil
}

func applyUserConfigValue(next *itypes.UserConfig, key, value string) error {
	switch key {
	case "similarity-threshold":
		parsed, err := parseConfigFloatValue(value, key)
		if err != nil {
			return err
		}
		next.SimilarityThreshold = parsed
	case "max-edit-distance":
		parsed, err := parseConfigIntValue(value, key)
		if err != nil {
			return err
		}
		next.MaxEditDistance = parsed
	case "max-fix-passes":
		parsed, err := parseConfigIntValue(value, key)
		if err != nil {
			return err
		}
		next.MaxFixPasses = parsed
	case "auto-learn-threshold":
		parsed, err := parseConfigIntValue(value, key)
		if err != nil {
			return err
		}
		next.AutoLearnThreshold = parsed
	case "keyboard":
		if err := applyKeyboardLayoutValue(next, value); err != nil {
			return err
		}
	case "history.enabled":
		parsed, err := parseConfigBoolValue(value, key)
		if err != nil {
			return err
		}
		next.History.Enabled = parsed
	case "experimental.long-option-correction.enabled":
		parsed, err := parseConfigBoolValue(value, key)
		if err != nil {
			return err
		}
		next.Experimental.LongOptionCorrection.Enabled = parsed
	default:
		return applyRuleScopeValue(next, key, value)
	}

	return nil
}

func parseConfigFloatValue(value, key string) (float64, error) {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid float value %q for %s", value, key)
	}
	return parsed, nil
}

func parseConfigIntValue(value, key string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid int value %q for %s", value, key)
	}
	return parsed, nil
}

func parseConfigBoolValue(value, key string) (bool, error) {
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("invalid bool value %q for %s", value, key)
	}
	return parsed, nil
}

func applyKeyboardLayoutValue(next *itypes.UserConfig, value string) error {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if !supportedKeyboardLayouts[normalized] {
		return fmt.Errorf("unsupported keyboard layout: %s", normalized)
	}
	next.Keyboard = normalized
	return nil
}

func applyRuleScopeValue(next *itypes.UserConfig, key, value string) error {
	scope, ok := parseRuleScopeKey(key)
	if !ok {
		return fmt.Errorf("unknown config key: %s", key)
	}

	parsed, err := parseConfigBoolValue(value, key)
	if err != nil {
		return err
	}
	if _, exists := next.Rules[scope]; !exists {
		return fmt.Errorf("unknown rule scope: %s", scope)
	}
	next.Rules[scope] = itypes.RuleSetConfig{Enabled: parsed}
	return nil
}

// ValidateUserConfig checks whether the user config matches allowed ranges and known enums.
func ValidateUserConfig(u itypes.UserConfig) error {
	if u.SimilarityThreshold < minSimilarityThreshold || u.SimilarityThreshold > maxSimilarityThreshold {
		return fmt.Errorf("similarity_threshold must be between %.1f and %.1f", minSimilarityThreshold, maxSimilarityThreshold)
	}
	if u.MaxEditDistance < minMaxEditDistance {
		return fmt.Errorf("max_edit_distance must be >= %d", minMaxEditDistance)
	}
	if u.MaxFixPasses < minMaxFixPasses {
		return fmt.Errorf("max_fix_passes must be >= %d", minMaxFixPasses)
	}
	if u.AutoLearnThreshold < minAutoLearnThreshold {
		return fmt.Errorf("auto_learn_threshold must be >= %d", minAutoLearnThreshold)
	}

	keyboard := strings.ToLower(strings.TrimSpace(u.Keyboard))
	if !supportedKeyboardLayouts[keyboard] {
		return fmt.Errorf("keyboard must be one of: %s", strings.Join(sortedKeyboardLayouts(), ", "))
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
	if err := ValidateUserConfig(userCfg); err != nil {
		fmt.Fprintf(os.Stderr, "typo: invalid config file %s: %v\n", configFile, err)
		return
	}
	if unknownScopes := unknownRuleScopes(userCfg.Rules); len(unknownScopes) > 0 {
		fmt.Fprintf(
			os.Stderr,
			"typo: config file %s contains unknown rule scopes that will be preserved but ignored by this version: %s\n",
			configFile,
			strings.Join(unknownScopes, ", "),
		)
	}

	c.User = userCfg
}

func applyFileConfig(dst *itypes.UserConfig, src fileUserConfig) {
	if src.SimilarityThreshold != nil {
		dst.SimilarityThreshold = *src.SimilarityThreshold
	}
	if src.MaxEditDistance != nil {
		dst.MaxEditDistance = *src.MaxEditDistance
	}
	if src.MaxFixPasses != nil {
		dst.MaxFixPasses = *src.MaxFixPasses
	}
	if src.AutoLearnThreshold != nil {
		dst.AutoLearnThreshold = *src.AutoLearnThreshold
	}
	if src.Keyboard != nil {
		dst.Keyboard = strings.ToLower(strings.TrimSpace(*src.Keyboard))
	}
	if src.History != nil && src.History.Enabled != nil {
		dst.History.Enabled = *src.History.Enabled
	}
	if src.Experimental != nil &&
		src.Experimental.LongOptionCorrection != nil &&
		src.Experimental.LongOptionCorrection.Enabled != nil {
		dst.Experimental.LongOptionCorrection.Enabled = *src.Experimental.LongOptionCorrection.Enabled
	}
	for scope, rule := range src.Rules {
		if rule.Enabled == nil {
			continue
		}
		dst.Rules[scope] = itypes.RuleSetConfig{Enabled: *rule.Enabled}
	}
}

func (c *Config) saveUserConfig(user itypes.UserConfig) error {
	if err := ValidateUserConfig(user); err != nil {
		return err
	}
	if err := c.EnsureConfigDir(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(user, "", "  ")
	if err != nil {
		return err
	}

	configFile := c.ConfigFilePath()
	if configFile == "" {
		return nil
	}
	return storage.WriteFileAtomic(configFile, data, 0600)
}

func cloneUserConfig(src itypes.UserConfig) itypes.UserConfig {
	dst := src
	dst.Rules = cloneRuleSetConfigs(src.Rules)
	return dst
}

func cloneRuleSetConfigs(src map[string]itypes.RuleSetConfig) map[string]itypes.RuleSetConfig {
	if src == nil {
		return nil
	}

	dst := make(map[string]itypes.RuleSetConfig, len(src))
	for scope, rule := range src {
		dst[scope] = rule
	}
	return dst
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

func unknownRuleScopes(rules map[string]itypes.RuleSetConfig) []string {
	scopes := make([]string, 0)
	for scope := range rules {
		if !isKnownRuleScope(scope) {
			scopes = append(scopes, scope)
		}
	}
	sort.Strings(scopes)
	return scopes
}

func sortedRuleScopes(rules map[string]itypes.RuleSetConfig) []string {
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
