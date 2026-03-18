package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/yuluo-yx/typo/internal/config"
)

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

	if !bytes.Contains([]byte(output), []byte("bindkey")) {
		t.Error("Expected zsh integration to contain 'bindkey'")
	}
}

func TestDoctor(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

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

	oldEnv := os.Getenv("TYPO_SHELL_INTEGRATION")
	defer os.Setenv("TYPO_SHELL_INTEGRATION", oldEnv)
	os.Setenv("TYPO_SHELL_INTEGRATION", "1")

	oldPath := os.Getenv("PATH")
	defer os.Setenv("PATH", oldPath)
	// Set PATH without go bin
	os.Setenv("PATH", "/usr/bin:/bin")

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

	// Should fail because Go bin not in PATH
	if code != 1 {
		t.Errorf("Expected exit code 1, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("not in PATH")) {
		t.Errorf("Expected 'not in PATH', got: %s", output)
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
