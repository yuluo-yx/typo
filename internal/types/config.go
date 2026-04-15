package types

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
