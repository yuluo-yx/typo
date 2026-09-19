//go:build unix

package cmd

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func bashRegressionShells(t *testing.T) []string {
	t.Helper()
	var shells []string
	seen := make(map[string]bool)
	for _, name := range []string{"bash", "/bin/bash"} {
		path, err := exec.LookPath(name)
		if err == nil && !seen[path] {
			seen[path] = true
			shells = append(shells, path)
		}
	}
	if len(shells) == 0 {
		t.Skip("bash is not available")
	}
	return shells
}

func runBashRegression(t *testing.T, shell, script string) (string, int) {
	t.Helper()
	dir := t.TempDir()
	initPath := filepath.Join(dir, "typo.bash")
	if err := os.WriteFile(initPath, []byte(bashIntegrationScript), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, shell, "--noprofile", "--norc", "-c", script, "bash", initPath)
	cmd.Env = append(os.Environ(), "HOME="+dir, "TMPDIR="+dir)
	cmd.WaitDelay = 500 * time.Millisecond
	output, err := cmd.CombinedOutput()
	// Release a fixture child on both assertion failures and prompt timeouts.
	if file, openErr := os.OpenFile(filepath.Join(dir, "release.fifo"), os.O_RDWR, 0600); openErr == nil {
		_, _ = file.WriteString("release\n")
		_ = file.Close()
	}
	if ctx.Err() != nil {
		t.Fatalf("shell did not finish before deadline: %s", output)
	}
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		t.Fatalf("shell failed to run: %v\n%s", err, output)
	}
	return string(output), cmd.ProcessState.ExitCode()
}

func TestBashBackgroundCaptureDoesNotBlockOrMixCommands(t *testing.T) {
	for _, shell := range bashRegressionShells(t) {
		t.Run(shell, func(t *testing.T) {
			output, code := runBashRegression(t, shell, `
source "$1"
trap - DEBUG
mkfifo "$TMPDIR/release.fifo"
_typo_preexec
old_cache="$TYPO_STDERR_CACHE"
old_fifo="$_TYPO_STDERR_FIFO"
old_tee="$_TYPO_TEE_PID"
(
    read -r release < "$TMPDIR/release.fifo"
    printf 'late-background\n' >&2
) &
background_pid=$!
_typo_precmd

# This line is unreachable if precmd waits for the background writer.
printf 'prompt-ready\n'
_typo_preexec
[[ "$TYPO_STDERR_CACHE" != "$old_cache" ]] || exit 71
[[ ! -e "$old_cache" && ! -e "$old_fifo" ]] || exit 72
printf 'current-foreground\n' >&2
printf 'release\n' > "$TMPDIR/release.fifo"
wait "$background_pid" || exit 73
_typo_precmd
wait "$old_tee" || exit 74
grep -q '^current-foreground$' "$TYPO_STDERR_CACHE" || exit 75
grep -q 'late-background' "$TYPO_STDERR_CACHE" && exit 76
cache="$TYPO_STDERR_CACHE"
fifo="$_TYPO_STDERR_FIFO"
_typo_bashexit
[[ ! -e "$cache" && ! -e "$fifo" ]] || exit 77
`)
			if code != 0 || !strings.Contains(output, "prompt-ready") || !strings.Contains(output, "late-background") {
				t.Fatalf("capture isolation failed: code=%d output=%q", code, output)
			}
		})
	}
}

func TestBashExistingHooksReceiveOriginalStatus(t *testing.T) {
	for _, shell := range bashRegressionShells(t) {
		for _, tt := range []struct {
			name   string
			script string
			want   string
			code   int
		}{
			{
				name: "prompt failure",
				script: `PROMPT_COMMAND='printf "prompt-status=%s\n" "$?"'
source "$1"
trap - DEBUG
false; eval "$PROMPT_COMMAND"`,
				want: "prompt-status=1", code: 0,
			},
			{
				name: "exit failure",
				script: `trap 'printf "exit-status=%s\n" "$?"' EXIT
source "$1"
trap - DEBUG
exit 7`,
				want: "exit-status=7", code: 7,
			},
			{
				name: "exit success",
				script: `trap 'printf "exit-status=%s\n" "$?"' EXIT
source "$1"
trap - DEBUG
exit 0`,
				want: "exit-status=0", code: 0,
			},
			{
				name: "errexit",
				script: `trap 'printf "exit-status=%s\n" "$?"' EXIT
source "$1"
trap - DEBUG
set -e
false`,
				want: "exit-status=1", code: 1,
			},
		} {
			t.Run(shell+"/"+tt.name, func(t *testing.T) {
				output, code := runBashRegression(t, shell, tt.script)
				if code != tt.code || !strings.Contains(output, tt.want) {
					t.Fatalf("got code=%d output=%q, want code=%d and %q", code, output, tt.code, tt.want)
				}
			})
		}
	}
}
