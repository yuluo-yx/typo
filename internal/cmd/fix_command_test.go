package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yuluo-yx/typo/internal/config"
	"github.com/yuluo-yx/typo/internal/engine"
	itypes "github.com/yuluo-yx/typo/internal/types"
)

func TestShouldRecordHistory(t *testing.T) {
	tests := []struct {
		name     string
		original string
		result   itypes.FixResult
		want     bool
	}{
		{name: "not fixed", original: "git status", result: itypes.FixResult{Fixed: false}, want: false},
		{name: "unchanged command", original: "git status", result: itypes.FixResult{Fixed: true, Command: "git status"}, want: false},
		{name: "permission sudo parser", original: "mkdir 1", result: itypes.FixResult{Fixed: true, Command: "sudo mkdir 1", Kind: "permission_sudo"}, want: false},
		{name: "parser assisted fix", original: "git remove -v", result: itypes.FixResult{Fixed: true, Command: "git remote -v", UsedParser: true}, want: false},
		{name: "normal accepted fix", original: "gut status", result: itypes.FixResult{Fixed: true, Command: "git status"}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRecordHistory(tt.original, tt.result); got != tt.want {
				t.Fatalf("shouldRecordHistory() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunAutoLearnWithinTimeout(t *testing.T) {
	oldTimeout := autoLearnFixTimeout
	oldRunner := autoLearnFromHistory
	defer func() {
		autoLearnFixTimeout = oldTimeout
		autoLearnFromHistory = oldRunner
	}()

	autoLearnFixTimeout = 20 * time.Millisecond
	started := make(chan struct{}, 1)
	autoLearnFromHistory = func(ctx context.Context, eng *engine.Engine, from, to string) itypes.AutoLearnDebugInfo {
		started <- struct{}{}
		<-ctx.Done()
		return itypes.AutoLearnDebugInfo{Attempted: true, TimedOut: true, Reason: ctx.Err().Error()}
	}

	start := time.Now()
	info := runAutoLearnWithinTimeout(engine.NewEngine(), "gut status", "git status")
	elapsed := time.Since(start)

	select {
	case <-started:
	default:
		t.Fatal("Expected auto learn worker to start")
	}
	if !info.Attempted || !info.TimedOut {
		t.Fatalf("Expected timeout debug info, got %+v", info)
	}
	if elapsed >= 250*time.Millisecond {
		t.Fatalf("Expected auto learn wait to stop at timeout, got %v", elapsed)
	}
}

func TestFixCommandDebugOutput(t *testing.T) {
	tmpHome := useTempHome(t)

	code, stdout, stderr := runCLI(t, []string{"typo", "fix", "--debug", "gut", "status"})
	if code != 0 {
		t.Fatalf("debug fix failed: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if strings.TrimSpace(stdout) != "git status" {
		t.Fatalf("expected fixed command in stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "typo: debug") {
		t.Fatalf("expected debug header, got %q", stderr)
	}
	if !strings.Contains(stderr, "stage pass=1 stage=rule") {
		t.Fatalf("expected rule stage in debug output, got %q", stderr)
	}
	if !strings.Contains(stderr, "timing total=") {
		t.Fatalf("expected timing line in debug output, got %q", stderr)
	}
	if !strings.Contains(stderr, "auto-learn attempted=yes") {
		t.Fatalf("expected auto-learn attempt in debug output, got %q", stderr)
	}
	historyFile := filepath.Join(tmpHome, ".typo", "usage_history.json")
	assertFileExists(t, historyFile, "debug fix should still record history")
}

func TestFixCommandDebugOutputNoMatch(t *testing.T) {
	useTempHome(t)

	code, stdout, stderr := runCLI(t, []string{"typo", "fix", "--debug", "xyzabc"})
	if code != 1 {
		t.Fatalf("expected no-match exit code, got code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout for no-match debug run, got %q", stdout)
	}
	if !strings.Contains(stderr, "typo: no correction found") {
		t.Fatalf("expected no correction message, got %q", stderr)
	}
	if !strings.Contains(stderr, "matched-stages=none") {
		t.Fatalf("expected empty stage summary, got %q", stderr)
	}
	if !strings.Contains(stderr, "timing total=") {
		t.Fatalf("expected timing line in no-match debug output, got %q", stderr)
	}
	if !strings.Contains(stderr, "auto-learn attempted=no") {
		t.Fatalf("expected auto-learn skipped summary, got %q", stderr)
	}
}

func TestFixCommandDebugJSONOutput(t *testing.T) {
	useTempHome(t)

	code, stdout, stderr := runCLI(t, []string{"typo", "fix", "--debug=json", "gut", "status"})
	if code != 0 {
		t.Fatalf("debug json fix failed: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if strings.TrimSpace(stdout) != "git status" {
		t.Fatalf("expected fixed command in stdout, got %q", stdout)
	}

	var payload struct {
		SchemaVersion string `json:"schema_version"`
		Fixed         bool   `json:"fixed"`
		Command       string `json:"command"`
		Source        string `json:"source"`
		Events        []struct {
			Stage string `json:"stage"`
		} `json:"events"`
		AutoLearn struct {
			Attempted bool `json:"attempted"`
		} `json:"auto_learn"`
	}
	if err := json.Unmarshal([]byte(stderr), &payload); err != nil {
		t.Fatalf("expected valid debug json, got err=%v stderr=%q", err, stderr)
	}
	if payload.SchemaVersion != "1" || !payload.Fixed || payload.Command != "git status" || payload.Source != "rule" {
		t.Fatalf("unexpected debug json payload: %+v", payload)
	}
	if len(payload.Events) == 0 || payload.Events[0].Stage != "rule" {
		t.Fatalf("expected rule event in debug json, got %+v", payload.Events)
	}
	if !payload.AutoLearn.Attempted {
		t.Fatalf("expected auto-learn attempt in debug json, got %+v", payload.AutoLearn)
	}
}

func TestFixCommandTraceFileWritesDebugJSON(t *testing.T) {
	useTempHome(t)
	traceFile := filepath.Join(t.TempDir(), "trace.json")

	code, stdout, stderr := runCLI(t, []string{"typo", "fix", "--trace-file", traceFile, "gut", "status"})
	if code != 0 {
		t.Fatalf("trace file fix failed: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if strings.TrimSpace(stdout) != "git status" {
		t.Fatalf("expected fixed command in stdout, got %q", stdout)
	}
	if strings.Contains(stderr, "typo: debug") || strings.Contains(stderr, "schema_version") {
		t.Fatalf("trace-file should not print debug output by itself, got stderr=%q", stderr)
	}

	data, err := os.ReadFile(traceFile)
	if err != nil {
		t.Fatalf("expected trace file to be written: %v", err)
	}
	var payload struct {
		SchemaVersion string `json:"schema_version"`
		Input         string `json:"input"`
		Fixed         bool   `json:"fixed"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("expected valid trace json, got err=%v data=%q", err, string(data))
	}
	if payload.SchemaVersion != "1" || payload.Input != "gut status" || !payload.Fixed {
		t.Fatalf("unexpected trace payload: %+v", payload)
	}
}

func TestExplainCommandOutput(t *testing.T) {
	tmpHome := useTempHome(t)

	code, stdout, stderr := runCLI(t, []string{"typo", "explain", "gut", "status"})
	if code != 0 {
		t.Fatalf("explain failed: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	for _, want := range []string{"Input: gut status", "rules: \"gut status\" -> \"git status\"", "Result:\n  git status"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected explain output to contain %q, got %q", want, stdout)
		}
	}
	if _, err := os.Stat(filepath.Join(tmpHome, ".typo", "usage_history.json")); !os.IsNotExist(err) {
		t.Fatalf("explain should not record history, stat err=%v", err)
	}
}

func TestFixCommand(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

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

	code := Run()

	if err := w.Close(); err != nil {
		t.Fatalf("Close pipe failed: %v", err)
	}
	os.Stderr = oldStderr

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("Read pipe failed: %v", err)
	}
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

	os.Args = []string{"typo", "fix", "gut", "status"}

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	code := Run()

	if err := w.Close(); err != nil {
		t.Fatalf("Close pipe failed: %v", err)
	}
	os.Stderr = oldStderr

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("Read pipe failed: %v", err)
	}
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

	code := Run()

	if err := wOut.Close(); err != nil {
		t.Fatalf("Close stdout pipe failed: %v", err)
	}
	if err := wErr.Close(); err != nil {
		t.Fatalf("Close stderr pipe failed: %v", err)
	}
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var bufOut, bufErr bytes.Buffer
	if _, err := bufOut.ReadFrom(rOut); err != nil {
		t.Fatalf("Read stdout pipe failed: %v", err)
	}
	if _, err := bufErr.ReadFrom(rErr); err != nil {
		t.Fatalf("Read stderr pipe failed: %v", err)
	}

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
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Fatalf("Remove temp file failed: %v", err)
		}
	}()

	stderrContent := "git: 'remove' is not a git command.\n\nThe most similar command is\n\tremote\n"
	if _, err := tmpFile.WriteString(stderrContent); err != nil {
		t.Fatalf("WriteString failed: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Close temp file failed: %v", err)
	}

	os.Args = []string{"typo", "fix", "-s", tmpFile.Name(), "git", "remove", "-v"}

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
	if !bytes.Contains([]byte(output), []byte("remote")) {
		t.Errorf("Expected fix output to contain 'remote', got %q", output)
	}
}

func TestLoadAliasContext(t *testing.T) {
	tmpDir := t.TempDir()
	contextFile := filepath.Join(tmpDir, "aliases.tsv")
	content := strings.Join([]string{
		"not-a-valid-line",
		"zsh\talias\tbad\tkubectl | rm",
		"zsh\talias\tk\tkubectl",
		"bash\tenv\tk\tk",
		"bash\tenv\t1BAD\t1BAD",
		"zsh\talias\tk\tkubctl",
		"fish\tabbr\tg\tgit",
		"bash\tfunction\twrap\tkubectl",
	}, "\n")
	if err := os.WriteFile(contextFile, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write alias context: %v", err)
	}

	entries := loadAliasContext(contextFile)
	if len(entries) != 4 {
		t.Fatalf("loadAliasContext() len = %d, want 4: %+v", len(entries), entries)
	}
	if entries[0].Name != "k" || entries[0].Expansion != "kubectl" {
		t.Fatalf("Expected duplicate alias to keep first kubectl entry, got %+v", entries[0])
	}
	if entries[1].Kind != "env" || entries[1].Name != "k" || entries[1].Expansion != "k" {
		t.Fatalf("Expected env entry with the same name as alias to be preserved, got %+v", entries[1])
	}
	if entries[2].Kind != "abbr" || entries[2].Name != "g" || entries[2].Expansion != "git" {
		t.Fatalf("Expected fish abbreviation entry, got %+v", entries[2])
	}
	if entries[3].Kind != "function" || entries[3].Name != "wrap" || entries[3].Expansion != "kubectl" {
		t.Fatalf("Expected simple function entry, got %+v", entries[3])
	}
}

func TestLoadAliasContextRejectsLargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	contextFile := filepath.Join(tmpDir, "aliases.tsv")
	if err := os.WriteFile(contextFile, []byte(strings.Repeat("x", aliasContextMaxFileSize+1)), 0600); err != nil {
		t.Fatalf("failed to write large alias context: %v", err)
	}

	if entries := loadAliasContext(contextFile); len(entries) != 0 {
		t.Fatalf("Expected large alias context to be ignored, got %+v", entries)
	}
}

func TestFixWithAliasContextFile(t *testing.T) {
	tmpDir := t.TempDir()
	contextFile := filepath.Join(tmpDir, "aliases.tsv")
	if err := os.WriteFile(contextFile, []byte("zsh\talias\tk\tkubectl\n"), 0600); err != nil {
		t.Fatalf("failed to write alias context: %v", err)
	}

	code, stdout, stderr := runCLI(t, []string{"typo", "fix", "--alias-context", contextFile, "k", "lgo"})
	if code != 0 || stdout != "k logs\n" {
		t.Fatalf("alias context fix failed: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestFixWithEnvContextFile(t *testing.T) {
	tmpDir := t.TempDir()
	contextFile := filepath.Join(tmpDir, "aliases.tsv")
	if err := os.WriteFile(contextFile, []byte("zsh\tenv\tHOME\tHOME\n"), 0600); err != nil {
		t.Fatalf("failed to write env context: %v", err)
	}

	code, stdout, stderr := runCLI(t, []string{"typo", "fix", "--alias-context", contextFile, "cd", "$HOEM/project"})
	if code != 0 || stdout != "cd $HOME/project\n" {
		t.Fatalf("env context fix failed: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestFixWithExitCodeAndPermissionDenied(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	tmpFile, err := os.CreateTemp("", "typo-permission-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Fatalf("Remove temp file failed: %v", err)
		}
	}()

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

	// Should still fix the command even without stderr file
	if code != 0 {
		t.Errorf("Expected exit code 0, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("git")) {
		t.Errorf("Expected fix output to contain 'git', got %q", output)
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

	code := Run()

	if err := wOut.Close(); err != nil {
		t.Fatalf("Close stdout pipe failed: %v", err)
	}
	if err := wErr.Close(); err != nil {
		t.Fatalf("Close stderr pipe failed: %v", err)
	}
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	var bufOut, bufErr bytes.Buffer
	if _, err := bufOut.ReadFrom(rOut); err != nil {
		t.Fatalf("Read stdout pipe failed: %v", err)
	}
	if _, err := bufErr.ReadFrom(rErr); err != nil {
		t.Fatalf("Read stderr pipe failed: %v", err)
	}
	output := bufOut.String() + bufErr.String()

	if code != 0 {
		t.Errorf("Expected exit code 0, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("git")) {
		t.Errorf("Expected output to contain 'git', got %q", output)
	}
}

func TestFixWritesUsageHistory(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

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

	os.Args = []string{"typo", "fix", "gut", "status"}

	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	code := Run()

	if err := w.Close(); err != nil {
		t.Fatalf("Close pipe failed: %v", err)
	}
	os.Stdout = oldStdout

	if code != 0 {
		t.Fatalf("Expected exit code 0, got %d", code)
	}

	cfg := config.Load()
	if _, err := os.Stat(filepath.Join(cfg.ConfigDir, "usage_history.json")); err != nil {
		t.Fatalf("Expected usage_history.json to exist, got %v", err)
	}
}

func TestFixAutoLearnsRepeatedHistoryWithoutPrompt(t *testing.T) {
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

	for i := 0; i < 3; i++ {
		code, stdout, stderr := runCLI(t, []string{"typo", "fix", "gut", "status"})
		if code != 0 {
			t.Fatalf("fix #%d failed: code=%d stdout=%q stderr=%q", i+1, code, stdout, stderr)
		}
		if !strings.Contains(stdout, "git status") {
			t.Fatalf("fix #%d should return git status, got %q", i+1, stdout)
		}
		if strings.Contains(stderr, "auto-learn") {
			t.Fatalf("fix #%d should stay silent about auto learn, got %q", i+1, stderr)
		}
	}

	cfg := config.Load()
	data, err := os.ReadFile(filepath.Join(cfg.ConfigDir, "rules.json"))
	if err != nil {
		t.Fatalf("Read rules.json failed: %v", err)
	}

	var rules []itypes.Rule
	if err := json.Unmarshal(data, &rules); err != nil {
		t.Fatalf("Unmarshal rules.json failed: %v", err)
	}

	found := false
	for _, rule := range rules {
		if rule.From == "gut status" && rule.To == "git status" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Expected auto-learned full command rule, got %+v", rules)
	}

	history := engine.NewHistory(cfg.ConfigDir)
	entry, ok := history.Lookup("gut status")
	if !ok {
		t.Fatal("Expected history entry to remain after auto-learn")
	}
	if !entry.RuleApplied {
		t.Fatalf("Expected history entry to be frozen after auto-learn, got %+v", entry)
	}
}

func TestFixAutoLearnDisabledByConfig(t *testing.T) {
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
	cfg.User.AutoLearnThreshold = 0
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	for i := 0; i < 3; i++ {
		code, stdout, stderr := runCLI(t, []string{"typo", "fix", "gut", "status"})
		if code != 0 {
			t.Fatalf("fix #%d failed: code=%d stdout=%q stderr=%q", i+1, code, stdout, stderr)
		}
		if !strings.Contains(stdout, "git status") {
			t.Fatalf("fix #%d should return git status, got %q", i+1, stdout)
		}
	}

	if _, err := os.Stat(filepath.Join(cfg.ConfigDir, "rules.json")); !os.IsNotExist(err) {
		t.Fatalf("Expected auto learn disabled to keep rules.json absent, got %v", err)
	}
}

func TestFixWithPermissionParser_DoesNotWriteUsageHistory(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

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

	tmpFile, err := os.CreateTemp("", "typo-permission-history-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Fatalf("Remove temp file failed: %v", err)
		}
	}()

	if _, err := tmpFile.WriteString("mkdir: /root/test: Permission denied\n"); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	os.Args = []string{"typo", "fix", "--exit-code", "1", "-s", tmpFile.Name(), "mkdir", "/root/test"}

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
		t.Fatalf("Expected exit code 0, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("sudo mkdir /root/test")) {
		t.Fatalf("Expected permission fix output, got %q", output)
	}

	cfg := config.Load()
	if _, err := os.Stat(filepath.Join(cfg.ConfigDir, "usage_history.json")); !os.IsNotExist(err) {
		t.Fatalf("Expected permission fix to skip history persistence, got %v", err)
	}
}

func TestFixWithParser_DoesNotWriteUsageHistory(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

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

	tmpFile, err := os.CreateTemp("", "typo-parser-history-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Fatalf("Remove temp file failed: %v", err)
		}
	}()

	if _, err := tmpFile.WriteString("git: 'remove' is not a git command.\n\nThe most similar command is\n\tremote\n"); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	os.Args = []string{"typo", "fix", "-s", tmpFile.Name(), "git", "remove", "-v"}

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
		t.Fatalf("Expected exit code 0, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("git remote -v")) {
		t.Fatalf("Expected parser fix output, got %q", output)
	}

	cfg := config.Load()
	if _, err := os.Stat(filepath.Join(cfg.ConfigDir, "usage_history.json")); !os.IsNotExist(err) {
		t.Fatalf("Expected parser fix to skip history persistence, got %v", err)
	}
}

func TestFixNoHistoryFlag_DoesNotWriteUsageHistory(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

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

	os.Args = []string{"typo", "fix", "--no-history", "gut", "status"}

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
		t.Fatalf("Expected exit code 0, got %d", code)
	}
	if !bytes.Contains([]byte(output), []byte("git status")) {
		t.Fatalf("Expected fix output, got %q", output)
	}

	cfg := config.Load()
	if _, err := os.Stat(filepath.Join(cfg.ConfigDir, "usage_history.json")); !os.IsNotExist(err) {
		t.Fatalf("Expected --no-history to skip history persistence, got %v", err)
	}
}

func TestFixUsesGlobalHistoryDisabledConfig(t *testing.T) {
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
	cfg.User.History.Enabled = false
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	code, stdout, stderr := runCLI(t, []string{"typo", "fix", "gut", "status"})
	if code != 0 {
		t.Fatalf("fix failed: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "git status") {
		t.Fatalf("expected fixed command in stdout, got %q", stdout)
	}

	historyFile := filepath.Join(cfg.ConfigDir, "usage_history.json")
	if _, err := os.Stat(historyFile); !os.IsNotExist(err) {
		t.Fatalf("expected history file to stay absent, got %v", err)
	}
}
