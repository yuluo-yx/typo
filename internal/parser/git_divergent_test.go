package parser

import (
	"testing"
)

func TestGitDivergentPullPreservesShellAndExplicitStrategy(t *testing.T) {
	for _, tt := range []struct{ command, want string }{
		{"git pull --quiet > 'pull log' 2>&1", "git pull --rebase --quiet > 'pull log' 2>&1"},
		{"LABEL=ok git -C 'repo path' pull --autostash origin main # keep", "LABEL=ok git -C 'repo path' pull --rebase --autostash origin main # keep"},
		{"git -C '--rebase' pull", "git -C '--rebase' pull --rebase"},
		{"git pull --set-upstream origin main", "git pull --rebase --set-upstream origin main"},
		{"git pull -r origin main", ""},
		{"git pull '--no-rebase' origin main", ""},
		{"git pull \"--rebase=merges\" origin main", ""},
		{"git pull \"$PULL_OPTIONS\"", ""},
		{"git pull $(choose_remote)", ""},
		{"git pull -- origin main", "git pull --rebase -- origin main"},
		{"git pull -- origin --rebase", "git pull --rebase -- origin --rebase"},
	} {
		t.Run(tt.command, func(t *testing.T) {
			p := NewGitParser()
			got := p.parseDivergentPullRebase(tt.command, gitDivergentPullStderr())
			if got.Fixed != (tt.want != "") || got.Command != tt.want {
				t.Fatalf("Parse() = %+v, want %q", got, tt.want)
			}
			if got.Fixed && p.parseDivergentPullRebase(got.Command, gitDivergentPullStderr()).Fixed {
				t.Fatal("rebase correction must be idempotent")
			}
		})
	}
}
