package e2e

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newRealGitWorkflowEnv(t *testing.T, branch string) (*e2eEnv, string, []string) {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	env := newE2EEnv(t)
	env.removeSubcommandCache(t)
	env.writeBinScript(t, "git", "#!/bin/sh\nexec \"$TYPO_TEST_GIT\" \"$@\"\n")
	extra := []string{
		"TYPO_TEST_GIT=" + normalizeShellExecPath(git),
		"LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=" + filepath.Join(env.home, ".gitconfig"),
		"GIT_TERMINAL_PROMPT=0",
	}
	runRealGitWorkflow(t, env, git, extra, "init", "--initial-branch="+branch)
	runRealGitWorkflow(t, env, git, extra, "config", "--global", "user.name", "Regression")
	runRealGitWorkflow(t, env, git, extra, "config", "--global", "user.email", "regression@example.invalid")
	runRealGitWorkflow(t, env, git, extra, "config", "--global", "commit.gpgsign", "false")
	runRealGitWorkflow(t, env, git, extra, "commit", "--allow-empty", "-m", "seed")
	return env, git, extra
}

func TestE2EZshGitPullNoUpstreamUsesCapturedFailure(t *testing.T) {
	zsh, lookupErr := exec.LookPath("zsh")
	if lookupErr != nil {
		t.Skip("zsh is not installed")
	}
	copyCommand, copyErr := exec.LookPath("cp")
	if copyErr != nil {
		t.Skip("cp is not installed")
	}
	const binding = "git pull --set-upstream origin main"
	for _, tt := range []struct {
		name, buffer, want, bind, changeDirectory string
	}{
		{"empty prompt", "", binding, "1", "0"},
		{"recalled failed command", "git pull", binding, "1", "0"},
		{"edited command ignores stale error", "git pull --quiet", "git pull --quiet", "0", "0"},
		{"changed directory ignores stale error", "git pull", "git pull", "0", "1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			env, git, extra := newRealGitWorkflowEnv(t, "main")
			remote := filepath.Join(env.tmpDir, "remote.git")
			runRealGitWorkflow(t, env, git, extra, "init", "--bare", "--initial-branch=main", remote)
			runRealGitWorkflow(t, env, git, extra, "remote", "add", "origin", remote)
			runRealGitWorkflow(t, env, git, extra, "push", "origin", "main")
			seed := filepath.Join(env.tmpDir, "remote-seed")
			runRealGitWorkflow(t, env, git, extra, "clone", remote, seed)
			runRealGitWorkflow(t, env, git, extra, "-C", seed, "commit", "--allow-empty", "-m", "remote advance")
			runRealGitWorkflow(t, env, git, extra, "-C", seed, "push", "origin", "main")
			initScript := env.initZshScript(t)
			extra = append(extra, "COPY_COMMAND="+normalizeShellExecPath(copyCommand), "PULL_BUFFER="+tt.buffer, "EXPECTED_BUFFER="+tt.want, "EXPECT_BINDING="+tt.bind, "CHANGE_DIRECTORY="+tt.changeDirectory)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			// Exercise the embedded shell integration with Git's real stderr, not
			// a parser fixture or a manually supplied typo fix -s argument.
			cmd := exec.CommandContext(ctx, zsh, "-f", "-c", `
zle() { true; }
bindkey() { true; }
source "$1"
print -s -- 'git pull'
_typo_preexec 'git pull'
git pull >/dev/null
_typo_precmd
[[ "$TYPO_LAST_EXIT_CODE" -ne 0 ]] || exit 61

# Wait for the async stderr tee to deliver the complete upstream hint.
deadline=$((SECONDS + 5))
while [[ "$(<"$TYPO_STDERR_CACHE")" != *'git branch --set-upstream-to=origin/<branch> main'* ]]; do
    (( SECONDS >= deadline )) && exit 62
    sleep 0.01
done
repo="$PWD"
head="$(git rev-parse HEAD)"
"$COPY_COMMAND" .git/index "$TMPDIR/index.before" || exit 71
"$COPY_COMMAND" .git/FETCH_HEAD "$TMPDIR/fetch.before" || exit 72
if [[ "$CHANGE_DIRECTORY" == 1 ]]; then cd "$HOME"; fi
BUFFER="$PULL_BUFFER"
_typo_fix_command
[[ "$BUFFER" == "$EXPECTED_BUFFER" ]] || { print -r -- "unexpected buffer: $BUFFER"; exit 63; }
git -C "$repo" rev-parse --abbrev-ref '@{upstream}' >/dev/null 2>&1 && exit 64
if [[ "$EXPECT_BINDING" == 1 ]]; then
    [[ "$(git rev-parse HEAD)" == "$head" ]] || exit 67
    command cmp .git/index "$TMPDIR/index.before" || exit 68
    command cmp .git/FETCH_HEAD "$TMPDIR/fetch.before" || exit 69
    # Execute the actual widget result through zsh, not reconstructed argv.
    eval "$BUFFER" >/dev/null || exit 65
    [[ "$(git rev-parse --abbrev-ref '@{upstream}')" == origin/main ]] || exit 66
    [[ "$(git rev-parse HEAD)" != "$head" ]] || exit 73
    [[ "$(git rev-parse HEAD)" == "$(git rev-parse origin/main)" ]] || exit 74
    git pull --ff-only >/dev/null || exit 70
fi
print -r -- "$BUFFER"
`, "zsh", initScript)
			cmd.Dir = env.root
			cmd.Env = env.commandEnv(extra...)
			output, runErr := cmd.CombinedOutput()
			if runErr != nil {
				t.Fatalf("missing-upstream shell workflow failed: %v\n%s", runErr, output)
			}
		})
	}
}

func runRealGitWorkflow(t *testing.T, env *e2eEnv, git string, extra []string, args ...string) string {
	t.Helper()
	cmd := exec.Command(git, args...)
	cmd.Dir = env.root
	cmd.Env = env.commandEnv(extra...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %q failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func readGitWorkflowFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestE2EGitDivergentPullExecutesRebase(t *testing.T) {
	for _, forcedUpdate := range []bool{false, true} {
		name := "divergent branches"
		if forcedUpdate {
			name = "forced remote update"
		}
		t.Run(name, func(t *testing.T) {
			env, git, extra, stderrFile := newDivergentGitWorkflowEnv(t, forcedUpdate)
			head := runRealGitWorkflow(t, env, git, extra, "rev-parse", "HEAD")
			tracked := runRealGitWorkflow(t, env, git, extra, "rev-parse", "origin/main")
			gitDir := filepath.Join(env.root, ".git")
			snapshots := make(map[string][]byte)
			for _, file := range []string{"config", "index", "FETCH_HEAD"} {
				snapshots[file] = readGitWorkflowFile(t, filepath.Join(gitDir, file))
			}
			if result := env.run(t, "config", "set", "candidates.enabled", "true"); result.code != 0 {
				t.Fatalf("failed to enable candidate selection: %+v", result)
			}
			for _, selectMode := range []bool{false, true} {
				args := []string{"fix", "--no-history", "--exit-code", "128", "-s", stderrFile}
				if selectMode {
					args = append(args, "--select")
				}
				result := env.runWithEnv(t, extra, append(args, "git pull")...)
				assertE2EStdoutEquals(t, result, "git pull --rebase\n", "divergent pull did not suggest rebase")
			}
			assertZshDivergentPullCorrection(t, env, extra, stderrFile)
			if after := runRealGitWorkflow(t, env, git, extra, "rev-parse", "HEAD"); after != head {
				t.Fatalf("correction modified local history: %s -> %s", head, after)
			}
			if after := runRealGitWorkflow(t, env, git, extra, "rev-parse", "origin/main"); after != tracked {
				t.Fatalf("correction modified the tracking branch: %s -> %s", tracked, after)
			}
			for file, before := range snapshots {
				if !bytes.Equal(before, readGitWorkflowFile(t, filepath.Join(gitDir, file))) {
					t.Fatalf("correction modified .git/%s", file)
				}
			}
			// Execute the actual CLI output and verify that local work survives.
			result := env.runWithEnv(t, extra, "fix", "--no-history", "--exit-code", "128", "-s", stderrFile, "git pull")
			executeGitCorrection(t, env, extra, result)
			runRealGitWorkflow(t, env, git, extra, "merge-base", "--is-ancestor", tracked, "HEAD")
			if subject := runRealGitWorkflow(t, env, git, extra, "log", "-1", "--format=%s"); subject != "local change" {
				t.Fatalf("rebase lost the local commit: %q", subject)
			}
			for _, file := range []string{"local.txt", "remote.txt"} {
				if got := string(readGitWorkflowFile(t, filepath.Join(env.root, file))); got != file+"\n" {
					t.Fatalf("rebase lost file %s: %q", file, got)
				}
			}
			if !bytes.Equal(snapshots["config"], readGitWorkflowFile(t, filepath.Join(gitDir, "config"))) {
				t.Fatal("one-time rebase changed repository configuration")
			}
			runRealGitWorkflow(t, env, git, extra, "pull", "--ff-only")
		})
	}
}

func newDivergentGitWorkflowEnv(t *testing.T, forcedUpdate bool) (*e2eEnv, string, []string, string) {
	t.Helper()
	env, git, extra := newRealGitWorkflowEnv(t, "main")
	root := runRealGitWorkflow(t, env, git, extra, "rev-parse", "HEAD")
	remote := filepath.Join(env.tmpDir, "remote.git")
	runRealGitWorkflow(t, env, git, extra, "init", "--bare", "--initial-branch=main", remote)
	runRealGitWorkflow(t, env, git, extra, "remote", "add", "origin", remote)
	runRealGitWorkflow(t, env, git, extra, "commit", "--allow-empty", "-m", "old remote history")
	runRealGitWorkflow(t, env, git, extra, "push", "--set-upstream", "origin", "main")
	seed := filepath.Join(env.tmpDir, "remote-seed")
	runRealGitWorkflow(t, env, git, extra, "clone", remote, seed)
	if err := os.WriteFile(filepath.Join(env.root, "local.txt"), []byte("local.txt\n"), 0600); err != nil {
		t.Fatal(err)
	}
	runRealGitWorkflow(t, env, git, extra, "add", "local.txt")
	runRealGitWorkflow(t, env, git, extra, "commit", "-m", "local change")
	if forcedUpdate {
		// Rewrite only the disposable local remote; never touch a public repository.
		runRealGitWorkflow(t, env, git, extra, "-C", seed, "switch", "-c", "rewritten", root)
	}
	if err := os.WriteFile(filepath.Join(seed, "remote.txt"), []byte("remote.txt\n"), 0600); err != nil {
		t.Fatal(err)
	}
	runRealGitWorkflow(t, env, git, extra, "-C", seed, "add", "remote.txt")
	runRealGitWorkflow(t, env, git, extra, "-C", seed, "commit", "-m", "remote change")
	pushArgs := []string{"-C", seed, "push"}
	if forcedUpdate {
		pushArgs = append(pushArgs, "--force")
	}
	runRealGitWorkflow(t, env, git, extra, append(pushArgs, "origin", "HEAD:main")...)

	failed := exec.Command(git, "pull")
	failed.Dir = env.root
	failed.Env = env.commandEnv(extra...)
	stderr, err := failed.CombinedOutput()
	if err == nil || !strings.Contains(string(stderr), "Need to specify how to reconcile divergent branches") {
		t.Fatalf("expected a real divergent-pull error: %v\n%s", err, stderr)
	}
	if forcedUpdate && !strings.Contains(string(stderr), "forced update") {
		t.Fatalf("expected a real forced-update notice, got %s", stderr)
	}
	return env, git, extra, env.writeTempFile(t, "divergent-pull.stderr", string(stderr))
}

func assertZshDivergentPullCorrection(t *testing.T, env *e2eEnv, extra []string, stderrFile string) {
	t.Helper()
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Log("zsh is not installed; skipping the shell buffer assertion")
		return
	}
	initScript := env.initZshScript(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	// Supply the real Git failure to the widget without re-running the pull.
	cmd := exec.CommandContext(ctx, zsh, "-f", "-c", `
zle() { true; }
bindkey() { true; }
source "$1"
TYPO_STDERR_CACHE="$2"
_TYPO_LAST_COMMAND='git pull'
TYPO_LAST_EXIT_CODE=128
_TYPO_LAST_COMMAND_PWD="$PWD"
print -s -- 'git pull'
for BUFFER in '' 'git pull'; do
    _typo_fix_command
    [[ "$BUFFER" == "git pull --rebase" ]] || { print -r -- "unexpected buffer: $BUFFER"; exit 61; }
done
`, "zsh", initScript, stderrFile)
	cmd.Dir = env.root
	cmd.Env = env.commandEnv(extra...)
	if output, runErr := cmd.CombinedOutput(); runErr != nil {
		t.Fatalf("divergent-pull shell workflow failed: %v\n%s", runErr, output)
	}
}

func TestE2EGitAliasCorrectionDoesNotExecuteAlias(t *testing.T) {
	for _, alias := range []string{"commit", "commit -s -m", "!printf executed > \"$HOME/alias-executed\""} {
		t.Run(alias, func(t *testing.T) {
			env, git, extra := newRealGitWorkflowEnv(t, "main")
			runRealGitWorkflow(t, env, git, extra, "config", "--global", "alias.c", alias)
			if err := os.WriteFile(filepath.Join(env.root, "staged.txt"), []byte("staged content\n"), 0600); err != nil {
				t.Fatal(err)
			}
			runRealGitWorkflow(t, env, git, extra, "add", "staged.txt")
			head := runRealGitWorkflow(t, env, git, extra, "rev-parse", "HEAD")
			indexPath := filepath.Join(env.root, ".git", "index")
			index := readGitWorkflowFile(t, indexPath)
			for _, command := range []string{`gti c -m "xxxx"`, `gti c -m "message with spaces"`, `git c -m "xxxx"`} {
				got := env.runWithEnv(t, extra, "fix", "--no-history", command)
				if strings.HasPrefix(command, "gti ") {
					assertE2EStdoutEquals(t, got, strings.Replace(command, "gti ", "git ", 1)+"\n", "Git alias arguments were changed")
				} else if got.code != 1 || got.stdout != "" {
					t.Fatalf("valid alias invocation should remain unchanged: %+v", got)
				}
				if after := runRealGitWorkflow(t, env, git, extra, "rev-parse", "HEAD"); after != head {
					t.Fatalf("correction committed staged files: HEAD=%s -> %s, message=%q", head, after, runRealGitWorkflow(t, env, git, extra, "log", "-1", "--format=%B"))
				}
				if !bytes.Equal(index, readGitWorkflowFile(t, indexPath)) {
					t.Fatal("correction modified the Git index")
				}
				if _, err := os.Stat(filepath.Join(env.home, "alias-executed")); !os.IsNotExist(err) {
					t.Fatalf("shell alias was executed during correction: %v", err)
				}
			}
		})
	}
}

func TestE2EGitPullCorrectionSetsUpstreamAndPulls(t *testing.T) {
	for _, tt := range []struct{ remote, branch string }{{"origin", "main"}, {"upstream", "feature/topic"}} {
		t.Run(tt.remote+"/"+tt.branch, func(t *testing.T) {
			env, git, extra := newRealGitWorkflowEnv(t, tt.branch)
			remote := filepath.Join(env.tmpDir, "remote.git")
			runRealGitWorkflow(t, env, git, extra, "init", "--bare", "--initial-branch="+tt.branch, remote)
			runRealGitWorkflow(t, env, git, extra, "remote", "add", tt.remote, remote)
			runRealGitWorkflow(t, env, git, extra, "push", tt.remote, tt.branch)
			failed := exec.Command(git, "pull")
			failed.Dir = env.root
			failed.Env = env.commandEnv(extra...)
			stderr, err := failed.CombinedOutput()
			if err == nil || !strings.Contains(string(stderr), "There is no tracking information") {
				t.Fatalf("expected a real missing-upstream error: %v\n%s", err, stderr)
			}
			stderrFile := env.writeTempFile(t, "pull.stderr", string(stderr))

			// Advance the remote after the failure; the correction must fetch and integrate it.
			seed := filepath.Join(env.tmpDir, "remote-seed")
			runRealGitWorkflow(t, env, git, extra, "clone", remote, seed)
			runRealGitWorkflow(t, env, git, extra, "-C", seed, "commit", "--allow-empty", "-m", "remote advance")
			runRealGitWorkflow(t, env, git, extra, "-C", seed, "push", "origin", tt.branch)
			head := runRealGitWorkflow(t, env, git, extra, "rev-parse", "HEAD")
			indexPath := filepath.Join(env.root, ".git", "index")
			index := readGitWorkflowFile(t, indexPath)
			fetchPath := filepath.Join(env.root, ".git", "FETCH_HEAD")
			fetch := readGitWorkflowFile(t, fetchPath)
			target := tt.remote + "/" + tt.branch
			tracking := runRealGitWorkflow(t, env, git, extra, "rev-parse", target)
			suffix := " --set-upstream " + tt.remote + " " + tt.branch
			want := "git pull" + suffix
			var pullResult e2eResult
			for _, command := range []struct{ input, output string }{
				{"git pull", want},
				{"git pull --rebase --quiet", "git pull --rebase --quiet" + suffix},
				{"git -C '" + env.root + "' pull --quiet", "git -C '" + env.root + "' pull --quiet" + suffix},
				{"git pull && echo ready", want + " && echo ready"},
				{"echo ready && git pull", "echo ready && " + want},
				{"sudo git pull", "sudo " + want},
			} {
				got := env.runWithEnv(t, extra, "fix", "--debug", "--no-history", "--exit-code", "1", "-s", stderrFile, command.input)
				assertE2EStdoutEquals(t, got, command.output+"\n", "pull correction failed for "+command.input)
				if command.input == "git pull" {
					pullResult = got
				}
			}
			unbound := exec.Command(git, "rev-parse", "--abbrev-ref", "@{upstream}")
			unbound.Dir = env.root
			unbound.Env = env.commandEnv(extra...)
			if err := unbound.Run(); err == nil {
				t.Fatal("generating the correction already changed the upstream")
			}
			// Generating suggestions must not execute the pull.
			if after := runRealGitWorkflow(t, env, git, extra, "rev-parse", "HEAD"); after != head {
				t.Fatalf("suggestion changed HEAD: %s -> %s", head, after)
			}
			if !bytes.Equal(index, readGitWorkflowFile(t, indexPath)) || !bytes.Equal(fetch, readGitWorkflowFile(t, fetchPath)) {
				t.Fatal("suggestion changed the index or FETCH_HEAD")
			}
			if after := runRealGitWorkflow(t, env, git, extra, "rev-parse", target); after != tracking {
				t.Fatalf("suggestion fetched the remote: %s -> %s", tracking, after)
			}
			executeGitCorrection(t, env, extra, pullResult)
			if upstream := runRealGitWorkflow(t, env, git, extra, "rev-parse", "--abbrev-ref", "@{upstream}"); upstream != target {
				t.Fatalf("upstream = %q, want %q", upstream, target)
			}
			remoteHead := runRealGitWorkflow(t, env, git, extra, "-C", seed, "rev-parse", "HEAD")
			if after := runRealGitWorkflow(t, env, git, extra, "rev-parse", "HEAD"); after != remoteHead || after == head {
				t.Fatalf("corrected pull did not advance to the remote: HEAD=%s, remote=%s, before=%s", after, remoteHead, head)
			}
		})
	}
}

func TestE2EGitPullRepairsMissingUpstreamThenDivergence(t *testing.T) {
	env, git, extra, _ := newDivergentGitWorkflowEnv(t, false)
	runRealGitWorkflow(t, env, git, extra, "branch", "--unset-upstream")
	command := "git pull"
	for _, step := range []struct{ signal, want string }{
		{"There is no tracking information", "git pull --set-upstream origin main"},
		{"Need to specify how to reconcile divergent branches", "git pull --rebase --set-upstream origin main"},
	} {
		failed := exec.Command("sh", "-c", command)
		failed.Dir = env.root
		failed.Env = env.commandEnv(extra...)
		stderr, err := failed.CombinedOutput()
		if err == nil || !strings.Contains(string(stderr), step.signal) {
			t.Fatalf("expected %q from %q: %v\n%s", step.signal, command, err, stderr)
		}
		stderrFile := env.writeTempFile(t, "pull-step.stderr", string(stderr))
		result := env.runWithEnv(t, extra, "fix", "--no-history", "--exit-code", "1", "-s", stderrFile, command)
		assertE2EStdoutEquals(t, result, step.want+"\n", "pull repair chain lost the original operation")
		command = strings.TrimSpace(result.stdout)
	}
	executeGitCorrection(t, env, extra, e2eResult{stdout: command})
	runRealGitWorkflow(t, env, git, extra, "merge-base", "--is-ancestor", "origin/main", "HEAD")
	if subject := runRealGitWorkflow(t, env, git, extra, "log", "-1", "--format=%s"); subject != "local change" {
		t.Fatalf("combined repair lost the local commit: %q", subject)
	}
	if upstream := runRealGitWorkflow(t, env, git, extra, "rev-parse", "--abbrev-ref", "@{upstream}"); upstream != "origin/main" {
		t.Fatalf("combined repair lost upstream tracking: %q", upstream)
	}
}

// executeGitCorrection runs exactly the command emitted by the CLI in an isolated repository.
func executeGitCorrection(t *testing.T, env *e2eEnv, extra []string, result e2eResult) {
	t.Helper()
	if result.code != 0 || strings.TrimSpace(result.stdout) == "" {
		t.Fatalf("expected an executable correction: %+v", result)
	}
	cmd := exec.Command("sh", "-c", strings.TrimSpace(result.stdout))
	cmd.Dir = env.root
	cmd.Env = env.commandEnv(extra...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("correction %q failed: %v\n%s", result.stdout, err, output)
	}
}
