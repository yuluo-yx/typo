package parser

import (
	"regexp"
	"strings"

	itypes "github.com/yuluo-yx/typo/internal/types"
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
	return parserNameNPM
}

// Parse parses npm error output.
func (p *NpmParser) Parse(ctx itypes.ParserContext) itypes.ParserResult {
	cmd := ctx.Command
	stderr := ctx.Stderr

	if !isNpmCommand(cmd) {
		return itypes.ParserResult{Fixed: false}
	}

	// Try "command not found" pattern
	matches := p.notFoundRegex.FindStringSubmatch(stderr)
	if len(matches) >= 2 {
		wrongCmd := matches[1]
		// Try to find "did you mean" suggestion
		suggestMatches := p.didYouMeanRegex.FindStringSubmatch(stderr)
		if len(suggestMatches) >= 2 {
			suggested := suggestMatches[1]
			fixed := ""
			call, err := parseShellCall(cmd)
			if err != nil {
				fixed = strings.Replace(cmd, wrongCmd, suggested, 1)
			} else {
				var ok bool
				fixed, ok = call.replaceSubcommand(parserNameNPM, wrongCmd, suggested, npmParserOptionsWithValues)
				if !ok {
					return itypes.ParserResult{Fixed: false}
				}
			}
			return itypes.ParserResult{
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
		call, err := parseShellCall(cmd)
		if err == nil {
			fixed, ok := call.replaceSubcommand(parserNameNPM, "", suggested, npmParserOptionsWithValues)
			if ok {
				return itypes.ParserResult{
					Fixed:   true,
					Command: fixed,
					Message: "npm suggested: " + suggested,
				}
			}
			return itypes.ParserResult{Fixed: false}
		}

		// Fall back to the original permissive logic when shell parsing fails
		// so interactive fixes still have a chance to recover.
		parts := strings.Fields(cmd)
		if len(parts) >= 2 {
			fixed := parts[0] + " " + suggested
			if len(parts) > 2 {
				fixed += " " + strings.Join(parts[2:], " ")
			}
			return itypes.ParserResult{
				Fixed:   true,
				Command: fixed,
				Message: "npm suggested: " + suggested,
			}
		}
	}

	return itypes.ParserResult{Fixed: false}
}

func isNpmCommand(cmd string) bool {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return false
	}
	return parts[0] == parserNameNPM || strings.HasPrefix(parts[0], npmCommandPrefix)
}
