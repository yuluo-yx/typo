//go:build unix

package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeRecordingTool installs a stub that appends its arguments to a log file.
func writeRecordingTool(t *testing.T, dir, name, logFile, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	script := "#!/bin/sh\necho \"$*\" >> '" + logFile + "'\n" + body
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write tool stub: %v", err)
	}
	return path
}

func readInvocations(t *testing.T, logFile string) []string {
	t.Helper()
	data, err := os.ReadFile(logFile)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("read invocation log: %v", err)
	}
	return strings.Fields(strings.ReplaceAll(strings.TrimSpace(string(data)), "\n", " "))
}

func TestGetChildren_DoesNotExecuteUnknownTools(t *testing.T) {
	binDir := t.TempDir()
	logFile := filepath.Join(t.TempDir(), "calls.log")
	writeRecordingTool(t, binDir, "deploytool", logFile, "printf 'Commands:\\n  release   Release\\n'\n")
	t.Setenv("PATH", binDir)

	r := NewToolTreeRegistry(t.TempDir())
	if got := r.GetChildren("deploytool", nil); len(got) != 0 {
		t.Fatalf("unknown tool must not yield probed children, got %v", got)
	}
	if calls := readInvocations(t, logFile); len(calls) != 0 {
		t.Fatalf("unknown tool was executed: %v", calls)
	}
}

func TestGetChildren_DiscoverUnknownOptIn(t *testing.T) {
	binDir := t.TempDir()
	logFile := filepath.Join(t.TempDir(), "calls.log")
	writeRecordingTool(t, binDir, "deploytool", logFile, "printf 'Commands:\\n  release   Release\\n  rollback  Roll back\\n'\n")
	t.Setenv("PATH", binDir)

	r := NewToolTreeRegistry(t.TempDir())
	r.SetDiscoverUnknownTools(true)
	got := r.GetChildren("deploytool", nil)
	if !hasString(got, "release") || !hasString(got, "rollback") {
		t.Fatalf("opt-in discovery should parse help output, got %v", got)
	}
}

func TestGetHelpOutput_NeverRunsPositionalHelpForUnknownTools(t *testing.T) {
	binDir := t.TempDir()
	workDir := t.TempDir()
	logFile := filepath.Join(t.TempDir(), "calls.log")
	// Simulates rm: "--help" fails, and a positional "help" deletes a file named help.
	writeRecordingTool(t, binDir, "danger", logFile, "case \"$1\" in --help) exit 2;; esac\nrm -f -- \"$1\"\n")
	t.Setenv("PATH", binDir)
	victim := filepath.Join(workDir, "help")
	if err := os.WriteFile(victim, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workDir)

	r := &ToolTreeRegistry{helpTimeout: 5 * time.Second, discoverUnknown: true}
	if _, err := r.getHelpOutput("danger"); err == nil {
		t.Fatal("expected failing --help to be reported")
	}
	if calls := readInvocations(t, logFile); len(calls) != 1 || calls[0] != "--help" {
		t.Fatalf("expected only --help, got %v", calls)
	}
	if _, err := os.Stat(victim); err != nil {
		t.Fatalf("file named help in the working directory was removed: %v", err)
	}
}

func TestRunHelpCommand_RunsOutsideWorkingDirectory(t *testing.T) {
	binDir := t.TempDir()
	workDir := t.TempDir()
	logFile := filepath.Join(t.TempDir(), "calls.log")
	writeRecordingTool(t, binDir, "pwdtool", logFile, "pwd\n")
	t.Setenv("PATH", binDir)
	t.Chdir(workDir)

	r := &ToolTreeRegistry{helpTimeout: 5 * time.Second}
	output, err := r.runHelpCommand("pwdtool")
	if err != nil {
		t.Fatalf("runHelpCommand: %v", err)
	}
	resolvedWork, _ := filepath.EvalSymlinks(workDir)
	if got := strings.TrimSpace(output); got == workDir || got == resolvedWork {
		t.Fatalf("help probe ran in the user's working directory %q", got)
	}
}

func TestRunHelpCommand_CapsOutput(t *testing.T) {
	binDir := t.TempDir()
	logFile := filepath.Join(t.TempDir(), "calls.log")
	writeRecordingTool(t, binDir, "loud", logFile, "/usr/bin/head -c 3000000 /dev/zero | /usr/bin/tr '\\0' 'a'\n")
	t.Setenv("PATH", binDir)

	r := &ToolTreeRegistry{helpTimeout: 5 * time.Second}
	output, err := r.runHelpCommand("loud")
	if err != nil {
		t.Fatalf("runHelpCommand: %v", err)
	}
	if len(output) != maxHelpOutputBytes {
		t.Fatalf("expected output capped at %d bytes, got %d", maxHelpOutputBytes, len(output))
	}
}

func TestGetChildren_NegativeCacheSuppressesRepeatedProbes(t *testing.T) {
	binDir := t.TempDir()
	cacheDir := t.TempDir()
	logFile := filepath.Join(t.TempDir(), "calls.log")
	// pip is allowlisted but has no builtin tree, so an empty help output yields nothing.
	writeRecordingTool(t, binDir, "pip", logFile, "exit 0\n")
	t.Setenv("PATH", binDir)

	r := NewToolTreeRegistry(cacheDir)
	for range 3 {
		if got := r.GetChildren("pip", nil); len(got) != 0 {
			t.Fatalf("expected no children, got %v", got)
		}
	}
	if calls := readInvocations(t, logFile); len(calls) != 1 {
		t.Fatalf("expected a single probe per process, got %v", calls)
	}

	// A new process reads the persisted negative entry and skips the probe.
	if got := NewToolTreeRegistry(cacheDir).GetChildren("pip", nil); len(got) != 0 {
		t.Fatalf("expected no children, got %v", got)
	}
	if calls := readInvocations(t, logFile); len(calls) != 1 {
		t.Fatalf("negative cache did not persist across registries: %v", calls)
	}
}

func TestGetChildren_NegativeCacheInvalidatedByExecutableChange(t *testing.T) {
	binDir := t.TempDir()
	cacheDir := t.TempDir()
	logFile := filepath.Join(t.TempDir(), "calls.log")
	writeRecordingTool(t, binDir, "pip", logFile, "exit 0\n")
	t.Setenv("PATH", binDir)

	NewToolTreeRegistry(cacheDir).GetChildren("pip", nil)

	// Upgrading the tool changes its size, so the old negative entry must not apply.
	writeRecordingTool(t, binDir, "pip", logFile, "printf 'Commands:\\n  install    Install packages.\\n'\n")
	got := NewToolTreeRegistry(cacheDir).GetChildren("pip", nil)
	if !hasString(got, "install") {
		t.Fatalf("expected upgraded tool to be re-probed, got %v", got)
	}
	if calls := readInvocations(t, logFile); len(calls) != 2 {
		t.Fatalf("expected a second probe after the executable changed, got %v", calls)
	}
}

func TestGetChildren_NegativeCacheExpires(t *testing.T) {
	binDir := t.TempDir()
	logFile := filepath.Join(t.TempDir(), "calls.log")
	writeRecordingTool(t, binDir, "pip", logFile, "exit 0\n")
	t.Setenv("PATH", binDir)

	r := NewToolTreeRegistry("")
	r.GetChildren("pip", nil)
	key := probeKey("pip", nil)
	r.mu.Lock()
	entry := r.negative[key]
	entry.CheckedAt = time.Now().Add(-2 * defaultNegativeProbeTTL)
	r.negative[key] = entry
	delete(r.probed, key)
	r.mu.Unlock()

	r.GetChildren("pip", nil)
	if calls := readInvocations(t, logFile); len(calls) != 2 {
		t.Fatalf("expected expired negative entry to allow a new probe, got %v", calls)
	}
}
