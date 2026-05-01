package parser

import (
	"regexp"
	"strings"

	itypes "github.com/yuluo-yx/typo/internal/types"
)

// DockerParser parses docker command errors.
type DockerParser struct {
	didYouMeanRegex *regexp.Regexp
	notFoundRegex   *regexp.Regexp
}

// NewDockerParser creates a new DockerParser.
func NewDockerParser() *DockerParser {
	return &DockerParser{
		didYouMeanRegex: regexp.MustCompile(`(?s)docker: '([^']+)' is not a docker command\..*Similar command:\s+(\w+)`),
		notFoundRegex:   regexp.MustCompile(`(?s)unknown command:\s+([^\s]+).*Did you mean:\s+(\w+)`),
	}
}

// Name returns the parser name.
func (p *DockerParser) Name() string {
	return parserNameDocker
}

// Parse parses docker error output.
func (p *DockerParser) Parse(ctx itypes.ParserContext) itypes.ParserResult {
	cmd := ctx.Command
	stderr := ctx.Stderr

	if !isDockerCommand(cmd) {
		return itypes.ParserResult{Fixed: false}
	}

	// Try "did you mean" pattern
	matches := p.didYouMeanRegex.FindStringSubmatch(stderr)
	if len(matches) >= 3 {
		wrongCmd := matches[1]
		suggested := matches[2]
		fixed := ""
		call, err := parseShellCall(cmd)
		if err != nil {
			fixed = strings.Replace(cmd, wrongCmd, suggested, 1)
		} else {
			var ok bool
			fixed, ok = call.replaceSubcommand(parserNameDocker, wrongCmd, suggested, dockerParserOptionsWithValues)
			if !ok {
				return itypes.ParserResult{Fixed: false}
			}
		}
		return itypes.ParserResult{
			Fixed:   true,
			Command: fixed,
			Message: "docker suggested: " + suggested,
		}
	}

	// Try alternative "unknown command" pattern
	matches = p.notFoundRegex.FindStringSubmatch(stderr)
	if len(matches) >= 3 {
		wrongCmd := matches[1]
		suggested := matches[2]
		fixed := ""
		call, err := parseShellCall(cmd)
		if err != nil {
			fixed = strings.Replace(cmd, wrongCmd, suggested, 1)
		} else {
			var ok bool
			fixed, ok = call.replaceSubcommand(parserNameDocker, wrongCmd, suggested, dockerParserOptionsWithValues)
			if !ok {
				return itypes.ParserResult{Fixed: false}
			}
		}
		return itypes.ParserResult{
			Fixed:   true,
			Command: fixed,
			Message: "docker suggested: " + suggested,
		}
	}

	return itypes.ParserResult{Fixed: false}
}

func isDockerCommand(cmd string) bool {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return false
	}
	return parts[0] == parserNameDocker || strings.HasPrefix(parts[0], dockerCommandPrefix)
}
