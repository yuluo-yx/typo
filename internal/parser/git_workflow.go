package parser

import (
	"regexp"
	"strings"
	"unicode/utf8"

	itypes "github.com/yuluo-yx/typo/internal/types"
	"github.com/yuluo-yx/typo/internal/utils"
)

// gitBranchNameChars matches characters that may continue a branch name, so a
// branch is only recognized in stderr as a whole token.
const gitBranchNameChars = `A-Za-z0-9._/@+-`

// parsePushRejected turns a push rejected because the remote moved ahead into
// "git pull --rebase && git push". It only fires when the rejected ref is the
// checked-out branch and that branch tracks the same remote branch, so the pull
// integrates exactly the commits that blocked the push.
func (p *GitParser) parsePushRejected(ctx itypes.ParserContext) itypes.ParserResult {
	if !gitWorkflowContextAllowed(ctx) {
		return itypes.ParserResult{Fixed: false}
	}
	// A single status line keeps multi-ref pushes, which need per-ref decisions, unchanged.
	if len(gitRegexes().pushStatusLineRegex.FindAllStringIndex(ctx.Stderr, 2)) != 1 {
		return itypes.ParserResult{Fixed: false}
	}
	matches := gitRegexes().pushRejectedRegex.FindStringSubmatch(ctx.Stderr)
	if len(matches) < 3 || matches[1] != matches[2] || !gitUpstreamTokenRegex.MatchString(matches[1]) {
		return itypes.ParserResult{Fixed: false}
	}
	branch := matches[1]

	call, pushIndex, repositoryArgs, ok := plainGitPush(ctx.Command)
	if !ok || !p.isCurrentBranch(repositoryArgs, branch) {
		return itypes.ParserResult{Fixed: false}
	}
	upstream, ok := p.gitCurrentUpstream(repositoryArgs)
	if !ok || !strings.HasSuffix(upstream, "/"+branch) || !gitUpstreamTokenRegex.MatchString(upstream) {
		return itypes.ParserResult{Fixed: false}
	}

	start, _ := utils.ShellNodeRange(call.args[pushIndex], len(call.raw))
	pull := "pull --rebase && "
	if pushIndex == 0 {
		pull = gitCommandPrefix + pull
	}
	return itypes.ParserResult{
		Fixed:   true,
		Command: call.raw[:start] + pull + call.raw,
		Message: "rebasing onto " + upstream + " before pushing again",
	}
}

// plainGitPush parses a push without explicit targets or refspecs, which could
// push something other than the tracked branch.
func plainGitPush(command string) (*shellCall, int, []string, bool) {
	call, err := parseShellCall(command)
	if err != nil {
		return nil, 0, nil, false
	}
	pushIndex, repositoryArgs, ok := gitSubcommandContext(call.args, "push")
	if !ok {
		return nil, 0, nil, false
	}
	for _, word := range call.args[pushIndex+1:] {
		arg, static := staticShellWordValue(word)
		if !static || !gitPushUpstreamArgAllowed(arg) {
			return nil, 0, nil, false
		}
	}
	return call, pushIndex, repositoryArgs, true
}

// isCurrentBranch reports whether branch is checked out in the repository.
func (p *GitParser) isCurrentBranch(repositoryArgs []string, branch string) bool {
	current, ok := p.gitCurrentBranch(repositoryArgs)
	return ok && current == branch
}

// parseIdentityUnknown answers "Author identity unknown" with the first git
// config key that is still unset. git only prints placeholders ("Your Name",
// "you@example.com"), so the command ends with the key and the user types the
// value before running it; running it unchanged only reads the key.
func (p *GitParser) parseIdentityUnknown(ctx itypes.ParserContext) itypes.ParserResult {
	if !gitWorkflowContextAllowed(ctx) || !gitRegexes().identityEmailHintRegex.MatchString(ctx.Stderr) {
		return itypes.ParserResult{Fixed: false}
	}
	call, err := parseShellCall(ctx.Command)
	if err != nil {
		return itypes.ParserResult{Fixed: false}
	}
	index, repositoryArgs, ok := gitInvocationContext(call.args)
	if !ok || gitInvocationSubcommand(call.args, index) == "config" {
		return itypes.ParserResult{Fixed: false}
	}

	for _, key := range []string{"user.name", "user.email"} {
		if _, set := p.gitConfigValue(repositoryArgs, key); !set {
			return itypes.ParserResult{
				Fixed:   true,
				Command: "git config --global " + key,
				Message: "type your git " + key + " value before running",
			}
		}
	}
	return itypes.ParserResult{Fixed: false}
}

// parseBranchTypo corrects "git checkout|switch <name>" when <name> is not a
// branch but is close to exactly one existing local or remote branch name.
func (p *GitParser) parseBranchTypo(ctx itypes.ParserContext) itypes.ParserResult {
	if ctx.HasMultipleCommands || ctx.ShellParseFailed {
		return itypes.ParserResult{Fixed: false}
	}
	call, err := parseShellCall(ctx.Command)
	if err != nil {
		return itypes.ParserResult{Fixed: false}
	}
	index, repositoryArgs, ok := gitInvocationContext(call.args)
	if !ok {
		return itypes.ParserResult{Fixed: false}
	}
	if subcommand := gitInvocationSubcommand(call.args, index); subcommand != "checkout" && subcommand != "switch" {
		return itypes.ParserResult{Fixed: false}
	}

	target, name := gitSingleBranchArgument(call, index)
	if target == -1 || !gitUpstreamTokenRegex.MatchString(name) || !containsGitBranchToken(ctx.Stderr, name) {
		return itypes.ParserResult{Fixed: false}
	}
	branches, ok := p.gitBranchNames(repositoryArgs)
	if !ok {
		return itypes.ParserResult{Fixed: false}
	}
	best := closestGitBranch(name, branches)
	if best == "" {
		return itypes.ParserResult{Fixed: false}
	}

	return itypes.ParserResult{
		Fixed:   true,
		Command: call.replaceWord(target, best),
		Message: "using existing branch " + best,
	}
}

// gitSingleBranchArgument returns the only positional argument after the
// subcommand, allowing quiet flags. Any other option changes the meaning
// (for example -b creates a branch, -- selects paths), so it is rejected.
func gitSingleBranchArgument(call *shellCall, index int) (int, string) {
	target, name := -1, ""
	for i := index + 1; i < len(call.args); i++ {
		arg, ok := staticShellWordValue(call.args[i])
		if !ok {
			return -1, ""
		}
		switch {
		case gitQuietFlag(arg):
			continue
		case strings.HasPrefix(arg, "-") || target != -1:
			return -1, ""
		}
		target, name = i, arg
	}
	return target, name
}

func gitQuietFlag(arg string) bool {
	return arg == "-q" || arg == "--quiet"
}

func containsGitBranchToken(stderr, name string) bool {
	pattern := `(?:^|[^` + gitBranchNameChars + `])` + regexp.QuoteMeta(name) + `(?:$|[^` + gitBranchNameChars + `])`
	matched, err := regexp.MatchString(pattern, stderr)
	return err == nil && matched
}

// closestGitBranch returns the unique branch within the typo budget, or ""
// when the name already exists, nothing is close, or the best match is tied.
func closestGitBranch(name string, branches []string) string {
	limit := 2
	if utf8.RuneCountInString(name) <= 4 {
		limit = 1
	}
	best, bestDistance, tied := "", limit+1, false
	for _, branch := range branches {
		if branch == name {
			return ""
		}
		if !gitUpstreamTokenRegex.MatchString(branch) {
			continue
		}
		distance := osaDistance(name, branch)
		switch {
		case distance < bestDistance:
			best, bestDistance, tied = branch, distance, false
		case distance == bestDistance:
			tied = true
		}
	}
	if best == "" || tied {
		return ""
	}
	return best
}

// osaDistance is the optimal string alignment distance: Levenshtein plus
// adjacent transpositions, which covers the most common branch-name typos.
func osaDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	rows := make([][]int, len(ra)+1)
	for i := range rows {
		rows[i] = make([]int, len(rb)+1)
		rows[i][0] = i
	}
	for j := range rows[0] {
		rows[0][j] = j
	}
	for i := 1; i <= len(ra); i++ {
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			rows[i][j] = min(rows[i-1][j]+1, rows[i][j-1]+1, rows[i-1][j-1]+cost)
			if i > 1 && j > 1 && ra[i-1] == rb[j-2] && ra[i-2] == rb[j-1] {
				rows[i][j] = min(rows[i][j], rows[i-2][j-2]+1)
			}
		}
	}
	return rows[len(ra)][len(rb)]
}

// gitWorkflowContextAllowed rejects contexts where rewriting the whole command
// could lose shell structure: compound lines, sudo wrappers, or unparsed input.
func gitWorkflowContextAllowed(ctx itypes.ParserContext) bool {
	return !ctx.HasMultipleCommands && !ctx.HasPrivilegeWrapper && !ctx.ShellParseFailed
}
