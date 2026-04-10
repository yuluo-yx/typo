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
	if err := writeShellCommandFixture(path, script); err != nil {
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

func TestE2EBashEscBindingMatchesBashVersion(t *testing.T) {
	env := newE2EEnv(t)

	initScript := env.initBashScript(t)
	result := env.runBash(t, initScript, `
set -o emacs
source "$1"
trap - DEBUG
bind_s="$(bind -S 2>/dev/null)"

if (( BASH_VERSINFO[0] >= 5 )); then
	bind_x="$(bind -X 2>/dev/null)"
	[[ "$bind_x" == *'"\e\e": "_typo_fix_command"'* ]] || exit 48
	[[ "$bind_x" != *'"\C-x\C-_": "_typo_fix_command"'* ]] || exit 49
	[[ "$bind_s" != *'"\e\e" outputs "\C-x\C-_"'* ]] || exit 50
else
	[[ "$bind_s" == *'\e\e outputs \C-x\C-_'* || "$bind_s" == *'"\e\e" outputs "\C-x\C-_"'* ]] || exit 51
fi
`)
	if result.code != 0 {
		t.Fatalf("bash esc binding selection failed: stdout=%q stderr=%q code=%d", result.stdout, result.stderr, result.code)
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
stderr_content=""
for _attempt in {1..50}; do
  stderr_content="$(<"$TYPO_STDERR_CACHE")"
  [[ "$stderr_content" == *"The most similar command is"* ]] && break
  sleep 0.02
done
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
[[ "$elapsed" -lt 2 ]] || exit 51
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

func TestE2ECloudDynamicSubcommandDiscovery(t *testing.T) {
	tests := []struct {
		name          string
		tool          string
		script        string
		command       string
		want          string
		cachePath     string
		cacheChildren []string
	}{
		{
			name:    "aws nested discovery",
			tool:    "aws",
			command: "awss s3 lss",
			want:    "aws s3 ls\n",
			script: `#!/bin/sh
if [ "$1" = "--help" ]; then
  printf 'SERVICES\n  s3          Amazon Simple Storage Service\n'
  exit 0
fi
if [ "$1" = "s3" ] && [ "$2" = "help" ]; then
  printf 'COMMANDS\n  ls          List objects\n  cp          Copy objects\n'
  exit 0
fi
exit 1
`,
			cachePath:     "s3",
			cacheChildren: []string{"ls", "cp"},
		},
		{
			name:    "gcloud nested discovery",
			tool:    "gcloud",
			command: "gclodu copmute isntances listt", //nolint:misspell
			want:    "gcloud compute instances list\n",
			script: `#!/bin/sh
if [ "$1" = "--help" ]; then
  printf 'GROUPS\n    compute     Read and write Compute Engine resources\n'
  exit 0
fi
if [ "$1" = "compute" ] && [ "$2" = "--help" ]; then
  printf 'GROUPS\n    instances   Read and write Compute Engine VM instances\n'
  exit 0
fi
if [ "$1" = "compute" ] && [ "$2" = "instances" ] && [ "$3" = "--help" ]; then
  printf 'COMMANDS\n    list        List Compute Engine instances\n'
  exit 0
fi
exit 1
`,
			cachePath:     "compute instances",
			cacheChildren: []string{"list"},
		},
		{
			name:    "az nested discovery",
			tool:    "az",
			command: "azz gorup lisr",
			want:    "az group list\n",
			script: `#!/bin/sh
if [ "$1" = "--help" ]; then
  printf 'Subgroups:\n  group        Manage resource groups.\n'
  exit 0
fi
if [ "$1" = "group" ] && [ "$2" = "--help" ]; then
  printf 'Commands:\n  list         List resource groups.\n'
  exit 0
fi
exit 1
`,
			cachePath:     "group",
			cacheChildren: []string{"list"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newE2EEnv(t)
			env.removeSubcommandCache(t)
			env.writeBinScript(t, tt.tool, tt.script)

			result := env.run(t, "fix", tt.command)
			if result.code != 0 || result.stdout != tt.want {
				t.Fatalf("dynamic cloud discovery failed: stdout=%q stderr=%q code=%d", result.stdout, result.stderr, result.code)
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
				if cache.Tool != tt.tool {
					continue
				}
				children := cache.Children[tt.cachePath]
				if len(children) != len(tt.cacheChildren) {
					t.Fatalf("expected cache path %q children %v, got %v", tt.cachePath, tt.cacheChildren, children)
				}
				for i, child := range tt.cacheChildren {
					if children[i] != child {
						t.Fatalf("expected cache path %q children[%d] = %q, got %q", tt.cachePath, i, child, children[i])
					}
				}
				return
			}

			t.Fatalf("tool %s was not cached after dynamic discovery: %s", tt.tool, data)
		})
	}
}
