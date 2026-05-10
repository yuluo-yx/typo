package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yuluo-yx/typo/internal/config"
	itypes "github.com/yuluo-yx/typo/internal/types"
)

func runCLI(t *testing.T, args []string) (int, string, string) {
	t.Helper()

	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = args

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

	var outBuf, errBuf bytes.Buffer
	_, _ = outBuf.ReadFrom(rOut)
	_, _ = errBuf.ReadFrom(rErr)

	return code, outBuf.String(), errBuf.String()
}

func assertCLISucceedsWithOutput(t *testing.T, args []string, wantIn string, action string) {
	t.Helper()

	code, stdout, stderr := runCLI(t, args)
	if code != 0 {
		t.Fatalf("%s failed: code=%d stdout=%q stderr=%q", action, code, stdout, stderr)
	}
	if !strings.Contains(stdout, wantIn) {
		t.Fatalf("%s missing output %q: stdout=%q stderr=%q", action, wantIn, stdout, stderr)
	}
}

func assertCLISucceeds(t *testing.T, args []string, action string) {
	t.Helper()

	code, stdout, stderr := runCLI(t, args)
	if code != 0 {
		t.Fatalf("%s failed: code=%d stdout=%q stderr=%q", action, code, stdout, stderr)
	}
}

func assertConfigValue(t *testing.T, key string, want string, context string) {
	t.Helper()

	code, stdout, stderr := runCLI(t, []string{"typo", "config", "get", key})
	if code != 0 || strings.TrimSpace(stdout) != want {
		t.Fatalf("config get %s failed %s: code=%d stdout=%q stderr=%q", key, context, code, stdout, stderr)
	}
}

func assertCLIDoesNotCorrect(t *testing.T, args []string, unexpected string, context string) {
	t.Helper()

	code, stdout, stderr := runCLI(t, args)
	if code == 0 && strings.Contains(stdout, unexpected) {
		t.Fatalf("%s: stdout=%q stderr=%q", context, stdout, stderr)
	}
}

func useTempHome(t *testing.T) string {
	t.Helper()

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	return tmpHome
}

func assertFileExists(t *testing.T, path string, context string) {
	t.Helper()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("%s: expected file %s to exist, got %v", context, path, err)
	}
}

func writeHistoryEntries(t *testing.T, configDir string, entries []itypes.HistoryEntry) {
	t.Helper()

	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Create config dir failed: %v", err)
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		t.Fatalf("Marshal history failed: %v", err)
	}

	historyFile := filepath.Join(configDir, "usage_history.json")
	if err := os.WriteFile(historyFile, data, 0600); err != nil {
		t.Fatalf("Write history failed: %v", err)
	}
}

func seedStatsHistoryFixture(t *testing.T, reference time.Time) {
	t.Helper()

	cfg := config.Load()
	writeHistoryEntries(t, cfg.ConfigDir, []itypes.HistoryEntry{
		{From: "gti status", To: "git status", Timestamp: reference.AddDate(0, 0, -3).Unix(), Count: 7},
		{From: "dcoker ps", To: "docker ps", Timestamp: reference.AddDate(0, 0, -10).Unix(), Count: 4},
		{From: "gut commit", To: "git commit", Timestamp: reference.AddDate(0, 0, -1).Unix(), Count: 5},
		{From: "stauts", To: "status", Timestamp: reference.AddDate(0, 0, -45).Unix(), Count: 9},
	})
}

func TestConfigCommandLifecycle(t *testing.T) {
	tmpHome := useTempHome(t)
	assertCLISucceeds(t, []string{"typo", "config", "gen"}, "config gen")
	configPath := filepath.Join(tmpHome, ".typo", "config.json")
	assertFileExists(t, configPath, "config gen")
	assertConfigValue(t, "keyboard", "qwerty", "after config gen")
	assertCLISucceeds(t, []string{"typo", "config", "set", "keyboard", "dvorak"}, "config set keyboard")
	assertCLISucceeds(t, []string{"typo", "config", "set", "auto-learn-threshold", "5"}, "config set auto-learn-threshold")
	assertCLISucceeds(t, []string{"typo", "config", "set", "candidates.enabled", "true"}, "config set candidates enabled")
	assertCLISucceeds(t, []string{"typo", "config", "set", "candidates.limit", "5"}, "config set candidates limit")
	assertCLISucceeds(t, []string{"typo", "config", "set", "experimental.long-option-correction.enabled", "true"}, "config set experimental long option")
	assertConfigValue(t, "auto-learn-threshold", "5", "after config set")
	assertConfigValue(t, "candidates.enabled", "true", "after config set")
	assertConfigValue(t, "candidates.limit", "5", "after config set")
	assertConfigValue(t, "experimental.long-option-correction.enabled", "true", "after config set")
	assertCLISucceedsWithOutput(t, []string{"typo", "config", "list"}, "keyboard=dvorak", "config list")
	assertCLISucceedsWithOutput(t, []string{"typo", "config", "list"}, "auto-learn-threshold=5", "config list")
	assertCLISucceedsWithOutput(t, []string{"typo", "config", "list"}, "candidates.enabled=true", "config list")
	assertCLISucceedsWithOutput(t, []string{"typo", "config", "list"}, "candidates.limit=5", "config list")
	assertCLISucceedsWithOutput(t, []string{"typo", "config", "list"}, "experimental.long-option-correction.enabled=true", "config list")
	assertCLISucceeds(t, []string{"typo", "config", "reset"}, "config reset")
	assertConfigValue(t, "keyboard", "qwerty", "after config reset")
	assertConfigValue(t, "auto-learn-threshold", "3", "after config reset")
	assertConfigValue(t, "candidates.enabled", "false", "after config reset")
	assertConfigValue(t, "candidates.limit", "3", "after config reset")
	assertConfigValue(t, "experimental.long-option-correction.enabled", "false", "after config reset")
}

func TestStatsCommandSummary(t *testing.T) {
	useTempHome(t)

	reference := time.Date(2026, time.April, 21, 12, 0, 0, 0, time.UTC)
	oldStatsNow := statsNow
	statsNow = func() time.Time { return reference }
	defer func() { statsNow = oldStatsNow }()

	seedStatsHistoryFixture(t, reference)

	code, stdout, stderr := runCLI(t, []string{"typo", "stats"})
	if code != 0 {
		t.Fatalf("stats failed: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "Top typos (last 30 days):") {
		t.Fatalf("expected stats header, got stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stdout, "gti status -> git status") {
		t.Fatalf("expected git status pair, got stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stdout, "dcoker ps -> docker ps") {
		t.Fatalf("expected docker pair, got stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stdout, "gut commit -> git commit") {
		t.Fatalf("expected git commit pair, got stdout=%q stderr=%q", stdout, stderr)
	}
	if strings.Contains(stdout, "stauts -> status") {
		t.Fatalf("expected old entry to stay filtered out, got stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stdout, "Total accepted corrections: 16") {
		t.Fatalf("expected total corrections summary, got stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stdout, "Most typoed tool: git (12)") {
		t.Fatalf("expected most typoed tool summary, got stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestStatsCommandSummaryWithFlags(t *testing.T) {
	useTempHome(t)

	reference := time.Date(2026, time.April, 21, 12, 0, 0, 0, time.UTC)
	oldStatsNow := statsNow
	statsNow = func() time.Time { return reference }
	defer func() { statsNow = oldStatsNow }()

	seedStatsHistoryFixture(t, reference)

	code, stdout, stderr := runCLI(t, []string{"typo", "stats", "--since", "7", "--top", "1"})
	if code != 0 {
		t.Fatalf("stats with flags failed: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "Top typos (last 7 days):") {
		t.Fatalf("expected custom stats header, got stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stdout, "gti status -> git status") {
		t.Fatalf("expected top pair in filtered stats, got stdout=%q stderr=%q", stdout, stderr)
	}
	if strings.Contains(stdout, "gut commit -> git commit") {
		t.Fatalf("expected --top 1 to truncate output, got stdout=%q stderr=%q", stdout, stderr)
	}
	if strings.Contains(stdout, "dcoker ps -> docker ps") {
		t.Fatalf("expected --since 7 to filter old docker entry, got stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stdout, "Total accepted corrections: 12") {
		t.Fatalf("expected filtered total corrections summary, got stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stdout, "Most typoed tool: git (12)") {
		t.Fatalf("expected filtered most typoed tool summary, got stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestStatsCommandErrors(t *testing.T) {
	useTempHome(t)

	tests := []struct {
		name   string
		args   []string
		wantIn string
	}{
		{name: "since must be positive", args: []string{"typo", "stats", "--since", "0"}, wantIn: "--since must be greater than 0"},
		{name: "top must be positive", args: []string{"typo", "stats", "--top", "0"}, wantIn: "--top must be greater than 0"},
		{name: "no positional args", args: []string{"typo", "stats", "extra"}, wantIn: "stats does not accept positional arguments"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runCLI(t, tt.args)
			if code == 0 {
				t.Fatalf("expected stats error, got code=0 stdout=%q stderr=%q", stdout, stderr)
			}
			if !strings.Contains(stderr, tt.wantIn) {
				t.Fatalf("expected stderr to contain %q, got stdout=%q stderr=%q", tt.wantIn, stdout, stderr)
			}
		})
	}
}

func TestConfigGenRequiresForce(t *testing.T) {
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

	if code, _, stderr := runCLI(t, []string{"typo", "config", "gen"}); code != 0 {
		t.Fatalf("initial config gen failed: %q", stderr)
	}

	code, _, stderr := runCLI(t, []string{"typo", "config", "gen"})
	if code == 0 {
		t.Fatal("config gen should fail when config already exists")
	}
	if !strings.Contains(stderr, "--force") {
		t.Fatalf("expected stderr to mention --force, got %q", stderr)
	}

	code, _, stderr = runCLI(t, []string{"typo", "config", "gen", "--force"})
	if code != 0 {
		t.Fatalf("config gen --force failed: %q", stderr)
	}
}

func TestConfigCommandErrors(t *testing.T) {
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

	tests := []struct {
		name   string
		args   []string
		wantIn string
	}{
		{name: "missing subcommand", args: []string{"typo", "config"}, wantIn: "subcommand required"},
		{name: "missing get key", args: []string{"typo", "config", "get"}, wantIn: "<key> required"},
		{name: "unknown get key", args: []string{"typo", "config", "get", "unknown"}, wantIn: "unknown config key"},
		{name: "missing set value", args: []string{"typo", "config", "set", "keyboard"}, wantIn: "<key> and <value> required"},
		{name: "invalid set value", args: []string{"typo", "config", "set", "history.enabled", "maybe"}, wantIn: "invalid bool value"},
		{name: "invalid experimental set value", args: []string{"typo", "config", "set", "experimental.long-option-correction.enabled", "maybe"}, wantIn: "invalid bool value"},
		{name: "invalid auto learn threshold", args: []string{"typo", "config", "set", "auto-learn-threshold", "wat"}, wantIn: "invalid int value"},
		{name: "gen positional args", args: []string{"typo", "config", "gen", "extra"}, wantIn: "does not accept positional arguments"},
		{name: "gen invalid flag", args: []string{"typo", "config", "gen", "--wat"}, wantIn: "flag provided but not defined"},
		{name: "unknown subcommand", args: []string{"typo", "config", "wat"}, wantIn: "Unknown subcommand"},
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
}

func TestConfigCommandWriteFailures(t *testing.T) {
	oldHome := os.Getenv("HOME")
	defer func() {
		if err := os.Setenv("HOME", oldHome); err != nil {
			t.Fatalf("Restore HOME failed: %v", err)
		}
	}()

	tmpFile, err := os.CreateTemp("", "typo-home-file-*")
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Fatalf("Remove temp file failed: %v", err)
		}
	}()
	_ = tmpFile.Close()

	if err := os.Setenv("HOME", tmpFile.Name()); err != nil {
		t.Fatalf("Setenv HOME failed: %v", err)
	}

	tests := []struct {
		name string
		args []string
	}{
		{name: "set fails", args: []string{"typo", "config", "set", "keyboard", "dvorak"}},
		{name: "reset fails", args: []string{"typo", "config", "reset"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runCLI(t, tt.args)
			if code == 0 {
				t.Fatalf("expected failure, got code=0 stdout=%q stderr=%q", stdout, stderr)
			}
			if !strings.Contains(stderr, "Error:") {
				t.Fatalf("expected write failure stderr, got stdout=%q stderr=%q", stdout, stderr)
			}
		})
	}
}
