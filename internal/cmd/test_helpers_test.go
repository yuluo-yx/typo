package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
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

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Create pipe failed: %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("Close pipe failed: %v", err)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("Read pipe failed: %v", err)
	}

	return buf.String()
}

func runZshIntegrationScript(t *testing.T, script string, extraEnv ...string) []byte {
	t.Helper()

	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh is not available")
	}

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "typo.zsh")
	if err := os.WriteFile(scriptPath, []byte(zshIntegrationScript), 0600); err != nil {
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

func runBashIntegrationScript(t *testing.T, script string, extraEnv ...string) []byte {
	t.Helper()

	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is not available")
	}

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "typo.bash")
	if err := os.WriteFile(scriptPath, []byte(bashIntegrationScript), 0600); err != nil {
		t.Fatalf("Failed to write bash script: %v", err)
	}

	cmd := exec.Command("bash", "-c", script, "bash", scriptPath)
	cmd.Env = append(os.Environ(), "TMPDIR="+tmpDir, "HOME="+tmpDir)
	cmd.Env = append(cmd.Env, extraEnv...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash integration regression failed: %v\noutput:\n%s", err, output)
	}

	return output
}

func runPowerShellIntegrationScript(t *testing.T, script string, extraEnv ...string) []byte {
	t.Helper()

	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("pwsh is not available")
	}

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "typo.ps1")
	if err := os.WriteFile(scriptPath, []byte(powerShellIntegrationScript), 0600); err != nil {
		t.Fatalf("Failed to write PowerShell script: %v", err)
	}

	harnessPath := filepath.Join(tmpDir, "harness.ps1")
	harness := "param([string]$InitScriptPath)\n" + script
	if err := os.WriteFile(harnessPath, []byte(harness), 0600); err != nil {
		t.Fatalf("Failed to write PowerShell harness: %v", err)
	}

	cmd := exec.Command("pwsh", "-NoLogo", "-NoProfile", "-File", harnessPath, "-InitScriptPath", scriptPath)
	cmd.Env = append(os.Environ(), "TMPDIR="+tmpDir, "HOME="+tmpDir)
	cmd.Env = append(cmd.Env, extraEnv...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("PowerShell integration regression failed: %v\noutput:\n%s", err, output)
	}

	return output
}

func runFishIntegrationScript(t *testing.T, script string, extraEnv ...string) []byte {
	t.Helper()

	if _, err := exec.LookPath("fish"); err != nil {
		t.Skip("fish is not available")
	}

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "typo.fish")
	if err := os.WriteFile(scriptPath, []byte(fishIntegrationScript), 0600); err != nil {
		t.Fatalf("Failed to write fish script: %v", err)
	}

	cmd := exec.Command("fish", "-c", script, scriptPath)
	cmd.Env = append(os.Environ(),
		"TMPDIR="+tmpDir,
		"HOME="+tmpDir,
		"XDG_CONFIG_HOME="+filepath.Join(tmpDir, ".config"),
	)
	cmd.Env = append(cmd.Env, extraEnv...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fish integration regression failed: %v\noutput:\n%s", err, output)
	}

	return output
}
