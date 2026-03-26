package e2e

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/yuluo-yx/typo/internal/commands"
)

type e2eResult struct {
	stdout string
	stderr string
	code   int
}

type e2eEnv struct {
	root   string
	home   string
	tmpDir string
	binDir string
	bin    string
}

func newE2EEnv(t *testing.T) *e2eEnv {
	t.Helper()

	root := repoRoot(t)
	base := t.TempDir()
	home := filepath.Join(base, "home")
	tmpDir := filepath.Join(base, "tmp")
	binDir := filepath.Join(base, "bin")

	for _, dir := range []string{home, tmpDir, binDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create e2e directory: %v", err)
		}
	}

	bin := filepath.Join(binDir, "typo")
	build := exec.Command("go", "build", "-o", bin, "./cmd/typo")
	build.Dir = root
	output, err := build.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build typo binary: %v\n%s", err, output)
	}

	env := &e2eEnv{
		root:   root,
		home:   home,
		tmpDir: tmpDir,
		binDir: binDir,
		bin:    bin,
	}

	env.seedCommandStubs(t, "git", "docker", "npm", "kubectl", "brew", "yarn", "cargo", "helm", "terraform", "sudo")
	env.seedSubcommands(t, map[string][]string{
		"git":     {"status", "remote", "commit", "checkout", "branch", "pull", "push"},
		"docker":  {"build", "ps", "images", "run", "logs", "compose"},
		"npm":     {"install", "list", "run", "test", "ci"},
		"kubectl": {"apply", "describe", "get", "logs"},
		"brew":    {"install", "update", "upgrade", "list"},
	})

	return env
}

func repoRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to locate e2e source file")
	}

	root := filepath.Clean(filepath.Join(filepath.Dir(filename), ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("failed to locate repository root: %v", err)
	}

	return root
}

func (e *e2eEnv) configDir() string {
	return filepath.Join(e.home, ".typo")
}

func (e *e2eEnv) seedCommandStubs(t *testing.T, names ...string) {
	t.Helper()

	for _, name := range names {
		path := filepath.Join(e.binDir, name)
		script := "#!/bin/sh\nexit 0\n"
		if err := os.WriteFile(path, []byte(script), 0755); err != nil {
			t.Fatalf("failed to write command stub %s: %v", name, err)
		}
	}
}

func (e *e2eEnv) seedSubcommands(t *testing.T, tools map[string][]string) {
	t.Helper()

	if err := os.MkdirAll(e.configDir(), 0755); err != nil {
		t.Fatalf("failed to create config directory: %v", err)
	}

	caches := make([]commands.SubcommandCache, 0, len(tools))
	now := time.Now()
	for tool, subcommands := range tools {
		caches = append(caches, commands.SubcommandCache{
			Tool:        tool,
			Subcommands: subcommands,
			UpdatedAt:   now,
		})
	}

	data, err := json.MarshalIndent(caches, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal subcommand cache: %v", err)
	}

	cacheFile := filepath.Join(e.configDir(), "subcommands.json")
	if err := os.WriteFile(cacheFile, data, 0600); err != nil {
		t.Fatalf("failed to write subcommand cache: %v", err)
	}
}

func (e *e2eEnv) writeTempFile(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(e.tmpDir, name)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return path
}

func (e *e2eEnv) commandEnv(extra ...string) []string {
	filtered := make([]string, 0, len(os.Environ())+len(extra)+4)
	for _, item := range os.Environ() {
		if strings.HasPrefix(item, "HOME=") ||
			strings.HasPrefix(item, "PATH=") ||
			strings.HasPrefix(item, "TMPDIR=") ||
			strings.HasPrefix(item, "ZDOTDIR=") ||
			strings.HasPrefix(item, "TYPO_SHELL_INTEGRATION=") {
			continue
		}
		filtered = append(filtered, item)
	}

	filtered = append(filtered,
		"HOME="+e.home,
		"PATH="+e.binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"TMPDIR="+e.tmpDir,
		"ZDOTDIR="+e.home,
	)
	filtered = append(filtered, extra...)

	return filtered
}

func (e *e2eEnv) run(t *testing.T, args ...string) e2eResult {
	t.Helper()
	return e.runWithEnv(t, nil, args...)
}

func (e *e2eEnv) runWithEnv(t *testing.T, extraEnv []string, args ...string) e2eResult {
	t.Helper()

	cmd := exec.Command(e.bin, args...)
	cmd.Dir = e.root
	cmd.Env = e.commandEnv(extraEnv...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to execute typo: %v", err)
		}
	}

	return e2eResult{
		stdout: stdout.String(),
		stderr: stderr.String(),
		code:   code,
	}
}

func (e *e2eEnv) initZshScript(t *testing.T) string {
	t.Helper()

	result := e.run(t, "init", "zsh")
	if result.code != 0 {
		t.Fatalf("failed to generate zsh init script: %s", result.stderr)
	}

	scriptPath := filepath.Join(e.tmpDir, "typo-init.zsh")
	if err := os.WriteFile(scriptPath, []byte(result.stdout), 0600); err != nil {
		t.Fatalf("failed to write zsh init script: %v", err)
	}

	return scriptPath
}

func (e *e2eEnv) runZsh(t *testing.T, initScriptPath, script string) e2eResult {
	t.Helper()

	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh is not available; skipping zsh e2e test")
	}

	cmd := exec.Command("zsh", "-f", "-c", script, "zsh", initScriptPath)
	cmd.Dir = e.root
	cmd.Env = e.commandEnv()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to execute zsh e2e script: %v", err)
		}
	}

	return e2eResult{
		stdout: stdout.String(),
		stderr: stderr.String(),
		code:   code,
	}
}

func TestE2EReadmeExamples(t *testing.T) {
	env := newE2EEnv(t)

	version := env.run(t, "version")
	if version.code != 0 || !strings.Contains(version.stdout, "typo") {
		t.Fatalf("unexpected version output: stdout=%q stderr=%q code=%d", version.stdout, version.stderr, version.code)
	}

	initZsh := env.run(t, "init", "zsh")
	if initZsh.code != 0 || !strings.Contains(initZsh.stdout, "bindkey '\\e\\e'") {
		t.Fatalf("unexpected init zsh output: stdout=%q stderr=%q code=%d", initZsh.stdout, initZsh.stderr, initZsh.code)
	}

	fixCases := []struct {
		name    string
		command string
		want    string
	}{
		{name: "readme gut stauts", command: "gut stauts", want: "git status\n"},
		{name: "readme dcoker ps", command: "dcoker ps", want: "docker ps\n"},
		{name: "smart git subcommand", command: "git stattus", want: "git status\n"},
		{name: "smart docker subcommand", command: "docker biuld", want: "docker build\n"},
		{name: "smart kubectl subcommand", command: "kubectl aplly -f deployment.yaml", want: "kubectl apply -f deployment.yaml\n"},
	}

	for _, tt := range fixCases {
		t.Run(tt.name, func(t *testing.T) {
			result := env.run(t, "fix", tt.command)
			if result.code != 0 || result.stdout != tt.want {
				t.Fatalf("unexpected fix result: stdout=%q stderr=%q code=%d", result.stdout, result.stderr, result.code)
			}
		})
	}

	gitStderr := env.writeTempFile(t, "git.stderr", "git: 'remove' is not a git command.\n\nThe most similar command is\n\tremote\n")
	dockerStderr := env.writeTempFile(t, "docker.stderr", "unknown command: imagesa\n\nDid you mean: images?")
	npmStderr := env.writeTempFile(t, "npm.stderr", "npm ERR! Did you mean list?")

	parserCases := []struct {
		name       string
		command    string
		stderrFile string
		want       string
	}{
		{name: "git stderr parser", command: "git remove -v", stderrFile: gitStderr, want: "git remote -v\n"},
		{name: "docker stderr parser", command: "docker imagesa", stderrFile: dockerStderr, want: "docker images\n"},
		{name: "npm stderr parser", command: "npm ist --depth=0", stderrFile: npmStderr, want: "npm list --depth=0\n"},
	}

	for _, tt := range parserCases {
		t.Run(tt.name, func(t *testing.T) {
			result := env.run(t, "fix", "-s", tt.stderrFile, tt.command)
			if result.code != 0 || result.stdout != tt.want {
				t.Fatalf("unexpected stderr parser result: stdout=%q stderr=%q code=%d", result.stdout, result.stderr, result.code)
			}
		})
	}

	doctor := env.runWithEnv(t, []string{"TYPO_SHELL_INTEGRATION=1"}, "doctor")
	if doctor.code != 0 || !strings.Contains(doctor.stdout, "All checks passed") {
		t.Fatalf("doctor e2e failed: stdout=%q stderr=%q code=%d", doctor.stdout, doctor.stderr, doctor.code)
	}
}

func TestE2ELearnHistoryWorkflow(t *testing.T) {
	env := newE2EEnv(t)

	learn := env.run(t, "learn", "gut", "mygit")
	if learn.code != 0 || !strings.Contains(learn.stdout, "Learned: gut -> mygit") {
		t.Fatalf("learn failed: stdout=%q stderr=%q code=%d", learn.stdout, learn.stderr, learn.code)
	}

	fixLearned := env.run(t, "fix", "gut status")
	if fixLearned.code != 0 || fixLearned.stdout != "mygit status\n" {
		t.Fatalf("fix after learn failed: stdout=%q stderr=%q code=%d", fixLearned.stdout, fixLearned.stderr, fixLearned.code)
	}

	historyList := env.run(t, "history", "list")
	if historyList.code != 0 || !strings.Contains(historyList.stdout, "gut status -> mygit status") {
		t.Fatalf("unexpected history list result: stdout=%q stderr=%q code=%d", historyList.stdout, historyList.stderr, historyList.code)
	}

	historyClear := env.run(t, "history", "clear")
	if historyClear.code != 0 || !strings.Contains(historyClear.stdout, "History cleared") {
		t.Fatalf("history clear failed: stdout=%q stderr=%q code=%d", historyClear.stdout, historyClear.stderr, historyClear.code)
	}

	fixAfterClear := env.run(t, "fix", "gut status")
	if fixAfterClear.code != 0 || fixAfterClear.stdout != "mygit status\n" {
		t.Fatalf("learned rule did not survive history clear: stdout=%q stderr=%q code=%d", fixAfterClear.stdout, fixAfterClear.stderr, fixAfterClear.code)
	}
}

func TestE2ERulesWorkflow(t *testing.T) {
	env := newE2EEnv(t)
	customTypo := "zzzztypoexamplecmd"

	addRule := env.run(t, "rules", "add", customTypo, "mytool")
	if addRule.code != 0 || !strings.Contains(addRule.stdout, "Added rule: "+customTypo+" -> mytool") {
		t.Fatalf("rules add failed: stdout=%q stderr=%q code=%d", addRule.stdout, addRule.stderr, addRule.code)
	}

	fixCustomRule := env.run(t, "fix", customTypo+" status")
	if fixCustomRule.code != 0 || fixCustomRule.stdout != "mytool status\n" {
		t.Fatalf("custom rule fix failed: stdout=%q stderr=%q code=%d", fixCustomRule.stdout, fixCustomRule.stderr, fixCustomRule.code)
	}

	rulesList := env.run(t, "rules", "list")
	if rulesList.code != 0 || !strings.Contains(rulesList.stdout, customTypo+" -> mytool") {
		t.Fatalf("unexpected rules list result: stdout=%q stderr=%q code=%d", rulesList.stdout, rulesList.stderr, rulesList.code)
	}

	removeRule := env.run(t, "rules", "remove", customTypo)
	if removeRule.code != 0 || !strings.Contains(removeRule.stdout, "Removed rule: "+customTypo) {
		t.Fatalf("rules remove failed: stdout=%q stderr=%q code=%d", removeRule.stdout, removeRule.stderr, removeRule.code)
	}

	rulesListAfterRemove := env.run(t, "rules", "list")
	if rulesListAfterRemove.code != 0 || strings.Contains(rulesListAfterRemove.stdout, customTypo+" -> mytool") {
		t.Fatalf("rules list still contains removed custom rule: stdout=%q stderr=%q code=%d", rulesListAfterRemove.stdout, rulesListAfterRemove.stderr, rulesListAfterRemove.code)
	}

	fixAfterRemove := env.run(t, "fix", customTypo+" status")
	if fixAfterRemove.stdout == "mytool status\n" {
		t.Fatalf("removed rule still wins after deletion: stdout=%q stderr=%q code=%d", fixAfterRemove.stdout, fixAfterRemove.stderr, fixAfterRemove.code)
	}
}

func TestE2EUninstallWorkflow(t *testing.T) {
	env := newE2EEnv(t)

	prepare := env.run(t, "learn", "gut", "mygit")
	if prepare.code != 0 {
		t.Fatalf("failed to prepare uninstall scenario: stdout=%q stderr=%q code=%d", prepare.stdout, prepare.stderr, prepare.code)
	}

	zshrc := filepath.Join(env.home, ".zshrc")
	if err := os.WriteFile(zshrc, []byte("eval \"$(typo init zsh)\"\n"), 0600); err != nil {
		t.Fatalf("failed to write .zshrc: %v", err)
	}

	uninstall := env.run(t, "uninstall")
	if uninstall.code != 0 {
		t.Fatalf("uninstall failed: stdout=%q stderr=%q code=%d", uninstall.stdout, uninstall.stderr, uninstall.code)
	}
	if _, err := os.Stat(env.configDir()); !os.IsNotExist(err) {
		t.Fatalf("config directory still exists after uninstall: %v", err)
	}
	if !strings.Contains(uninstall.stdout, "eval \"$(typo init zsh)\"") || !strings.Contains(uninstall.stdout, env.bin) {
		t.Fatalf("uninstall output is incomplete: stdout=%q stderr=%q", uninstall.stdout, uninstall.stderr)
	}
}

func TestE2EZshIntegrationDailyFlow(t *testing.T) {
	env := newE2EEnv(t)
	initScript := env.initZshScript(t)

	bufferFix := env.runZsh(t, initScript, `
zle() { true; }
bindkey() { true; }
source "$1"
BUFFER="gti stauts && dcoker ps"
_typo_fix_command
[[ "$BUFFER" == "git status && docker ps" ]] || { print -r -- "$BUFFER"; exit 21; }
print -r -- "$BUFFER"
`)
	if bufferFix.code != 0 || !strings.Contains(bufferFix.stdout, "git status && docker ps") {
		t.Fatalf("zsh buffer fix failed: stdout=%q stderr=%q code=%d", bufferFix.stdout, bufferFix.stderr, bufferFix.code)
	}

	stderrFix := env.runZsh(t, initScript, `
zle() { true; }
bindkey() { true; }
source "$1"
cat > "$TYPO_STDERR_CACHE" <<'EOF'
git: 'remove' is not a git command.

The most similar command is
	remote
EOF
BUFFER="sudo git remove -v && dcoker ps"
_typo_fix_command
[[ "$BUFFER" == "sudo git remote -v && docker ps" ]] || { print -r -- "$BUFFER"; exit 22; }
print -r -- "$BUFFER"
`)
	if stderrFix.code != 0 || !strings.Contains(stderrFix.stdout, "sudo git remote -v && docker ps") {
		t.Fatalf("zsh stderr cache fix failed: stdout=%q stderr=%q code=%d", stderrFix.stdout, stderrFix.stderr, stderrFix.code)
	}
}

func TestE2EComplexFixFlows(t *testing.T) {
	env := newE2EEnv(t)
	gitStderr := env.writeTempFile(t, "complex-git.stderr", "git: 'remove' is not a git command.\n\nThe most similar command is\n\tremote\n")

	tests := []struct {
		name               string
		args               []string
		wantStdout         string
		wantStderrContains string
		wantCode           int
	}{
		{
			name:       "semicolon preserves separators",
			args:       []string{"fix", "gut status; echo ok"},
			wantStdout: "git status; echo ok\n",
			wantCode:   0,
		},
		{
			name:       "and list fixes multiple typos",
			args:       []string{"fix", "gti stauts && dcoker ps"},
			wantStdout: "git status && docker ps\n",
			wantCode:   0,
		},
		{
			name:       "second command only in and list",
			args:       []string{"fix", "echo ok && dcoker ps"},
			wantStdout: "echo ok && docker ps\n",
			wantCode:   0,
		},
		{
			name:       "pipe preserves unchanged tail command",
			args:       []string{"fix", "gut status | grep main"},
			wantStdout: "git status | grep main\n",
			wantCode:   0,
		},
		{
			name:       "pipe fixes downstream command",
			args:       []string{"fix", "echo ok | dcoker ps"},
			wantStdout: "echo ok | docker ps\n",
			wantCode:   0,
		},
		{
			name:       "command wrapper with or list",
			args:       []string{"fix", "command gti status || true"},
			wantStdout: "command git status || true\n",
			wantCode:   0,
		},
		{
			name:       "sudo wrapper after semicolon",
			args:       []string{"fix", "true; sudo gti status"},
			wantStdout: "true; sudo git status\n",
			wantCode:   0,
		},
		{
			name:       "wrapper and subcommand typos in one line",
			args:       []string{"fix", "sudo git -C repo stauts || dcoker ps"},
			wantStdout: "sudo git -C repo status || docker ps\n",
			wantCode:   0,
		},
		{
			name:       "multiple wrappers in one line",
			args:       []string{"fix", "sudo -u root gti status && env FOO=1 gut status"},
			wantStdout: "sudo -u root git status && env FOO=1 git status\n",
			wantCode:   0,
		},
		{
			name:       "quoted arguments survive compound fix",
			args:       []string{"fix", `gut commit -m "a   b" && dcoker ps`},
			wantStdout: "git commit -m \"a   b\" && docker ps\n",
			wantCode:   0,
		},
		{
			name:       "parser assisted fix still fixes later typo",
			args:       []string{"fix", "-s", gitStderr, "sudo git remove -v && dcoker ps"},
			wantStdout: "sudo git remote -v && docker ps\n",
			wantCode:   0,
		},
		{
			name:               "compound no-op returns no correction",
			args:               []string{"fix", "git status && echo ok"},
			wantStderrContains: "typo: no correction found",
			wantCode:           1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := env.run(t, tt.args...)
			if result.code != tt.wantCode {
				t.Fatalf("unexpected exit code: stdout=%q stderr=%q code=%d want=%d", result.stdout, result.stderr, result.code, tt.wantCode)
			}
			if result.stdout != tt.wantStdout {
				t.Fatalf("unexpected stdout: got=%q want=%q stderr=%q code=%d", result.stdout, tt.wantStdout, result.stderr, result.code)
			}
			if tt.wantStderrContains != "" && !strings.Contains(result.stderr, tt.wantStderrContains) {
				t.Fatalf("unexpected stderr: got=%q want substring=%q code=%d", result.stderr, tt.wantStderrContains, result.code)
			}
		})
	}
}

func TestE2EZshIntegrationComplexFlows(t *testing.T) {
	env := newE2EEnv(t)
	initScript := env.initZshScript(t)

	complexFix := env.runZsh(t, initScript, `
zle() { true; }
bindkey() { true; }
source "$1"
BUFFER='gut status | grep main && dcoker ps'
_typo_fix_command
[[ "$BUFFER" == "git status | grep main && docker ps" ]] || { print -r -- "$BUFFER"; exit 23; }
print -r -- "$BUFFER"
`)
	if complexFix.code != 0 || !strings.Contains(complexFix.stdout, "git status | grep main && docker ps") {
		t.Fatalf("zsh complex flow fix failed: stdout=%q stderr=%q code=%d", complexFix.stdout, complexFix.stderr, complexFix.code)
	}
}
