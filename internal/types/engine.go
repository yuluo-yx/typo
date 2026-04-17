package types

// FixResult represents the result of a fix attempt.
type FixResult struct {
	Fixed      bool   // Whether a fix was found.
	Command    string // The corrected command.
	Source     string // Where the fix came from, such as history, rule, parser, distance, or subcommand.
	Message    string // Optional message to display.
	Kind       string // Internal result tag used for extra handling.
	UsedParser bool   // Whether parser context was consumed in the fix chain.
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
	From      string `json:"from"`
	To        string `json:"to"`
	Timestamp int64  `json:"timestamp,omitempty"`
	Count     int    `json:"count,omitempty"`
}
