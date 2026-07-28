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
	resolveBranchUpstream      gitBranchUpstreamResolver
}

var gitUpstreamTokenRegex = regexp.MustCompile(`^[A-Za-z0-9._][A-Za-z0-9._/@+-]*$`)

// NewGitParser creates a new GitParser.
func NewGitParser() *GitParser {
	return &GitParser{
		didYouMeanRegex:            regexp.MustCompile(`(?s)git: '([^']+)' is not a git command\..*The most similar commands? (?:is|are)\s+(\w+)`),
		pullNoTrackingRegex:        regexp.MustCompile(`(?m)^There is no tracking information for the current branch\.[\t\r ]*$`),
		pullSetUpstreamRegex:       regexp.MustCompile(`(?m)^[ \t]*git branch --set-upstream-to(?:=|[ \t]+)([^\s]+)(?:[ \t]+([^\s]+))?[\t\r ]*$`),
		branchUpstreamMissingRegex: regexp.MustCompile(`(?m)^fatal: the requested upstream branch '([^'\r\n]+)' does not exist[\t\r ]*$`),
		pushNoUpstreamFatalRegex:   regexp.MustCompile(`(?m)^fatal: The current branch ([^\s]+) has no upstream branch\.[\t\r ]*$`),
		pushNoUpstreamRegex:        regexp.MustCompile(`(?m)^[ \t]*git push (?:-u|--set-upstream)[ \t]+([^\s]+)[ \t]+([^\s]+)[ \t\r]*$`),
		divergentBranchesRegex:     regexp.MustCompile(`(?i)You have divergent branches and need to specify how to reconcile them\.`),
		reconcileDivergenceRegex:   regexp.MustCompile(`(?i)fatal: Need to specify how to reconcile divergent branches\.`),
		placeholderRegex:           regexp.MustCompile(`^<[^>\s]+>$`),
		notGitRepoRegex:            regexp.MustCompile(`fatal: not a git repository`),
		resolveBranchUpstream:      resolveGitBranchUpstream,
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

	if result := p.parsePullNoTracking(cmd, stderr); result.Fixed {
		return result
	}

	if result := p.parseBranchSetUpstreamMissing(cmd, stderr); result.Fixed {
		return result
	}

	if result := p.parsePushNoUpstream(cmd, stderr); result.Fixed {
		return result
	}

	if result := p.parseDivergentPullRebase(cmd, stderr); result.Fixed {
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

func (p *GitParser) parsePullNoTracking(cmd, stderr string) itypes.ParserResult {
	if !p.pullNoTrackingRegex.MatchString(stderr) || gitCommandHasUpstreamFlag(cmd) {
		return itypes.ParserResult{Fixed: false}
	}

	call, err := parseShellCall(cmd)
	if err != nil {
		return itypes.ParserResult{Fixed: false}
	}
	pullIndex, repositoryArgs, ok := gitPullCommandContext(call.args)
	if !ok || !gitPullUpstreamArgsAllowed(call.args[pullIndex+1:]) {
		return itypes.ParserResult{Fixed: false}
	}

	remote, branch, ok := p.parsePullUpstreamHint(stderr)
	if !ok {
		return itypes.ParserResult{Fixed: false}
	}
	target, ok := p.resolveBranchUpstream(repositoryArgs, remote, branch)
	if !ok || target != remote+"/"+branch {
		return itypes.ParserResult{Fixed: false}
	}

	insertion := " --set-upstream " + remote + " " + branch
	return itypes.ParserResult{
		Fixed:   true,
		Command: call.insertAfterWord(len(call.args)-1, insertion),
		Message: "adding upstream tracking: " + target,
	}
}

func (p *GitParser) parsePullUpstreamHint(stderr string) (string, string, bool) {
	allMatches := p.pullSetUpstreamRegex.FindAllStringSubmatch(stderr, -1)
	if len(allMatches) != 1 || len(allMatches[0]) < 3 {
		return "", "", false
	}
	matches := allMatches[0]

	target := matches[1]
	localBranch := matches[2]
	if localBranch == "" || p.placeholderRegex.MatchString(localBranch) {
		return "", "", false
	}

	remote, upstreamBranch, ok := strings.Cut(target, "/")
	if !ok || p.placeholderRegex.MatchString(remote) {
		return "", "", false
	}
	if p.placeholderRegex.MatchString(upstreamBranch) {
		upstreamBranch = localBranch
	}
	remote, upstreamBranch, ok = p.validateUpstreamTarget(remote, upstreamBranch)
	if !ok || upstreamBranch != localBranch {
		return "", "", false
	}

	return remote, localBranch, true
}

func gitPullCommandContext(args []*syntax.Word) (int, []string, bool) {
	if len(args) == 0 {
		return 0, nil, false
	}
	if gitPrefixedSubcommand(args[0].Lit()) == "pull" {
		return 0, nil, true
	}

	pullIndex := findShellSubcommandIndex(args, parserNameGit, gitParserOptionsWithValues)
	if pullIndex == -1 || args[pullIndex].Lit() != "pull" {
		return 0, nil, false
	}
	repositoryArgs, ok := gitRepositorySelectorArgs(args, pullIndex)
	if !ok {
		return 0, nil, false
	}
	return pullIndex, repositoryArgs, true
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

func (p *GitParser) parseBranchSetUpstreamMissing(cmd, stderr string) itypes.ParserResult {
	matches := p.branchUpstreamMissingRegex.FindStringSubmatch(stderr)
	if len(matches) < 2 {
		return itypes.ParserResult{Fixed: false}
	}

	call, err := parseShellCall(cmd)
	if err != nil {
		return itypes.ParserResult{Fixed: false}
	}
	request, ok := parseGitBranchUpstreamRequest(call)
	if !ok || request.remote != matches[1] {
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

	branchIndex, repositoryArgs, ok := gitBranchCommandContext(call.args)
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

func gitBranchCommandContext(args []*syntax.Word) (int, []string, bool) {
	if gitPrefixedSubcommand(args[0].Lit()) == "branch" {
		return 0, nil, true
	}

	branchIndex := findShellSubcommandIndex(args, parserNameGit, gitParserOptionsWithValues)
	if branchIndex == -1 || args[branchIndex].Lit() != "branch" {
		return 0, nil, false
	}
	repositoryArgs, ok := gitRepositorySelectorArgs(args, branchIndex)
	if !ok {
		return 0, nil, false
	}
	return branchIndex, repositoryArgs, true
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
		case arg == "--quiet" || arg == "-q":
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

func (p *GitParser) parsePushNoUpstream(cmd, stderr string) itypes.ParserResult {
	if gitSubcommand(cmd) != "push" || gitCommandHasUpstreamFlag(cmd) {
		return itypes.ParserResult{Fixed: false}
	}

	remote, branch, ok := p.parsePushUpstreamHint(stderr)
	if !ok {
		return itypes.ParserResult{Fixed: false}
	}

	fixed, ok := addGitPushUpstream(cmd, remote, branch)
	if !ok {
		return itypes.ParserResult{Fixed: false}
	}

	return itypes.ParserResult{
		Fixed:   true,
		Command: fixed,
		Message: "adding push upstream tracking: " + remote + "/" + branch,
	}
}

func (p *GitParser) parsePushUpstreamHint(stderr string) (string, string, bool) {
	fatalMatches := p.pushNoUpstreamFatalRegex.FindStringSubmatch(stderr)
	matches := p.pushNoUpstreamRegex.FindStringSubmatch(stderr)
	if len(fatalMatches) < 2 || len(matches) < 3 || fatalMatches[1] != matches[2] {
		return "", "", false
	}
	return p.validateUpstreamTarget(matches[1], matches[2])
}

func (p *GitParser) validateUpstreamTarget(remote, branch string) (string, string, bool) {
	if remote == "" || branch == "" || strings.HasPrefix(remote, "-") || strings.HasPrefix(branch, "-") {
		return "", "", false
	}
	if p.placeholderRegex.MatchString(remote) || p.placeholderRegex.MatchString(branch) {
		return "", "", false
	}
	if !gitUpstreamTokenRegex.MatchString(remote) || !gitUpstreamTokenRegex.MatchString(branch) {
		return "", "", false
	}

	return remote, branch, true
}

func addGitPushUpstream(cmd, remote, branch string) (string, bool) {
	if !gitUpstreamTokenRegex.MatchString(remote) || !gitUpstreamTokenRegex.MatchString(branch) {
		return "", false
	}
	call, err := parseShellCall(cmd)
	if err != nil || len(call.args) == 0 {
		return "", false
	}
	insertion := " --set-upstream " + remote + " " + branch
	if gitPrefixedSubcommand(call.args[0].Lit()) == "push" {
		if !gitPushUpstreamArgsAllowed(gitShellArgsLits(call.args[1:])) {
			return "", false
		}
		return call.insertAfterWord(len(call.args)-1, insertion), true
	}

	index := findShellSubcommandIndex(call.args, parserNameGit, gitParserOptionsWithValues)
	if index == -1 || call.args[index].Lit() != "push" ||
		!gitPushUpstreamArgsAllowed(gitShellArgsLits(call.args[index+1:])) {
		return "", false
	}
	return call.insertAfterWord(len(call.args)-1, insertion), true
}

func gitPushUpstreamArgsAllowed(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "-v", "--verbose", "--no-verbose", "-q", "--quiet", "--no-quiet", "--progress", "--no-progress":
			continue
		case "-n", "--dry-run", "--no-dry-run", "--porcelain", "--no-porcelain":
			continue
		}
		return false
	}
	return true
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

func gitCommandHasPullReconcileFlag(cmd string) bool {
	for _, part := range shellCommandWords(cmd) {
		switch {
		case part == "--rebase", part == "--no-rebase", part == "--ff-only":
			return true
		case strings.HasPrefix(part, "--rebase="):
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

func addGitPullRebaseFlag(cmd string) (string, bool) {
	call, err := parseShellCall(cmd)
	if err != nil || len(call.args) == 0 {
		return "", false
	}

	if gitPrefixedSubcommand(call.args[0].Lit()) == "pull" {
		return call.insertAfterWord(0, " --rebase"), true
	}

	index := findShellSubcommandIndex(call.args, parserNameGit, gitParserOptionsWithValues)
	if index != -1 && call.args[index].Lit() == "pull" {
		return call.insertAfterWord(index, " --rebase"), true
	}

	return "", false
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
