package parser

import (
	"strings"

	itypes "github.com/yuluo-yx/typo/internal/types"
)

// Registry manages all available parsers.
type Registry struct {
	parsers   []itypes.Parser
	byCommand map[string]parserRoute
}

type parserRoute struct {
	parser itypes.Parser
	index  int
}

// NewRegistry creates a new parser registry with default parsers.
func NewRegistry() *Registry {
	r := &Registry{}
	r.Register(NewGitParser())
	r.Register(NewDockerParser())
	r.Register(NewNpmParser())
	r.Register(NewPermissionParser())
	r.Register(NewGenericParser())
	return r
}

// Register adds a parser to the registry.
func (r *Registry) Register(p itypes.Parser) {
	r.parsers = append(r.parsers, p)
	key := dedicatedParserNameKey(p.Name())
	if key == "" {
		return
	}
	if r.byCommand == nil {
		r.byCommand = make(map[string]parserRoute)
	}
	if _, exists := r.byCommand[key]; !exists {
		r.byCommand[key] = parserRoute{parser: p, index: len(r.parsers) - 1}
	}
}

// Parse tries all registered parsers and returns the first successful result.
func (r *Registry) Parse(ctx itypes.ParserContext) itypes.ParserResult {
	if key := commandParserKey(ctx.Command); key != "" {
		if route, ok := r.byCommand[key]; ok {
			result := route.parser.Parse(ctx)
			if result.Fixed {
				return withParserName(result, route.parser)
			}

			// 专属 parser 只是快速首试；generic 和 permission 仍需保持原始有序兜底。
			return r.parseFallback(ctx, route.index)
		}
	}
	return r.parseFallback(ctx, -1)
}

func (r *Registry) parseFallback(ctx itypes.ParserContext, skipIndex int) itypes.ParserResult {
	for i, p := range r.parsers {
		if i == skipIndex {
			continue
		}
		result := p.Parse(ctx)
		if result.Fixed {
			return withParserName(result, p)
		}
	}
	return itypes.ParserResult{Fixed: false}
}

func withParserName(result itypes.ParserResult, p itypes.Parser) itypes.ParserResult {
	if result.Parser == "" {
		result.Parser = p.Name()
	}
	return result
}

func commandParserKey(command string) string {
	return dedicatedCommandKey(firstCommandWord(command))
}

func dedicatedParserNameKey(name string) string {
	switch name {
	case "git", "docker", "npm":
		return name
	default:
		return ""
	}
}

func dedicatedCommandKey(name string) string {
	switch {
	case name == "git" || strings.HasPrefix(name, "git-"):
		return "git"
	case name == "docker" || strings.HasPrefix(name, "docker-"):
		return "docker"
	case name == "npm" || strings.HasPrefix(name, "npm-"):
		return "npm"
	default:
		return ""
	}
}
