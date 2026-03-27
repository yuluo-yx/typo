package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"testing"

	installscript "github.com/yuluo-yx/typo/install"
	"github.com/yuluo-yx/typo/internal/config"
	"github.com/yuluo-yx/typo/internal/engine"
)

func TestMain(m *testing.M) {
	tmpHome, err := os.MkdirTemp("", "typo-main-test-home-*")
	if err != nil {
		panic(err)
	}

	oldHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", tmpHome); err != nil {
		panic(err)
	}

	code := m.Run()
	_ = os.Setenv("HOME", oldHome)
	_ = os.RemoveAll(tmpHome)
	os.Exit(code)
}

func runZshIntegrationScript(t *testing.T, script string, extraEnv ...string) []byte {
	t.Helper()

	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh is not available")
	}

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "typo.zsh")
	if err := os.WriteFile(scriptPath, []byte(installscript.ZshScript), 0600); err != nil {
		t.Fatalf("Failed to write zsh script: %v", err)
	}

	cmd := exec.Command("zsh", "-f", "-c", script, "zsh", scriptPath)
	cmd.Env = append(os.Environ(), "TMPDIR="+tmpDir, "HOME="+tmpDir, "ZDOTDIR="+tmpDir)
	cmd.Env = append(cmd.Env, extraEnv...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("zsh integration regression failed: %v\noutput:\n%s", err, output)
	}

	return output
}

func TestRun(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantOutput string
	}{
		{
			name:       "help",
			args:       []string{"typo", "help"},
			wantCode:   0,
			wantOutput: "typo",
		},
		{
			name:       "help flag short",
			args:       []string{"typo", "-h"},
			wantCode:   0,
			wantOutput: "typo",
		},
		{
			name:       "help flag long",
			args:       []string{"typo", "--help"},
			wantCode:   0,
			wantOutput: "typo",
		},
		{
			name:       "version",
			args:       []string{"typo", "version"},
			wantCode:   0,
			wantOutput: "typo",
		},
		{
			name:       "no args",
			args:       []string{"typo"},
			wantCode:   1,
			wantOutput: "",
		},
		{
			name:       "unknown command",
			args:       []string{"typo", "unknown"},
			wantCode:   1,
			wantOutput: "Unknown command",
		},
		{
			name:       "fix without command",
			args:       []string{"typo", "fix"},
			wantCode:   1,
			wantOutput: "command required",
		},
		{
			name:       "init zsh",
			args:       []string{"typo", "init", "zsh"},
			wantCode:   0,
			wantOutput: "bindkey",
		},
		{
			name:       "init unsupported",
			args:       []string{"typo", "init", "bash"},
			wantCode:   1,
			wantOutput: "Unsupported",
		},
		{
			name:       "learn without args",
			args:       []string{"typo", "learn"},
			wantCode:   1,
			wantOutput: "required",
		},
		{
			name:       "rules without subcommand",
			args:       []string{"typo", "rules"},
			wantCode:   1,
			wantOutput: "subcommand",
		},
		{
			name:       "history without subcommand",
			args:       []string{"typo", "history"},
			wantCode:   1,
			wantOutput: "subcommand",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Args = tt.args

			oldStdout := os.Stdout
			oldStderr := os.Stderr
			rOut, wOut, _ := os.Pipe()
			rErr, wErr, _ := os.Pipe()
			os.Stdout = wOut
			os.Stderr = wErr

			code := run()

			wOut.Close()
			wErr.Close()
			os.Stdout = oldStdout
			os.Stderr = oldStderr

			var bufOut, bufErr bytes.Buffer
			bufOut.ReadFrom(rOut)
			bufErr.ReadFrom(rErr)
			output := bufOut.String() + bufErr.String()

			if code != tt.wantCode {
				t.Errorf("run() = %d, want %d", code, tt.wantCode)
			}
			if tt.wantOutput != "" && !bytes.Contains([]byte(output), []byte(tt.wantOutput)) {
				t.Errorf("Expected output to contain %q, got %q", tt.wantOutput, output)
			}
		})
	}
}

func TestMainProcess(t *testing.T) {
	if os.Getenv("TYPO_TEST_MAIN_PROCESS") == "1" {
		os.Args = []string{"typo", "version"}
		main()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestMainProcess")
	cmd.Env = append(os.Environ(), "TYPO_TEST_MAIN_PROCESS=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("main subprocess failed: %v\noutput:\n%s", err, output)
	}
	if !bytes.Contains(output, []byte("typo")) {
		t.Fatalf("Expected main subprocess output to contain version text, got %q", output)
	}
}

func TestFixCommand(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"typo", "fix", "gut", "status"}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := run()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if code != 0 {
		t.Errorf("Expected exit code 0, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("git")) {
		t.Errorf("Expected fix output to contain 'git', got %q", output)
	}
}

func TestFixDokcerPrefersDocker(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	oldHome := os.Getenv("HOME")
	oldPath := os.Getenv("PATH")
	defer os.Setenv("HOME", oldHome)
	defer os.Setenv("PATH", oldPath)

	tmpHome := t.TempDir()
	tmpBin := t.TempDir()

	for _, name := range []string{"docker", "colcrt"} {
		path := filepath.Join(tmpBin, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
			t.Fatalf("Failed to create command stub %s: %v", name, err)
		}
	}

	if err := os.Setenv("HOME", tmpHome); err != nil {
		t.Fatalf("Setenv HOME failed: %v", err)
	}
	if err := os.Setenv("PATH", tmpBin); err != nil {
		t.Fatalf("Setenv PATH failed: %v", err)
	}

	os.Args = []string{"typo", "fix", "dokcer", "ps"}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := run()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if code != 0 {
		t.Fatalf("Expected exit code 0, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("docker ps")) {
		t.Fatalf("Expected docker ps, got %q", output)
	}
}

func TestFixNoMatch(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"typo", "fix", "asdfasdfasdfasdf"}

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	code := run()

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if code != 1 {
		t.Errorf("Expected exit code 1, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("no correction")) {
		t.Errorf("Expected 'no correction' message, got %q", output)
	}
}

func TestFixHistoryWriteError(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	tmpFile, err := os.CreateTemp("", "typo-home-file-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)
	os.Setenv("HOME", tmpFile.Name())

	os.Args = []string{"typo", "fix", "gut", "status"}

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	code := run()

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if code != 1 {
		t.Fatalf("Expected exit code 1 when history write fails, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("Error:")) {
		t.Fatalf("Expected history write error output, got %q", output)
	}
}

func TestFixValidCommandDoesNotReturnSuccess(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"typo", "fix", "git", "status"}

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	code := run()

	wOut.Close()
	wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var bufOut, bufErr bytes.Buffer
	bufOut.ReadFrom(rOut)
	bufErr.ReadFrom(rErr)

	if code != 1 {
		t.Fatalf("Expected exit code 1 for unchanged valid command, got %d", code)
	}
	if bufOut.Len() != 0 {
		t.Fatalf("Expected no stdout for unchanged valid command, got %q", bufOut.String())
	}
	if !bytes.Contains(bufErr.Bytes(), []byte("no correction found")) {
		t.Fatalf("Expected no correction message, got %q", bufErr.String())
	}
}

func TestFixWithStderrFile(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Create a temp file with stderr content
	tmpFile, err := os.CreateTemp("", "typo-stderr-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	stderrContent := "git: 'remove' is not a git command.\n\nThe most similar command is\n\tremote\n"
	tmpFile.WriteString(stderrContent)
	tmpFile.Close()

	os.Args = []string{"typo", "fix", "-s", tmpFile.Name(), "git", "remove", "-v"}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := run()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if code != 0 {
		t.Errorf("Expected exit code 0, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("remote")) {
		t.Errorf("Expected fix output to contain 'remote', got %q", output)
	}
}

func TestFixWithExitCodeAndPermissionDenied(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	tmpFile, err := os.CreateTemp("", "typo-permission-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString("mkdir: 1: Permission denied\n"); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	os.Args = []string{"typo", "fix", "--exit-code", "1", "-s", tmpFile.Name(), "mkdir", "1"}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := run()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if code != 0 {
		t.Fatalf("Expected exit code 0, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("sudo mkdir 1")) {
		t.Fatalf("Expected permission fix output, got %q", output)
	}
}

func TestFixWithGlobalOptionBeforeSubcommand(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"typo", "fix", "git", "-C", "repo", "stauts"}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := run()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if code != 0 {
		t.Fatalf("Expected exit code 0, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("git -C repo status")) {
		t.Fatalf("Expected corrected command, got %q", output)
	}
}

func TestFixWithSudoWrappedCommand(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"typo", "fix", "sudo gti status"}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := run()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if code != 0 {
		t.Fatalf("Expected exit code 0, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("sudo git status")) {
		t.Fatalf("Expected wrapped command to be corrected, got %q", output)
	}
}

func TestFixPreservesQuotedArguments(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"typo", "fix", "gut commit -m 'a   b'"}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := run()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if code != 0 {
		t.Fatalf("Expected exit code 0, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("git commit -m 'a   b'")) {
		t.Fatalf("Expected quoted argument spacing to be preserved, got %q", output)
	}
}

func TestFixPreservesCompoundCommandWithSemicolon(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"typo", "fix", "gut status; echo ok"}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := run()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if code != 0 {
		t.Fatalf("Expected exit code 0, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("git status; echo ok")) {
		t.Fatalf("Expected semicolon command to be preserved, got %q", output)
	}
}

func TestFixWithSudoWrappedCompoundCommand(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"typo", "fix", "sudo gti status && echo ok"}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := run()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if code != 0 {
		t.Fatalf("Expected exit code 0, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("sudo git status && echo ok")) {
		t.Fatalf("Expected wrapped compound command to be corrected, got %q", output)
	}
}

func TestFixCanCorrectMultipleTyposInCompoundCommand(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"typo", "fix", "gti status && dcoker ps"}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := run()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if code != 0 {
		t.Fatalf("Expected exit code 0, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("git status && docker ps")) {
		t.Fatalf("Expected both typos to be corrected, got %q", output)
	}
}

func TestFixWithNonexistentStderrFile(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Use a non-existent file
	os.Args = []string{"typo", "fix", "-s", "/nonexistent/file/12345", "gut", "status"}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := run()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Should still fix the command even without stderr file
	if code != 0 {
		t.Errorf("Expected exit code 0, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("git")) {
		t.Errorf("Expected fix output to contain 'git', got %q", output)
	}
}

func TestRulesList(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"typo", "rules", "list"}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := run()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if code != 0 {
		t.Errorf("Expected exit code 0, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("git")) {
		t.Errorf("Expected rules list to contain 'git', got %q", output)
	}
}

func TestRulesAddRemove(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Add rule
	os.Args = []string{"typo", "rules", "add", "testcmd123", "realcmd123"}

	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	code := run()

	w.Close()
	os.Stdout = oldStdout

	if code != 0 {
		t.Errorf("Expected exit code 0 for add, got %d", code)
	}

	// Remove rule
	os.Args = []string{"typo", "rules", "remove", "testcmd123"}

	_, w, _ = os.Pipe()
	os.Stdout = w

	code = run()

	w.Close()
	os.Stdout = oldStdout

	if code != 0 {
		t.Errorf("Expected exit code 0 for remove, got %d", code)
	}
}

func TestRulesAddMissingArgs(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"typo", "rules", "add", "onlyone"}

	oldStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	code := run()

	w.Close()
	os.Stderr = oldStderr

	if code != 1 {
		t.Errorf("Expected exit code 1, got %d", code)
	}
}

func TestRulesRemoveMissingArgs(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"typo", "rules", "remove"}

	oldStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	code := run()

	w.Close()
	os.Stderr = oldStderr

	if code != 1 {
		t.Errorf("Expected exit code 1, got %d", code)
	}
}

func TestRulesUnknownSubcommand(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"typo", "rules", "unknown"}

	oldStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	code := run()

	w.Close()
	os.Stderr = oldStderr

	if code != 1 {
		t.Errorf("Expected exit code 1, got %d", code)
	}
}

func TestHistoryList(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"typo", "history", "list"}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := run()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	_ = buf.String()

	if code != 0 {
		t.Errorf("Expected exit code 0, got %d", code)
	}
}

func TestHistoryClear(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"typo", "history", "clear"}

	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	code := run()

	w.Close()
	os.Stdout = oldStdout

	if code != 0 {
		t.Errorf("Expected exit code 0, got %d", code)
	}
}

func TestRulesRemoveNonexistent(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"typo", "rules", "remove", "nonexistentrule12345"}

	oldStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	code := run()

	w.Close()
	os.Stderr = oldStderr

	if code != 1 {
		t.Errorf("Expected exit code 1 for nonexistent rule, got %d", code)
	}
}

func TestHistoryUnknownSubcommand(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"typo", "history", "unknown"}

	oldStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	code := run()

	w.Close()
	os.Stderr = oldStderr

	if code != 1 {
		t.Errorf("Expected exit code 1, got %d", code)
	}
}

func TestCmdLearnError(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Create a file where a directory should be (will cause save to fail)
	tmpFile, err := os.CreateTemp("", "typo-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Set config dir to the file path
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)
	os.Setenv("HOME", tmpFile.Name())

	os.Args = []string{"typo", "learn", "wrongcmd", "rightcmd"}

	oldStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	code := run()

	w.Close()
	os.Stderr = oldStderr

	if code != 1 {
		t.Errorf("Expected exit code 1 for learn error, got %d", code)
	}
}

func TestHistoryClearError(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Create a file where a directory should be
	tmpFile, err := os.CreateTemp("", "typo-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)
	os.Setenv("HOME", tmpFile.Name())

	os.Args = []string{"typo", "history", "clear"}

	oldStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	code := run()

	w.Close()
	os.Stderr = oldStderr

	if code != 1 {
		t.Errorf("Expected exit code 1 for history clear error, got %d", code)
	}
}

func TestRulesAddError(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Create a file where a directory should be
	tmpFile, err := os.CreateTemp("", "typo-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)
	os.Setenv("HOME", tmpFile.Name())

	os.Args = []string{"typo", "rules", "add", "fromcmd", "tocmd"}

	oldStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	code := run()

	w.Close()
	os.Stderr = oldStderr

	if code != 1 {
		t.Errorf("Expected exit code 1 for rules add error, got %d", code)
	}
}

func TestFixWithMessage(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"typo", "fix", "gut", "status"}

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	code := run()

	wOut.Close()
	wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var bufOut, bufErr bytes.Buffer
	bufOut.ReadFrom(rOut)
	bufErr.ReadFrom(rErr)
	output := bufOut.String() + bufErr.String()

	if code != 0 {
		t.Errorf("Expected exit code 0, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("git")) {
		t.Errorf("Expected output to contain 'git', got %q", output)
	}
}

func TestCreateEngineWithEmptyPath(t *testing.T) {
	oldPath := os.Getenv("PATH")
	defer os.Setenv("PATH", oldPath)

	os.Setenv("PATH", "")

	cfg := &config.Config{ConfigDir: ""}
	eng := createEngine(cfg)

	if eng == nil {
		t.Error("Expected engine to be created even with empty PATH")
	}
}

func TestInitMissingShell(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"typo", "init"}

	oldStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	code := run()

	w.Close()
	os.Stderr = oldStderr

	if code != 1 {
		t.Errorf("Expected exit code 1, got %d", code)
	}
}

func TestPrintUsage(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printUsage()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !bytes.Contains([]byte(output), []byte("typo")) {
		t.Error("Expected usage to contain 'typo'")
	}
}

func TestCmdVersion(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmdVersion()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !bytes.Contains([]byte(output), []byte("typo")) {
		t.Error("Expected version output to contain 'typo'")
	}
}

func TestResolveVersionInfoUsesBuildInfoFallback(t *testing.T) {
	oldVersion, oldCommit, oldDate := version, commit, date
	oldReadBuildInfo := readBuildInfo
	t.Cleanup(func() {
		version = oldVersion
		commit = oldCommit
		date = oldDate
		readBuildInfo = oldReadBuildInfo
	})

	version = "dev"
	commit = "none"
	date = "unknown"
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Main: debug.Module{Version: "v1.2.3"},
			Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "abcdef1234567890"},
				{Key: "vcs.time", Value: "2026-03-24T14:13:08Z"},
			},
		}, true
	}

	gotVersion, gotCommit, gotDate := resolveVersionInfo()

	if gotVersion != "v1.2.3" {
		t.Fatalf("resolveVersionInfo() version = %q, want %q", gotVersion, "v1.2.3")
	}
	if gotCommit != "abcdef1" {
		t.Fatalf("resolveVersionInfo() commit = %q, want %q", gotCommit, "abcdef1")
	}
	if gotDate != "2026-03-24" {
		t.Fatalf("resolveVersionInfo() date = %q, want %q", gotDate, "2026-03-24")
	}
}

func TestResolveVersionInfoKeepsInjectedValues(t *testing.T) {
	oldVersion, oldCommit, oldDate := version, commit, date
	oldReadBuildInfo := readBuildInfo
	t.Cleanup(func() {
		version = oldVersion
		commit = oldCommit
		date = oldDate
		readBuildInfo = oldReadBuildInfo
	})

	version = "v9.9.9"
	commit = "1234567"
	date = "2026-03-01"
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Main: debug.Module{Version: "v1.2.3"},
			Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "abcdef1234567890"},
				{Key: "vcs.time", Value: "2026-03-24T14:13:08Z"},
			},
		}, true
	}

	gotVersion, gotCommit, gotDate := resolveVersionInfo()

	if gotVersion != "v9.9.9" || gotCommit != "1234567" || gotDate != "2026-03-01" {
		t.Fatalf("resolveVersionInfo() = (%q, %q, %q), want (%q, %q, %q)", gotVersion, gotCommit, gotDate, "v9.9.9", "1234567", "2026-03-01")
	}
}

func TestShortRevision(t *testing.T) {
	if got := shortRevision("abcdef123456"); got != "abcdef1" {
		t.Fatalf("shortRevision() = %q, want %q", got, "abcdef1")
	}
	if got := shortRevision("abc123"); got != "abc123" {
		t.Fatalf("shortRevision() short input = %q, want %q", got, "abc123")
	}
}

func TestFormatBuildDate(t *testing.T) {
	if got := formatBuildDate("2026-03-24T14:13:08Z"); got != "2026-03-24" {
		t.Fatalf("formatBuildDate() = %q, want %q", got, "2026-03-24")
	}
	if got := formatBuildDate("not-a-date"); got != "not-a-date" {
		t.Fatalf("formatBuildDate() invalid input = %q, want raw input", got)
	}
}

func TestPrintZshIntegration(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printZshIntegration()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if output != installscript.ZshScript {
		t.Error("Expected zsh integration output to match embedded install script")
	}
	if !bytes.Contains([]byte(output), []byte("_typo_cleanup_stale_caches")) {
		t.Error("Expected zsh integration to include stale cache cleanup")
	}
}

func TestPrintZshIntegrationAddsTrailingNewline(t *testing.T) {
	original := installscript.ZshScript
	installscript.ZshScript = "echo test"
	t.Cleanup(func() {
		installscript.ZshScript = original
	})

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printZshIntegration()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	if buf.String() != "echo test\n" {
		t.Fatalf("Expected trailing newline to be appended, got %q", buf.String())
	}
}

func TestGetGoBinDir(t *testing.T) {
	oldGoBin := os.Getenv("GOBIN")
	oldGoPath := os.Getenv("GOPATH")
	oldHome := os.Getenv("HOME")
	defer os.Setenv("GOBIN", oldGoBin)
	defer os.Setenv("GOPATH", oldGoPath)
	defer os.Setenv("HOME", oldHome)

	if err := os.Setenv("GOBIN", "/tmp/custom-bin"); err != nil {
		t.Fatalf("Setenv GOBIN failed: %v", err)
	}
	if got := getGoBinDir(); got != "/tmp/custom-bin" {
		t.Fatalf("getGoBinDir() with GOBIN = %q", got)
	}

	os.Unsetenv("GOBIN")
	if err := os.Setenv("GOPATH", "/tmp/custom-gopath"); err != nil {
		t.Fatalf("Setenv GOPATH failed: %v", err)
	}
	if got := getGoBinDir(); got != "/tmp/custom-gopath/bin" {
		t.Fatalf("getGoBinDir() with GOPATH = %q", got)
	}

	os.Unsetenv("GOPATH")
	tmpHome := t.TempDir()
	if err := os.Setenv("HOME", tmpHome); err != nil {
		t.Fatalf("Setenv HOME failed: %v", err)
	}
	if got := getGoBinDir(); got != filepath.Join(tmpHome, "go", "bin") {
		t.Fatalf("getGoBinDir() default = %q", got)
	}
}

func TestGetGoBinDir_UserHomeError(t *testing.T) {
	oldGoBin := os.Getenv("GOBIN")
	oldGoPath := os.Getenv("GOPATH")
	oldUserHomeDir := userHomeDir
	defer os.Setenv("GOBIN", oldGoBin)
	defer os.Setenv("GOPATH", oldGoPath)
	defer func() { userHomeDir = oldUserHomeDir }()

	os.Unsetenv("GOBIN")
	os.Unsetenv("GOPATH")
	userHomeDir = func() (string, error) {
		return "", os.ErrNotExist
	}

	if got := getGoBinDir(); got != "" {
		t.Fatalf("Expected empty go bin dir on home lookup error, got %q", got)
	}
}

func TestCheckGoBinTypo(t *testing.T) {
	oldGoBin := os.Getenv("GOBIN")
	oldGoPath := os.Getenv("GOPATH")
	defer os.Setenv("GOBIN", oldGoBin)
	defer os.Setenv("GOPATH", oldGoPath)

	goBinDir := t.TempDir()
	os.Unsetenv("GOPATH")
	if err := os.Setenv("GOBIN", goBinDir); err != nil {
		t.Fatalf("Setenv GOBIN failed: %v", err)
	}

	if got := checkGoBinTypo(); got != "" {
		t.Fatalf("Expected empty path when typo binary is missing, got %q", got)
	}

	typoPath := filepath.Join(goBinDir, "typo")
	if err := os.WriteFile(typoPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("Failed to write typo stub: %v", err)
	}

	if got := checkGoBinTypo(); got != goBinDir {
		t.Fatalf("Expected go bin dir %q, got %q", goBinDir, got)
	}
}

func TestCheckGoBinTypo_EmptyGoBinDir(t *testing.T) {
	oldGoBin := os.Getenv("GOBIN")
	oldGoPath := os.Getenv("GOPATH")
	oldUserHomeDir := userHomeDir
	defer os.Setenv("GOBIN", oldGoBin)
	defer os.Setenv("GOPATH", oldGoPath)
	defer func() { userHomeDir = oldUserHomeDir }()

	os.Unsetenv("GOBIN")
	os.Unsetenv("GOPATH")
	userHomeDir = func() (string, error) {
		return "", os.ErrNotExist
	}

	if got := checkGoBinTypo(); got != "" {
		t.Fatalf("Expected empty go bin dir result, got %q", got)
	}
}

func TestSameDirAndPathContainsDir(t *testing.T) {
	if !sameDir("/tmp/dir", "/tmp/dir/") {
		t.Fatal("Expected directories with trailing slash to match")
	}
	if sameDir("", "/tmp/dir") {
		t.Fatal("Expected empty dir to not match")
	}
	if !pathContainsDir("/usr/bin:/tmp/dir:/bin", "/tmp/dir") {
		t.Fatal("Expected path to contain directory")
	}
	if pathContainsDir("/usr/bin:/bin", "/tmp/dir") {
		t.Fatal("Expected path to not contain directory")
	}
}

func TestDoctor(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	oldLookPath := lookPath
	defer func() { lookPath = oldLookPath }()
	lookPath = func(file string) (string, error) {
		return "/usr/local/bin/typo", nil
	}

	oldEnv := os.Getenv("TYPO_SHELL_INTEGRATION")
	defer os.Setenv("TYPO_SHELL_INTEGRATION", oldEnv)
	os.Unsetenv("TYPO_SHELL_INTEGRATION")

	os.Args = []string{"typo", "doctor"}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := run()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if code != 1 {
		t.Errorf("Expected exit code 1 (shell integration not loaded), got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("Checking typo configuration")) {
		t.Error("Expected doctor output to contain 'Checking typo configuration'")
	}
	if !bytes.Contains([]byte(output), []byte("typo command")) {
		t.Error("Expected doctor output to contain 'typo command'")
	}
	if !bytes.Contains([]byte(output), []byte("shell integration")) {
		t.Error("Expected doctor output to contain 'shell integration'")
	}
}

func TestDoctorWithShellIntegration(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	oldLookPath := lookPath
	defer func() { lookPath = oldLookPath }()
	lookPath = func(file string) (string, error) {
		return "/usr/local/bin/typo", nil
	}

	oldEnv := os.Getenv("TYPO_SHELL_INTEGRATION")
	defer os.Setenv("TYPO_SHELL_INTEGRATION", oldEnv)
	os.Setenv("TYPO_SHELL_INTEGRATION", "1")

	os.Args = []string{"typo", "doctor"}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := run()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if code != 0 {
		t.Errorf("Expected exit code 0, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("shell integration: ✓ loaded")) {
		t.Errorf("Expected shell integration to be loaded, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("All checks passed")) {
		t.Errorf("Expected 'All checks passed', got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("Go bin PATH")) {
		t.Error("Expected doctor output to contain 'Go bin PATH'")
	}
}

func TestDoctorGoBinNotInPath(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	oldLookPath := lookPath
	defer func() { lookPath = oldLookPath }()

	oldEnv := os.Getenv("TYPO_SHELL_INTEGRATION")
	defer os.Setenv("TYPO_SHELL_INTEGRATION", oldEnv)
	os.Setenv("TYPO_SHELL_INTEGRATION", "1")

	tmpDir := t.TempDir()
	goBinDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(goBinDir, 0755); err != nil {
		t.Fatalf("Failed to create go bin dir: %v", err)
	}

	oldGoBin := os.Getenv("GOBIN")
	defer os.Setenv("GOBIN", oldGoBin)
	os.Setenv("GOBIN", goBinDir)

	oldPath := os.Getenv("PATH")
	defer os.Setenv("PATH", oldPath)
	os.Setenv("PATH", "/usr/bin:/bin")

	lookPath = func(file string) (string, error) {
		return filepath.Join(goBinDir, "typo"), nil
	}

	os.Args = []string{"typo", "doctor"}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := run()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if code != 1 {
		t.Errorf("Expected exit code 1, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("not in PATH")) {
		t.Errorf("Expected Go bin PATH warning, got: %s", output)
	}
}

func TestDoctorTypoMissingFromPath(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	oldLookPath := lookPath
	defer func() { lookPath = oldLookPath }()
	lookPath = func(file string) (string, error) {
		return "", os.ErrNotExist
	}

	oldEnv := os.Getenv("TYPO_SHELL_INTEGRATION")
	defer os.Setenv("TYPO_SHELL_INTEGRATION", oldEnv)
	os.Setenv("TYPO_SHELL_INTEGRATION", "1")

	os.Args = []string{"typo", "doctor"}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := run()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if code != 1 {
		t.Errorf("Expected exit code 1, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("not found in PATH")) {
		t.Errorf("Expected missing PATH message, got: %s", output)
	}
}

func TestFixWritesUsageHistory(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"typo", "fix", "gut", "status"}

	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	code := run()

	w.Close()
	os.Stdout = oldStdout

	if code != 0 {
		t.Fatalf("Expected exit code 0, got %d", code)
	}

	cfg := config.Load()
	if _, err := os.Stat(filepath.Join(cfg.ConfigDir, "usage_history.json")); err != nil {
		t.Fatalf("Expected usage_history.json to exist, got %v", err)
	}
}

func TestLearnSurvivesHistoryClear(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"typo", "learn", "gut", "mygit"}
	if code := run(); code != 0 {
		t.Fatalf("Expected learn to succeed, got %d", code)
	}

	os.Args = []string{"typo", "history", "clear"}
	if code := run(); code != 0 {
		t.Fatalf("Expected history clear to succeed, got %d", code)
	}

	os.Args = []string{"typo", "fix", "gut", "status"}
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := run()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if code != 0 {
		t.Fatalf("Expected fix to succeed, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("mygit status")) {
		t.Fatalf("Expected learned rule to survive history clear, got %q", output)
	}
}

func TestLearnOverridesConflictingHistory(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	oldHome := os.Getenv("HOME")
	oldPath := os.Getenv("PATH")
	defer os.Setenv("HOME", oldHome)
	defer os.Setenv("PATH", oldPath)

	tmpHome := t.TempDir()
	tmpBin := t.TempDir()

	for _, name := range []string{"docker", "colcrt"} {
		path := filepath.Join(tmpBin, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
			t.Fatalf("Failed to create command stub %s: %v", name, err)
		}
	}

	if err := os.Setenv("HOME", tmpHome); err != nil {
		t.Fatalf("Setenv HOME failed: %v", err)
	}
	if err := os.Setenv("PATH", tmpBin); err != nil {
		t.Fatalf("Setenv PATH failed: %v", err)
	}

	cfg := config.Load()
	history := engine.NewHistory(cfg.ConfigDir)
	if err := history.Record("dokcer ps", "colcrt ps"); err != nil {
		t.Fatalf("Seed history failed: %v", err)
	}

	os.Args = []string{"typo", "learn", "dokcer", "docker"}
	if code := run(); code != 0 {
		t.Fatalf("Expected learn to succeed, got %d", code)
	}

	history = engine.NewHistory(cfg.ConfigDir)
	if _, ok := history.Lookup("dokcer ps"); ok {
		t.Fatal("Expected conflicting history to be cleared after learn")
	}

	os.Args = []string{"typo", "fix", "dokcer", "ps"}
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := run()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if code != 0 {
		t.Fatalf("Expected fix to succeed, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("docker ps")) {
		t.Fatalf("Expected learned rule to override old history, got %q", output)
	}
}

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

func TestUninstall(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Create a temp config directory
	tmpDir, err := os.MkdirTemp("", "typo-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)
	os.Setenv("HOME", tmpDir)

	// Create ~/.typo directory
	typoDir := tmpDir + "/.typo"
	if err := os.MkdirAll(typoDir, 0755); err != nil {
		t.Fatalf("Failed to create .typo dir: %v", err)
	}

	os.Args = []string{"typo", "uninstall"}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := run()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if code != 0 {
		t.Errorf("Expected exit code 0, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("Uninstalling typo")) {
		t.Error("Expected output to contain 'Uninstalling typo'")
	}
	if !bytes.Contains([]byte(output), []byte("Removing config directory")) {
		t.Error("Expected output to contain 'Removing config directory'")
	}
	if !bytes.Contains([]byte(output), []byte("Uninstallation complete")) {
		t.Error("Expected output to contain 'Uninstallation complete'")
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
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)
	os.Setenv("HOME", tmpDir)

	// Don't create .typo directory

	os.Args = []string{"typo", "uninstall"}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := run()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if code != 0 {
		t.Errorf("Expected exit code 0, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("Uninstallation complete")) {
		t.Errorf("Expected 'Uninstallation complete', got: %s", output)
	}
}

func TestUninstallWithZshrcHint(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	tmpDir, err := os.MkdirTemp("", "typo-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)
	os.Setenv("HOME", tmpDir)

	if err := os.WriteFile(filepath.Join(tmpDir, ".zshrc"), []byte("eval \"$(typo init zsh)\"\n"), 0600); err != nil {
		t.Fatalf("Failed to create .zshrc: %v", err)
	}

	os.Args = []string{"typo", "uninstall"}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := run()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if code != 0 {
		t.Fatalf("Expected uninstall to succeed, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("please remove the following line from ~/.zshrc")) {
		t.Fatalf("Expected .zshrc cleanup hint, got %q", output)
	}
}

func TestUninstallConfigRemoveFailure(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	tmpFile, err := os.CreateTemp("", "typo-home-file-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)
	os.Setenv("HOME", tmpFile.Name())

	os.Args = []string{"typo", "uninstall"}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := run()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
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

	code := run()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
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
