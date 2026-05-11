package cmd

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/yuluo-yx/typo/internal/config"
)

func TestRun(t *testing.T) {
	useTempHome(t)

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
			name:       "init bash",
			args:       []string{"typo", "init", "bash"},
			wantCode:   0,
			wantOutput: "bind -x",
		},
		{
			name:       "init fish",
			args:       []string{"typo", "init", "fish"},
			wantCode:   0,
			wantOutput: "bind escape,escape _typo_fix_command",
		},
		{
			name:       "init powershell",
			args:       []string{"typo", "init", "powershell"},
			wantCode:   0,
			wantOutput: "Set-PSReadLineKeyHandler",
		},
		{
			name:       "init pwsh alias",
			args:       []string{"typo", "init", "pwsh"},
			wantCode:   0,
			wantOutput: "Set-PSReadLineKeyHandler",
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
		{
			name:       "stats",
			args:       []string{"typo", "stats"},
			wantCode:   0,
			wantOutput: "No accepted corrections",
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

			if code != tt.wantCode {
				t.Errorf("run() = %d, want %d", code, tt.wantCode)
			}
			if tt.wantOutput != "" && !bytes.Contains([]byte(output), []byte(tt.wantOutput)) {
				t.Errorf("Expected output to contain %q, got %q", tt.wantOutput, output)
			}
		})
	}
}

func TestRunInvalidFlagReturnsOne(t *testing.T) {
	if os.Getenv("TYPO_TEST_RUN_INVALID_FLAG") == "1" {
		args := strings.Split(os.Getenv("TYPO_TEST_RUN_ARGS"), "\n")
		os.Args = args
		os.Exit(Run())
	}

	tests := []struct {
		name string
		args []string
	}{
		{name: "fix", args: []string{"typo", "fix", "--bad"}},
		{name: "explain", args: []string{"typo", "explain", "--bad"}},
		{name: "stats", args: []string{"typo", "stats", "--bad"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=TestRunInvalidFlagReturnsOne")
			cmd.Env = append(
				os.Environ(),
				"TYPO_TEST_RUN_INVALID_FLAG=1",
				"TYPO_TEST_RUN_ARGS="+strings.Join(tt.args, "\n"),
			)

			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("expected invalid flag to exit with code 1, got success; output:\n%s", output)
			}
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("expected ExitError, got %T: %v", err, err)
			}
			if got := exitErr.ExitCode(); got != 1 {
				t.Fatalf("exit code = %d, want 1; output:\n%s", got, output)
			}
			if !bytes.Contains(output, []byte("flag provided but not defined")) {
				t.Fatalf("expected flag parser error, got:\n%s", output)
			}
		})
	}
}

func TestDiscoverCommandsWithinTimeout(t *testing.T) {
	t.Run("returns loader result within budget", func(t *testing.T) {
		result := discoverCommandsWithinTimeout(func() []string {
			return []string{"foo", "bar"}
		}, 100*time.Millisecond)

		if len(result) != 2 || result[0] != "foo" || result[1] != "bar" {
			t.Fatalf("unexpected discovery result: %v", result)
		}
	})

	t.Run("returns quickly when loader blocks", func(t *testing.T) {
		start := time.Now()
		result := discoverCommandsWithinTimeout(func() []string {
			time.Sleep(time.Second)
			return []string{"slow"}
		}, 50*time.Millisecond)
		elapsed := time.Since(start)

		if result != nil {
			t.Fatalf("expected nil result after timeout, got %v", result)
		}
		if elapsed >= 500*time.Millisecond {
			t.Fatalf("expected timed discovery to return quickly, got %v", elapsed)
		}
	})

	t.Run("returns nil for nil loader", func(t *testing.T) {
		if got := discoverCommandsWithinTimeout(nil, 10*time.Millisecond); got != nil {
			t.Fatalf("expected nil loader result, got %v", got)
		}
	})

	t.Run("uses direct loader when timeout disabled", func(t *testing.T) {
		if got := discoverCommandsWithinTimeout(func() []string { return []string{"direct"} }, 0); len(got) != 1 || got[0] != "direct" {
			t.Fatalf("unexpected direct discovery result: %v", got)
		}
	})
}

func TestCreateEngineWithEmptyPath(t *testing.T) {
	oldPath := os.Getenv("PATH")
	defer func() {
		if err := os.Setenv("PATH", oldPath); err != nil {
			t.Fatalf("Restore PATH failed: %v", err)
		}
	}()

	if err := os.Setenv("PATH", ""); err != nil {
		t.Fatalf("Setenv PATH failed: %v", err)
	}

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

	code := Run()

	if err := w.Close(); err != nil {
		t.Fatalf("Close pipe failed: %v", err)
	}
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

	if err := w.Close(); err != nil {
		t.Fatalf("Close pipe failed: %v", err)
	}
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("Read pipe failed: %v", err)
	}
	output := buf.String()

	if !bytes.Contains([]byte(output), []byte("typo")) {
		t.Error("Expected usage to contain 'typo'")
	}
	if !bytes.Contains([]byte(output), []byte("experimental.long-option-correction.enabled")) {
		t.Error("Expected usage to mention experimental long-option correction")
	}
}

func TestCmdVersion(t *testing.T) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmdVersion()

	if err := w.Close(); err != nil {
		t.Fatalf("Close pipe failed: %v", err)
	}
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("Read pipe failed: %v", err)
	}
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
	script, code := printIntegrationScript("zsh")
	if code != 0 {
		t.Fatalf("Expected code 0, got %d", code)
	}
	output := captureStdout(t, func() {
		printScript(script)
	})

	if output != zshIntegrationScript {
		t.Error("Expected zsh integration output to match embedded install script")
	}
	if !bytes.Contains([]byte(output), []byte("_typo_cleanup_stale_caches")) {
		t.Error("Expected zsh integration to include stale cache cleanup")
	}
}

func TestPrintZshIntegrationAddsTrailingNewline(t *testing.T) {
	output := captureStdout(t, func() {
		printScript("echo test")
	})

	if output != "echo test\n" {
		t.Fatalf("Expected trailing newline to be appended, got %q", output)
	}
}

func TestPrintBashIntegration(t *testing.T) {
	script, code := printIntegrationScript("bash")
	if code != 0 {
		t.Fatalf("Expected code 0, got %d", code)
	}
	output := captureStdout(t, func() {
		printScript(script)
	})

	if output != bashIntegrationScript {
		t.Error("Expected bash integration output to match embedded install script")
	}
	if !bytes.Contains([]byte(output), []byte("_typo_cleanup_stale_caches")) {
		t.Error("Expected bash integration to include stale cache cleanup")
	}
}

func TestPrintBashIntegrationAddsTrailingNewline(t *testing.T) {
	output := captureStdout(t, func() {
		printScript("echo test")
	})

	if output != "echo test\n" {
		t.Fatalf("Expected trailing newline to be appended, got %q", output)
	}
}

func TestPrintFishIntegration(t *testing.T) {
	script, code := printIntegrationScript("fish")
	if code != 0 {
		t.Fatalf("Expected code 0, got %d", code)
	}
	output := captureStdout(t, func() {
		printScript(script)
	})

	if output != fishIntegrationScript {
		t.Error("Expected fish integration output to match embedded install script")
	}
	if !bytes.Contains([]byte(output), []byte("bind escape,escape _typo_fix_command")) {
		t.Error("Expected fish integration to bind Escape,Escape")
	}
}

func TestPrintFishIntegrationAddsTrailingNewline(t *testing.T) {
	output := captureStdout(t, func() {
		printScript("echo test")
	})

	if output != "echo test\n" {
		t.Fatalf("Expected trailing newline to be appended, got %q", output)
	}
}

func TestPrintPowerShellIntegration(t *testing.T) {
	script, code := printIntegrationScript("powershell")
	if code != 0 {
		t.Fatalf("Expected code 0, got %d", code)
	}
	output := captureStdout(t, func() {
		printScript(script)
	})

	if output != powerShellIntegrationScript {
		t.Error("Expected PowerShell integration output to match embedded install script")
	}
	if !bytes.Contains([]byte(output), []byte(`Set-PSReadLineKeyHandler -Chord "Escape,Escape"`)) &&
		!bytes.Contains([]byte(output), []byte("Set-PSReadLineKeyHandler -Chord Escape,Escape")) {
		t.Error("Expected PowerShell integration to bind Escape,Escape with or without quotes")
	}
}

func TestPowerShellIntegrationEnterWrapsExternalCommands(t *testing.T) {
	script, code := printIntegrationScript("powershell")
	if code != 0 {
		t.Fatalf("Expected code 0, got %d", code)
	}

	if !strings.Contains(script, "__typo_ShouldWrapAcceptedLine") {
		t.Fatal("Expected PowerShell integration to keep external-command wrapping predicate")
	}
	if !strings.Contains(script, "$global:TYPO_PS_STATE.PendingOriginalLine = $line") {
		t.Fatal("Expected PowerShell Enter handler to queue external commands for stderr capture")
	}
	if !strings.Contains(script, "__typo_InvokeAcceptedLine") {
		t.Fatal("Expected PowerShell integration to invoke queued external commands")
	}
}

func TestPrintPowerShellIntegrationAddsTrailingNewline(t *testing.T) {
	output := captureStdout(t, func() {
		printScript("Write-Host test")
	})

	if output != "Write-Host test\n" {
		t.Fatalf("Expected trailing newline to be appended, got %q", output)
	}
}
