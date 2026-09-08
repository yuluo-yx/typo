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

const genericParserName = "generic"

// Capture complete quoted or unquoted error tokens; the replacement suggestion
// is validated separately and never incorporates the reported token's bytes.
const reportedCommandTokenPattern = `(?:'([^'\r\n]+)'|"([^"\r\n]+)"|` + "`([^`\r\n]+)`" + `|([^\s'"` + "`" + `]+))`

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
			`(?i)did you mean (?:this|one of these)\?[^\n]*\n[ \t]+([\w][\w-]*)(?:\s|$)`,
		)
		genericWrongRegexes = []*regexp.Regexp{
			regexp.MustCompile(`(?i)(?:unknown command|no such subcommand)[: ]+` + reportedCommandTokenPattern),
			regexp.MustCompile(`(?i)command ` + reportedCommandTokenPattern + ` (?:is not defined|not found)`),
		}
	})
	return genericInlineRegex, genericNextLineRegex
}

// Name returns the parser name.
func (p *GenericParser) Name() string {
	return genericParserName
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
	call, err := parseShellCall(cmd)
	if err != nil || len(call.args) < 2 {
		return itypes.ParserResult{Fixed: false}
	}

	wrong := p.extractWrongCommand(stderr)
	fixed := ""
	ok := false
	if wrong != "" {
		fixed, ok = replaceReportedShellWord(call, wrong, suggested)
	} else {
		// Without a reported token, only the immediate positional argument is known.
		// Unknown tools may give any leading option a separate value.
		word, static := staticShellWordValue(call.args[1])
		if static && word != "" && !strings.HasPrefix(word, "-") {
			fixed, ok = call.replaceWord(1, suggested), true
		}
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
			for _, token := range m[1:] {
				if token != "" {
					return token
				}
			}
		}
	}
	return ""
}

func replaceReportedShellWord(call *shellCall, wrong, replacement string) (string, bool) {
	index := -1
	afterSeparator := false
	for i := 1; i < len(call.args); i++ {
		word, static := staticShellWordValue(call.args[i])
		if static && word == "--" {
			afterSeparator = true
		}
		if static && word == wrong {
			if index != -1 || afterSeparator {
				return "", false
			}
			index = i
		}
	}
	if index == -1 {
		return "", false
	}
	// A word directly after an unknown option may be its value.
	if index > 1 {
		previous, static := staticShellWordValue(call.args[index-1])
		if !static || genericOptionMayTakeNextValue(previous) {
			return "", false
		}
	}
	return call.replaceWord(index, replacement), true
}

func genericOptionMayTakeNextValue(arg string) bool {
	if !strings.HasPrefix(arg, "-") {
		return false
	}
	// A long option with an explicit inline value cannot consume the next word.
	name, _, inline := strings.Cut(arg, "=")
	return !inline || !strings.HasPrefix(name, "--") || len(name) == 2
}
