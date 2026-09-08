package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestE2EWrapperFixPreservesShellSemantics(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires POSIX env and executable scripts")
	}
	env := newE2EEnv(t)
	env.writeBinScript(t, "argprinter", "#!/bin/sh\nprintf 'mode=<%s>\\n' \"$MODE\"\nprintf 'arg=<%s>\\n' \"$@\"\n")
	prefixes := []string{
		"env MODE='hello world' ",
		`env "MODE=hello world" `,
		`env MODE="hello $LABEL" `,
		"env -- MODE='hello world' ",
		`env -iu UNUSED PATH="$PATH" MODE='hello world' `,
		"command -- env MODE='hello world' ",
		`env MODE="hello $(printf world)" `,
	}
	for _, shell := range []string{"bash", "zsh"} {
		t.Run(shell, func(t *testing.T) {
			if _, err := exec.LookPath(shell); err != nil {
				t.Skipf("%s is not installed", shell)
			}
			for _, prefix := range prefixes {
				t.Run(prefix, func(t *testing.T) {
					input := prefix + "argprintr status -- 'a  b'"
					want := prefix + "argprinter status -- 'a  b'"
					fixed := env.run(t, "fix", "--no-history", input)
					assertE2EStdoutEquals(t, fixed, want+"\n", "wrapper correction changed syntax")
					// Only execute the expected fixture command after checking the exact result.
					cmd := exec.Command(shell, "-c", strings.TrimSuffix(fixed.stdout, "\n"))
					cmd.Dir = env.root
					cmd.Env = env.commandEnv("LABEL=world")
					output, err := cmd.CombinedOutput()
					if err != nil || string(output) != "mode=<hello world>\narg=<status>\narg=<-->\narg=<a  b>\n" {
						t.Fatalf("shell semantics changed: %v\n%s", err, output)
					}
				})
			}
		})
	}
}

func TestE2EWrapperAnalysisDoesNotExecuteSubstitution(t *testing.T) {
	env := newE2EEnv(t)
	for _, prefix := range []string{
		`env MODE="$(printf marker > analysis-marker.txt)" `,
		`sudo --user="$(printf marker > analysis-marker.txt)" `,
	} {
		input := prefix + "gti status"
		got := env.run(t, "fix", "--no-history", input)
		assertE2EStdoutEquals(t, got, prefix+"git status\n", "substitution was rewritten")
		if _, err := os.Stat(filepath.Join(env.root, "analysis-marker.txt")); !os.IsNotExist(err) {
			t.Fatalf("analysis executed command substitution: %v", err)
		}
	}
}

func TestE2EWrapperValuesNeverBecomeCandidates(t *testing.T) {
	env := newE2EEnv(t)
	for _, command := range []string{"sudo -nu gti git status", "env -iu gti git status", "env MODE='gti' git status"} {
		for _, selectMode := range []bool{false, true} {
			args := []string{"fix", "--no-history"}
			if selectMode {
				args = append(args, "--select")
			}
			got := env.run(t, append(args, command)...)
			if got.code != 1 || got.stdout != "" {
				t.Fatalf("option value was corrected for %q: %+v", command, got)
			}
		}
	}
}

func TestE2EUnquotedExpansionCanIntroduceExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires POSIX env and executable scripts")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is not installed")
	}
	env := newE2EEnv(t)
	env.writeBinScript(t, "argprinter", "#!/bin/sh\nprintf 'mode=<%s>\\n' \"$MODE\"\nprintf 'arg=<%s>\\n' \"$@\"\n")
	input := `env MODE=$VALUES argprintr status`
	cmd := exec.Command("bash", "-c", input)
	cmd.Dir = env.root
	cmd.Env = env.commandEnv("VALUES=prod argprinter")
	output, err := cmd.CombinedOutput()
	if err != nil || string(output) != "mode=<prod>\narg=<argprintr>\narg=<status>\n" {
		t.Fatalf("shell did not establish the expected argument boundary: %v\n%s", err, output)
	}
	got := env.run(t, "fix", "--no-history", input)
	if got.code != 1 || got.stdout != "" {
		t.Fatalf("data following the expanded executable was corrected: %+v", got)
	}
}
