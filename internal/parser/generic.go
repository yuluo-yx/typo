package parser

import (
	"regexp"
	"strings"

	itypes "github.com/yuluo-yx/typo/internal/types"
)

// GenericParser catches "did you mean" hints emitted by any CLI, covering tools
// that do not have a dedicated parser (e.g. rustup, cargo, helm, gh, kubectl,
// pnpm, poetry, pip).
type GenericParser struct {
	// inlineRegex matches suggestions on the same line as the hint phrase,
	// e.g. "Did you mean 'target'?" or "maybe you meant `build`".
	inlineRegex *regexp.Regexp
	// nextLineRegex matches suggestions that appear on the line immediately
	// following "Did you mean this?" or "Did you mean one of these?",
	// e.g. helm, gh, kubectl, poetry.
	nextLineRegex *regexp.Regexp
}

// NewGenericParser creates a new GenericParser.
func NewGenericParser() *GenericParser {
	return &GenericParser{
		inlineRegex: regexp.MustCompile(
			"(?i)(?:did you mean|maybe you meant|perhaps you meant)" +
				`\s+['` + "`" + `"]([\w][\w-]*)['` + "`" + `"][?!.]?`,
		),
		nextLineRegex: regexp.MustCompile(
			`(?i)did you mean (?:this|one of these)\?[^\n]*\n[ \t]+([\w][\w-]*)`,
		),
	}
}

// Name returns the parser name.
func (p *GenericParser) Name() string {
	return "generic"
}

// Parse parses generic error output.
func (p *GenericParser) Parse(ctx itypes.ParserContext) itypes.ParserResult {
	cmd := ctx.Command
	stderr := ctx.Stderr

	suggested := p.extractSuggestion(stderr)
	if suggested == "" {
		return itypes.ParserResult{Fixed: false}
	}

	// Ignore flag-correction hints (e.g. pnpm suggesting --save for --savde).
	if strings.HasPrefix(suggested, "-") {
		return itypes.ParserResult{Fixed: false}
	}

	parts := strings.Fields(cmd)
	if len(parts) < 2 {
		return itypes.ParserResult{Fixed: false}
	}
	binary := parts[0]

	call, err := parseShellCall(cmd)
	if err != nil {
		// Fallback: reconstruct as "binary suggestion [rest...]".
		fixed := binary + " " + suggested
		if len(parts) > 2 {
			fixed += " " + strings.Join(parts[2:], " ")
		}
		return itypes.ParserResult{
			Fixed:   true,
			Command: fixed,
			Message: "generic suggested: " + suggested,
		}
	}

	// expected is empty so replaceSubcommand replaces whatever positional
	// argument is at the subcommand position, regardless of its current value.
	fixed, ok := call.replaceSubcommand(binary, "", suggested, genericParserOptionsWithValues)
	if !ok {
		return itypes.ParserResult{Fixed: false}
	}

	return itypes.ParserResult{
		Fixed:   true,
		Command: fixed,
		Message: "generic suggested: " + suggested,
	}
}

// extractSuggestion returns the first plausible correction found in stderr,
// or an empty string if none is found.
func (p *GenericParser) extractSuggestion(stderr string) string {
	if m := p.inlineRegex.FindStringSubmatch(stderr); len(m) >= 2 {
		return m[1]
	}
	if m := p.nextLineRegex.FindStringSubmatch(stderr); len(m) >= 2 {
		return m[1]
	}
	return ""
}
