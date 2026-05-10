package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuluo-yx/typo/internal/config"
	"github.com/yuluo-yx/typo/internal/engine"
	itypes "github.com/yuluo-yx/typo/internal/types"
)

func TestRulesList(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"typo", "rules", "list"}

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

	code := Run()

	if err := w.Close(); err != nil {
		t.Fatalf("Close pipe failed: %v", err)
	}
	os.Stdout = oldStdout

	if code != 0 {
		t.Errorf("Expected exit code 0 for add, got %d", code)
	}

	// Remove rule
	os.Args = []string{"typo", "rules", "remove", "testcmd123"}

	_, w, _ = os.Pipe()
	os.Stdout = w

	code = Run()

	if err := w.Close(); err != nil {
		t.Fatalf("Close pipe failed: %v", err)
	}
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

	code := Run()

	if err := w.Close(); err != nil {
		t.Fatalf("Close pipe failed: %v", err)
	}
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

	code := Run()

	if err := w.Close(); err != nil {
		t.Fatalf("Close pipe failed: %v", err)
	}
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

	code := Run()

	if err := w.Close(); err != nil {
		t.Fatalf("Close pipe failed: %v", err)
	}
	os.Stderr = oldStderr

	if code != 1 {
		t.Errorf("Expected exit code 1, got %d", code)
	}
}

func TestRulesEnableDisableLifecycle(t *testing.T) {
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)

	tmpHome := t.TempDir()
	if err := os.Setenv("HOME", tmpHome); err != nil {
		t.Fatalf("Setenv HOME failed: %v", err)
	}

	steps := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "disable git",
			run: func(t *testing.T) {
				assertCLISucceedsWithOutput(t, []string{"typo", "rules", "disable", "git"}, "Disabled rule scope: git", "rules disable git")
				assertConfigValue(t, "rules.git.enabled", "false", "after disable")
			},
		},
		{
			name: "list disabled rule",
			run: func(t *testing.T) {
				assertCLISucceedsWithOutput(t, []string{"typo", "rules", "list"}, "gut -> git [git] (disabled)", "rules list")
			},
		},
		{
			name: "fix skips disabled rule",
			run: func(t *testing.T) {
				assertCLIDoesNotCorrect(t, []string{"typo", "fix", "--no-history", "gut", "status"}, "git status", "fix should not correct to git when git scope is disabled")
			},
		},
		{
			name: "enable git",
			run: func(t *testing.T) {
				assertCLISucceedsWithOutput(t, []string{"typo", "rules", "enable", "git"}, "Enabled rule scope: git", "rules enable git")
				assertConfigValue(t, "rules.git.enabled", "true", "after enable")
			},
		},
		{
			name: "fix recovers after re-enable",
			run: func(t *testing.T) {
				assertCLISucceedsWithOutput(t, []string{"typo", "fix", "gut", "status"}, "git status", "fix should recover after re-enabling git")
			},
		},
	}

	for _, step := range steps {
		t.Run(step.name, step.run)
	}
}

func TestRulesEnableDisableErrors(t *testing.T) {
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)

	tmpHome := t.TempDir()
	if err := os.Setenv("HOME", tmpHome); err != nil {
		t.Fatalf("Setenv HOME failed: %v", err)
	}

	tests := []struct {
		name   string
		args   []string
		wantIn string
	}{
		{
			name:   "disable missing scope",
			args:   []string{"typo", "rules", "disable"},
			wantIn: "requires exactly one <scope>",
		},
		{
			name:   "enable extra args",
			args:   []string{"typo", "rules", "enable", "git", "extra"},
			wantIn: "requires exactly one <scope>",
		},
		{
			name:   "unknown scope",
			args:   []string{"typo", "rules", "disable", "rust"},
			wantIn: "valid options:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runCLI(t, tt.args)
			if code == 0 {
				t.Fatalf("expected failure, got code=0 stdout=%q stderr=%q", stdout, stderr)
			}
			if !strings.Contains(stdout+stderr, tt.wantIn) {
				t.Fatalf("expected output to contain %q, stdout=%q stderr=%q", tt.wantIn, stdout, stderr)
			}
		})
	}

	code, stdout, stderr := runCLI(t, []string{"typo", "rules", "disable", "rust"})
	if code == 0 {
		t.Fatalf("expected unknown scope failure, got stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stderr, "unknown rule scope: rust") {
		t.Fatalf("expected unknown scope error, got stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stderr, "docker") || !strings.Contains(stderr, "git") || !strings.Contains(stderr, "system") {
		t.Fatalf("expected valid scope list in error, got stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestRulesEnableDisableSupportsUnknownPresentScope(t *testing.T) {
	oldHome := os.Getenv("HOME")
	defer os.Setenv("HOME", oldHome)

	tmpHome := t.TempDir()
	if err := os.Setenv("HOME", tmpHome); err != nil {
		t.Fatalf("Setenv HOME failed: %v", err)
	}

	cfg := config.Load()
	cfg.User.Rules["rust"] = itypes.RuleSetConfig{Enabled: false}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	code, stdout, stderr := runCLI(t, []string{"typo", "rules", "enable", "rust"})
	if code != 0 {
		t.Fatalf("rules enable rust failed: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "Enabled rule scope: rust") {
		t.Fatalf("expected enable rust output, got stdout=%q stderr=%q", stdout, stderr)
	}

	code, stdout, stderr = runCLI(t, []string{"typo", "config", "get", "rules.rust.enabled"})
	if code != 0 || strings.TrimSpace(stdout) != "true" {
		t.Fatalf("config get rules.rust.enabled failed after enable: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	code, stdout, stderr = runCLI(t, []string{"typo", "rules", "disable", "rust"})
	if code != 0 {
		t.Fatalf("rules disable rust failed: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "Disabled rule scope: rust") {
		t.Fatalf("expected disable rust output, got stdout=%q stderr=%q", stdout, stderr)
	}

	code, stdout, stderr = runCLI(t, []string{"typo", "config", "get", "rules.rust.enabled"})
	if code != 0 || strings.TrimSpace(stdout) != "false" {
		t.Fatalf("config get rules.rust.enabled failed after disable: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestHistoryList(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"typo", "history", "list"}

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

	code := Run()

	if err := w.Close(); err != nil {
		t.Fatalf("Close pipe failed: %v", err)
	}
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

	code := Run()

	if err := w.Close(); err != nil {
		t.Fatalf("Close pipe failed: %v", err)
	}
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

	code := Run()

	if err := w.Close(); err != nil {
		t.Fatalf("Close pipe failed: %v", err)
	}
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
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Fatalf("Remove temp file failed: %v", err)
		}
	}()
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Close temp file failed: %v", err)
	}

	// Set config dir to the file path
	oldHome := os.Getenv("HOME")
	defer func() {
		if err := os.Setenv("HOME", oldHome); err != nil {
			t.Fatalf("Restore HOME failed: %v", err)
		}
	}()
	if err := os.Setenv("HOME", tmpFile.Name()); err != nil {
		t.Fatalf("Setenv HOME failed: %v", err)
	}

	os.Args = []string{"typo", "learn", "wrongcmd", "rightcmd"}

	oldStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	code := Run()

	if err := w.Close(); err != nil {
		t.Fatalf("Close pipe failed: %v", err)
	}
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
		t.Fatalf("Setenv HOME failed: %v", err)
	}

	os.Args = []string{"typo", "history", "clear"}

	oldStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	code := Run()

	if err := w.Close(); err != nil {
		t.Fatalf("Close pipe failed: %v", err)
	}
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
		t.Fatalf("Setenv HOME failed: %v", err)
	}

	os.Args = []string{"typo", "rules", "add", "fromcmd", "tocmd"}

	oldStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	code := Run()

	if err := w.Close(); err != nil {
		t.Fatalf("Close pipe failed: %v", err)
	}
	os.Stderr = oldStderr

	if code != 1 {
		t.Errorf("Expected exit code 1 for rules add error, got %d", code)
	}
}

func TestLearnSurvivesHistoryClear(t *testing.T) {
	useTempHome(t)

	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"typo", "learn", "gut", "mygit"}
	if code := Run(); code != 0 {
		t.Fatalf("Expected learn to succeed, got %d", code)
	}

	os.Args = []string{"typo", "history", "clear"}
	if code := Run(); code != 0 {
		t.Fatalf("Expected history clear to succeed, got %d", code)
	}

	os.Args = []string{"typo", "fix", "gut", "status"}
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
	defer func() {
		if err := os.Setenv("HOME", oldHome); err != nil {
			t.Fatalf("Restore HOME failed: %v", err)
		}
	}()
	defer func() {
		if err := os.Setenv("PATH", oldPath); err != nil {
			t.Fatalf("Restore PATH failed: %v", err)
		}
	}()

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
	if code := Run(); code != 0 {
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
		t.Fatalf("Expected fix to succeed, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("docker ps")) {
		t.Fatalf("Expected learned rule to override old history, got %q", output)
	}
}

func TestCreateEngineAppliesDisabledRuleScopes(t *testing.T) {
	cfg := &config.Config{
		ConfigDir: t.TempDir(),
		User:      config.DefaultUserConfig(),
	}
	cfg.User.Rules["git"] = itypes.RuleSetConfig{Enabled: false}

	eng := createEngine(cfg)
	if got := eng.Fix("grt status", ""); got.Command == "git status" {
		t.Fatalf("expected disabled git scope to prevent fixing to git, got %+v", got)
	}
}

func TestDisabledCommandsFromConfig(t *testing.T) {
	if got := disabledCommandsFromConfig(nil); got != nil {
		t.Fatalf("disabledCommandsFromConfig(nil) = %v, want nil", got)
	}

	cfg := &config.Config{User: config.DefaultUserConfig()}
	cfg.User.Rules["python"] = itypes.RuleSetConfig{Enabled: false}
	cfg.User.Rules["git"] = itypes.RuleSetConfig{Enabled: false}
	cfg.User.Rules["system"] = itypes.RuleSetConfig{Enabled: false}

	got := disabledCommandsFromConfig(cfg)
	wantSet := map[string]bool{
		"python":  true,
		"python3": true,
		"git":     true,
	}
	for _, name := range got {
		delete(wantSet, name)
	}
	if len(wantSet) != 0 {
		t.Fatalf("disabledCommandsFromConfig() missing commands: %v, got %v", wantSet, got)
	}
	for _, name := range got {
		if name == "system" {
			t.Fatalf("disabledCommandsFromConfig() should not disable system, got %v", got)
		}
	}
}

func TestDisabledCommandsFromConfigIgnoresUnknownScopesWithWarning(t *testing.T) {
	cfg := &config.Config{User: config.DefaultUserConfig()}
	cfg.User.Rules["git"] = itypes.RuleSetConfig{Enabled: false}
	cfg.User.Rules["rust"] = itypes.RuleSetConfig{Enabled: false}

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	got := disabledCommandsFromConfig(cfg)

	if err := w.Close(); err != nil {
		t.Fatalf("Close pipe failed: %v", err)
	}
	os.Stderr = oldStderr

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("Read pipe failed: %v", err)
	}
	output := buf.String()

	if !strings.Contains(output, "unknown disabled rule scopes") || !strings.Contains(output, "rust") {
		t.Fatalf("expected unknown disabled scope warning, got %q", output)
	}
	for _, name := range got {
		if name == "rust" {
			t.Fatalf("disabledCommandsFromConfig() should ignore unknown scope names, got %v", got)
		}
	}
	foundGit := false
	for _, name := range got {
		if name == "git" {
			foundGit = true
			break
		}
	}
	if !foundGit {
		t.Fatalf("disabledCommandsFromConfig() should keep known disabled commands, got %v", got)
	}
}

func TestRuleScopeDisabledCommandsCoversDefaultScopes(t *testing.T) {
	for scope := range config.DefaultUserConfig().Rules {
		if _, ok := ruleScopeDisabledCommands[scope]; !ok {
			t.Fatalf("ruleScopeDisabledCommands missing scope %q", scope)
		}
	}
}

func TestCreateEngineFallsBackToDefaultKeyboard(t *testing.T) {
	cfg := &config.Config{
		ConfigDir: t.TempDir(),
		User:      config.DefaultUserConfig(),
	}
	cfg.User.Keyboard = "invalid"

	eng := createEngine(cfg)
	if got := eng.Fix("gut status", ""); !got.Fixed || got.Command != "git status" {
		t.Fatalf("expected engine to fall back to default keyboard and still fix command, got %+v", got)
	}
}
