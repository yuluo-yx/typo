package parser

import (
	"slices"
	"strings"
	"testing"

	itypes "github.com/yuluo-yx/typo/internal/types"
)

func TestGitPullNoTrackingSuggestsBranchBindingOnly(t *testing.T) {
	const stderr = "There is no tracking information for the current branch.\n\n    git branch --set-upstream-to=upstream/<branch> feature/topic\n"
	const binding = "branch --set-upstream-to=upstream/feature/topic feature/topic"
	tests := []struct {
		name, command, want string
		repositoryArgs      []string
	}{
		{"plain pull", "git pull", "git " + binding, nil},
		{"pull-only flags", "git pull --rebase=merges --autostash --ff-only --quiet", "git " + binding, nil},
		{"repository path", "git -C 'repo path' pull --rebase", "git -C 'repo path' " + binding, []string{"-C", "repo path"}},
		{"explicit repository", "git --git-dir 'repo path/.git' --work-tree 'repo path' pull", "git --git-dir 'repo path/.git' --work-tree 'repo path' " + binding, []string{"--git-dir", "repo path/.git", "--work-tree", "repo path"}},
		{"redirect after flags", "git pull --quiet > 'pull output' 2>&1", "git " + binding + " > 'pull output' 2>&1", nil},
		{"redirect before flags", "git pull > 'pull output' --quiet", "git " + binding + " > 'pull output'", nil},
		{"redirect between flags", "git pull --quiet > 'pull output' --rebase 2>&1", "git " + binding + " > 'pull output' 2>&1", nil},
		{"shell suffix", "git pull --rebase; echo ready", "git " + binding + "; echo ready", nil},
		{"comment", "git pull --quiet # keep this comment", "git " + binding + " # keep this comment", nil},
		{"executable form", "git-pull --quiet", "git " + binding, nil},
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
				t.Fatalf("branch binding must be idempotent: %+v", next)
			}
		})
	}
}

func TestShellCallReplaceArgsFromPreservesAssignmentsAndLineContinuations(t *testing.T) {
	const raw = "LABEL=ok git pull \\\n --quiet > 'out file' --rebase"
	call, err := parseShellCall(raw)
	if err != nil {
		t.Fatal(err)
	}
	const want = "LABEL=ok git branch --set-upstream-to=origin/main main \\\n > 'out file'"
	got := call.replaceArgsFrom(1, "branch --set-upstream-to=origin/main main")
	if got != want {
		t.Fatalf("replaceArgsFrom() = %q, want %q", got, want)
	}
	if _, err := parseShellCall(got); err != nil {
		t.Fatalf("replacement produced invalid shell syntax: %v", err)
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
	if !got.Fixed || got.Command != "git branch --set-upstream-to=origin/main main > 'out file'" {
		t.Fatalf("long pull argument list was not replaced safely: %+v", got)
	}
}

func BenchmarkShellCallReplacePullArguments(b *testing.B) {
	for _, size := range []struct {
		name  string
		count int
	}{{"10", 10}, {"1000", 1000}, {"10000", 10000}} {
		b.Run(size.name, func(b *testing.B) {
			const replacement = "branch --set-upstream-to=origin/main main"
			call, err := parseShellCall("git pull " + strings.Repeat("--quiet ", size.count) + "> 'out file'")
			if err != nil {
				b.Fatal(err)
			}
			if got := call.replaceArgsFrom(1, replacement); got != "git "+replacement+" > 'out file'" {
				b.Fatalf("unexpected replacement: %q", got)
			}
			b.ReportAllocs()
			for b.Loop() {
				call.replaceArgsFrom(1, replacement)
			}
		})
	}
}
