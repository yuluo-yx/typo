package parser

import (
	"os"
	"os/exec"
	"testing"
)

func TestResolveGitBranchUpstreamRequiresBothRealRefs(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	run := func(args ...string) {
		t.Helper()
		prefix := []string{"-C", dir, "-c", "user.name=Regression", "-c", "user.email=regression@example.invalid", "-c", "commit.gpgsign=false"}
		if output, err := exec.Command(git, append(prefix, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %q failed: %v\n%s", args, err, output)
		}
	}
	run("init", "--initial-branch=main")
	run("commit", "--allow-empty", "-m", "seed")
	run("update-ref", "refs/remotes/origin/main", "HEAD")
	run("branch", "local-only")
	run("update-ref", "refs/remotes/origin/remote-only", "HEAD")
	for _, tt := range []struct {
		branch string
		want   bool
	}{
		{"main", true},
		{"local-only", false},
		{"remote-only", false},
		{"missing", false},
	} {
		t.Run(tt.branch, func(t *testing.T) {
			target, ok := resolveGitBranchUpstream([]string{"-C", dir}, "origin", tt.branch)
			if ok != tt.want || ok && target != "origin/"+tt.branch || !ok && target != "" {
				t.Fatalf("resolveGitBranchUpstream(%q) = (%q, %v), want success=%v", tt.branch, target, ok, tt.want)
			}
		})
	}
}
