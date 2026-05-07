package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckDoctorShellIntegrationWithoutShellName(t *testing.T) {
	t.Setenv("TYPO_SHELL_INTEGRATION", "")

	stdout := captureStdout(t, func() {
		if !checkDoctorShellIntegration("", "") {
			t.Fatalf("missing shell integration should return true")
		}
	})

	for _, want := range []string{
		"To enable shell integration, add one of the following:",
		"eval \"$(typo init zsh)\"",
		"Invoke-Expression (& typo init powershell)",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout missing %q: %q", want, stdout)
		}
	}
}

func TestCheckDoctorInstallMethodUnknownAndWithoutShellName(t *testing.T) {
	stdout := captureStdout(t, func() {
		checkDoctorInstallMethod("", "", "")
	})

	for _, want := range []string{
		"unable to determine",
		"Install/update:",
		"typo update: unsupported",
		"Shell setup: run typo doctor from zsh, bash, fish, or PowerShell",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout missing %q: %q", want, stdout)
		}
	}
}

func TestCheckDoctorGoBinPathBranches(t *testing.T) {
	oldUserHomeDir := userHomeDir
	defer func() { userHomeDir = oldUserHomeDir }()

	t.Run("go bin configured", func(t *testing.T) {
		tmpHome := t.TempDir()
		goBin := filepath.Join(tmpHome, "go", "bin")
		typoPath := filepath.Join(goBin, "typo")
		userHomeDir = func() (string, error) {
			return tmpHome, nil
		}
		t.Setenv("GOBIN", "")
		t.Setenv("GOPATH", filepath.Join(tmpHome, "go"))
		t.Setenv("PATH", strings.Join([]string{"/usr/bin", goBin}, string(os.PathListSeparator)))

		stdout := captureStdout(t, func() {
			if checkDoctorGoBinPath("bash", "~/.bashrc", typoPath) {
				t.Fatalf("configured Go bin should not be an error")
			}
		})
		if !strings.Contains(stdout, "configured") {
			t.Fatalf("stdout missing configured message: %q", stdout)
		}
	})

	t.Run("go bin missing path without shell", func(t *testing.T) {
		tmpHome := t.TempDir()
		goBin := filepath.Join(tmpHome, "go", "bin")
		typoPath := filepath.Join(goBin, "typo")
		userHomeDir = func() (string, error) {
			return tmpHome, nil
		}
		t.Setenv("GOBIN", "")
		t.Setenv("GOPATH", filepath.Join(tmpHome, "go"))
		t.Setenv("PATH", "/usr/bin")

		stdout := captureStdout(t, func() {
			if !checkDoctorGoBinPath("", "", typoPath) {
				t.Fatalf("missing Go bin PATH should be an error")
			}
		})
		if !strings.Contains(stdout, "If you installed typo with 'go install', add to your shell config:") {
			t.Fatalf("stdout missing generic shell config hint: %q", stdout)
		}
	})
}

func TestDoctorInstallPathFallsBackToExecutable(t *testing.T) {
	origExecutable := executable
	defer func() { executable = origExecutable }()

	executable = func() (string, error) {
		return "/tmp/typo", nil
	}
	if got := doctorInstallPath(""); got != "/tmp/typo" {
		t.Fatalf("doctorInstallPath fallback = %q", got)
	}
	executable = func() (string, error) {
		return "", os.ErrNotExist
	}
	if got := doctorInstallPath(""); got != "" {
		t.Fatalf("doctorInstallPath failing fallback = %q", got)
	}
}

func TestDoctorShellMisconfigurationEdges(t *testing.T) {
	if got := doctorShellMisconfiguration(""); got != "" {
		t.Fatalf("empty shell should not report misconfiguration: %q", got)
	}

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	configPath := filepath.Join(tmpHome, ".zshrc")
	if err := os.WriteFile(configPath, []byte("eval \"$(typo init zsh)\"\n"), 0600); err != nil {
		t.Fatalf("write zshrc: %v", err)
	}
	if got := doctorShellMisconfiguration("zsh"); got != "" {
		t.Fatalf("matching shell init should not report: %q", got)
	}

	if err := os.WriteFile(configPath, []byte("eval \"$(typo init bash)\"\n"), 0600); err != nil {
		t.Fatalf("write zshrc: %v", err)
	}
	if got := doctorShellMisconfiguration("zsh"); !strings.Contains(got, "does not match zsh") {
		t.Fatalf("mismatched shell init not reported: %q", got)
	}
}

func TestDoctorInstallCandidatePathsIncludesSymlinkTarget(t *testing.T) {
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "target")
	linkDir := filepath.Join(tmpDir, "link")
	if err := os.Mkdir(targetDir, 0755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	target := filepath.Join(targetDir, "typo")
	if err := os.WriteFile(target, []byte("bin"), 0755); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := os.Mkdir(linkDir, 0755); err != nil {
		t.Fatalf("mkdir link: %v", err)
	}
	link := filepath.Join(linkDir, "typo")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	candidates := doctorInstallCandidatePaths(link)
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatalf("resolve target: %v", err)
	}
	if len(candidates) != 2 || candidates[0] != filepath.Clean(link) || candidates[1] != filepath.Clean(resolvedTarget) {
		t.Fatalf("doctorInstallCandidatePaths = %#v", candidates)
	}
}
