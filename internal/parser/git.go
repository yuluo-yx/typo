package parser

import (
	"regexp"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	itypes "github.com/yuluo-yx/typo/internal/types"
	"github.com/yuluo-yx/typo/internal/utils"
)

// GitParser parses git command errors.
type GitParser struct {
	didYouMeanRegex          *regexp.Regexp
	noUpstreamRegex          *regexp.Regexp
	legacyUpstreamRegex      *regexp.Regexp
	divergentBranchesRegex   *regexp.Regexp
	reconcileDivergenceRegex *regexp.Regexp
	pushRejectedRegex        *regexp.Regexp
	pushNeedsPullRegex       *regexp.Regexp
	placeholderRegex         *regexp.Regexp
	notGitRepoRegex          *regexp.Regexp
}

// NewGitParser creates a new GitParser.
func NewGitParser() *GitParser {
	return &GitParser{
		didYouMeanRegex:          regexp.MustCompile(`(?s)git: '([^']+)' is not a git command\..*The most similar commands? (?:is|are)\s+(\w+)`),
		noUpstreamRegex:          regexp.MustCompile(`git branch --set-upstream-to(?:=|\s+)([^\s]+)(?:\s+([^\s]+))?`),
		legacyUpstreamRegex:      regexp.MustCompile(`git branch --set-upstream\s+([^\s]+)\s+([^\s]+)`),
		divergentBranchesRegex:   regexp.MustCompile(`(?i)You have divergent branches and need to specify how to reconcile them\.`),
		reconcileDivergenceRegex: regexp.MustCompile(`(?i)fatal: Need to specify how to reconcile divergent branches\.`),
		pushRejectedRegex:        regexp.MustCompile(`(?i)(?:fetch first|non-fast-forward)`),
		pushNeedsPullRegex:       regexp.MustCompile(`(?is)(?:git pull|integrate the remote changes|before pushing again)`),
		placeholderRegex:         regexp.MustCompile(`^<[^>\s]+>$`),
		notGitRepoRegex:          regexp.MustCompile(`fatal: not a git repository`),
	}
}

// Name returns the parser name.
func (p *GitParser) Name() string {
	return parserNameGit
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

	if result := p.parseDivergentPullRebase(cmd, stderr); result.Fixed {
		return result
	}

	if result := p.parsePushRejectedNeedsPull(cmd, stderr); result.Fixed {
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

	fixed, ok := call.replaceSubcommand(parserNameGit, wrongCmd, suggested, gitParserOptionsWithValues)
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

	remote, branch, ok := p.parseUpstreamHint(stderr)
	if !ok {
		return itypes.ParserResult{Fixed: false}
	}

	// Add --set-upstream flag to the command
	return itypes.ParserResult{
		Fixed:   true,
		Command: cmd + " --set-upstream " + remote + " " + branch,
		Message: "adding upstream tracking: " + remote + "/" + branch,
	}
}

func (p *GitParser) parseUpstreamHint(stderr string) (string, string, bool) {
	matches := p.noUpstreamRegex.FindStringSubmatch(stderr)
	if len(matches) >= 2 {
		localBranch := ""
		if len(matches) >= 3 {
			localBranch = matches[2]
		}
		return p.parseUpstreamTarget(matches[1], localBranch)
	}

	matches = p.legacyUpstreamRegex.FindStringSubmatch(stderr)
	if len(matches) >= 3 {
		localBranch := matches[1]
		upstreamTarget := matches[2]
		return p.parseUpstreamTarget(upstreamTarget, localBranch)
	}

	return "", "", false
}

func (p *GitParser) parseUpstreamTarget(target, localBranch string) (string, string, bool) {
	remote, upstreamBranch, ok := strings.Cut(target, "/")
	if !ok {
		return "", "", false
	}

	if p.placeholderRegex.MatchString(remote) {
		remote = "origin"
	}

	branch := upstreamBranch
	if p.placeholderRegex.MatchString(branch) && localBranch != "" {
		branch = localBranch
	}
	if p.placeholderRegex.MatchString(branch) {
		return "", "", false
	}

	return remote, branch, true
}

func (p *GitParser) parseDivergentPullRebase(cmd, stderr string) itypes.ParserResult {
	if gitSubcommand(cmd) != "pull" || gitCommandHasPullReconcileFlag(cmd) {
		return itypes.ParserResult{Fixed: false}
	}
	if !p.divergentBranchesRegex.MatchString(stderr) || !p.reconcileDivergenceRegex.MatchString(stderr) {
		return itypes.ParserResult{Fixed: false}
	}

	fixed, ok := addGitPullRebaseFlag(cmd)
	if !ok {
		return itypes.ParserResult{Fixed: false}
	}

	return itypes.ParserResult{
		Fixed:   true,
		Command: fixed,
		Message: "adding git pull rebase strategy",
	}
}

func (p *GitParser) parsePushRejectedNeedsPull(cmd, stderr string) itypes.ParserResult {
	if gitSubcommand(cmd) != "push" {
		return itypes.ParserResult{Fixed: false}
	}
	if !p.pushRejectedRegex.MatchString(stderr) || !p.pushNeedsPullRegex.MatchString(stderr) {
		return itypes.ParserResult{Fixed: false}
	}

	fixed, ok := replaceGitPushWithPull(cmd)
	if !ok {
		return itypes.ParserResult{Fixed: false}
	}

	return itypes.ParserResult{
		Fixed:   true,
		Command: fixed,
		Message: "retry after integrating remote changes with git pull",
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
	if parts[0] != parserNameGit {
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
	if strings.HasPrefix(first, gitCommandPrefix) && len(first) > len(gitCommandPrefix) {
		return strings.TrimPrefix(first, gitCommandPrefix)
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
	name, _, hasInlineValue := utils.SplitInlineValue(arg)
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

func gitCommandHasPullReconcileFlag(cmd string) bool {
	for _, part := range strings.Fields(cmd) {
		switch {
		case part == "--rebase", part == "--no-rebase", part == "--ff-only":
			return true
		case strings.HasPrefix(part, "--rebase="):
			return true
		}
	}

	return false
}

func replaceGitPushWithPull(cmd string) (string, bool) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return "", false
	}

	if gitPrefixedSubcommand(parts[0]) == "push" {
		return replaceGitPrefixedPushWithPull(cmd)
	}

	call, err := parseShellCall(cmd)
	if err == nil {
		index := findShellSubcommandIndex(call.args, parserNameGit, gitParserOptionsWithValues)
		if index != -1 && call.args[index].Lit() == "push" {
			if gitPushArgsBlockPull(gitShellArgsLits(call.args[index+1:])) {
				return "", false
			}
			fixed := call.replaceWord(index, "pull")
			return normalizeGitPullFromPush(fixed)
		}
	}

	for i, part := range parts {
		if part != "push" {
			continue
		}
		if gitPushArgsBlockPull(parts[i+1:]) {
			return "", false
		}
		parts[i] = "pull"
		return normalizeGitPullFields(parts), true
	}

	return "", false
}

func replaceGitPrefixedPushWithPull(cmd string) (string, bool) {
	call, err := parseShellCall(cmd)
	if err == nil && len(call.args) > 0 && gitPrefixedSubcommand(call.args[0].Lit()) == "push" {
		if gitPushArgsBlockPull(gitShellArgsLits(call.args[1:])) {
			return "", false
		}
		fixed := call.replaceWord(0, gitCommandPrefix+"pull")
		return normalizeGitPullFromPush(fixed)
	}

	parts := strings.Fields(cmd)
	if len(parts) == 0 || gitPrefixedSubcommand(parts[0]) != "push" {
		return "", false
	}
	if gitPushArgsBlockPull(parts[1:]) {
		return "", false
	}
	parts[0] = gitCommandPrefix + "pull"
	return normalizeGitPullFields(parts), true
}

func gitShellArgsLits(args []*syntax.Word) []string {
	lits := make([]string, 0, len(args))
	for _, arg := range args {
		lits = append(lits, arg.Lit())
	}
	return lits
}

func gitPushArgsBlockPull(args []string) bool {
	for _, arg := range args {
		if arg == "--" {
			return true
		}
		if strings.Contains(arg, ":") {
			return true
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			continue
		}
		if gitPushOptionAllowedForPull(arg) {
			continue
		}
		return true
	}

	return false
}

func gitPushOptionAllowedForPull(arg string) bool {
	switch {
	case arg == "-u":
		return true
	case arg == "-v", arg == "-q":
		return true
	case arg == "--set-upstream", arg == "--no-set-upstream":
		return true
	case strings.HasPrefix(arg, "--set-upstream="):
		return true
	case arg == "--verbose", arg == "--no-verbose":
		return true
	case arg == "--quiet", arg == "--no-quiet":
		return true
	case arg == "--progress", arg == "--no-progress":
		return true
	default:
		return false
	}
}

func normalizeGitPullFromPush(cmd string) (string, bool) {
	call, err := parseShellCall(cmd)
	if err == nil {
		fixed := cmd
		for i := len(call.args) - 1; i >= 0; i-- {
			if call.args[i].Lit() != "-u" {
				continue
			}
			start, end := utils.ShellNodeRange(call.args[i], len(cmd))
			fixed = fixed[:start] + "--set-upstream" + fixed[end:]
		}
		return fixed, true
	}

	return normalizeGitPullFields(strings.Fields(cmd)), true
}

func normalizeGitPullFields(parts []string) string {
	for i, part := range parts {
		if part == "-u" {
			parts[i] = "--set-upstream"
		}
	}
	return strings.Join(parts, " ")
}

func addGitPullRebaseFlag(cmd string) (string, bool) {
	if strings.HasPrefix(strings.Fields(cmd)[0], gitCommandPrefix+"pull") {
		return addGitPrefixedPullRebaseFlag(cmd)
	}

	call, err := parseShellCall(cmd)
	if err == nil {
		index := findShellSubcommandIndex(call.args, parserNameGit, gitParserOptionsWithValues)
		if index != -1 && call.args[index].Lit() == "pull" {
			return call.insertAfterWord(index, " --rebase"), true
		}
	}

	parts := strings.Fields(cmd)
	for i, part := range parts {
		if part == "pull" {
			parts = append(parts[:i+1], append([]string{"--rebase"}, parts[i+1:]...)...)
			return strings.Join(parts, " "), true
		}
	}

	return "", false
}

func addGitPrefixedPullRebaseFlag(cmd string) (string, bool) {
	call, err := parseShellCall(cmd)
	if err == nil && len(call.args) > 0 && gitPrefixedSubcommand(call.args[0].Lit()) == "pull" {
		return call.insertAfterWord(0, " --rebase"), true
	}

	parts := strings.Fields(cmd)
	if len(parts) == 0 || gitPrefixedSubcommand(parts[0]) != "pull" {
		return "", false
	}

	parts = append(parts[:1], append([]string{"--rebase"}, parts[1:]...)...)
	return strings.Join(parts, " "), true
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
	return parts[0] == parserNameGit || strings.HasPrefix(parts[0], gitCommandPrefix)
}
