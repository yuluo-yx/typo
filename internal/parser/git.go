package parser

import (
	"regexp"
	"strings"
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
func (p *GitParser) Parse(ctx Context) Result {
	cmd := ctx.Command
	stderr := ctx.Stderr

	// Check if it's a git command
	if !isGitCommand(cmd) {
		return Result{Fixed: false}
	}

	// Try to parse "did you mean" errors
	if result := p.parseDidYouMean(cmd, stderr); result.Fixed {
		return result
	}

	// Try to parse "no upstream" errors
	if result := p.parseNoUpstream(cmd, stderr); result.Fixed {
		return result
	}

	return Result{Fixed: false}
}

func (p *GitParser) parseDidYouMean(cmd, stderr string) Result {
	matches := p.didYouMeanRegex.FindStringSubmatch(stderr)
	if len(matches) < 3 {
		return Result{Fixed: false}
	}

	wrongCmd := matches[1]
	suggested := matches[2]

	// Replace the wrong command with the suggested one
	fixed := strings.Replace(cmd, wrongCmd, suggested, 1)

	return Result{
		Fixed:   true,
		Command: fixed,
		Message: "git suggested: " + suggested,
	}
}

func (p *GitParser) parseNoUpstream(cmd, stderr string) Result {
	matches := p.noUpstreamRegex.FindStringSubmatch(stderr)
	if len(matches) < 3 {
		return Result{Fixed: false}
	}

	remote := matches[1]
	upstreamBranch := matches[2]
	localBranch := ""
	if len(matches) >= 4 {
		localBranch = matches[3]
	}

	branch := upstreamBranch
	if p.placeholderRegex.MatchString(branch) && localBranch != "" {
		branch = localBranch
	}
	if p.placeholderRegex.MatchString(branch) {
		return Result{Fixed: false}
	}

	// Add --set-upstream flag to the command
	return Result{
		Fixed:   true,
		Command: cmd + " --set-upstream " + remote + " " + branch,
		Message: "adding upstream tracking: " + remote + "/" + branch,
	}
}

func isGitCommand(cmd string) bool {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return false
	}
	return parts[0] == "git" || strings.HasPrefix(parts[0], "git-")
}
