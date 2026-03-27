package parser

import (
	"regexp"
	"strings"
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
	return "docker"
}

// Parse parses docker error output.
func (p *DockerParser) Parse(ctx Context) Result {
	cmd := ctx.Command
	stderr := ctx.Stderr

	if !isDockerCommand(cmd) {
		return Result{Fixed: false}
	}

	// Try "did you mean" pattern
	matches := p.didYouMeanRegex.FindStringSubmatch(stderr)
	if len(matches) >= 3 {
		wrongCmd := matches[1]
		suggested := matches[2]
		fixed := strings.Replace(cmd, wrongCmd, suggested, 1)
		return Result{
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
		fixed := strings.Replace(cmd, wrongCmd, suggested, 1)
		return Result{
			Fixed:   true,
			Command: fixed,
			Message: "docker suggested: " + suggested,
		}
	}

	return Result{Fixed: false}
}

func isDockerCommand(cmd string) bool {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return false
	}
	return parts[0] == "docker" || strings.HasPrefix(parts[0], "docker-")
}
