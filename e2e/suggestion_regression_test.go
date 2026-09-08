package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestE2ESuggestionCommandBoundaries(t *testing.T) {
	env := newE2EEnv(t)
	env.writeBinScript(t, "acmetool", "#!/bin/sh\nexit 1\n")
	tests := []struct {
		command, stderr, want string
	}{
		{"git cherry-pik abc123", "git: 'cherry-pik' is not a git command.\nThe most similar command is\n\tcherry-pick\n", "git cherry-pick abc123"},
		{"sudo git -C 'my repo' cherry-pik abc123 > 'out file'", "git: 'cherry-pik' is not a git command.\nThe most similar command is\n\tcherry-pick\n", "sudo git -C 'my repo' cherry-pick abc123 > 'out file'"},
		{"echo ready && git cherry-pik abc123", "git: 'cherry-pik' is not a git command.\nThe most similar command is\n\tcherry-pick\n", "echo ready && git cherry-pick abc123"},
		{"npm --prefix 'my app' run-scrip build", "npm ERR! Did you mean run-script?", "npm --prefix 'my app' run-script build"},
		{"acmetool --directory buid buid", "Unknown command 'buid'. Did you mean 'build'?", ""},
		{"acmetool --directory project buid", "Did you mean 'build'?", ""},
		{"acmetool --directory project buid", "Unknown command 'buid'. Did you mean 'build'?", "acmetool --directory project build"},
		{"acmetool 'buid' --release", "Unknown command 'buid'. Did you mean 'build'?", "acmetool build --release"},
		{"acmetool remote remote.list", `Unknown command "remote.list". Did you mean "remote-list"?`, "acmetool remote remote-list"},
		{"acmetool remote", `Unknown command "remote.list". Did you mean "remote-list"?`, ""},
		{"acmetool -- file remote.list", `Unknown command "remote.list". Did you mean "remote-list"?`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.command+tt.stderr, func(t *testing.T) {
			stderrFile := filepath.Join(t.TempDir(), "stderr.txt")
			if err := os.WriteFile(stderrFile, []byte(tt.stderr), 0600); err != nil {
				t.Fatal(err)
			}
			got := env.run(t, "fix", "--no-history", "--exit-code", "1", "-s", stderrFile, tt.command)
			if tt.want == "" {
				if got.code != 1 || got.stdout != "" {
					t.Fatalf("ambiguous hint was applied: %+v", got)
				}
				return
			}
			assertE2EStdoutEquals(t, got, tt.want+"\n", "suggestion changed command boundaries")
		})
	}
}

func TestE2ERealGitHyphenatedSuggestion(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	env := newE2EEnv(t)
	failed := exec.Command(git, "-c", "help.autocorrect=0", "cherry-pik", "abc123")
	failed.Dir = env.root
	failed.Env = append(os.Environ(), "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	output, err := failed.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "cherry-pick") {
		t.Fatalf("git did not emit the expected suggestion: %v\n%s", err, output)
	}
	stderrFile := filepath.Join(t.TempDir(), "git-stderr.txt")
	if err := os.WriteFile(stderrFile, output, 0600); err != nil {
		t.Fatal(err)
	}
	got := env.run(t, "fix", "--no-history", "--exit-code", "1", "-s", stderrFile, "git cherry-pik abc123")
	assertE2EStdoutEquals(t, got, "git cherry-pick abc123\n", "real git suggestion was truncated")
}
