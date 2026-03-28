package parser

// Context carries the execution context for a single fix attempt.
type Context struct {
	Command             string
	Stderr              string
	ExitCode            int
	HasMultipleCommands bool
	HasRedirection      bool
	HasPrivilegeWrapper bool
	ShellParseFailed    bool
}

// Result represents the result of error parsing.
type Result struct {
	Fixed   bool   // Whether a fix was found
	Command string // The corrected command
	Message string // Optional message to display
	Kind    string // Internal result tag used to distinguish fix categories.
}

const (
	// ResultKindPermissionSudo identifies fixes that prepend sudo after a permission error.
	ResultKindPermissionSudo = "permission_sudo"
)

// Parser defines the interface for error output parsers.
type Parser interface {
	// Name returns the parser name (e.g., "git", "npm")
	Name() string

	// Parse parses the stderr output and returns a correction result.
	Parse(ctx Context) Result
}

// Registry manages all available parsers.
type Registry struct {
	parsers []Parser
}

// NewRegistry creates a new parser registry with default parsers.
func NewRegistry() *Registry {
	r := &Registry{}
	r.Register(NewGitParser())
	r.Register(NewDockerParser())
	r.Register(NewNpmParser())
	r.Register(NewPermissionParser())
	return r
}

// Register adds a parser to the registry.
func (r *Registry) Register(p Parser) {
	r.parsers = append(r.parsers, p)
}

// Parse tries all registered parsers and returns the first successful result.
func (r *Registry) Parse(ctx Context) Result {
	for _, p := range r.parsers {
		result := p.Parse(ctx)
		if result.Fixed {
			return result
		}
	}
	return Result{Fixed: false}
}
