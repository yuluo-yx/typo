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
