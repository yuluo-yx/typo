package parser

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	itypes "github.com/yuluo-yx/typo/internal/types"
)

// Git 2.49 stderr captured from real repositories under LC_ALL=C and zh_CN.UTF-8.
const (
	gitPushNoUpstreamZH = "致命错误：当前分支 feat 没有对应的上游分支。\n" +
		"为推送当前分支并建立与远程上游的跟踪，使用\n\n" +
		"    git push --set-upstream origin feat\n\n" +
		"为了让没有追踪上游的分支自动配置，参见 'git help config' 中的 push.autoSetupRemote。\n"
	gitPullNoTrackingZH = "当前分支没有跟踪信息。\n" +
		"请指定您要合并哪一个分支。\n" +
		"详见 git-pull(1)。\n\n" +
		"    git pull <远程> <分支>\n\n" +
		"如果您想要为此分支创建跟踪信息，您可以执行：\n\n" +
		"    git branch --set-upstream-to=origin/<分支> feat\n\n"
	gitBranchUpstreamMissingZH = "致命错误：请求的上游分支 'origin' 不存在\n" +
		"提示：\n" +
		"提示： 如果您正计划基于远程一个现存的上游分支开始您的工作，\n" +
		"提示： 您可能需要执行 \"git fetch\" 来获取分支。\n"
	gitDivergentPullZH = "提示： 您有偏离的分支，需要指定如何调和它们。您可以在执行下一次\n" +
		"提示： pull 操作之前执行下面一条命令来抑制本消息：\n" +
		"提示：\n" +
		"提示：   git config pull.rebase false  # 合并\n" +
		"提示：   git config pull.rebase true   # 变基\n" +
		"提示：   git config pull.ff only       # 仅快进\n" +
		"提示：\n" +
		"致命错误：需要指定如何调和偏离的分支。\n"
	gitPushRejectedEN = "To github.com:example/repo.git\n" +
		" ! [rejected]        main -> main (fetch first)\n" +
		"error: failed to push some refs to 'github.com:example/repo.git'\n" +
		"hint: Updates were rejected because the remote contains work that you do not\n" +
		"hint: have locally. This is usually caused by another repository pushing to\n" +
		"hint: the same ref. If you want to integrate the remote changes, use\n" +
		"hint: 'git pull' before pushing again.\n"
	gitPushRejectedZH = "To github.com:example/repo.git\n" +
		" ! [rejected]        main -> main (non-fast-forward)\n" +
		"错误：无法推送一些引用到 'github.com:example/repo.git'\n" +
		"提示： 更新被拒绝，因为您当前分支的最新提交落后于其对应的远程分支。\n" +
		"提示： 如果您希望先与远程变更合并，请在推送前执行 'git pull'。\n"
	gitIdentityUnknownEN = "Author identity unknown\n\n" +
		"*** Please tell me who you are.\n\n" +
		"Run\n\n" +
		"  git config --global user.email \"you@example.com\"\n" +
		"  git config --global user.name \"Your Name\"\n\n" +
		"to set your account's default identity.\n" +
		"Omit --global to set the identity only in this repository.\n\n" +
		"fatal: no email was given and auto-detection is disabled\n"
	gitIdentityUnknownZH = "作者身份未知\n\n" +
		"*** 请告诉我您是谁。\n\n" +
		"运行\n\n" +
		"  git config --global user.email \"you@example.com\"\n" +
		"  git config --global user.name \"Your Name\"\n\n" +
		"来设置您账号的缺省身份标识。\n" +
		"如果仅在本仓库设置身份标识，则省略 --global 参数。\n\n" +
		"致命错误：未提供邮件地址且自动探测被禁用\n"
)

const (
	gitQueryBranch   = "symbolic-ref --quiet --short HEAD"
	gitQueryUpstream = "rev-parse --abbrev-ref --symbolic-full-name @{upstream}"
	gitQueryName     = "config --get user.name"
	gitQueryEmail    = "config --get user.email"
	gitQueryRefs     = "for-each-ref --format=%(refname) refs/heads refs/remotes"
)

// fakeGit answers read-only git queries keyed by their joined arguments; any
// other query fails like an unset config key or a missing ref.
type fakeGit struct {
	responses map[string]string
	calls     []string
}

func (f *fakeGit) run(_ context.Context, args []string) ([]byte, error) {
	key := strings.Join(args, " ")
	f.calls = append(f.calls, key)
	if output, ok := f.responses[key]; ok {
		return []byte(output + "\n"), nil
	}
	return nil, errors.New("exit status 1")
}

func newFakeGitParser(responses map[string]string) (*GitParser, *fakeGit) {
	fake := &fakeGit{responses: responses}
	p := NewGitParser()
	p.runGit = fake.run
	p.resolveBranchUpstream = func(_ []string, remote, branch string) (string, bool) {
		if branch == "" {
			branch = "feat" // the checked-out branch
		}
		return remote + "/" + branch, true
	}
	return p, fake
}

type gitWorkflowCase struct {
	name      string
	cmd       string
	stderr    string
	exitCode  int
	responses map[string]string
	multiple  bool
	want      string
}

func runGitWorkflowCases(t *testing.T, tests []gitWorkflowCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, fake := newFakeGitParser(tt.responses)
			ctx := itypes.ParserContext{Command: tt.cmd, Stderr: tt.stderr, ExitCode: tt.exitCode, HasMultipleCommands: tt.multiple}
			got := p.Parse(ctx)
			if got.Fixed != (tt.want != "") || got.Command != tt.want {
				t.Fatalf("Parse() = %+v, want %q (git calls %q)", got, tt.want, fake.calls)
			}
			if got.Fixed {
				ctx.Command = got.Command
				if next := p.Parse(ctx); next.Fixed && next.Command != got.Command {
					t.Fatalf("correction must be stable, second pass gave %+v", next)
				}
			}
		})
	}
}

func TestGitParser_LocalizedUpstreamFixes(t *testing.T) {
	onFeat := map[string]string{gitQueryBranch: "feat"}
	onMain := map[string]string{gitQueryBranch: "main"}
	runGitWorkflowCases(t, []gitWorkflowCase{
		{name: "zh push", cmd: "git push", stderr: gitPushNoUpstreamZH, exitCode: 128, responses: onFeat, want: "git push --set-upstream origin feat"},
		{name: "zh push with remote", cmd: "git push origin", stderr: gitPushNoUpstreamZH, exitCode: 128, responses: onFeat, want: "git push --set-upstream origin feat"},
		{name: "zh push in repo", cmd: "git -C repo push -v", stderr: gitPushNoUpstreamZH, exitCode: 128, responses: map[string]string{"-C repo " + gitQueryBranch: "feat"}, want: "git -C repo push -v --set-upstream origin feat"},
		{name: "zh push git-push form", cmd: "git-push", stderr: gitPushNoUpstreamZH, exitCode: 128, responses: onFeat, want: "git-push --set-upstream origin feat"},
		{name: "zh push unknown exit code", cmd: "git push", stderr: gitPushNoUpstreamZH, exitCode: -1, responses: onFeat, want: "git push --set-upstream origin feat"},
		{name: "zh push needs failure", cmd: "git push", stderr: gitPushNoUpstreamZH, exitCode: 0, responses: onFeat},
		{name: "zh push hint must name current branch", cmd: "git push", stderr: gitPushNoUpstreamZH, exitCode: 128, responses: onMain},
		{name: "zh push detached head", cmd: "git push", stderr: gitPushNoUpstreamZH, exitCode: 128},
		{name: "zh push dynamic selector", cmd: `git -C "$REPO" push`, stderr: gitPushNoUpstreamZH, exitCode: 128, responses: onFeat},
		{name: "zh pull", cmd: "git pull", stderr: gitPullNoTrackingZH, exitCode: 1, responses: onFeat, want: "git pull --set-upstream origin feat"},
		{name: "zh pull rebase", cmd: "git pull --rebase", stderr: gitPullNoTrackingZH, exitCode: 1, responses: onFeat, want: "git pull --rebase --set-upstream origin feat"},
		{name: "zh pull other branch", cmd: "git pull", stderr: gitPullNoTrackingZH, exitCode: 1, responses: onMain},
		{name: "zh pull needs failure", cmd: "git pull", stderr: gitPullNoTrackingZH, exitCode: 0, responses: onFeat},
		{name: "zh branch upstream", cmd: "git branch --set-upstream-to=origin", stderr: gitBranchUpstreamMissingZH, exitCode: 128, want: "git branch --set-upstream-to=origin/feat"},
		{name: "zh branch upstream other remote", cmd: "git branch --set-upstream-to=upstream", stderr: gitBranchUpstreamMissingZH, exitCode: 128},
		{name: "zh branch upstream needs failure", cmd: "git branch --set-upstream-to=origin", stderr: gitBranchUpstreamMissingZH},
		{name: "zh divergent", cmd: "git pull", stderr: gitDivergentPullZH, exitCode: 128, want: "git pull --rebase"},
		{name: "zh divergent needs every hint", cmd: "git pull", stderr: strings.ReplaceAll(gitDivergentPullZH, "pull.ff only", "pull.ff"), exitCode: 128},
		{name: "zh divergent needs failure", cmd: "git pull", stderr: gitDivergentPullZH},
	})
}

func TestGitParser_LocalizedBranchUpstreamUsesCurrentBranchResolver(t *testing.T) {
	p, _ := newFakeGitParser(nil)
	p.resolveBranchUpstream = func(_ []string, remote, branch string) (string, bool) {
		if remote != "origin" || branch != "" {
			t.Fatalf("unexpected resolve(%q, %q)", remote, branch)
		}
		return "", false
	}
	got := p.Parse(itypes.ParserContext{Command: "git branch -u origin", Stderr: gitBranchUpstreamMissingZH, ExitCode: 128})
	if got.Fixed {
		t.Fatalf("missing remote branch must not be guessed: %+v", got)
	}
}

func TestGitParser_PushRejectedRebasesBeforePushing(t *testing.T) {
	tracking := map[string]string{gitQueryBranch: "main", gitQueryUpstream: "origin/main"}
	runGitWorkflowCases(t, []gitWorkflowCase{
		{name: "en fetch first", cmd: "git push", stderr: gitPushRejectedEN, exitCode: 1, responses: tracking, want: "git pull --rebase && git push"},
		{name: "zh non-fast-forward", cmd: "git push", stderr: gitPushRejectedZH, exitCode: 1, responses: tracking, want: "git pull --rebase && git push"},
		{name: "flags and selector", cmd: "git -C 'my repo' push -q", stderr: gitPushRejectedEN, exitCode: 1,
			responses: map[string]string{"-C my repo " + gitQueryBranch: "main", "-C my repo " + gitQueryUpstream: "origin/main"},
			want:      "git -C 'my repo' pull --rebase && git -C 'my repo' push -q"},
		{name: "git-push form", cmd: "git-push", stderr: gitPushRejectedEN, exitCode: 1, responses: tracking, want: "git-pull --rebase && git-push"},
		{name: "needs failure", cmd: "git push", stderr: gitPushRejectedEN, responses: tracking},
		{name: "no upstream", cmd: "git push", stderr: gitPushRejectedEN, exitCode: 1, responses: map[string]string{gitQueryBranch: "main"}},
		{name: "upstream is another branch", cmd: "git push", stderr: gitPushRejectedEN, exitCode: 1, responses: map[string]string{gitQueryBranch: "main", gitQueryUpstream: "origin/release"}},
		{name: "rejected ref is not checked out", cmd: "git push", stderr: gitPushRejectedEN, exitCode: 1, responses: map[string]string{gitQueryBranch: "feat", gitQueryUpstream: "origin/main"}},
		{name: "explicit target", cmd: "git push origin main", stderr: gitPushRejectedEN, exitCode: 1, responses: tracking},
		{name: "force push", cmd: "git push --force-with-lease", stderr: gitPushRejectedEN, exitCode: 1, responses: tracking},
		{name: "compound command", cmd: "git push", stderr: gitPushRejectedEN, exitCode: 1, responses: tracking, multiple: true},
		{name: "multiple refs", cmd: "git push", stderr: gitPushRejectedEN + " ! [rejected]        dev -> dev (fetch first)\n", exitCode: 1, responses: tracking},
		{name: "renamed ref", cmd: "git push", stderr: strings.Replace(gitPushRejectedEN, "main -> main", "main -> release", 1), exitCode: 1, responses: tracking},
		{name: "rejected for another reason", cmd: "git push", stderr: strings.Replace(gitPushRejectedEN, "(fetch first)", "(stale info)", 1), exitCode: 1, responses: tracking},
	})
}

func TestGitParser_IdentityUnknownSuggestsConfig(t *testing.T) {
	const setName, setEmail = "git config --global user.name", "git config --global user.email"
	runGitWorkflowCases(t, []gitWorkflowCase{
		{name: "en commit", cmd: "git commit -m 'msg'", stderr: gitIdentityUnknownEN, exitCode: 128, want: setName},
		{name: "zh commit", cmd: "git commit -m 'msg'", stderr: gitIdentityUnknownZH, exitCode: 128, want: setName},
		{name: "name already set", cmd: "git commit", stderr: gitIdentityUnknownEN, exitCode: 128, responses: map[string]string{gitQueryName: "Real Name"}, want: setEmail},
		{name: "email already set", cmd: "git merge dev", stderr: gitIdentityUnknownEN, exitCode: 128, responses: map[string]string{gitQueryEmail: "a@example.com"}, want: setName},
		{name: "repository config", cmd: "git -C repo commit", stderr: gitIdentityUnknownEN, exitCode: 128, responses: map[string]string{"-C repo " + gitQueryName: "Real Name"}, want: setEmail},
		{name: "fully configured", cmd: "git commit", stderr: gitIdentityUnknownEN, exitCode: 128, responses: map[string]string{gitQueryName: "a", gitQueryEmail: "b"}},
		{name: "needs failure", cmd: "git commit", stderr: gitIdentityUnknownEN},
		{name: "needs git hint", cmd: "git commit", stderr: "Author identity unknown\n", exitCode: 128},
		{name: "git config itself", cmd: "git config --global user.email", stderr: gitIdentityUnknownEN, exitCode: 1},
		{name: "compound command", cmd: "git commit", stderr: gitIdentityUnknownEN, exitCode: 128, multiple: true},
	})
}

func TestGitParser_BranchTypo(t *testing.T) {
	refs := map[string]string{gitQueryRefs: "refs/heads/main\nrefs/heads/feature/login\nrefs/remotes/origin/HEAD\nrefs/remotes/origin/main\nrefs/remotes/origin/release"}
	runGitWorkflowCases(t, []gitWorkflowCase{
		{name: "en checkout", cmd: "git checkout mian", stderr: "error: pathspec 'mian' did not match any file(s) known to git\n", exitCode: 1, responses: refs, want: "git checkout main"},
		{name: "zh checkout", cmd: "git checkout mian", stderr: "错误：路径规格 'mian' 未匹配任何 git 已知文件\n", exitCode: 1, responses: refs, want: "git checkout main"},
		{name: "en switch", cmd: "git switch -q mian", stderr: "fatal: invalid reference: mian\n", exitCode: 128, responses: refs, want: "git switch -q main"},
		{name: "zh switch", cmd: "git switch mian", stderr: "致命错误：无效引用：mian\n", exitCode: 128, responses: refs, want: "git switch main"},
		{name: "remote branch", cmd: "git switch relaese", stderr: "fatal: invalid reference: relaese\n", exitCode: 128, responses: refs, want: "git switch release"},
		{name: "slashed branch", cmd: "git checkout feature/lgoin", stderr: "error: pathspec 'feature/lgoin' did not match any file(s) known to git\n", exitCode: 1, responses: refs, want: "git checkout feature/login"},
		{name: "existing branch", cmd: "git checkout main", stderr: "error: pathspec 'main' did not match\n", exitCode: 1, responses: refs},
		{name: "too far", cmd: "git checkout xyz", stderr: "error: pathspec 'xyz' did not match\n", exitCode: 1, responses: refs},
		{name: "ambiguous", cmd: "git checkout dev1", stderr: "fatal: invalid reference: dev1\n", exitCode: 128, responses: map[string]string{gitQueryRefs: "refs/heads/dev2\nrefs/heads/dev3"}},
		{name: "name missing from stderr", cmd: "git checkout mian", stderr: "error: something else\n", exitCode: 1, responses: refs},
		{name: "name only as substring", cmd: "git checkout mian", stderr: "error: pathspec 'xmian' did not match\n", exitCode: 1, responses: refs},
		{name: "create branch", cmd: "git checkout -b mian", stderr: "fatal: invalid reference: mian\n", exitCode: 128, responses: refs},
		{name: "path checkout", cmd: "git checkout -- mian", stderr: "error: pathspec 'mian' did not match\n", exitCode: 1, responses: refs},
		{name: "two arguments", cmd: "git checkout mian file", stderr: "error: pathspec 'mian' did not match\n", exitCode: 1, responses: refs},
		{name: "dynamic argument", cmd: `git checkout "$B"`, stderr: "fatal: invalid reference: mian\n", exitCode: 128, responses: refs},
		{name: "not a repository", cmd: "git checkout mian", stderr: "error: pathspec 'mian' did not match\n", exitCode: 1},
		{name: "needs failure", cmd: "git checkout mian", stderr: "error: pathspec 'mian' did not match\n", responses: refs},
	})
}

func TestOSADistance(t *testing.T) {
	for _, tt := range []struct {
		a, b string
		want int
	}{
		{"", "", 0}, {"main", "main", 0}, {"mian", "main", 1}, {"", "abc", 3},
		{"relaese", "release", 1}, {"mai", "main", 1}, {"kitten", "sitting", 3}, {"分支", "分枝", 1},
	} {
		if got := osaDistance(tt.a, tt.b); got != tt.want {
			t.Errorf("osaDistance(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestGitQueryUsesDefaultRunnerWhenUnset(t *testing.T) {
	p := &GitParser{}
	if _, ok := p.gitConfigValue([]string{"-C", t.TempDir()}, "typo.test.unset"); ok {
		t.Fatal("unset key must report false")
	}
}

func TestGitRepositoryQueriesAgainstRealGit(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	run := func(args ...string) {
		t.Helper()
		prefix := []string{"-C", dir, "-c", "user.name=shown", "-c", "user.email=regression@example.invalid", "-c", "commit.gpgsign=false"}
		if output, err := exec.Command(git, append(prefix, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %q failed: %v\n%s", args, err, output)
		}
	}
	run("init", "--initial-branch=main")
	run("commit", "--allow-empty", "-m", "seed")
	run("branch", "feature/login")
	run("update-ref", "refs/remotes/origin/main", "HEAD")
	run("update-ref", "refs/remotes/origin/release", "HEAD")
	run("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	run("remote", "add", "origin", dir)
	run("config", "branch.main.remote", "origin")
	run("config", "branch.main.merge", "refs/heads/main")

	p := NewGitParser()
	repository := []string{"-C", dir}
	if branch, ok := p.gitCurrentBranch(repository); !ok || branch != "main" {
		t.Fatalf("gitCurrentBranch() = %q, %v", branch, ok)
	}
	if upstream, ok := p.gitCurrentUpstream(repository); !ok || upstream != "origin/main" {
		t.Fatalf("gitCurrentUpstream() = %q, %v", upstream, ok)
	}
	names, ok := p.gitBranchNames(repository)
	slices.Sort(names)
	if !ok || !slices.Equal(names, []string{"feature/login", "main", "release"}) {
		t.Fatalf("gitBranchNames() = %q, %v", names, ok)
	}
	if _, ok := p.gitConfigValue(repository, "user.email"); ok {
		t.Fatal("user.email must be unset in the isolated repository")
	}

	run("checkout", "--detach")
	if _, ok := p.gitCurrentBranch(repository); ok {
		t.Fatal("detached HEAD must not report a branch")
	}
	if _, ok := p.gitCurrentUpstream(repository); ok {
		t.Fatal("detached HEAD must not report an upstream")
	}
}
