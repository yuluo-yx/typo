package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestZshIntegrationCleansAndRotatesStderrCache(t *testing.T) {
	runZshIntegrationScript(t, `
zle() { true; }
bindkey() { true; }
source "$1"

stale="${TMPDIR:-/tmp}/typo-stderr-stale-test"
print -n "old" > "$stale"
touch -t 202401010101 "$stale"
_typo_cleanup_stale_caches
[[ -e "$stale" ]] && exit 21

_typo_preexec
print -u2 "first"
_typo_precmd
sleep 0.1

_typo_preexec
print -u2 "second"
_typo_precmd
sleep 0.1

grep -q "second" "$TYPO_STDERR_CACHE" || exit 22
grep -q "first" "$TYPO_STDERR_CACHE" && exit 23

cache="$TYPO_STDERR_CACHE"
_typo_zshexit
if [[ -e "$cache" ]]; then
    exit 24
fi
`)
}

func TestZshIntegrationIsolatesParentAndChildCaches(t *testing.T) {
	runZshIntegrationScript(t, `
zle() { true; }
bindkey() { true; }
source "$1"

env | grep -q '^TYPO_STDERR_CACHE=' && exit 31
env | grep -q '^TYPO_ORIG_STDERR_FD=' && exit 32

parent_cache="$TYPO_STDERR_CACHE"
[[ -n "$parent_cache" ]] || exit 33
[[ -f "$parent_cache" ]] || exit 34

child_cache=$(zsh -f -c '
zle() { true; }
bindkey() { true; }
source "$1"
print -r -- "$TYPO_STDERR_CACHE"
_typo_zshexit
' zsh "$1")

[[ -n "$child_cache" ]] || exit 35
[[ "$child_cache" == "$parent_cache" ]] && exit 36
[[ -e "$parent_cache" ]] || exit 37

_typo_preexec
print -u2 "parent-still-works"
_typo_precmd
sleep 0.1
grep -q "parent-still-works" "$parent_cache" || exit 38

_typo_zshexit
[[ ! -e "$parent_cache" ]] || exit 39
`)
}

func TestZshIntegrationFallsBackWhenMktempFails(t *testing.T) {
	runZshIntegrationScript(t, `
zle() { true; }
bindkey() { true; }
mktemp() { return 1; }
source "$1"

expected="${TMPDIR:-/tmp}/typo-stderr-$$"
[[ "$TYPO_STDERR_CACHE" == "$expected" ]] || exit 41
[[ "$TYPO_STDERR_CACHE_OWNER" == "$$" ]] || exit 42
[[ -f "$expected" ]] || exit 43

_typo_preexec
print -u2 "fallback-stderr"
_typo_precmd
sleep 0.1
grep -q "fallback-stderr" "$expected" || exit 44

_typo_zshexit
[[ ! -e "$expected" ]] || exit 45
`)
}

func TestZshIntegrationWritesTargetedAliasContext(t *testing.T) {
	runZshIntegrationScript(t, `
zle() { true; }
bindkey() { true; }
source "$1"

alias k=kubectl
alias kk=k
alias g=git
alias d=docker
export TYPO_TEST_ENV_CONTEXT=1
ktf() { terraform "$@"; }
unused_wrap() { docker "$@"; }

_typo_write_alias_context 'sudo kk lgo && g stauts && ktf valdiate'

grep -q '^zsh	alias	kk	k$' "$TYPO_ALIAS_CONTEXT" || exit 71
grep -q '^zsh	alias	k	kubectl$' "$TYPO_ALIAS_CONTEXT" || exit 72
grep -q '^zsh	alias	g	git$' "$TYPO_ALIAS_CONTEXT" || exit 73
grep -q '^zsh	function	ktf	terraform$' "$TYPO_ALIAS_CONTEXT" || exit 74
grep -q '^zsh	alias	d	docker$' "$TYPO_ALIAS_CONTEXT" && exit 75
grep -q '^zsh	function	unused_wrap	docker$' "$TYPO_ALIAS_CONTEXT" && exit 76
grep -q '^zsh	env	TYPO_TEST_ENV_CONTEXT	TYPO_TEST_ENV_CONTEXT$' "$TYPO_ALIAS_CONTEXT" || exit 77
true
`)
}

func TestBashIntegrationCleansAndRotatesStderrCache(t *testing.T) {
	runBashIntegrationScript(t, `
source "$1"
trap - DEBUG

stale="${TMPDIR:-/tmp}/typo-stderr-stale-test"
printf "old" > "$stale"
touch -t 202401010101 "$stale"
_typo_cleanup_stale_caches
[[ -e "$stale" ]] && exit 51

_typo_preexec
printf "first\n" >&2
_typo_precmd
sleep 0.1

_typo_preexec
printf "second\n" >&2
_typo_precmd
sleep 0.1

grep -q "second" "$TYPO_STDERR_CACHE" || exit 52
grep -q "first" "$TYPO_STDERR_CACHE" && exit 53

cache="$TYPO_STDERR_CACHE"
_typo_bashexit
[[ ! -e "$cache" ]] || exit 54
`)
}

func TestBashIntegrationFallsBackWhenMktempFails(t *testing.T) {
	runBashIntegrationScript(t, `
mktemp() { return 1; }
source "$1"
trap - DEBUG

expected="${TMPDIR:-/tmp}/typo-stderr-$$"
[[ "$TYPO_STDERR_CACHE" == "$expected" ]] || exit 61
[[ "$TYPO_STDERR_CACHE_OWNER" == "$$" ]] || exit 62
[[ -f "$expected" ]] || exit 63

_typo_preexec
printf "fallback-stderr\n" >&2
_typo_precmd
sleep 0.1
grep -q "fallback-stderr" "$expected" || exit 64

_typo_bashexit
[[ ! -e "$expected" ]] || exit 65
`)
}

func TestBashIntegrationWritesTargetedAliasContext(t *testing.T) {
	runBashIntegrationScript(t, `
source "$1"
trap - DEBUG

alias k=kubectl
alias kk=k
alias g=git
alias d=docker
export TYPO_TEST_ENV_CONTEXT=1
ktf() { terraform "$@"; }
unused_wrap() { docker "$@"; }

_typo_write_alias_context 'sudo kk lgo && g stauts && ktf valdiate'

grep -q '^bash	alias	kk	k$' "$TYPO_ALIAS_CONTEXT" || exit 66
grep -q '^bash	alias	k	kubectl$' "$TYPO_ALIAS_CONTEXT" || exit 67
grep -q '^bash	alias	g	git$' "$TYPO_ALIAS_CONTEXT" || exit 68
grep -q '^bash	function	ktf	terraform$' "$TYPO_ALIAS_CONTEXT" || exit 69
grep -q '^bash	alias	d	docker$' "$TYPO_ALIAS_CONTEXT" && exit 70
grep -q '^bash	function	unused_wrap	docker$' "$TYPO_ALIAS_CONTEXT" && exit 71
grep -q '^bash	env	TYPO_TEST_ENV_CONTEXT	TYPO_TEST_ENV_CONTEXT$' "$TYPO_ALIAS_CONTEXT" || exit 72
true
`)
}

func TestBashIntegrationPreservesExistingExitTrap(t *testing.T) {
	output := runBashIntegrationScript(t, `
trap 'printf "previous-exit-trap\n"' EXIT
source "$1"
`)

	if !bytes.Contains(output, []byte("previous-exit-trap")) {
		t.Fatalf("expected previous EXIT trap to run, got:\n%s", output)
	}
}

func TestBashIntegrationFallsBackWhenDirectEscBindingIsNotActive(t *testing.T) {
	runBashIntegrationScript(t, `
(( BASH_VERSINFO[0] >= 5 )) || exit 0

TYPO_DIRECT_ATTEMPTS=0
TYPO_FALLBACK_COMMAND_BINDING=0
TYPO_FALLBACK_MACRO_BINDING=0

bind() {
    if [[ "$1" == "-x" && "$2" == '"\e\e":"_typo_fix_command"' ]]; then
        TYPO_DIRECT_ATTEMPTS=$((TYPO_DIRECT_ATTEMPTS + 1))
        return 0
    fi
    if [[ "$1" == "-X" ]]; then
        return 0
    fi
    if [[ "$1" == "-x" && "$2" == '"\C-x\C-_":"_typo_fix_command"' ]]; then
        TYPO_FALLBACK_COMMAND_BINDING=1
        return 0
    fi
    if [[ "$1" == '"\e\e":"\C-x\C-_"' ]]; then
        TYPO_FALLBACK_MACRO_BINDING=1
        return 0
    fi
    return 0
}

source "$1"
trap - DEBUG

[[ "$TYPO_DIRECT_ATTEMPTS" -eq 1 ]] || exit 71
[[ "$TYPO_FALLBACK_COMMAND_BINDING" -eq 1 ]] || exit 72
[[ "$TYPO_FALLBACK_MACRO_BINDING" -eq 1 ]] || exit 73
`)
}

func TestPowerShellIntegrationRegistersHandlersAndState(t *testing.T) {
	output := runPowerShellIntegrationScript(t, `
. $InitScriptPath
$handlers = @(
    @(Get-PSReadLineKeyHandler -Bound -ErrorAction SilentlyContinue)
    @(Get-PSReadLineKeyHandler -ErrorAction SilentlyContinue)
)
$handler = $handlers | Where-Object {
    $props = $_.PSObject.Properties.Name
    (($props -contains "BriefDescription") -and $_.BriefDescription -eq "typo-fix-command") -or
    (($props -contains "Description") -and $_.Description -eq "typo-fix-command") -or
    (($props -contains "Key") -and (([string]$_.Key) -replace "\s+", "") -eq "Escape,Escape")
} | Select-Object -First 1
if ($null -eq $handler) {
    throw "missing typo fix handler"
}
if ($env:TYPO_ACTIVE_SHELL -ne "powershell") {
    throw "missing TYPO_ACTIVE_SHELL"
}
if ($env:TYPO_SHELL_INTEGRATION -ne "1") {
    throw "missing TYPO_SHELL_INTEGRATION"
}
if (-not (Test-Path -LiteralPath $env:TYPO_STDERR_CACHE)) {
    throw "missing stderr cache"
}
$env:TYPO_TEST_ENV_CONTEXT = "1"
Set-Alias -Name k -Value kubectl -Scope Global
$aliasContext = __typo_WriteAliasContext
if (-not (Test-Path -LiteralPath $aliasContext)) {
    throw "missing alias context"
}
$aliasContent = Get-Content -LiteralPath $aliasContext -Raw
$expectedAlias = "powershell" + [char]9 + "alias" + [char]9 + "k" + [char]9 + "kubectl"
if (-not $aliasContent.Contains($expectedAlias)) {
    throw "missing alias context entry"
}
$expectedEnv = "powershell" + [char]9 + "env" + [char]9 + "TYPO_TEST_ENV_CONTEXT" + [char]9 + "TYPO_TEST_ENV_CONTEXT"
if (-not $aliasContent.Contains($expectedEnv)) {
    throw "missing env context entry"
}
Write-Output "ok"
`)

	if !bytes.Contains(output, []byte("ok")) {
		t.Fatalf("Expected PowerShell integration smoke test output, got %q", output)
	}
}

func TestFishIntegrationRegistersBindingStateAndFixes(t *testing.T) {
	output := runFishIntegrationScript(t, `
set -g TYPO_TEST_BUFFER "gti stauts && dcoker ps"
set -g TYPO_TEST_CURSOR 0
set -g TYPO_TEST_ARGS ""

function commandline
    switch "$argv[1]"
        case -b
            printf "%s\n" "$TYPO_TEST_BUFFER"
        case -r
            set -g TYPO_TEST_BUFFER "$argv[2]"
        case -C
            set -g TYPO_TEST_CURSOR "$argv[2]"
        case -f
            true
    end
end

function typo
    set -g TYPO_TEST_ARGS (string join " " -- $argv)
    if contains -- --exit-code $argv
        printf "%s\n" "git status"
    else
        printf "%s\n" "git status && docker ps"
    end
end

function history
    printf "%s\n" "git stauts"
end

source "$argv[1]"

test "$TYPO_ACTIVE_SHELL" = fish; or exit 81
test "$TYPO_SHELL_INTEGRATION" = 1; or exit 82
bind | string match -q "*bind escape,escape _typo_fix_command*"; or exit 83

_typo_fix_command
test "$TYPO_TEST_BUFFER" = "git status && docker ps"; or begin; printf "%s\n" "$TYPO_TEST_BUFFER"; exit 84; end
string match -q "*--no-history*" "$TYPO_TEST_ARGS"; or exit 85
string match -q "*--select*" "$TYPO_TEST_ARGS"; or exit 90

set -g TYPO_TEST_BUFFER ""
set -gx TYPO_LAST_EXIT_CODE 1
_typo_fix_command
test "$TYPO_TEST_BUFFER" = "git status"; or begin; printf "%s\n" "$TYPO_TEST_BUFFER"; exit 86; end
string match -q "*--exit-code 1*" "$TYPO_TEST_ARGS"; or exit 87
string match -q "*--select*" "$TYPO_TEST_ARGS"; or exit 91

_typo_preexec git stauts
test "$TYPO_LAST_COMMAND" = "git stauts"; or exit 88
false
_typo_postexec
test "$TYPO_LAST_EXIT_CODE" = 1; or exit 89

printf "%s\n" "ok"
`)

	if !bytes.Contains(output, []byte("ok")) {
		t.Fatalf("Expected fish integration smoke test output, got %q", output)
	}
}

func TestUninstall(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Create a temp config directory
	tmpDir, err := os.MkdirTemp("", "typo-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Fatalf("RemoveAll failed: %v", err)
		}
	}()

	oldHome := os.Getenv("HOME")
	defer func() {
		if err := os.Setenv("HOME", oldHome); err != nil {
			t.Fatalf("Restore HOME failed: %v", err)
		}
	}()
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("Setenv failed: %v", err)
	}

	// Create ~/.typo directory
	typoDir := tmpDir + "/.typo"
	if err := os.MkdirAll(typoDir, 0755); err != nil {
		t.Fatalf("Failed to create .typo dir: %v", err)
	}

	os.Args = []string{"typo", "uninstall"}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := Run()

	if err := w.Close(); err != nil {
		t.Fatalf("Close pipe failed: %v", err)
	}
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("Read pipe failed: %v", err)
	}
	output := buf.String()

	if code != 0 {
		t.Errorf("Expected exit code 0, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("Cleaning up typo")) {
		t.Error("Expected output to contain 'Cleaning up typo'")
	}
	if !bytes.Contains([]byte(output), []byte("Removing config directory")) {
		t.Error("Expected output to contain 'Removing config directory'")
	}
	if !bytes.Contains([]byte(output), []byte("Local cleanup complete")) {
		t.Error("Expected output to contain 'Local cleanup complete'")
	}

	// Verify config directory was removed
	if _, err := os.Stat(typoDir); !os.IsNotExist(err) {
		t.Error("Expected .typo directory to be removed")
	}
}

func TestUninstallNonexistentConfig(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	tmpDir, err := os.MkdirTemp("", "typo-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Fatalf("RemoveAll failed: %v", err)
		}
	}()

	oldHome := os.Getenv("HOME")
	defer func() {
		if err := os.Setenv("HOME", oldHome); err != nil {
			t.Fatalf("Restore HOME failed: %v", err)
		}
	}()
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("Setenv failed: %v", err)
	}

	// Don't create .typo directory

	os.Args = []string{"typo", "uninstall"}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := Run()

	if err := w.Close(); err != nil {
		t.Fatalf("Close pipe failed: %v", err)
	}
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("Read pipe failed: %v", err)
	}
	output := buf.String()

	if code != 0 {
		t.Errorf("Expected exit code 0, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("Local cleanup complete")) {
		t.Errorf("Expected 'Local cleanup complete', got: %s", output)
	}
}

func TestUninstallWithZshrcHint(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	tmpDir, err := os.MkdirTemp("", "typo-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Fatalf("RemoveAll failed: %v", err)
		}
	}()

	oldHome := os.Getenv("HOME")
	defer func() {
		if err := os.Setenv("HOME", oldHome); err != nil {
			t.Fatalf("Restore HOME failed: %v", err)
		}
	}()
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("Setenv failed: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, ".zshrc"), []byte("eval \"$(typo init zsh)\"\n"), 0600); err != nil {
		t.Fatalf("Failed to create .zshrc: %v", err)
	}

	os.Args = []string{"typo", "uninstall"}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := Run()

	if err := w.Close(); err != nil {
		t.Fatalf("Close pipe failed: %v", err)
	}
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("Read pipe failed: %v", err)
	}
	output := buf.String()

	if code != 0 {
		t.Fatalf("Expected uninstall to succeed, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("manual cleanup required in ~/.zshrc")) {
		t.Fatalf("Expected .zshrc cleanup hint, got %q", output)
	}
}

func TestUninstallWithBashrcHint(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	tmpDir, err := os.MkdirTemp("", "typo-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Fatalf("RemoveAll failed: %v", err)
		}
	}()

	oldHome := os.Getenv("HOME")
	defer func() {
		if err := os.Setenv("HOME", oldHome); err != nil {
			t.Fatalf("Restore HOME failed: %v", err)
		}
	}()
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("Setenv failed: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, ".bashrc"), []byte("eval \"$(typo init bash)\"\n"), 0600); err != nil {
		t.Fatalf("Failed to create .bashrc: %v", err)
	}

	os.Args = []string{"typo", "uninstall"}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := Run()

	if err := w.Close(); err != nil {
		t.Fatalf("Close pipe failed: %v", err)
	}
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("Read pipe failed: %v", err)
	}
	output := buf.String()

	if code != 0 {
		t.Fatalf("Expected uninstall to succeed, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("manual cleanup required in ~/.bashrc")) {
		t.Fatalf("Expected .bashrc cleanup hint, got %q", output)
	}
}

func TestUninstallWithFishConfigHint(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	tmpDir, err := os.MkdirTemp("", "typo-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Fatalf("RemoveAll failed: %v", err)
		}
	}()

	oldHome := os.Getenv("HOME")
	defer func() {
		if err := os.Setenv("HOME", oldHome); err != nil {
			t.Fatalf("Restore HOME failed: %v", err)
		}
	}()
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("Setenv failed: %v", err)
	}

	fishConfig := filepath.Join(tmpDir, ".config", "fish", "config.fish")
	if err := os.MkdirAll(filepath.Dir(fishConfig), 0755); err != nil {
		t.Fatalf("Failed to create fish config dir: %v", err)
	}
	if err := os.WriteFile(fishConfig, []byte("typo init fish | source\n"), 0600); err != nil {
		t.Fatalf("Failed to create fish config: %v", err)
	}

	os.Args = []string{"typo", "uninstall"}

	output := captureStdout(t, func() {
		code := Run()
		if code != 0 {
			t.Fatalf("Expected uninstall to succeed, got %d", code)
		}
	})

	if !bytes.Contains([]byte(output), []byte("manual cleanup required in ~/.config/fish/config.fish")) {
		t.Fatalf("Expected fish cleanup hint, got %q", output)
	}
	if !bytes.Contains([]byte(output), []byte("typo init fish | source")) {
		t.Fatalf("Expected fish init cleanup command, got %q", output)
	}
}

func TestUninstallWithPowerShellHint(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	tmpDir, err := os.MkdirTemp("", "typo-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Fatalf("RemoveAll failed: %v", err)
		}
	}()

	oldHome := os.Getenv("HOME")
	defer func() {
		if err := os.Setenv("HOME", oldHome); err != nil {
			t.Fatalf("Restore HOME failed: %v", err)
		}
	}()
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("Setenv failed: %v", err)
	}

	oldShell := os.Getenv("SHELL")
	defer func() {
		if err := os.Setenv("SHELL", oldShell); err != nil {
			t.Fatalf("Restore SHELL failed: %v", err)
		}
	}()
	if err := os.Unsetenv("SHELL"); err != nil {
		t.Fatalf("Unsetenv SHELL failed: %v", err)
	}

	oldChannel := os.Getenv("POWERSHELL_DISTRIBUTION_CHANNEL")
	defer func() {
		if err := os.Setenv("POWERSHELL_DISTRIBUTION_CHANNEL", oldChannel); err != nil {
			t.Fatalf("Restore POWERSHELL_DISTRIBUTION_CHANNEL failed: %v", err)
		}
	}()
	if err := os.Setenv("POWERSHELL_DISTRIBUTION_CHANNEL", "PowerShell 7.5"); err != nil {
		t.Fatalf("Setenv POWERSHELL_DISTRIBUTION_CHANNEL failed: %v", err)
	}

	os.Args = []string{"typo", "uninstall"}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := Run()

	if err := w.Close(); err != nil {
		t.Fatalf("Close pipe failed: %v", err)
	}
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("Read pipe failed: %v", err)
	}
	output := buf.String()

	if code != 0 {
		t.Fatalf("Expected uninstall to succeed, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("manual cleanup required in $PROFILE.CurrentUserCurrentHost")) {
		t.Fatalf("Expected PowerShell cleanup hint, got %q", output)
	}
	if !bytes.Contains([]byte(output), []byte("Invoke-Expression (& typo init powershell | Out-String)")) {
		t.Fatalf("Expected PowerShell init cleanup command, got %q", output)
	}
}

func TestUninstallConfigRemoveFailure(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	tmpFile, err := os.CreateTemp("", "typo-home-file-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Fatalf("Remove temp file failed: %v", err)
		}
	}()
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Close temp file failed: %v", err)
	}

	oldHome := os.Getenv("HOME")
	defer func() {
		if err := os.Setenv("HOME", oldHome); err != nil {
			t.Fatalf("Restore HOME failed: %v", err)
		}
	}()
	if err := os.Setenv("HOME", tmpFile.Name()); err != nil {
		t.Fatalf("Setenv failed: %v", err)
	}

	os.Args = []string{"typo", "uninstall"}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := Run()

	if err := w.Close(); err != nil {
		t.Fatalf("Close pipe failed: %v", err)
	}
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("Read pipe failed: %v", err)
	}
	output := buf.String()

	if code != 1 {
		t.Fatalf("Expected uninstall to fail when config removal errors, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("failed:")) {
		t.Fatalf("Expected config removal failure in output, got %q", output)
	}
}

func TestUninstallInjectedErrors(t *testing.T) {
	oldArgs := os.Args
	oldUserHomeDir := userHomeDir
	oldExecutable := executable
	oldRemoveAll := removeAll
	defer func() { os.Args = oldArgs }()
	defer func() { userHomeDir = oldUserHomeDir }()
	defer func() { executable = oldExecutable }()
	defer func() { removeAll = oldRemoveAll }()

	os.Args = []string{"typo", "uninstall"}
	userHomeDir = func() (string, error) {
		return "", os.ErrNotExist
	}
	executable = func() (string, error) {
		return "", os.ErrNotExist
	}
	removeAll = func(path string) error {
		return os.ErrPermission
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := Run()

	if err := w.Close(); err != nil {
		t.Fatalf("Close pipe failed: %v", err)
	}
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("Read pipe failed: %v", err)
	}
	output := buf.String()

	if code != 1 {
		t.Fatalf("Expected uninstall to fail on injected errors, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("cannot determine home directory")) {
		t.Fatalf("Expected home directory error, got %q", output)
	}
	if !bytes.Contains([]byte(output), []byte("cannot determine binary location")) {
		t.Fatalf("Expected executable error, got %q", output)
	}
	if !bytes.Contains([]byte(output), []byte("failed:")) {
		t.Fatalf("Expected removeAll failure, got %q", output)
	}
}
