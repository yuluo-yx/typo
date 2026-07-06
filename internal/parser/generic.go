package parser

import (
	"regexp"
	"strings"
	"sync"

	itypes "github.com/yuluo-yx/typo/internal/types"
)

// GenericParser catches "did you mean" hints emitted by any CLI, covering tools
// that do not have a dedicated parser (e.g. rustup, cargo, helm, gh, kubectl,
// pnpm, poetry, pip).
type GenericParser struct{}

var (
	genericParserRegexOnce sync.Once
	genericInlineRegex     *regexp.Regexp
	genericNextLineRegex   *regexp.Regexp
	genericWrongRegexes    []*regexp.Regexp
)

// NewGenericParser creates a new GenericParser.
func NewGenericParser() *GenericParser {
	return &GenericParser{}
}

func genericParserRegexes() (*regexp.Regexp, *regexp.Regexp) {
	genericParserRegexOnce.Do(func() {
		genericInlineRegex = regexp.MustCompile(
			"(?i)(?:did you mean|maybe you meant|perhaps you meant)" +
				`\s+['` + "`" + `"]([\w][\w-]*)['` + "`" + `"][?!.]?`,
		)
		genericNextLineRegex = regexp.MustCompile(
			`(?i)did you mean (?:this|one of these)\?[^\n]*\n[ \t]+([\w][\w-]*)`,
		)
		genericWrongRegexes = []*regexp.Regexp{
			regexp.MustCompile("(?i)(?:unknown command|no such subcommand)[: ]+['`\"]?([\\w][\\w-]*)"),
			regexp.MustCompile("(?i)command ['`\"]?([\\w][\\w-]*)['`\"]? (?:is not defined|not found)"),
		}
	})
	return genericInlineRegex, genericNextLineRegex
}

// Name returns the parser name.
func (p *GenericParser) Name() string {
	return "generic"
}

// Parse parses generic error output.
func (p *GenericParser) Parse(ctx itypes.ParserContext) itypes.ParserResult {
	cmd := ctx.Command
	stderr := ctx.Stderr
	if stderr == "" {
		return itypes.ParserResult{Fixed: false}
	}

	suggested := p.extractSuggestion(stderr)
	if suggested == "" {
		return itypes.ParserResult{Fixed: false}
	}

	// Ignore flag-correction hints (e.g. pnpm suggesting --save for --savde).
	if strings.HasPrefix(suggested, "-") {
		return itypes.ParserResult{Fixed: false}
	}
	wrong := p.extractWrongCommand(stderr)

	parts := strings.Fields(cmd)
	if len(parts) < 2 {
		return itypes.ParserResult{Fixed: false}
	}
	binary := parts[0]

	call, err := parseShellCall(cmd)
	if err != nil {
		if wrong != "" {
			fixed := strings.Replace(cmd, wrong, suggested, 1)
			if fixed != cmd {
				return itypes.ParserResult{
					Fixed:   true,
					Command: fixed,
					Message: "generic suggested: " + suggested,
				}
			}
		}

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

	fixed := ""
	ok := false
	if wrong != "" {
		fixed, ok = replaceReportedShellWord(call, wrong, suggested)
	} else {
		// expected is empty so replaceSubcommand replaces whatever positional
		// argument is at the subcommand position, regardless of its current value.
		fixed, ok = call.replaceSubcommand(binary, "", suggested, genericParserOptionsWithValues)
	}
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
	inlineRegex, nextLineRegex := genericParserRegexes()
	if m := inlineRegex.FindStringSubmatch(stderr); len(m) >= 2 {
		return m[1]
	}
	if m := nextLineRegex.FindStringSubmatch(stderr); len(m) >= 2 {
		return m[1]
	}
	return ""
}

func (p *GenericParser) extractWrongCommand(stderr string) string {
	genericParserRegexes()
	for _, re := range genericWrongRegexes {
		if m := re.FindStringSubmatch(stderr); len(m) >= 2 {
			return m[1]
		}
	}
	return ""
}

func replaceReportedShellWord(call *shellCall, wrong, replacement string) (string, bool) {
	for i := 1; i < len(call.args); i++ {
		if call.args[i].Lit() == wrong {
			return call.replaceWord(i, replacement), true
		}
	}
	return "", false
}
