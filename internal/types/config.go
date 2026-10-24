package types

// UserConfig represents settings persisted in the user config file.
type UserConfig struct {
	SimilarityThreshold float64                  `json:"similarity_threshold"`
	MaxEditDistance     int                      `json:"max_edit_distance"`
	MaxFixPasses        int                      `json:"max_fix_passes"`
	AutoLearnThreshold  int                      `json:"auto_learn_threshold"`
	Keyboard            string                   `json:"keyboard"`
	History             HistoryConfig            `json:"history"`
	Candidates          CandidateConfig          `json:"candidates"`
	Paths               PathConfig               `json:"paths"`
	Experimental        ExperimentalConfig       `json:"experimental"`
	Subcommands         SubcommandConfig         `json:"subcommands"`
	Rules               map[string]RuleSetConfig `json:"rules"`
}

// HistoryConfig controls persistence for correction history.
type HistoryConfig struct {
	Enabled bool `json:"enabled"`
}

// CandidateConfig controls interactive correction candidate selection.
type CandidateConfig struct {
	Enabled bool `json:"enabled"`
	Limit   int  `json:"limit"`
	// FZF renders the candidate menu with an installed fzf binary; the builtin menu is used otherwise.
	FZF bool `json:"fzf"`
}

// PathConfig controls correction of missing path arguments.
type PathConfig struct {
	Enabled bool `json:"enabled"`
}

// SubcommandConfig controls dynamic subcommand discovery through tool help output.
type SubcommandConfig struct {
	// DiscoverUnknown allows running `<tool> --help` for tools outside the builtin allowlist.
	DiscoverUnknown bool `json:"discover_unknown"`
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
