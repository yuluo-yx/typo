package types

// UserConfig represents settings persisted in the user config file.
type UserConfig struct {
	SimilarityThreshold float64                  `json:"similarity_threshold"`
	MaxEditDistance     int                      `json:"max_edit_distance"`
	MaxFixPasses        int                      `json:"max_fix_passes"`
	AutoLearnThreshold  int                      `json:"auto_learn_threshold"`
	Keyboard            string                   `json:"keyboard"`
	History             HistoryConfig            `json:"history"`
	Experimental        ExperimentalConfig       `json:"experimental"`
	Rules               map[string]RuleSetConfig `json:"rules"`
}

// HistoryConfig controls persistence for correction history.
type HistoryConfig struct {
	Enabled bool `json:"enabled"`
}

// ExperimentalConfig groups opt-in experimental behavior switches.
type ExperimentalConfig struct {
	LongOptionCorrection LongOptionCorrectionConfig `json:"long_option_correction"`
}

// LongOptionCorrectionConfig controls experimental long-option typo correction.
type LongOptionCorrectionConfig struct {
	Enabled bool `json:"enabled"`
}

// RuleSetConfig controls whether a single rule set is enabled.
type RuleSetConfig struct {
	Enabled bool `json:"enabled"`
}
