package parser

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// Allow cold Git executables and wrappers time to start while bounding lookup latency.
const gitRepositoryQueryTimeout = 2 * time.Second

// gitCurrentBranchArgs prints the checked-out branch and fails on detached HEAD.
var gitCurrentBranchArgs = []string{"symbolic-ref", "--quiet", "--short", "HEAD"}

type gitBranchUpstreamResolver func(repositoryArgs []string, remote, localBranch string) (string, bool)

type gitCommandRunner func(ctx context.Context, args []string) ([]byte, error)

func resolveGitBranchUpstream(repositoryArgs []string, remote, localBranch string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), gitRepositoryQueryTimeout)
	defer cancel()

	return resolveGitBranchUpstreamWithRunner(
		ctx,
		repositoryArgs,
		remote,
		localBranch,
		runGitCommand,
	)
}

func resolveGitBranchUpstreamWithRunner(
	ctx context.Context,
	repositoryArgs []string,
	remote string,
	localBranch string,
	run gitCommandRunner,
) (string, bool) {
	branch := localBranch
	if branch == "" {
		args := appendGitRepositoryArgs(repositoryArgs, gitCurrentBranchArgs...)
		output, err := run(ctx, args)
		if err != nil {
			return "", false
		}
		branch = strings.TrimSpace(string(output))
	}

	if !gitUpstreamTokenRegex.MatchString(remote) ||
		!gitUpstreamTokenRegex.MatchString(branch) {
		return "", false
	}
	upstream := remote + "/" + branch
	refs := make([]string, 0, 2)
	if localBranch != "" {
		refs = append(refs, "refs/heads/"+branch)
	}
	refs = append(refs, "refs/remotes/"+upstream)
	// show-ref --verify fails if any requested ref is absent. Check both refs
	// in one process so startup cost is paid once for an explicit branch.
	if !gitRefsExist(ctx, repositoryArgs, refs, run) {
		return "", false
	}

	return upstream, true
}

func gitRefsExist(
	ctx context.Context,
	repositoryArgs []string,
	refs []string,
	run gitCommandRunner,
) bool {
	args := appendGitRepositoryArgs(
		repositoryArgs,
		"show-ref",
		"--verify",
		"--quiet",
	)
	args = append(args, refs...)
	if _, err := run(ctx, args); err != nil {
		return false
	}
	return true
}

func appendGitRepositoryArgs(repositoryArgs []string, args ...string) []string {
	result := make([]string, 0, len(repositoryArgs)+len(args))
	result = append(result, repositoryArgs...)
	return append(result, args...)
}

func runGitCommand(ctx context.Context, args []string) ([]byte, error) {
	return exec.CommandContext(ctx, parserNameGit, args...).Output()
}

// gitQuery runs one read-only git command and returns its trimmed output.
func (p *GitParser) gitQuery(repositoryArgs []string, args ...string) (string, bool) {
	run := p.runGit
	if run == nil {
		run = runGitCommand
	}
	ctx, cancel := context.WithTimeout(context.Background(), gitRepositoryQueryTimeout)
	defer cancel()

	output, err := run(ctx, appendGitRepositoryArgs(repositoryArgs, args...))
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(output)), true
}

// gitCurrentBranch returns the checked-out branch; detached HEAD reports false.
func (p *GitParser) gitCurrentBranch(repositoryArgs []string) (string, bool) {
	branch, ok := p.gitQuery(repositoryArgs, gitCurrentBranchArgs...)
	return branch, ok && branch != ""
}

// gitCurrentUpstream returns the upstream of the checked-out branch, such as origin/main.
func (p *GitParser) gitCurrentUpstream(repositoryArgs []string) (string, bool) {
	upstream, ok := p.gitQuery(repositoryArgs, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	return upstream, ok && upstream != ""
}

// gitConfigValue returns a configured value; unset keys report false.
func (p *GitParser) gitConfigValue(repositoryArgs []string, key string) (string, bool) {
	value, ok := p.gitQuery(repositoryArgs, "config", "--get", key)
	return value, ok && value != ""
}

// gitBranchNames lists local branch names plus remote branch names without
// their remote prefix, which checkout and switch resolve by name.
func (p *GitParser) gitBranchNames(repositoryArgs []string) ([]string, bool) {
	output, ok := p.gitQuery(repositoryArgs, "for-each-ref", "--format=%(refname)", "refs/heads", "refs/remotes")
	if !ok {
		return nil, false
	}

	seen := make(map[string]bool)
	names := make([]string, 0)
	for _, ref := range strings.Split(output, "\n") {
		ref = strings.TrimSpace(ref)
		var name string
		if local, ok := strings.CutPrefix(ref, "refs/heads/"); ok {
			name = local
		} else if remote, ok := strings.CutPrefix(ref, "refs/remotes/"); ok {
			_, name, _ = strings.Cut(remote, "/")
		}
		if name == "" || name == "HEAD" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names, true
}
