package commands

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestGitNestedHelpOnlyProbesKnownBuiltins(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell executable fixture requires a POSIX system")
	}
	dir := t.TempDir()
	trace := filepath.Join(dir, "trace")
	t.Setenv("PATH", dir)
	t.Setenv("GIT_HELP_TRACE", trace)
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$GIT_HELP_TRACE\"\nprintf 'usage: git remote add <name> <url>\\n'\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	r := NewToolTreeRegistry("")
	// Allow instrumented test runs time to start the help fixture process.
	r.helpTimeout = 5 * time.Second
	// Discovery and cache entries do not make aliases or external commands safe.
	r.storeChildren("git", nil, []string{"c", "custom-external"})
	for _, name := range []string{"c", "custom-external", "--help"} {
		t.Run(name, func(t *testing.T) {
			out, err := r.getNestedHelpOutput("git", []string{name})
			if err != nil || out != "" {
				t.Fatalf("untrusted Git command was probed: output=%q err=%v", out, err)
			}
			if children := r.GetChildren("git", []string{name}); len(children) != 0 {
				t.Fatalf("unexpected alias or external command children: %v", children)
			}
		})
	}
	if _, err := os.Stat(trace); !os.IsNotExist(err) {
		t.Fatalf("Git was executed for an untrusted command: %v", err)
	}
	out, err := r.getNestedHelpOutput("git", []string{"remote"})
	if err != nil || !strings.Contains(out, "git remote add") {
		t.Fatalf("builtin help discovery failed: output=%q err=%v", out, err)
	}
	data, err := os.ReadFile(trace)
	if err != nil || string(data) != "remote -h\n" {
		t.Fatalf("unexpected help calls: %q err=%v", data, err)
	}
}

func TestGitAliasHelpDoesNotCommitStagedFiles(t *testing.T) {
	git, lookupErr := exec.LookPath("git")
	if lookupErr != nil {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(dir, ".gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("LC_ALL", "C")
	t.Chdir(dir)
	run := func(args ...string) string {
		t.Helper()
		out, err := exec.Command(git, args...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %q failed: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init")
	run("config", "user.name", "Regression")
	run("config", "user.email", "regression@example.invalid")
	run("config", "commit.gpgsign", "false")
	run("config", "--global", "alias.c", "commit -s -m")
	run("commit", "--allow-empty", "-m", "seed")
	if err := os.WriteFile(filepath.Join(dir, "staged.txt"), []byte("staged content\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", "staged.txt")
	head := run("rev-parse", "HEAD")
	indexPath := filepath.Join(dir, ".git", "index")
	index, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	r := &ToolTreeRegistry{helpTimeout: 5 * time.Second}
	// Probe directly so the regression cannot pass merely because discovery timed out.
	out, probeErr := r.getNestedHelpOutput("git", []string{"c"})
	afterIndex, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	if after := run("rev-parse", "HEAD"); after != head || !bytes.Equal(index, afterIndex) {
		t.Fatalf("help probing mutated the repository: HEAD=%s -> %s, message=%q", head, after, run("log", "-1", "--format=%B"))
	}
	if probeErr != nil || out != "" {
		t.Fatalf("alias help must be skipped: output=%q err=%v", out, probeErr)
	}
}
