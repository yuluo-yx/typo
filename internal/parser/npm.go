package parser

import (
	"regexp"
	"strings"
)

// NpmParser parses npm command errors.
type NpmParser struct {
	notFoundRegex   *regexp.Regexp
	didYouMeanRegex *regexp.Regexp
}

// NewNpmParser creates a new NpmParser.
func NewNpmParser() *NpmParser {
	return &NpmParser{
		notFoundRegex:   regexp.MustCompile(`(?s)npm ERR!.*command\s+([^\s]+)\s+not found`),
		didYouMeanRegex: regexp.MustCompile(`(?s)npm ERR!.*Did you mean\s+(\w+)`),
	}
}

// Name returns the parser name.
func (p *NpmParser) Name() string {
	return "npm"
}

// Parse parses npm error output.
func (p *NpmParser) Parse(cmd, stderr string) Result {
	if !isNpmCommand(cmd) {
		return Result{Fixed: false}
	}

	// Try "command not found" pattern
	matches := p.notFoundRegex.FindStringSubmatch(stderr)
	if len(matches) >= 2 {
		wrongCmd := matches[1]
		// Try to find "did you mean" suggestion
		suggestMatches := p.didYouMeanRegex.FindStringSubmatch(stderr)
		if len(suggestMatches) >= 2 {
			suggested := suggestMatches[1]
			fixed := strings.Replace(cmd, wrongCmd, suggested, 1)
			return Result{
				Fixed:   true,
				Command: fixed,
				Message: "npm suggested: " + suggested,
			}
		}
	}

	// Try just "did you mean" pattern
	matches = p.didYouMeanRegex.FindStringSubmatch(stderr)
	if len(matches) >= 2 {
		suggested := matches[1]
		// Replace the second word in command (the npm subcommand)
		parts := strings.Fields(cmd)
		if len(parts) >= 2 {
			fixed := parts[0] + " " + suggested
			if len(parts) > 2 {
				fixed += " " + strings.Join(parts[2:], " ")
			}
			return Result{
				Fixed:   true,
				Command: fixed,
				Message: "npm suggested: " + suggested,
			}
		}
	}

	return Result{Fixed: false}
}

func isNpmCommand(cmd string) bool {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return false
	}
	return parts[0] == "npm" || strings.HasPrefix(parts[0], "npm-")
}
