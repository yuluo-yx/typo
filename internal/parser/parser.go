package parser

// Result represents the result of error parsing.
type Result struct {
	Fixed   bool   // Whether a fix was found
	Command string // The corrected command
	Message string // Optional message to display
}

// Parser defines the interface for error output parsers.
type Parser interface {
	// Name returns the parser name (e.g., "git", "npm")
	Name() string

	// Parse parses the stderr output and returns a correction result.
	// cmd is the original command that was executed.
	// stderr is the error output from the command.
	Parse(cmd, stderr string) Result
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
	return r
}

// Register adds a parser to the registry.
func (r *Registry) Register(p Parser) {
	r.parsers = append(r.parsers, p)
}

// Parse tries all registered parsers and returns the first successful result.
func (r *Registry) Parse(cmd, stderr string) Result {
	for _, p := range r.parsers {
		result := p.Parse(cmd, stderr)
		if result.Fixed {
			return result
		}
	}
	return Result{Fixed: false}
}
