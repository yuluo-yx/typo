package parser

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

const gitRepositoryQueryTimeout = 500 * time.Millisecond

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
	if localBranch != "" {
		localRef := "refs/heads/" + branch
		if !gitRefExists(ctx, repositoryArgs, localRef, run) {
			return "", false
		}
	}

	upstream := remote + "/" + branch
	remoteRef := "refs/remotes/" + upstream
	if !gitRefExists(ctx, repositoryArgs, remoteRef, run) {
		return "", false
	}

	return upstream, true
}

func gitRefExists(
	ctx context.Context,
	repositoryArgs []string,
	ref string,
	run gitCommandRunner,
) bool {
	args := appendGitRepositoryArgs(
		repositoryArgs,
		"show-ref",
		"--verify",
		"--quiet",
		ref,
	)
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
