package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestMarkdownLintRespectsGitInventory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires POSIX make and xargs")
	}
	for _, tool := range []string{"git", "make", "xargs"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed", tool)
		}
	}
	dir := t.TempDir()
	files := map[string]string{
		"-release.md":                            "# Release\n",
		".gitignore":                             "ignored/\n",
		"tracked.md":                             "# Tracked\n",
		"new document.md":                        "# New\n",
		"notes.markdown":                         "# Notes\n",
		"ignored/local.md":                       "# Ignored\n",
		"ignored/tracked.md":                     "# Tracked despite ignore\n",
		"tools/linter/codespell/.codespell.skip": "\n",
		"fixture.mk":                             "LOG_TARGET = true\n.PHONY: install-markdownlint\ninstall-markdownlint:\n\t@:\n",
		"lint.sh":                                "#!/bin/sh\nif [ \"$1\" = --version ]; then exit 0; fi\nprintf '%s\\n' \"$@\" > \"$LINT_ARGUMENTS_FILE\"\nexit \"${LINT_EXIT_CODE:-0}\"\n",
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(filepath.Join(dir, "lint.sh"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-f", "tracked.md", "ignored/tracked.md"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git fixture failed: %v\n%s", err, output)
		}
	}
	for _, exitCode := range []string{"0", "1"} {
		cmd := exec.Command("make", "-f", filepath.Join(sharedRepoRoot, "tools/make/linter.mk"), "-f", "fixture.mk", "markdown-lint", "MARKDOWNLINT=./lint.sh")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "LINT_ARGUMENTS_FILE="+filepath.Join(dir, "arguments.txt"), "LINT_EXIT_CODE="+exitCode)
		output, err := cmd.CombinedOutput()
		if (err == nil) != (exitCode == "0") {
			t.Fatalf("lint exit status was not propagated: %v\n%s", err, output)
		}
		data, err := os.ReadFile(filepath.Join(dir, "arguments.txt"))
		if err != nil {
			t.Fatal(err)
		}
		args := strings.Split(strings.TrimSpace(string(data)), "\n")
		want := []string{"--config", "./tools/linter/markdownlint/markdown_lint_config.yml", "--", "-release.md", "new document.md", "notes.markdown", "ignored/tracked.md", "tracked.md"}
		if !slices.Equal(args, want) {
			t.Fatalf("lint arguments = %q, want %q", args, want)
		}
	}
}
