package parser

import (
	"regexp"
	"strings"

	itypes "github.com/yuluo-yx/typo/internal/types"
)

// GitParser parses git command errors.
type GitParser struct {
	didYouMeanRegex  *regexp.Regexp
	noUpstreamRegex  *regexp.Regexp
	placeholderRegex *regexp.Regexp
	notGitRepoRegex  *regexp.Regexp
}

// NewGitParser creates a new GitParser.
func NewGitParser() *GitParser {
	return &GitParser{
		didYouMeanRegex:  regexp.MustCompile(`(?s)git: '([^']+)' is not a git command\..*The most similar commands? (?:is|are)\s+(\w+)`),
		noUpstreamRegex:  regexp.MustCompile(`git branch --set-upstream-to=([^/\s]+)/([^\s]+)(?:\s+([^\s]+))?`),
		placeholderRegex: regexp.MustCompile(`^<[^>\s]+>$`),
		notGitRepoRegex:  regexp.MustCompile(`fatal: not a git repository`),
	}
}

// Name returns the parser name.
func (p *GitParser) Name() string {
	return "git"
}

// Parse parses git error output.
func (p *GitParser) Parse(ctx itypes.ParserContext) itypes.ParserResult {
	cmd := ctx.Command
	stderr := ctx.Stderr

	// Check if it's a git command
	if !isGitCommand(cmd) {
		return itypes.ParserResult{Fixed: false}
	}

	// Try to parse "did you mean" errors
	if result := p.parseDidYouMean(cmd, stderr); result.Fixed {
		return result
	}

	// Try to parse "no upstream" errors
	if result := p.parseNoUpstream(cmd, stderr); result.Fixed {
		return result
	}

	return itypes.ParserResult{Fixed: false}
}

func (p *GitParser) parseDidYouMean(cmd, stderr string) itypes.ParserResult {
	matches := p.didYouMeanRegex.FindStringSubmatch(stderr)
	if len(matches) < 3 {
		return itypes.ParserResult{Fixed: false}
	}

	wrongCmd := matches[1]
	suggested := matches[2]
	fixed := ""

	call, err := parseShellCall(cmd)
	if err != nil {
		fixed = strings.Replace(cmd, wrongCmd, suggested, 1)
		return itypes.ParserResult{
			Fixed:   true,
			Command: fixed,
			Message: "git suggested: " + suggested,
		}
	}

	fixed, ok := call.replaceSubcommand("git", wrongCmd, suggested, gitParserOptionsWithValues)
	if !ok {
		return itypes.ParserResult{Fixed: false}
	}

	return itypes.ParserResult{
		Fixed:   true,
		Command: fixed,
		Message: "git suggested: " + suggested,
	}
}

func (p *GitParser) parseNoUpstream(cmd, stderr string) itypes.ParserResult {
	if gitSubcommand(cmd) != "pull" || gitCommandHasUpstreamFlag(cmd) {
		return itypes.ParserResult{Fixed: false}
	}

	matches := p.noUpstreamRegex.FindStringSubmatch(stderr)
	if len(matches) < 3 {
		return itypes.ParserResult{Fixed: false}
	}

	remote := matches[1]
	upstreamBranch := matches[2]
	localBranch := ""
	if len(matches) >= 4 {
		localBranch = matches[3]
	}

	if p.placeholderRegex.MatchString(remote) {
		return itypes.ParserResult{Fixed: false}
	}

	branch := upstreamBranch
	if p.placeholderRegex.MatchString(branch) && localBranch != "" {
		branch = localBranch
	}
	if p.placeholderRegex.MatchString(branch) {
		return itypes.ParserResult{Fixed: false}
	}

	// Add --set-upstream flag to the command
	return itypes.ParserResult{
		Fixed:   true,
		Command: cmd + " --set-upstream " + remote + " " + branch,
		Message: "adding upstream tracking: " + remote + "/" + branch,
	}
}

func gitSubcommand(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return ""
	}

	if subcommand := gitPrefixedSubcommand(parts[0]); subcommand != "" {
		return subcommand
	}
	if parts[0] != "git" {
		return ""
	}

	expectValue := false
	for i := 1; i < len(parts); i++ {
		arg := parts[i]
		if expectValue {
			expectValue = false
			continue
		}

		switch gitOptionState(arg) {
		case gitOptionConsumesNextValue:
			expectValue = true
			continue
		case gitOptionHandled:
			continue
		case gitOptionUnknown:
			return ""
		}

		return arg
	}

	return ""
}

func gitPrefixedSubcommand(first string) string {
	if strings.HasPrefix(first, "git-") && len(first) > len("git-") {
		return strings.TrimPrefix(first, "git-")
	}

	return ""
}

type gitOptionParseState int

const (
	gitOptionNotAnOption gitOptionParseState = iota
	gitOptionHandled
	gitOptionConsumesNextValue
	gitOptionUnknown
)

func gitOptionState(arg string) gitOptionParseState {
	switch {
	case arg == "--":
		return gitOptionUnknown
	case strings.HasPrefix(arg, "--"):
		return gitLongOptionState(arg)
	case strings.HasPrefix(arg, "-") && arg != "-":
		return gitShortOptionState(arg)
	default:
		return gitOptionNotAnOption
	}
}

func gitLongOptionState(arg string) gitOptionParseState {
	name, hasInlineValue := splitLongOption(arg)
	if gitGlobalOptionsWithValues[name] {
		if hasInlineValue {
			return gitOptionHandled
		}
		return gitOptionConsumesNextValue
	}
	if gitGlobalOptions[name] {
		return gitOptionHandled
	}
	return gitOptionUnknown
}

func gitShortOptionState(arg string) gitOptionParseState {
	if gitGlobalOptionsWithValues[arg] {
		return gitOptionConsumesNextValue
	}
	if gitGlobalOptions[arg] || len(arg) > 2 {
		return gitOptionHandled
	}
	return gitOptionUnknown
}

func splitLongOption(arg string) (string, bool) {
	if eq := strings.IndexByte(arg, '='); eq >= 0 {
		return arg[:eq], true
	}

	return arg, false
}

func gitCommandHasUpstreamFlag(cmd string) bool {
	for _, part := range strings.Fields(cmd) {
		switch {
		case part == "--set-upstream", part == "--set-upstream-to":
			return true
		case strings.HasPrefix(part, "--set-upstream="), strings.HasPrefix(part, "--set-upstream-to="):
			return true
		}
	}

	return false
}

var gitGlobalOptions = map[string]bool{
	"--bare":                 true,
	"--help":                 true,
	"--literal-pathspecs":    true,
	"--man-path":             true,
	"--no-literal-pathspecs": true,
	"--no-optional-locks":    true,
	"--no-pager":             true,
	"--no-replace-objects":   true,
	"--no-verbose":           true,
	"--paginate":             true,
	"--version":              true,
	"-h":                     true,
	"-p":                     true,
	"-P":                     true,
}

var gitGlobalOptionsWithValues = map[string]bool{
	"--config-env":   true,
	"--exec-path":    true,
	"--git-dir":      true,
	"--namespace":    true,
	"--super-prefix": true,
	"--work-tree":    true,
	"-C":             true,
	"-c":             true,
}

func isGitCommand(cmd string) bool {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return false
	}
	return parts[0] == "git" || strings.HasPrefix(parts[0], "git-")
}
