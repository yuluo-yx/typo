package parser

import itypes "github.com/yuluo-yx/typo/internal/types"

// Registry manages all available parsers.
type Registry struct {
	parsers []itypes.Parser
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
func (r *Registry) Register(p itypes.Parser) {
	r.parsers = append(r.parsers, p)
}

// Parse tries all registered parsers and returns the first successful result.
func (r *Registry) Parse(ctx itypes.ParserContext) itypes.ParserResult {
	for _, p := range r.parsers {
		result := p.Parse(ctx)
		if result.Fixed {
			return result
		}
	}
	return itypes.ParserResult{Fixed: false}
}
