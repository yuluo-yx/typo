package parser

import (
	"regexp"
	"strings"
	"sync"

	"mvdan.cc/sh/v3/syntax"

	itypes "github.com/yuluo-yx/typo/internal/types"
	"github.com/yuluo-yx/typo/internal/utils"
)

// GitParser parses git command errors.
type GitParser struct {
	resolveBranchUpstream gitBranchUpstreamResolver
	// runGit executes read-only git queries that verify repository state.
	runGit gitCommandRunner
}

// gitPatterns holds the compiled patterns of GitParser.
type gitPatterns struct {
	didYouMeanRegex            *regexp.Regexp
	pullNoTrackingRegex        *regexp.Regexp
	pullSetUpstreamRegex       *regexp.Regexp
	branchUpstreamMissingRegex *regexp.Regexp
	pushNoUpstreamFatalRegex   *regexp.Regexp
	pushNoUpstreamRegex        *regexp.Regexp
	divergentBranchesRegex     *regexp.Regexp
	reconcileDivergenceRegex   *regexp.Regexp
	placeholderRegex           *regexp.Regexp
	notGitRepoRegex            *regexp.Regexp
	// The following signals are untranslated in every git locale: suggested
	// commands and push status lines are never localized.
	divergentConfigHintRegexes []*regexp.Regexp
	pushRejectedRegex          *regexp.Regexp
	pushStatusLineRegex        *regexp.Regexp
	identityEmailHintRegex     *regexp.Regexp
}

var gitUpstreamTokenRegex = regexp.MustCompile(`^[A-Za-z0-9._][A-Za-z0-9._/@+-]*$`)

// NewGitParser creates a new GitParser.
func NewGitParser() *GitParser {
	return &GitParser{
		resolveBranchUpstream: resolveGitBranchUpstream,
		runGit:                runGitCommand,
	}
}

// gitRegexes compiles the patterns on first use, so constructing a parser is cheap
// and patterns of parsers that never see matching input are never compiled.
var gitRegexes = sync.OnceValue(func() *gitPatterns {
	return &gitPatterns{
		didYouMeanRegex:            regexp.MustCompile(`(?s)git: '([^']+)' is not a git command\..*The most similar commands? (?:is|are)\s+([\w][\w-]*)(?:\s|$)`),
		pullNoTrackingRegex:        regexp.MustCompile(`(?m)^There is no tracking information for the current branch\.[\t\r ]*$`),
		pullSetUpstreamRegex:       regexp.MustCompile(`(?m)^[ \t]*git branch --set-upstream-to(?:=|[ \t]+)([^\s]+)(?:[ \t]+([^\s]+))?[\t\r ]*$`),
		branchUpstreamMissingRegex: regexp.MustCompile(`(?m)^fatal: the requested upstream branch '([^'\r\n]+)' does not exist[\t\r ]*$`),
		pushNoUpstreamFatalRegex:   regexp.MustCompile(`(?m)^fatal: The current branch ([^\s]+) has no upstream branch\.[\t\r ]*$`),
		pushNoUpstreamRegex:        regexp.MustCompile(`(?m)^[ \t]*git push (?:-u|--set-upstream)[ \t]+([^\s]+)[ \t]+([^\s]+)[ \t\r]*$`),
		divergentBranchesRegex:     regexp.MustCompile(`(?m)^hint: You have divergent branches and need to specify how to reconcile them\.[\t\r ]*$`),
		reconcileDivergenceRegex:   regexp.MustCompile(`(?m)^fatal: Need to specify how to reconcile divergent branches\.[\t\r ]*$`),
		placeholderRegex:           regexp.MustCompile(`^<[^>\s]+>$`),
		notGitRepoRegex:            regexp.MustCompile(`fatal: not a git repository`),
		divergentConfigHintRegexes: []*regexp.Regexp{
			regexp.MustCompile(`(?m)\bgit config pull\.rebase false\b`),
			regexp.MustCompile(`(?m)\bgit config pull\.rebase true\b`),
			regexp.MustCompile(`(?m)\bgit config pull\.ff only\b`),
		},
		pushRejectedRegex:      regexp.MustCompile(`(?m)^ ! \[rejected\][ \t]+([^\s]+) -> ([^\s]+) \((?:fetch first|non-fast-forward)\)[\t\r ]*$`),
		pushStatusLineRegex:    regexp.MustCompile(`(?m)^ ! `),
		identityEmailHintRegex: regexp.MustCompile(`(?m)^[ \t]*git config --global user\.email "you@example\.com"[\t\r ]*$`),
	}
})

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

	// Locale-independent fallbacks only run for commands known to have failed
	// (-1 means the exit code is unknown); they verify repository state instead
	// of relying on translated messages.
	failed := ctx.ExitCode != 0

	// Try to parse "did you mean" errors
	if result := p.parseDidYouMean(cmd, stderr); result.Fixed {
		return result
	}

	if result := p.parsePullNoTracking(cmd, stderr, failed); result.Fixed {
		return result
	}

	if result := p.parseBranchSetUpstreamMissing(cmd, stderr, failed); result.Fixed {
		return result
	}

	if result := p.parsePushNoUpstream(cmd, stderr, failed); result.Fixed {
		return result
	}

	if result := p.parseDivergentPull(cmd, stderr, failed); result.Fixed {
		return result
	}

	if !failed {
		return itypes.ParserResult{Fixed: false}
	}
	if result := p.parsePushRejected(ctx); result.Fixed {
		return result
	}
	if result := p.parseIdentityUnknown(ctx); result.Fixed {
		return result
	}
	return p.parseBranchTypo(ctx)
}

func (p *GitParser) parseDidYouMean(cmd, stderr string) itypes.ParserResult {
	matches := gitRegexes().didYouMeanRegex.FindStringSubmatch(stderr)
	if len(matches) < 3 {
		return itypes.ParserResult{Fixed: false}
	}

	wrongCmd := matches[1]
	suggested := matches[2]

	call, err := parseShellCall(cmd)
	if err != nil {
		return itypes.ParserResult{Fixed: false}
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

func (p *GitParser) parsePullNoTracking(cmd, stderr string, failed bool) itypes.ParserResult {
	english := gitRegexes().pullNoTrackingRegex.MatchString(stderr)
	if (!english && !failed) || gitCommandHasUpstreamFlag(cmd) {
		return itypes.ParserResult{Fixed: false}
	}

	call, err := parseShellCall(cmd)
	if err != nil {
		return itypes.ParserResult{Fixed: false}
	}
	pullIndex, repositoryArgs, ok := gitSubcommandContext(call.args, "pull")
	if !ok || !gitPullUpstreamArgsAllowed(call.args[pullIndex+1:]) {
		return itypes.ParserResult{Fixed: false}
	}

	remote, branch, ok := p.parsePullUpstreamHint(stderr)
	if !ok {
		return itypes.ParserResult{Fixed: false}
	}
	// Without the English signal, the untranslated hint must name the branch
	// that is actually checked out.
	if !english && !p.isCurrentBranch(repositoryArgs, branch) {
		return itypes.ParserResult{Fixed: false}
	}
	target, ok := p.resolveBranchUpstream(repositoryArgs, remote, branch)
	if !ok || target != remote+"/"+branch {
		return itypes.ParserResult{Fixed: false}
	}

	// Keep the requested pull and its options so executing the correction both
	// establishes tracking and integrates the remote changes.
	return itypes.ParserResult{
		Fixed:   true,
		Command: call.insertAfterWord(len(call.args)-1, " --set-upstream "+remote+" "+branch),
		Message: "pulling with upstream tracking: " + target,
	}
}

func (p *GitParser) parseDivergentPullRebase(cmd, stderr string) itypes.ParserResult {
	return p.parseDivergentPull(cmd, stderr, false)
}

// parseDivergentPull adds --rebase after a pull that git refused because the
// branches diverged. Failed commands also match on the untranslated
// "git config pull.*" suggestions so localized git output is recognized.
func (p *GitParser) parseDivergentPull(cmd, stderr string, failed bool) itypes.ParserResult {
	if !p.hasDivergentSignal(stderr, failed) {
		return itypes.ParserResult{Fixed: false}
	}
	return addGitPullRebase(cmd)
}

func (p *GitParser) hasDivergentSignal(stderr string, failed bool) bool {
	if gitRegexes().divergentBranchesRegex.MatchString(stderr) && gitRegexes().reconcileDivergenceRegex.MatchString(stderr) {
		return true
	}
	if !failed {
		return false
	}
	for _, hint := range gitRegexes().divergentConfigHintRegexes {
		if !hint.MatchString(stderr) {
			return false
		}
	}
	return true
}

func addGitPullRebase(cmd string) itypes.ParserResult {
	call, err := parseShellCall(cmd)
	if err != nil {
		return itypes.ParserResult{Fixed: false}
	}
	index := findShellSubcommandIndex(call.args, parserNameGit, gitParserOptionsWithValues)
	if gitPrefixedSubcommand(call.args[0].Lit()) == "pull" {
		index = 0
	} else if index == -1 || call.args[index].Lit() != "pull" {
		return itypes.ParserResult{Fixed: false}
	}
	// Inspect only pull arguments: global option values can contain flag names.
	// Preserve an explicit reconciliation choice instead of overriding it.
	for _, word := range call.args[index+1:] {
		arg, static := staticShellWordValue(word)
		if !static {
			return itypes.ParserResult{Fixed: false}
		}
		if arg == "--" {
			break
		}
		if arg == "-r" || arg == "--rebase" || arg == "--no-rebase" || arg == "--ff-only" || strings.HasPrefix(arg, "--rebase=") {
			return itypes.ParserResult{Fixed: false}
		}
	}
	return itypes.ParserResult{
		Fixed:   true,
		Command: call.insertAfterWord(index, " --rebase"),
		Message: "adding git pull rebase strategy",
	}
}

func (p *GitParser) parsePullUpstreamHint(stderr string) (string, string, bool) {
	allMatches := gitRegexes().pullSetUpstreamRegex.FindAllStringSubmatch(stderr, -1)
	if len(allMatches) != 1 || len(allMatches[0]) < 3 {
		return "", "", false
	}
	matches := allMatches[0]

	target := matches[1]
	localBranch := matches[2]
	if localBranch == "" || gitRegexes().placeholderRegex.MatchString(localBranch) {
		return "", "", false
	}

	remote, upstreamBranch, ok := strings.Cut(target, "/")
	if !ok || gitRegexes().placeholderRegex.MatchString(remote) {
		return "", "", false
	}
	if gitRegexes().placeholderRegex.MatchString(upstreamBranch) {
		upstreamBranch = localBranch
	}
	remote, upstreamBranch, ok = p.validateUpstreamTarget(remote, upstreamBranch)
	if !ok || upstreamBranch != localBranch {
		return "", "", false
	}

	return remote, localBranch, true
}

// gitInvocationContext locates the git subcommand word and returns its index
// together with the static repository selector arguments that precede it.
func gitInvocationContext(args []*syntax.Word) (int, []string, bool) {
	if len(args) == 0 {
		return 0, nil, false
	}
	if gitPrefixedSubcommand(args[0].Lit()) != "" {
		return 0, nil, true
	}

	index := findShellSubcommandIndex(args, parserNameGit, gitParserOptionsWithValues)
	if index == -1 {
		return 0, nil, false
	}
	repositoryArgs, ok := gitRepositorySelectorArgs(args, index)
	if !ok {
		return 0, nil, false
	}
	return index, repositoryArgs, true
}

// gitSubcommandContext is gitInvocationContext restricted to one subcommand.
func gitSubcommandContext(args []*syntax.Word, name string) (int, []string, bool) {
	index, repositoryArgs, ok := gitInvocationContext(args)
	if !ok || gitInvocationSubcommand(args, index) != name {
		return 0, nil, false
	}
	return index, repositoryArgs, true
}

func gitInvocationSubcommand(args []*syntax.Word, index int) string {
	if index == 0 {
		return gitPrefixedSubcommand(args[0].Lit())
	}
	return args[index].Lit()
}

func gitPullUpstreamArgsAllowed(args []*syntax.Word) bool {
	for _, word := range args {
		arg, ok := staticShellWordValue(word)
		if !ok {
			return false
		}
		switch arg {
		case "-q", "--quiet", "-v", "--verbose", "--progress", "--no-progress":
			continue
		case "--rebase", "--no-rebase", "--ff", "--no-ff", "--ff-only":
			continue
		case "--autostash", "--no-autostash", "--commit", "--no-commit":
			continue
		case "--edit", "--no-edit", "--signoff", "--no-signoff":
			continue
		case "--stat", "--no-stat", "--squash", "--no-squash":
			continue
		case "--verify", "--no-verify", "--prune", "--no-prune":
			continue
		case "--tags", "--no-tags":
			continue
		}
		if strings.HasPrefix(arg, "--rebase=") {
			continue
		}
		return false
	}
	return true
}

type gitBranchUpstreamRequest struct {
	repositoryArgs []string
	remote         string
	localBranch    string
	targetIndex    int
	targetPrefix   string
}

func (p *GitParser) parseBranchSetUpstreamMissing(cmd, stderr string, failed bool) itypes.ParserResult {
	matches := gitRegexes().branchUpstreamMissingRegex.FindStringSubmatch(stderr)
	english := len(matches) >= 2
	if !english && !failed {
		return itypes.ParserResult{Fixed: false}
	}

	call, err := parseShellCall(cmd)
	if err != nil {
		return itypes.ParserResult{Fixed: false}
	}
	request, ok := parseGitBranchUpstreamRequest(call)
	if !ok {
		return itypes.ParserResult{Fixed: false}
	}
	// Localized git still quotes the requested upstream verbatim.
	if english && request.remote != matches[1] ||
		!english && !strings.Contains(stderr, "'"+request.remote+"'") {
		return itypes.ParserResult{Fixed: false}
	}

	target, ok := p.resolveBranchUpstream(
		request.repositoryArgs,
		request.remote,
		request.localBranch,
	)
	if !ok || !gitUpstreamTokenRegex.MatchString(target) {
		return itypes.ParserResult{Fixed: false}
	}

	return itypes.ParserResult{
		Fixed:   true,
		Command: call.replaceWord(request.targetIndex, request.targetPrefix+target),
		Message: "setting branch upstream to " + target,
	}
}

func parseGitBranchUpstreamRequest(call *shellCall) (gitBranchUpstreamRequest, bool) {
	var request gitBranchUpstreamRequest
	if len(call.args) == 0 {
		return request, false
	}

	branchIndex, repositoryArgs, ok := gitSubcommandContext(call.args, "branch")
	if !ok {
		return request, false
	}
	request.repositoryArgs = repositoryArgs

	request, ok = parseGitBranchUpstreamArgs(call.args, branchIndex, request)
	if !ok || !validGitBranchUpstreamRequest(request) {
		return gitBranchUpstreamRequest{}, false
	}
	return request, true
}

func parseGitBranchUpstreamArgs(
	args []*syntax.Word,
	branchIndex int,
	request gitBranchUpstreamRequest,
) (gitBranchUpstreamRequest, bool) {
	hasTarget := false
	for i := branchIndex + 1; i < len(args); i++ {
		arg, isStatic := staticShellWordValue(args[i])
		if !isStatic {
			return gitBranchUpstreamRequest{}, false
		}
		switch {
		case strings.HasPrefix(arg, "--set-upstream-to="):
			if hasTarget {
				return gitBranchUpstreamRequest{}, false
			}
			request.remote = strings.TrimPrefix(arg, "--set-upstream-to=")
			request.targetIndex = i
			request.targetPrefix = "--set-upstream-to="
			hasTarget = true
		case arg == "--set-upstream-to" || arg == "-u":
			if hasTarget || i+1 >= len(args) {
				return gitBranchUpstreamRequest{}, false
			}
			i++
			request.remote, isStatic = staticShellWordValue(args[i])
			if !isStatic {
				return gitBranchUpstreamRequest{}, false
			}
			request.targetIndex = i
			hasTarget = true
		case gitQuietFlag(arg):
			continue
		case strings.HasPrefix(arg, "-"):
			return gitBranchUpstreamRequest{}, false
		default:
			if request.localBranch != "" {
				return gitBranchUpstreamRequest{}, false
			}
			request.localBranch = arg
		}
	}

	return request, hasTarget
}

func validGitBranchUpstreamRequest(request gitBranchUpstreamRequest) bool {
	remoteIsIncomplete := request.remote != "" && !strings.Contains(request.remote, "/")
	if !remoteIsIncomplete || strings.HasPrefix(request.remote, "-") {
		return false
	}
	if !gitUpstreamTokenRegex.MatchString(request.remote) {
		return false
	}
	if request.localBranch != "" && !gitUpstreamTokenRegex.MatchString(request.localBranch) {
		return false
	}

	return true
}

func gitRepositorySelectorArgs(args []*syntax.Word, branchIndex int) ([]string, bool) {
	repositoryArgs := []string{}
	for i := 1; i < branchIndex; i++ {
		arg, isStatic := staticShellWordValue(args[i])
		if !isStatic {
			return nil, false
		}
		name, suffix, hasInlineValue := utils.SplitInlineValue(arg)
		switch name {
		case "--git-dir", "--work-tree":
			if hasInlineValue {
				if suffix == "=" {
					return nil, false
				}
				repositoryArgs = append(repositoryArgs, arg)
				continue
			}
			if i+1 >= branchIndex {
				return nil, false
			}
			value, ok := staticShellWordValue(args[i+1])
			if !ok {
				return nil, false
			}
			repositoryArgs = append(repositoryArgs, arg, value)
			i++
		case "-C":
			if hasInlineValue || i+1 >= branchIndex {
				return nil, false
			}
			value, ok := staticShellWordValue(args[i+1])
			if !ok {
				return nil, false
			}
			repositoryArgs = append(repositoryArgs, arg, value)
			i++
		default:
			return nil, false
		}
	}

	return repositoryArgs, true
}

func (p *GitParser) parsePushNoUpstream(cmd, stderr string, failed bool) itypes.ParserResult {
	if gitSubcommand(cmd) != "push" || gitCommandHasUpstreamFlag(cmd) {
		return itypes.ParserResult{Fixed: false}
	}
	call, err := parseShellCall(cmd)
	if err != nil || len(call.args) == 0 {
		return itypes.ParserResult{Fixed: false}
	}
	pushIndex := 0
	if gitPrefixedSubcommand(call.args[0].Lit()) != "push" {
		pushIndex = findShellSubcommandIndex(call.args, parserNameGit, gitParserOptionsWithValues)
		if pushIndex == -1 || call.args[pushIndex].Lit() != "push" {
			return itypes.ParserResult{Fixed: false}
		}
	}

	var currentBranch func() (string, bool)
	if failed {
		currentBranch = func() (string, bool) {
			// For the git-push form pushIndex is 0 and there are no selectors.
			repositoryArgs, ok := gitRepositorySelectorArgs(call.args, pushIndex)
			if !ok {
				return "", false
			}
			return p.gitCurrentBranch(repositoryArgs)
		}
	}
	remote, branch, ok := p.parsePushUpstreamHint(stderr, currentBranch)
	if !ok {
		return itypes.ParserResult{Fixed: false}
	}

	fixed, ok := addGitPushUpstream(call, pushIndex, remote, branch)
	if !ok {
		return itypes.ParserResult{Fixed: false}
	}

	return itypes.ParserResult{
		Fixed:   true,
		Command: fixed,
		Message: "adding push upstream tracking: " + remote + "/" + branch,
	}
}

// parsePushUpstreamHint reads git's untranslated "git push --set-upstream"
// suggestion. The English fatal line must name the same branch; when it is
// absent (localized git), currentBranch must confirm the hinted branch.
func (p *GitParser) parsePushUpstreamHint(stderr string, currentBranch func() (string, bool)) (string, string, bool) {
	allMatches := gitRegexes().pushNoUpstreamRegex.FindAllStringSubmatch(stderr, -1)
	if len(allMatches) != 1 {
		return "", "", false
	}
	remote, branch, ok := p.validateUpstreamTarget(allMatches[0][1], allMatches[0][2])
	if !ok {
		return "", "", false
	}

	if fatalMatches := gitRegexes().pushNoUpstreamFatalRegex.FindStringSubmatch(stderr); len(fatalMatches) >= 2 {
		return remote, branch, fatalMatches[1] == branch
	}
	if currentBranch == nil {
		return "", "", false
	}
	current, ok := currentBranch()
	return remote, branch, ok && current == branch
}

func (p *GitParser) validateUpstreamTarget(remote, branch string) (string, string, bool) {
	if remote == "" || branch == "" || strings.HasPrefix(remote, "-") || strings.HasPrefix(branch, "-") {
		return "", "", false
	}
	if gitRegexes().placeholderRegex.MatchString(remote) || gitRegexes().placeholderRegex.MatchString(branch) {
		return "", "", false
	}
	if !gitUpstreamTokenRegex.MatchString(remote) || !gitUpstreamTokenRegex.MatchString(branch) {
		return "", "", false
	}

	return remote, branch, true
}

// addGitPushUpstream adds --set-upstream to a push. A lone positional that
// names the hinted remote ("git push origin") is completed in place.
func addGitPushUpstream(call *shellCall, pushIndex int, remote, branch string) (string, bool) {
	if !gitUpstreamTokenRegex.MatchString(remote) || !gitUpstreamTokenRegex.MatchString(branch) {
		return "", false
	}
	upstream := "--set-upstream " + remote + " " + branch
	positional := -1
	for i := pushIndex + 1; i < len(call.args); i++ {
		arg, ok := staticShellWordValue(call.args[i])
		if !ok {
			return "", false
		}
		if gitPushUpstreamArgAllowed(arg) {
			continue
		}
		if positional != -1 || arg != remote {
			return "", false
		}
		positional = i
	}
	if positional != -1 {
		return call.replaceWord(positional, upstream), true
	}
	return call.insertAfterWord(len(call.args)-1, " "+upstream), true
}

// gitPushUpstreamArgAllowed reports whether a push flag leaves the push
// target untouched, so adding an upstream cannot change what is pushed.
func gitPushUpstreamArgAllowed(arg string) bool {
	switch arg {
	case "-v", "--verbose", "--no-verbose", "-q", "--quiet", "--no-quiet", "--progress", "--no-progress":
		return true
	case "-n", "--dry-run", "--no-dry-run", "--porcelain", "--no-porcelain":
		return true
	}
	return false
}

func gitSubcommand(cmd string) string {
	parts := shellCommandWords(cmd)
	return gitSubcommandFromWords(parts)
}

func gitSubcommandFromWords(parts []string) string {
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

func shellCommandWords(cmd string) []string {
	call, err := parseShellCall(cmd)
	if err == nil {
		return gitShellArgsLits(call.args)
	}
	return strings.Fields(cmd)
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
	for _, part := range shellCommandWords(cmd) {
		switch {
		case part == "--set-upstream", part == "--set-upstream-to":
			return true
		case strings.HasPrefix(part, "--set-upstream="), strings.HasPrefix(part, "--set-upstream-to="):
			return true
		}
	}

	return false
}

func gitShellArgsLits(args []*syntax.Word) []string {
	lits := make([]string, 0, len(args))
	for _, arg := range args {
		lits = append(lits, arg.Lit())
	}
	return lits
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
