package types

import "time"

// FixResult represents the result of a fix attempt.
type FixResult struct {
	Fixed      bool          // Whether a fix was found.
	Command    string        // The corrected command.
	Source     string        // Where the fix came from, such as history, rule, parser, distance, or subcommand.
	Message    string        // Optional message to display.
	Kind       string        // Internal result tag used for extra handling.
	UsedParser bool          // Whether parser context was consumed in the fix chain.
	Debug      *FixDebugInfo // Optional debug trace for the current fix attempt.
}

// FixDebugInfo carries one `typo fix --debug` trace.
type FixDebugInfo struct {
	InputCommand           string
	AliasContextProvided   bool
	AliasContextUsed       bool
	AliasContextEntries    int
	LoadedPATHCommands     bool
	LoadedPATHCommandCount int
	TotalDuration          time.Duration
	EngineDuration         time.Duration
	UsedAlias              bool
	UsedParser             bool
	UsedHistory            bool
	UsedRule               bool
	UsedCommandTree        bool
	UsedSubcommand         bool
	UsedDistance           bool
	UsedEnv                bool
	UsedOption             bool
	Events                 []FixDebugEvent
	RejectedCandidates     []FixDebugCandidate
	AutoLearn              AutoLearnDebugInfo
}

// FixDebugEvent records one accepted debug step in the fix chain.
type FixDebugEvent struct {
	Pass    int
	Stage   string
	Before  string
	After   string
	Message string
}

// FixDebugCandidate records a candidate that looked promising but was rejected.
type FixDebugCandidate struct {
	Stage      string
	Input      string
	Candidate  string
	Distance   int
	Similarity float64
	Reason     string
}

// AutoLearnDebugInfo describes what happened in the background auto-learn step.
type AutoLearnDebugInfo struct {
	Attempted bool
	Triggered bool
	Persisted bool
	TimedOut  bool
	Duration  time.Duration
	Error     string
	Reason    string
}

// Rule is a stable data contract for a single correction rule.
// Storage, defaulting, matching, and lifecycle behavior remain in the engine package.
type Rule struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Scope  string `json:"scope,omitempty"`
	Enable bool   `json:"enable,omitempty"`
}

// HistoryEntry is a stable data contract for a single correction history entry.
// Recording, lookup, conflict cleanup, sorting, and persistence remain in the engine package.
type HistoryEntry struct {
	From        string `json:"from"`
	To          string `json:"to"`
	Timestamp   int64  `json:"timestamp,omitempty"`
	Count       int    `json:"count,omitempty"`
	RuleApplied bool   `json:"rule_applied,omitempty"`
}
