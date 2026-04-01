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

// Config 表示 typo 的运行时配置与本地配置目录。
type Config struct {
	ConfigDir string
	Debug     bool
	User      UserConfig
}

// UserConfig 表示持久化到用户配置文件中的设置。
type UserConfig struct {
	SimilarityThreshold float64                  `json:"similarity_threshold"`
	MaxEditDistance     int                      `json:"max_edit_distance"`
	MaxFixPasses        int                      `json:"max_fix_passes"`
	Keyboard            string                   `json:"keyboard"`
	History             HistoryConfig            `json:"history"`
	Rules               map[string]RuleSetConfig `json:"rules"`
}

// HistoryConfig 控制纠错历史记录的持久化行为。
type HistoryConfig struct {
	Enabled bool `json:"enabled"`
}

// RuleSetConfig 控制单个规则集的启用状态。
type RuleSetConfig struct {
	Enabled bool `json:"enabled"`
}

// Setting 表示用于 CLI 展示的单条配置项。
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

// DefaultConfigDir 返回默认的配置目录路径。
func DefaultConfigDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".typo")
}

// DefaultUserConfig 返回内置默认用户配置。
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

// Load 加载并合并默认配置与本地用户配置。
func Load() *Config {
	cfg := &Config{
		ConfigDir: DefaultConfigDir(),
		User:      DefaultUserConfig(),
	}
	cfg.loadUserConfig()
	return cfg
}

// EnsureConfigDir 确保配置目录存在。
func (c *Config) EnsureConfigDir() error {
	if c.ConfigDir == "" {
		return nil
	}
	return os.MkdirAll(c.ConfigDir, 0755)
}

// ConfigFilePath 返回配置文件的绝对路径。
func (c *Config) ConfigFilePath() string {
	if c.ConfigDir == "" {
		return ""
	}
	return filepath.Join(c.ConfigDir, configFileName)
}

// Save 校验并将当前用户配置写入磁盘。
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

// Reset 将用户配置恢复为默认值并写回磁盘。
func (c *Config) Reset() error {
	c.User = DefaultUserConfig()
	return c.Save()
}

// Generate 在目标位置生成默认配置文件。
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

// ListSettings 返回用于 `typo config list` 输出的配置项列表。
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

// Get 读取指定配置键对应的字符串值。
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

// Set 更新指定配置键并持久化到磁盘。
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
		c.User.Keyboard = strings.ToLower(strings.TrimSpace(value))
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

// Validate 检查用户配置是否满足允许范围与已知枚举。
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
