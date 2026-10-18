package parser

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// Allow cold Git executables and wrappers time to start while bounding lookup latency.
const gitRepositoryQueryTimeout = 2 * time.Second

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
		args := appendGitRepositoryArgs(
			repositoryArgs,
			"symbolic-ref",
			"--quiet",
			"--short",
			"HEAD",
		)
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
