package parser

import (
	"slices"
	"strings"
	"testing"

	itypes "github.com/yuluo-yx/typo/internal/types"
)

func TestGitPullNoTrackingPreservesPull(t *testing.T) {
	const stderr = "There is no tracking information for the current branch.\n\n    git branch --set-upstream-to=upstream/<branch> feature/topic\n"
	const upstream = " --set-upstream upstream feature/topic"
	tests := []struct {
		name, command, want string
		repositoryArgs      []string
	}{
		{"plain pull", "git pull", "git pull" + upstream, nil},
		{"pull-only flags", "git pull --rebase=merges --autostash --ff-only --quiet", "git pull --rebase=merges --autostash --ff-only --quiet" + upstream, nil},
		{"repository path", "git -C 'repo path' pull --rebase", "git -C 'repo path' pull --rebase" + upstream, []string{"-C", "repo path"}},
		{"explicit repository", "git --git-dir 'repo path/.git' --work-tree 'repo path' pull", "git --git-dir 'repo path/.git' --work-tree 'repo path' pull" + upstream, []string{"--git-dir", "repo path/.git", "--work-tree", "repo path"}},
		{"redirect after flags", "git pull --quiet > 'pull output' 2>&1", "git pull --quiet" + upstream + " > 'pull output' 2>&1", nil},
		{"redirect before flags", "git pull > 'pull output' --quiet", "git pull > 'pull output' --quiet" + upstream, nil},
		{"redirect between flags", "git pull --quiet > 'pull output' --rebase 2>&1", "git pull --quiet > 'pull output' --rebase" + upstream + " 2>&1", nil},
		{"shell suffix", "git pull --rebase; echo ready", "git pull --rebase" + upstream + "; echo ready", nil},
		{"comment", "git pull --quiet # keep this comment", "git pull --quiet" + upstream + " # keep this comment", nil},
		{"executable form", "git-pull --quiet", "git-pull --quiet" + upstream, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewGitParser()
			p.resolveBranchUpstream = func(args []string, remote, branch string) (string, bool) {
				if !slices.Equal(args, tt.repositoryArgs) || remote != "upstream" || branch != "feature/topic" {
					t.Fatalf("unexpected upstream resolution: args=%q remote=%q branch=%q", args, remote, branch)
				}
				return "upstream/feature/topic", true
			}
			got := p.Parse(itypes.ParserContext{Command: tt.command, Stderr: stderr})
			if !got.Fixed || got.Command != tt.want {
				t.Fatalf("Parse() = %+v, want %q", got, tt.want)
			}
			if next := p.Parse(itypes.ParserContext{Command: got.Command, Stderr: stderr}); next.Fixed {
				t.Fatalf("upstream correction must be idempotent: %+v", next)
			}
		})
	}
}

func TestGitPullNoTrackingPreservesLongShellSuffix(t *testing.T) {
	p := NewGitParser()
	p.resolveBranchUpstream = func(_ []string, remote, branch string) (string, bool) {
		return remote + "/" + branch, true
	}
	command := "git pull" + strings.Repeat(" --quiet", 10000) + " > 'out file'"
	stderr := "There is no tracking information for the current branch.\n    git branch --set-upstream-to=origin/<branch> main\n"
	got := p.Parse(itypes.ParserContext{Command: command, Stderr: stderr})
	if !got.Fixed || got.Command != "git pull"+strings.Repeat(" --quiet", 10000)+" --set-upstream origin main > 'out file'" {
		t.Fatalf("long pull argument list was not preserved: %+v", got)
	}
}

func BenchmarkShellCallInsertPullUpstream(b *testing.B) {
	for _, size := range []struct {
		name  string
		count int
	}{{"10", 10}, {"1000", 1000}, {"10000", 10000}} {
		b.Run(size.name, func(b *testing.B) {
			const insertion = " --set-upstream origin main"
			call, err := parseShellCall("git pull " + strings.Repeat("--quiet ", size.count) + "> 'out file'")
			if err != nil {
				b.Fatal(err)
			}
			if got := call.insertAfterWord(len(call.args)-1, insertion); got != "git pull "+strings.TrimSpace(strings.Repeat("--quiet ", size.count))+insertion+" > 'out file'" {
				b.Fatalf("unexpected replacement: %q", got)
			}
			b.ReportAllocs()
			for b.Loop() {
				call.insertAfterWord(len(call.args)-1, insertion)
			}
		})
	}
}
