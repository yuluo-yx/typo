package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuluo-yx/typo/internal/config"
	itypes "github.com/yuluo-yx/typo/internal/types"
)

func TestGetGoBinDir(t *testing.T) {
	oldGoBin := os.Getenv("GOBIN")
	oldGoPath := os.Getenv("GOPATH")
	oldHome := os.Getenv("HOME")
	defer func() {
		if err := os.Setenv("GOBIN", oldGoBin); err != nil {
			t.Fatalf("Restore GOBIN failed: %v", err)
		}
	}()
	defer func() {
		if err := os.Setenv("GOPATH", oldGoPath); err != nil {
			t.Fatalf("Restore GOPATH failed: %v", err)
		}
	}()
	defer func() {
		if err := os.Setenv("HOME", oldHome); err != nil {
			t.Fatalf("Restore HOME failed: %v", err)
		}
	}()

	if err := os.Setenv("GOBIN", "/tmp/custom-bin"); err != nil {
		t.Fatalf("Setenv GOBIN failed: %v", err)
	}
	if got := getGoBinDir(); got != "/tmp/custom-bin" {
		t.Fatalf("getGoBinDir() with GOBIN = %q", got)
	}

	if err := os.Unsetenv("GOBIN"); err != nil {
		t.Fatalf("Unsetenv GOBIN failed: %v", err)
	}
	if err := os.Setenv("GOPATH", "/tmp/custom-gopath"); err != nil {
		t.Fatalf("Setenv GOPATH failed: %v", err)
	}
	if got := getGoBinDir(); got != "/tmp/custom-gopath/bin" {
		t.Fatalf("getGoBinDir() with GOPATH = %q", got)
	}

	if err := os.Unsetenv("GOPATH"); err != nil {
		t.Fatalf("Unsetenv GOPATH failed: %v", err)
	}
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
	defer func() {
		if err := os.Setenv("GOBIN", oldGoBin); err != nil {
			t.Fatalf("Restore GOBIN failed: %v", err)
		}
	}()
	defer func() {
		if err := os.Setenv("GOPATH", oldGoPath); err != nil {
			t.Fatalf("Restore GOPATH failed: %v", err)
		}
	}()
	defer func() { userHomeDir = oldUserHomeDir }()

	if err := os.Unsetenv("GOBIN"); err != nil {
		t.Fatalf("Unsetenv GOBIN failed: %v", err)
	}
	if err := os.Unsetenv("GOPATH"); err != nil {
		t.Fatalf("Unsetenv GOPATH failed: %v", err)
	}
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
	defer func() {
		if err := os.Setenv("GOBIN", oldGoBin); err != nil {
			t.Fatalf("Restore GOBIN failed: %v", err)
		}
	}()
	defer func() {
		if err := os.Setenv("GOPATH", oldGoPath); err != nil {
			t.Fatalf("Restore GOPATH failed: %v", err)
		}
	}()

	goBinDir := t.TempDir()
	if err := os.Unsetenv("GOPATH"); err != nil {
		t.Fatalf("Unsetenv GOPATH failed: %v", err)
	}
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
	defer func() {
		if err := os.Setenv("GOBIN", oldGoBin); err != nil {
			t.Fatalf("Restore GOBIN failed: %v", err)
		}
	}()
	defer func() {
		if err := os.Setenv("GOPATH", oldGoPath); err != nil {
			t.Fatalf("Restore GOPATH failed: %v", err)
		}
	}()
	defer func() { userHomeDir = oldUserHomeDir }()

	if err := os.Unsetenv("GOBIN"); err != nil {
		t.Fatalf("Unsetenv GOBIN failed: %v", err)
	}
	if err := os.Unsetenv("GOPATH"); err != nil {
		t.Fatalf("Unsetenv GOPATH failed: %v", err)
	}
	userHomeDir = func() (string, error) {
		return "", os.ErrNotExist
	}

	if got := checkGoBinTypo(); got != "" {
		t.Fatalf("Expected empty go bin dir result, got %q", got)
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
	defer func() {
		if err := os.Setenv("TYPO_SHELL_INTEGRATION", oldEnv); err != nil {
			t.Fatalf("Restore TYPO_SHELL_INTEGRATION failed: %v", err)
		}
	}()
	if err := os.Unsetenv("TYPO_SHELL_INTEGRATION"); err != nil {
		t.Fatalf("Unsetenv TYPO_SHELL_INTEGRATION failed: %v", err)
	}
	oldShell := os.Getenv("SHELL")
	defer func() {
		if err := os.Setenv("SHELL", oldShell); err != nil {
			t.Fatalf("Restore SHELL failed: %v", err)
		}
	}()
	if err := os.Setenv("SHELL", "/bin/zsh"); err != nil {
		t.Fatalf("Setenv SHELL failed: %v", err)
	}

	os.Args = []string{"typo", "doctor"}

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
	if !bytes.Contains([]byte(output), []byte("config file")) {
		t.Error("Expected doctor output to contain 'config file'")
	}
	if !bytes.Contains([]byte(output), []byte("effective config")) {
		t.Error("Expected doctor output to contain 'effective config'")
	}
	if !bytes.Contains([]byte(output), []byte("keyboard=qwerty")) {
		t.Error("Expected doctor output to contain default keyboard setting")
	}
	if !bytes.Contains([]byte(output), []byte("shell: zsh")) {
		t.Error("Expected doctor output to contain current shell")
	}
}

func TestDoctorShowsBashHintsWhenShellIsBash(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	oldLookPath := lookPath
	defer func() { lookPath = oldLookPath }()
	lookPath = func(file string) (string, error) {
		return "/usr/local/bin/typo", nil
	}

	oldShell := os.Getenv("SHELL")
	defer func() {
		if err := os.Setenv("SHELL", oldShell); err != nil {
			t.Fatalf("Restore SHELL failed: %v", err)
		}
	}()
	if err := os.Setenv("SHELL", "/bin/bash"); err != nil {
		t.Fatalf("Setenv SHELL failed: %v", err)
	}

	oldIntegration := os.Getenv("TYPO_SHELL_INTEGRATION")
	defer func() {
		if err := os.Setenv("TYPO_SHELL_INTEGRATION", oldIntegration); err != nil {
			t.Fatalf("Restore TYPO_SHELL_INTEGRATION failed: %v", err)
		}
	}()
	if err := os.Unsetenv("TYPO_SHELL_INTEGRATION"); err != nil {
		t.Fatalf("Unsetenv TYPO_SHELL_INTEGRATION failed: %v", err)
	}

	os.Args = []string{"typo", "doctor"}

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
		t.Fatalf("Expected exit code 1, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("~/.bashrc")) {
		t.Fatalf("Expected bashrc hint in doctor output, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte(`eval "$(typo init bash)"`)) {
		t.Fatalf("Expected init bash hint in doctor output, got: %s", output)
	}
}

func TestDoctorShowsFishHintsWhenShellIsFish(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	oldLookPath := lookPath
	defer func() { lookPath = oldLookPath }()
	lookPath = func(file string) (string, error) {
		return "/usr/local/bin/typo", nil
	}

	t.Setenv("SHELL", "/opt/homebrew/bin/fish")
	t.Setenv("TYPO_ACTIVE_SHELL", "")
	t.Setenv("TYPO_SHELL_INTEGRATION", "")
	t.Setenv("POWERSHELL_DISTRIBUTION_CHANNEL", "")
	t.Setenv("PSModulePath", "")
	t.Setenv("PSExecutionPolicyPreference", "")

	os.Args = []string{"typo", "doctor"}

	output := captureStdout(t, func() {
		code := Run()
		if code != 1 {
			t.Fatalf("Expected exit code 1, got %d", code)
		}
	})

	if !bytes.Contains([]byte(output), []byte("shell: fish")) {
		t.Fatalf("Expected fish shell in doctor output, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("~/.config/fish/config.fish")) {
		t.Fatalf("Expected fish config hint in doctor output, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("typo init fish | source")) {
		t.Fatalf("Expected init fish hint in doctor output, got: %s", output)
	}
}

func TestDoctorShowsPowerShellHintsWhenShellIsPowerShell(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	oldLookPath := lookPath
	defer func() { lookPath = oldLookPath }()
	lookPath = func(file string) (string, error) {
		return "/usr/local/bin/typo", nil
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

	oldPSModulePath := os.Getenv("PSModulePath")
	defer func() {
		if err := os.Setenv("PSModulePath", oldPSModulePath); err != nil {
			t.Fatalf("Restore PSModulePath failed: %v", err)
		}
	}()
	if err := os.Setenv("PSModulePath", "/tmp/powershell-modules"); err != nil {
		t.Fatalf("Setenv PSModulePath failed: %v", err)
	}

	oldIntegration := os.Getenv("TYPO_SHELL_INTEGRATION")
	defer func() {
		if err := os.Setenv("TYPO_SHELL_INTEGRATION", oldIntegration); err != nil {
			t.Fatalf("Restore TYPO_SHELL_INTEGRATION failed: %v", err)
		}
	}()
	if err := os.Unsetenv("TYPO_SHELL_INTEGRATION"); err != nil {
		t.Fatalf("Unsetenv TYPO_SHELL_INTEGRATION failed: %v", err)
	}

	os.Args = []string{"typo", "doctor"}

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
		t.Fatalf("Expected exit code 1, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("$PROFILE.CurrentUserCurrentHost")) {
		t.Fatalf("Expected PowerShell profile hint in doctor output, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("Invoke-Expression (& typo init powershell | Out-String)")) {
		t.Fatalf("Expected PowerShell init hint in doctor output, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("shell: powershell")) {
		t.Fatalf("Expected detected shell to be powershell, got: %s", output)
	}
}

func TestDoctorJSONReportsChecksAndShellCapabilities(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	oldLookPath := lookPath
	defer func() { lookPath = oldLookPath }()
	lookPath = func(file string) (string, error) {
		return "/usr/local/bin/typo", nil
	}

	t.Setenv("SHELL", "/opt/homebrew/bin/fish")
	t.Setenv("TYPO_ACTIVE_SHELL", "")
	t.Setenv("TYPO_SHELL_INTEGRATION", "")
	t.Setenv("POWERSHELL_DISTRIBUTION_CHANNEL", "")
	t.Setenv("PSModulePath", "")
	t.Setenv("PSExecutionPolicyPreference", "")

	os.Args = []string{"typo", "doctor", "--json"}
	code := 0
	output := captureStdout(t, func() {
		code = Run()
	})

	if code != 1 {
		t.Fatalf("Expected doctor --json to preserve failing exit code, got %d output=%s", code, output)
	}
	if strings.Contains(output, "Checking typo configuration") {
		t.Fatalf("doctor --json should not print human output, got %q", output)
	}

	var payload struct {
		SchemaVersion int  `json:"schema_version"`
		OK            bool `json:"ok"`
		Shell         struct {
			Name                    string `json:"name"`
			ConfigFile              string `json:"config_file"`
			InitCommand             string `json:"init_command"`
			IntegrationLoaded       bool   `json:"integration_loaded"`
			StderrCacheSupported    bool   `json:"stderr_cache_supported"`
			AliasContextSupported   bool   `json:"alias_context_supported"`
			EnvironmentContextShown bool   `json:"environment_context_supported"`
		} `json:"shell"`
		Checks []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"checks"`
		Config struct {
			Settings []struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			} `json:"settings"`
		} `json:"config"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("doctor --json should emit valid JSON: %v output=%q", err, output)
	}
	if payload.SchemaVersion != 1 {
		t.Fatalf("schema_version = %d, want 1", payload.SchemaVersion)
	}
	if payload.OK {
		t.Fatalf("expected ok=false when shell integration is missing")
	}
	if payload.Shell.Name != "fish" || payload.Shell.ConfigFile != "~/.config/fish/config.fish" {
		t.Fatalf("unexpected shell payload: %+v", payload.Shell)
	}
	if payload.Shell.InitCommand != "typo init fish | source" {
		t.Fatalf("unexpected fish init command: %q", payload.Shell.InitCommand)
	}
	if payload.Shell.IntegrationLoaded {
		t.Fatalf("expected shell integration to be reported unloaded")
	}
	if payload.Shell.StderrCacheSupported {
		t.Fatalf("fish should report stderr_cache_supported=false until real stderr capture is implemented")
	}
	if !payload.Shell.AliasContextSupported || !payload.Shell.EnvironmentContextShown {
		t.Fatalf("fish should report alias and environment context support: %+v", payload.Shell)
	}
	if len(payload.Checks) != 6 {
		t.Fatalf("expected 6 doctor checks, got %+v", payload.Checks)
	}
	if len(payload.Config.Settings) == 0 {
		t.Fatalf("expected effective config settings in JSON payload")
	}
}

func TestDoctorJSONReportsSuccessfulConfiguredEnvironment(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	oldLookPath := lookPath
	defer func() { lookPath = oldLookPath }()

	tmpHome := t.TempDir()
	goBin := filepath.Join(tmpHome, "go", "bin")
	typoPath := filepath.Join(goBin, "typo")
	if err := os.MkdirAll(goBin, 0755); err != nil {
		t.Fatalf("mkdir go bin: %v", err)
	}
	lookPath = func(file string) (string, error) {
		return typoPath, nil
	}

	t.Setenv("HOME", tmpHome)
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", filepath.Join(tmpHome, "go"))
	t.Setenv("PATH", strings.Join([]string{"/usr/bin", goBin}, string(os.PathListSeparator)))
	t.Setenv("SHELL", "/bin/zsh")
	t.Setenv("TYPO_ACTIVE_SHELL", "")
	t.Setenv("TYPO_SHELL_INTEGRATION", "1")
	t.Setenv("POWERSHELL_DISTRIBUTION_CHANNEL", "")
	t.Setenv("PSModulePath", "")
	t.Setenv("PSExecutionPolicyPreference", "")

	cfg := config.Load()
	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	os.Args = []string{"typo", "doctor", "--json"}
	code := 0
	output := captureStdout(t, func() {
		code = Run()
	})
	if code != 0 {
		t.Fatalf("Expected doctor --json success, got %d output=%s", code, output)
	}

	var payload struct {
		OK    bool `json:"ok"`
		Shell struct {
			Name                 string `json:"name"`
			StderrCacheSupported bool   `json:"stderr_cache_supported"`
		} `json:"shell"`
		Config struct {
			FileExists bool `json:"file_exists"`
		} `json:"config"`
		Install struct {
			Method          string `json:"method"`
			UpdateSupported bool   `json:"update_supported"`
		} `json:"install"`
		GoBinPath struct {
			TypoInGoBin bool `json:"typo_in_go_bin"`
			Configured  bool `json:"configured"`
		} `json:"go_bin_path"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("doctor --json should emit valid JSON: %v output=%q", err, output)
	}
	if !payload.OK || payload.Shell.Name != "zsh" || !payload.Shell.StderrCacheSupported {
		t.Fatalf("unexpected success shell payload: %+v", payload)
	}
	if !payload.Config.FileExists {
		t.Fatalf("expected config file to be reported")
	}
	if payload.Install.Method != "go install" || payload.Install.UpdateSupported {
		t.Fatalf("unexpected install payload: %+v", payload.Install)
	}
	if !payload.GoBinPath.TypoInGoBin || !payload.GoBinPath.Configured {
		t.Fatalf("unexpected Go bin payload: %+v", payload.GoBinPath)
	}
}

func TestDoctorJSONReportsGoBinPathFailure(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	oldLookPath := lookPath
	defer func() { lookPath = oldLookPath }()

	tmpHome := t.TempDir()
	goBin := filepath.Join(tmpHome, "go", "bin")
	typoPath := filepath.Join(goBin, "typo")
	lookPath = func(file string) (string, error) {
		return typoPath, nil
	}

	t.Setenv("HOME", tmpHome)
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", filepath.Join(tmpHome, "go"))
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("SHELL", "/bin/bash")
	t.Setenv("TYPO_ACTIVE_SHELL", "")
	t.Setenv("TYPO_SHELL_INTEGRATION", "1")
	t.Setenv("POWERSHELL_DISTRIBUTION_CHANNEL", "")
	t.Setenv("PSModulePath", "")
	t.Setenv("PSExecutionPolicyPreference", "")

	os.Args = []string{"typo", "doctor", "--json"}
	code := 0
	output := captureStdout(t, func() {
		code = Run()
	})
	if code != 1 {
		t.Fatalf("Expected doctor --json to fail for missing Go bin PATH, got %d output=%s", code, output)
	}

	var payload struct {
		OK      bool `json:"ok"`
		Actions []struct {
			ID      string `json:"id"`
			Command string `json:"command"`
		} `json:"actions"`
		Checks []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("doctor --json should emit valid JSON: %v output=%q", err, output)
	}
	if payload.OK {
		t.Fatalf("expected ok=false for missing Go bin PATH")
	}
	foundGoBinFailure := false
	for _, check := range payload.Checks {
		if check.ID == "go_bin_path" && check.Status == "fail" {
			foundGoBinFailure = true
		}
	}
	if !foundGoBinFailure {
		t.Fatalf("expected go_bin_path failure in checks: %+v", payload.Checks)
	}
	if len(payload.Actions) == 0 || !strings.Contains(payload.Actions[0].Command, goBin) {
		t.Fatalf("expected PATH action for Go bin, got %+v", payload.Actions)
	}
}

func TestCurrentShellNamePrefersTypoActiveShell(t *testing.T) {
	oldActiveShell := os.Getenv("TYPO_ACTIVE_SHELL")
	defer func() {
		if err := os.Setenv("TYPO_ACTIVE_SHELL", oldActiveShell); err != nil {
			t.Fatalf("Restore TYPO_ACTIVE_SHELL failed: %v", err)
		}
	}()
	if err := os.Setenv("TYPO_ACTIVE_SHELL", "pwsh"); err != nil {
		t.Fatalf("Setenv TYPO_ACTIVE_SHELL failed: %v", err)
	}

	oldShell := os.Getenv("SHELL")
	defer func() {
		if err := os.Setenv("SHELL", oldShell); err != nil {
			t.Fatalf("Restore SHELL failed: %v", err)
		}
	}()
	if err := os.Setenv("SHELL", "/bin/zsh"); err != nil {
		t.Fatalf("Setenv SHELL failed: %v", err)
	}

	if got := currentShellName(); got != "powershell" {
		t.Fatalf("currentShellName() = %q, want %q", got, "powershell")
	}
}

func TestCurrentShellNameFallsBackToPowerShellEnvironment(t *testing.T) {
	oldActiveShell := os.Getenv("TYPO_ACTIVE_SHELL")
	defer func() {
		if err := os.Setenv("TYPO_ACTIVE_SHELL", oldActiveShell); err != nil {
			t.Fatalf("Restore TYPO_ACTIVE_SHELL failed: %v", err)
		}
	}()
	if err := os.Unsetenv("TYPO_ACTIVE_SHELL"); err != nil {
		t.Fatalf("Unsetenv TYPO_ACTIVE_SHELL failed: %v", err)
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

	if got := currentShellName(); got != "powershell" {
		t.Fatalf("currentShellName() = %q, want %q", got, "powershell")
	}
}

func TestShellHelpersSupportFish(t *testing.T) {
	if got := normalizeShellName(" fish "); got != "fish" {
		t.Fatalf("normalizeShellName(fish) = %q, want fish", got)
	}

	t.Setenv("TYPO_ACTIVE_SHELL", "fish")
	t.Setenv("SHELL", "/bin/zsh")
	t.Setenv("POWERSHELL_DISTRIBUTION_CHANNEL", "")
	t.Setenv("PSModulePath", "")
	t.Setenv("PSExecutionPolicyPreference", "")

	if got := currentShellName(); got != "fish" {
		t.Fatalf("currentShellName() = %q, want fish", got)
	}

	shellName, shellRC := detectShellIntegrationTarget()
	if shellName != "fish" || shellRC != "~/.config/fish/config.fish" {
		t.Fatalf("detectShellIntegrationTarget() = (%q, %q), want fish config", shellName, shellRC)
	}

	if got := shellInitCommand("fish"); got != "typo init fish | source" {
		t.Fatalf("shellInitCommand(fish) = %q", got)
	}
	if got := shellReloadCommand("fish", "~/.config/fish/config.fish"); got != "source ~/.config/fish/config.fish" {
		t.Fatalf("shellReloadCommand(fish) = %q", got)
	}
	if got := shellPathExportCommand("fish", "/tmp/typo-bin"); got != "fish_add_path /tmp/typo-bin" {
		t.Fatalf("shellPathExportCommand(fish) = %q", got)
	}
}

func TestDoctorInstallMethodDetection(t *testing.T) {
	homeDir := t.TempDir()
	customInstallDir := filepath.Join(t.TempDir(), "custom-bin")
	localAppData := filepath.Join(t.TempDir(), "AppData", "Local")

	tests := []struct {
		name  string
		path  string
		setup func(t *testing.T)
		want  string
	}{
		{
			name: "go install",
			path: filepath.Join(homeDir, "go", "bin", "typo"),
			setup: func(t *testing.T) {
				t.Helper()
				t.Setenv("GOPATH", filepath.Join(homeDir, "go"))
			},
			want: "go install",
		},
		{
			name: "homebrew prefix bin",
			path: "/opt/homebrew/bin/typo",
			setup: func(t *testing.T) {
				t.Helper()
				t.Setenv("HOMEBREW_PREFIX", "/opt/homebrew")
			},
			want: "Homebrew",
		},
		{
			name: "homebrew cellar",
			path: "/opt/homebrew/Cellar/typo/1.0.0/bin/typo",
			want: "Homebrew",
		},
		{
			name: "curl local bin",
			path: filepath.Join(homeDir, ".local", "bin", "typo"),
			want: "curl/install.sh",
		},
		{
			name: "curl custom install dir",
			path: filepath.Join(customInstallDir, "typo"),
			setup: func(t *testing.T) {
				t.Helper()
				t.Setenv("TYPO_INSTALL_DIR", customInstallDir)
			},
			want: "curl/install.sh",
		},
		{
			name: "windows quick install",
			path: filepath.Join(localAppData, "Programs", "typo", "bin", "typo.exe"),
			setup: func(t *testing.T) {
				t.Helper()
				t.Setenv("LOCALAPPDATA", localAppData)
			},
			want: "Windows quick install",
		},
		{
			name: "manual release",
			path: filepath.Join(t.TempDir(), "downloads", "typo"),
			want: "manual release",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOME", homeDir)
			t.Setenv("USERPROFILE", homeDir)
			t.Setenv("GOBIN", "")
			t.Setenv("GOPATH", filepath.Join(t.TempDir(), "go"))
			t.Setenv("HOMEBREW_PREFIX", "")
			t.Setenv("TYPO_INSTALL_DIR", "")
			t.Setenv("LOCALAPPDATA", "")
			if tt.setup != nil {
				tt.setup(t)
			}

			if got := detectDoctorInstallMethod(tt.path).name; got != tt.want {
				t.Fatalf("detectDoctorInstallMethod(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestDoctorReportsInstallMethodAndShellSetup(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	oldLookPath := lookPath
	defer func() { lookPath = oldLookPath }()

	homeDir := t.TempDir()
	typoPath := filepath.Join(homeDir, ".local", "bin", "typo")
	lookPath = func(file string) (string, error) {
		if file == "typo" {
			return typoPath, nil
		}
		return "", os.ErrNotExist
	}

	t.Setenv("HOME", homeDir)
	t.Setenv("SHELL", "/bin/bash")
	t.Setenv("TYPO_SHELL_INTEGRATION", "1")
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", filepath.Join(t.TempDir(), "go"))
	t.Setenv("HOMEBREW_PREFIX", "")
	t.Setenv("TYPO_INSTALL_DIR", "")
	t.Setenv("POWERSHELL_DISTRIBUTION_CHANNEL", "")
	t.Setenv("PSModulePath", "")
	t.Setenv("PSExecutionPolicyPreference", "")

	os.Args = []string{"typo", "doctor"}
	code := 0
	output := captureStdout(t, func() {
		code = Run()
	})

	if code != 0 {
		t.Fatalf("Expected exit code 0, got %d, output=%s", code, output)
	}
	if !bytes.Contains([]byte(output), []byte("install method: ✓ curl/install.sh")) {
		t.Fatalf("Expected curl install method in doctor output, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("Shell config: ~/.bashrc")) {
		t.Fatalf("Expected bash shell config in doctor output, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte(`Shell init: eval "$(typo init bash)"`)) {
		t.Fatalf("Expected bash shell init in doctor output, got: %s", output)
	}
}

func TestDoctorReportsShellMisconfiguration(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	oldLookPath := lookPath
	defer func() { lookPath = oldLookPath }()
	lookPath = func(file string) (string, error) {
		return "/usr/local/bin/typo", nil
	}

	homeDir := t.TempDir()
	fishConfig := filepath.Join(homeDir, ".config", "fish", "config.fish")
	if err := os.MkdirAll(filepath.Dir(fishConfig), 0755); err != nil {
		t.Fatalf("Create fish config dir failed: %v", err)
	}
	if err := os.WriteFile(fishConfig, []byte(`eval "$(typo init fish)"`+"\n"), 0600); err != nil {
		t.Fatalf("Write fish config failed: %v", err)
	}

	t.Setenv("HOME", homeDir)
	t.Setenv("SHELL", "/opt/homebrew/bin/fish")
	t.Setenv("TYPO_ACTIVE_SHELL", "")
	t.Setenv("TYPO_SHELL_INTEGRATION", "1")
	t.Setenv("POWERSHELL_DISTRIBUTION_CHANNEL", "")
	t.Setenv("PSModulePath", "")
	t.Setenv("PSExecutionPolicyPreference", "")

	os.Args = []string{"typo", "doctor"}
	code := 0
	output := captureStdout(t, func() {
		code = Run()
	})

	if code != 1 {
		t.Fatalf("Expected exit code 1 for shell misconfiguration, got %d, output=%s", code, output)
	}
	if !bytes.Contains([]byte(output), []byte("Common shell integration misconfiguration detected")) {
		t.Fatalf("Expected shell misconfiguration warning, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("typo init fish | source")) {
		t.Fatalf("Expected fish setup command in warning, got: %s", output)
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
	defer func() {
		if err := os.Setenv("TYPO_SHELL_INTEGRATION", oldEnv); err != nil {
			t.Fatalf("Restore TYPO_SHELL_INTEGRATION failed: %v", err)
		}
	}()
	if err := os.Setenv("TYPO_SHELL_INTEGRATION", "1"); err != nil {
		t.Fatalf("Setenv TYPO_SHELL_INTEGRATION failed: %v", err)
	}
	oldShell := os.Getenv("SHELL")
	defer func() {
		if err := os.Setenv("SHELL", oldShell); err != nil {
			t.Fatalf("Restore SHELL failed: %v", err)
		}
	}()
	if err := os.Setenv("SHELL", "/bin/zsh"); err != nil {
		t.Fatalf("Setenv SHELL failed: %v", err)
	}

	os.Args = []string{"typo", "doctor"}

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
	if !bytes.Contains([]byte(output), []byte("shell integration: ✓ loaded")) {
		t.Errorf("Expected shell integration to be loaded, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("All checks passed")) {
		t.Errorf("Expected 'All checks passed', got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("Go bin PATH")) {
		t.Error("Expected doctor output to contain 'Go bin PATH'")
	}
	if !bytes.Contains([]byte(output), []byte("config file")) {
		t.Error("Expected doctor output to contain 'config file'")
	}
	if !bytes.Contains([]byte(output), []byte("history.enabled=true")) {
		t.Error("Expected doctor output to contain effective config values")
	}
	if !bytes.Contains([]byte(output), []byte("shell: zsh")) {
		t.Error("Expected doctor output to contain current shell")
	}
}

func TestDoctorGoBinNotInPath(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	oldLookPath := lookPath
	defer func() { lookPath = oldLookPath }()

	oldEnv := os.Getenv("TYPO_SHELL_INTEGRATION")
	defer func() {
		if err := os.Setenv("TYPO_SHELL_INTEGRATION", oldEnv); err != nil {
			t.Fatalf("Restore TYPO_SHELL_INTEGRATION failed: %v", err)
		}
	}()
	if err := os.Setenv("TYPO_SHELL_INTEGRATION", "1"); err != nil {
		t.Fatalf("Setenv TYPO_SHELL_INTEGRATION failed: %v", err)
	}

	tmpDir := t.TempDir()
	goBinDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(goBinDir, 0755); err != nil {
		t.Fatalf("Failed to create go bin dir: %v", err)
	}

	oldGoBin := os.Getenv("GOBIN")
	defer func() {
		if err := os.Setenv("GOBIN", oldGoBin); err != nil {
			t.Fatalf("Restore GOBIN failed: %v", err)
		}
	}()
	if err := os.Setenv("GOBIN", goBinDir); err != nil {
		t.Fatalf("Setenv GOBIN failed: %v", err)
	}

	oldPath := os.Getenv("PATH")
	defer func() {
		if err := os.Setenv("PATH", oldPath); err != nil {
			t.Fatalf("Restore PATH failed: %v", err)
		}
	}()
	if err := os.Setenv("PATH", "/usr/bin:/bin"); err != nil {
		t.Fatalf("Setenv PATH failed: %v", err)
	}

	lookPath = func(file string) (string, error) {
		return filepath.Join(goBinDir, "typo"), nil
	}

	os.Args = []string{"typo", "doctor"}

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
	defer func() {
		if err := os.Setenv("TYPO_SHELL_INTEGRATION", oldEnv); err != nil {
			t.Fatalf("Restore TYPO_SHELL_INTEGRATION failed: %v", err)
		}
	}()
	if err := os.Setenv("TYPO_SHELL_INTEGRATION", "1"); err != nil {
		t.Fatalf("Setenv failed: %v", err)
	}

	os.Args = []string{"typo", "doctor"}

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
		t.Errorf("Expected exit code 1, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("not found in PATH")) {
		t.Errorf("Expected missing PATH message, got: %s", output)
	}
}

func TestDoctorPrintsCustomConfig(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	oldLookPath := lookPath
	defer func() { lookPath = oldLookPath }()
	lookPath = func(file string) (string, error) {
		return "/usr/local/bin/typo", nil
	}

	oldEnv := os.Getenv("TYPO_SHELL_INTEGRATION")
	defer func() {
		if err := os.Setenv("TYPO_SHELL_INTEGRATION", oldEnv); err != nil {
			t.Fatalf("Restore TYPO_SHELL_INTEGRATION failed: %v", err)
		}
	}()
	if err := os.Setenv("TYPO_SHELL_INTEGRATION", "1"); err != nil {
		t.Fatalf("Setenv failed: %v", err)
	}

	oldHome := os.Getenv("HOME")
	defer func() {
		if err := os.Setenv("HOME", oldHome); err != nil {
			t.Fatalf("Restore HOME failed: %v", err)
		}
	}()
	tmpHome := t.TempDir()
	if err := os.Setenv("HOME", tmpHome); err != nil {
		t.Fatalf("Setenv HOME failed: %v", err)
	}

	cfg := config.Load()
	cfg.User.Keyboard = "dvorak"
	cfg.User.History.Enabled = false
	cfg.User.Rules["docker"] = itypes.RuleSetConfig{Enabled: false}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	os.Args = []string{"typo", "doctor"}

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
		t.Fatalf("Expected exit code 0, got %d, output=%s", code, output)
	}
	if !bytes.Contains([]byte(output), []byte("config file: ✓")) {
		t.Fatalf("Expected doctor to report config file, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("keyboard=dvorak")) {
		t.Fatalf("Expected doctor to print keyboard=dvorak, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("history.enabled=false")) {
		t.Fatalf("Expected doctor to print history.enabled=false, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("rules.docker.enabled=false")) {
		t.Fatalf("Expected doctor to print rules.docker.enabled=false, got: %s", output)
	}
}

func TestDoctorPrintsExperimentalLongOptionConfig(t *testing.T) {
	oldHome := os.Getenv("HOME")
	defer func() {
		if err := os.Setenv("HOME", oldHome); err != nil {
			t.Fatalf("Restore HOME failed: %v", err)
		}
	}()

	tmpHome := t.TempDir()
	if err := os.Setenv("HOME", tmpHome); err != nil {
		t.Fatalf("Setenv HOME failed: %v", err)
	}

	cfg := config.Load()
	cfg.User.Experimental.LongOptionCorrection.Enabled = true
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	code, stdout, stderr := runCLI(t, []string{"typo", "doctor"})
	if code != 1 && code != 0 {
		t.Fatalf("doctor failed unexpectedly: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "experimental.long-option-correction.enabled=true") {
		t.Fatalf("expected doctor output to show experimental config, got stdout=%q stderr=%q", stdout, stderr)
	}
}
