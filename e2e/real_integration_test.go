package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuluo-yx/typo/internal/commands"
)

func (e *e2eEnv) writeBinScript(t *testing.T, name, script string) {
	t.Helper()

	path := filepath.Join(e.binDir, name)
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("failed to write executable %s: %v", name, err)
	}
}

func (e *e2eEnv) removeSubcommandCache(t *testing.T) {
	t.Helper()

	cacheFile := filepath.Join(e.configDir(), "subcommands.json")
	if err := os.Remove(cacheFile); err != nil && !os.IsNotExist(err) {
		t.Fatalf("failed to remove subcommand cache: %v", err)
	}
}

func TestE2EDynamicSubcommandDiscovery(t *testing.T) {
	env := newE2EEnv(t)
	env.removeSubcommandCache(t)
	env.writeBinScript(t, "git", `#!/bin/sh
if [ "$1" = "help" ] && [ "$2" = "-a" ]; then
  printf '  status           Show the working tree status\n'
  printf '  switch           Switch branches\n'
  printf '  restore          Restore working tree files\n'
  exit 0
fi
exit 1
`)

	result := env.run(t, "fix", "git stattus")
	if result.code != 0 || result.stdout != "git status\n" {
		t.Fatalf("dynamic subcommand discovery failed: stdout=%q stderr=%q code=%d", result.stdout, result.stderr, result.code)
	}

	cacheFile := filepath.Join(env.configDir(), "subcommands.json")
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		t.Fatalf("failed to read subcommand cache: %v", err)
	}

	var caches []commands.SubcommandCache
	if err := json.Unmarshal(data, &caches); err != nil {
		t.Fatalf("failed to parse subcommand cache: %v", err)
	}

	for _, cache := range caches {
		if cache.Tool == "git" {
			for _, subcommand := range cache.Subcommands {
				if subcommand == "status" {
					return
				}
			}
		}
	}

	t.Fatalf("git subcommands were not cached after dynamic discovery: %s", data)
}

func TestE2EZshPreviewFixDoesNotWriteUsageHistory(t *testing.T) {
	env := newE2EEnv(t)

	initScript := env.initZshScript(t)
	result := env.runZsh(t, initScript, `
zle() { true; }
bindkey() { true; }
source "$1"
BUFFER="gut status"
_typo_fix_command
[[ "$BUFFER" == "git status" ]] || exit 41
[[ ! -e "$HOME/.typo/usage_history.json" ]] || exit 42
print -r -- "$BUFFER"
`)
	if result.code != 0 || !strings.Contains(result.stdout, "git status") {
		t.Fatalf("zsh preview fix should not write usage history: stdout=%q stderr=%q code=%d", result.stdout, result.stderr, result.code)
	}
}

func TestE2EBashPreviewFixDoesNotWriteUsageHistory(t *testing.T) {
	env := newE2EEnv(t)

	initScript := env.initBashScript(t)
	result := env.runBash(t, initScript, `
source "$1"
trap - DEBUG
READLINE_LINE="gut status"
_typo_fix_command
[[ "$READLINE_LINE" == "git status" ]] || exit 46
[[ ! -e "$HOME/.typo/usage_history.json" ]] || exit 47
printf "%s\n" "$READLINE_LINE"
`)
	if result.code != 0 || !strings.Contains(result.stdout, "git status") {
		t.Fatalf("bash preview fix should not write usage history: stdout=%q stderr=%q code=%d", result.stdout, result.stderr, result.code)
	}
}

func TestE2EZshIntegrationCapturesLiveStderr(t *testing.T) {
	env := newE2EEnv(t)
	env.writeBinScript(t, "git", `#!/bin/sh
if [ "$1" = "remove" ]; then
  printf "git: 'remove' is not a git command.\n\nThe most similar command is\n\tremote\n" >&2
  exit 1
fi
exit 0
`)

	initScript := env.initZshScript(t)
	result := env.runZsh(t, initScript, `
zle() { true; }
bindkey() { true; }
source "$1"
cmd='git remove -v'
BUFFER="$cmd"
_typo_preexec
eval "$cmd" >/dev/null || true
_typo_precmd
stderr_content="$(<"$TYPO_STDERR_CACHE")"
[[ "$stderr_content" == *"The most similar command is"* ]] || exit 41
_typo_fix_command
[[ "$BUFFER" == "git remote -v" ]] || { print -r -- "$BUFFER"; exit 42; }
print -r -- "$BUFFER"
`)
	if result.code != 0 || !strings.Contains(result.stdout, "git remote -v") {
		t.Fatalf("live stderr capture flow failed: stdout=%q stderr=%q code=%d", result.stdout, result.stderr, result.code)
	}
}

func TestE2EZshIntegrationSkipsSlowDynamicDiscovery(t *testing.T) {
	env := newE2EEnv(t)
	env.removeSubcommandCache(t)
	env.writeBinScript(t, "go", `#!/bin/sh
if [ "$1" = "help" ]; then
  /bin/sleep 2
  printf 'Go is a tool for managing Go source code.\n\n'
  printf 'The commands are:\n\n'
  printf '\tvet         examine Go source code and report suspicious constructs\n'
  exit 0
fi
if [ "$1" = "vte" ]; then
  printf "go: unknown command: vte\n" >&2
  exit 1
fi
exit 0
`)

	initScript := env.initZshScript(t)
	result := env.runZsh(t, initScript, `
zle() { true; }
bindkey() { true; }
fc() { print -r -- "go vte ./..."; }
sed() { while IFS= read -r line; do print -r -- "$line"; done }
source "$1"
TYPO_LAST_EXIT_CODE=1
BUFFER=""
start=$SECONDS
_typo_fix_command
elapsed=$((SECONDS - start))
[[ "$elapsed" -lt 1 ]] || exit 51
[[ -z "$BUFFER" ]] || { print -r -- "$BUFFER"; exit 52; }
print -r -- "elapsed=$elapsed"
`)
	if result.code != 0 || !strings.Contains(result.stdout, "elapsed=") {
		t.Fatalf("slow dynamic discovery should not block zsh fix flow: stdout=%q stderr=%q code=%d", result.stdout, result.stderr, result.code)
	}
}

func TestE2EDynamicGoSubcommandDiscovery(t *testing.T) {
	env := newE2EEnv(t)
	env.removeSubcommandCache(t)
	env.writeBinScript(t, "go", `#!/bin/sh
if [ "$1" = "help" ]; then
  printf 'Go is a tool for managing Go source code.\n\n'
  printf 'Usage:\n\n'
  printf '\tgo <command> [arguments]\n\n'
  printf 'The commands are:\n\n'
  printf '\tbuild       compile packages and dependencies\n'
  printf '\ttest        test packages\n'
  printf '\tfmt         gofmt (reformat) package sources\n\n'
  printf 'Use "go help <command>" for more information about a command.\n'
  exit 0
fi
exit 1
`)

	result := env.run(t, "fix", "go biuld ./...")
	if result.code != 0 || result.stdout != "go build ./...\n" {
		t.Fatalf("dynamic go subcommand discovery failed: stdout=%q stderr=%q code=%d", result.stdout, result.stderr, result.code)
	}
}
