package types

// ParserContext carries the execution context for a single fix attempt.
type ParserContext struct {
	Command             string
	Stderr              string
	ExitCode            int
	HasMultipleCommands bool
	HasRedirection      bool
	HasPrivilegeWrapper bool
	ShellParseFailed    bool
	AliasContext        []AliasContextEntry
}

// ParserResult represents the result of error parsing.
type ParserResult struct {
	Fixed   bool   // Whether a fix was found.
	Command string // The corrected command.
	Message string // Optional message to display.
	Kind    string // Internal result tag used to distinguish fix categories.
	Parser  string // The parser that produced the result, when available.
}

// AliasContextEntry describes one shell-local correction context entry captured by shell integration.
// Kind can be alias-like command shorthands or environment variable names.
type AliasContextEntry struct {
	Shell     string
	Kind      string
	Name      string
	Expansion string
}

const (
	// FixKindPermissionSudo identifies fixes that prepend sudo after a permission error.
	FixKindPermissionSudo = "permission_sudo"
)

// Parser defines the interface for error output parsers.
type Parser interface {
	// Name returns the parser name, such as "git" or "npm".
	Name() string

	// Parse parses stderr output and returns a correction result.
	Parse(ctx ParserContext) ParserResult
}
