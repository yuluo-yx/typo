package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"strings"
	"testing"
)

type updateCommandCall struct {
	name     string
	args     []string
	extraEnv []string
}

func captureUpdateOutput(t *testing.T, fn func() int) (int, string, string) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("Create stdout pipe failed: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("Create stderr pipe failed: %v", err)
	}
	os.Stdout = wOut
	os.Stderr = wErr
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	code := fn()

	if err := wOut.Close(); err != nil {
		t.Fatalf("Close stdout pipe failed: %v", err)
	}
	if err := wErr.Close(); err != nil {
		t.Fatalf("Close stderr pipe failed: %v", err)
	}

	var outBuf, errBuf bytes.Buffer
	if _, err := outBuf.ReadFrom(rOut); err != nil {
		t.Fatalf("Read stdout pipe failed: %v", err)
	}
	if _, err := errBuf.ReadFrom(rErr); err != nil {
		t.Fatalf("Read stderr pipe failed: %v", err)
	}

	return code, outBuf.String(), errBuf.String()
}

func withUpdateTestHooks(t *testing.T, typoPath string) *[]updateCommandCall {
	t.Helper()

	origLookPath := lookPath
	origExecutable := executable
	origUserHomeDir := userHomeDir
	origDownloadFile := updateDownloadFile
	origLatestRelease := updateLatestRelease
	origMainCommit := updateMainCommit
	origRunCommand := updateRunCommand
	origCommandOutput := updateCommandOutput
	origVersion := version
	origCommit := commit
	origDate := date
	origReadBuildInfo := readBuildInfo

	calls := make([]updateCommandCall, 0)
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", filepath.Join(tmpHome, "go"))
	t.Setenv("HOMEBREW_PREFIX", "")
	t.Setenv("TYPO_INSTALL_DIR", filepath.Dir(typoPath))
	version = "1.0.0"
	commit = "none"
	date = UnknownValue
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return nil, false
	}
	userHomeDir = func() (string, error) {
		return tmpHome, nil
	}
	executable = func() (string, error) {
		return typoPath, nil
	}
	lookPath = func(file string) (string, error) {
		switch file {
		case "typo":
			return typoPath, nil
		case "bash":
			return "/bin/bash", nil
		case "go":
			return "/usr/local/bin/go", nil
		case "brew":
			return "/opt/homebrew/bin/brew", nil
		case "curl":
			return "/usr/bin/curl", nil
		default:
			return "", os.ErrNotExist
		}
	}
	updateDownloadFile = func(url, dst string) error {
		if url != installScriptURL {
			t.Fatalf("unexpected script URL: %s", url)
		}
		return os.WriteFile(dst, []byte("#!/usr/bin/env bash\n"), 0755)
	}
	updateLatestRelease = func() (string, error) {
		return "v1.1.0", nil
	}
	updateMainCommit = func() (string, error) {
		return "abcdef1234567890", nil
	}
	updateRunCommand = func(name string, args []string, extraEnv []string) error {
		calls = append(calls, updateCommandCall{
			name:     name,
			args:     append([]string(nil), args...),
			extraEnv: append([]string(nil), extraEnv...),
		})
		return nil
	}
	updateCommandOutput = func(name string, args []string, extraEnv []string) (string, error) {
		calls = append(calls, updateCommandCall{
			name:     name,
			args:     append([]string(nil), args...),
			extraEnv: append([]string(nil), extraEnv...),
		})
		return "", nil
	}

	t.Cleanup(func() {
		lookPath = origLookPath
		executable = origExecutable
		userHomeDir = origUserHomeDir
		updateDownloadFile = origDownloadFile
		updateLatestRelease = origLatestRelease
		updateMainCommit = origMainCommit
		updateRunCommand = origRunCommand
		updateCommandOutput = origCommandOutput
		version = origVersion
		commit = origCommit
		date = origDate
		readBuildInfo = origReadBuildInfo
	})

	return &calls
}

func TestUpdateProductionCodeDoesNotImportNetHTTP(t *testing.T) {
	data, err := os.ReadFile("update.go")
	if err != nil {
		t.Fatalf("read update.go: %v", err)
	}
	if strings.Contains(string(data), "\"net/http\"") {
		t.Fatalf("update.go must not import net/http; use external download tools to keep the binary small")
	}
}

func TestCmdUpdateUsesRunningBinaryInsteadOfPathLookup(t *testing.T) {
	runningTypo := filepath.Join(t.TempDir(), ".local", "bin", "typo")
	pathTypo := filepath.Join(t.TempDir(), "usr", "local", "bin", "typo")
	calls := withUpdateTestHooks(t, runningTypo)
	lookPath = func(file string) (string, error) {
		switch file {
		case "typo":
			return pathTypo, nil
		case "bash":
			return "/bin/bash", nil
		case "go":
			return "/usr/local/bin/go", nil
		default:
			return "", os.ErrNotExist
		}
	}

	code, stdout, stderr := captureUpdateOutput(t, func() int {
		return cmdUpdate([]string{"--dry-run"})
	})

	if code != 0 {
		t.Fatalf("cmdUpdate dry-run failed: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if len(*calls) != 0 {
		t.Fatalf("dry-run should not run commands, got %#v", *calls)
	}
	if !strings.Contains(stdout, "Update target: "+runningTypo) {
		t.Fatalf("update should target running binary, got stdout=%q", stdout)
	}
	if strings.Contains(stdout, pathTypo) {
		t.Fatalf("update should not target PATH lookup result, got stdout=%q", stdout)
	}
}

func TestCmdUpdateFlagParsingHandlesHelpAndBadFlags(t *testing.T) {
	code, _, stderr := captureUpdateOutput(t, func() int {
		return cmdUpdate([]string{"--help"})
	})
	if code != 0 {
		t.Fatalf("help should exit 0, got code=%d stderr=%q", code, stderr)
	}

	code, _, stderr = captureUpdateOutput(t, func() int {
		return cmdUpdate([]string{"--bogus"})
	})
	if code == 0 {
		t.Fatalf("bad flag should fail")
	}
	if !strings.Contains(stderr, "flag provided but not defined") {
		t.Fatalf("missing flag parse error, got %q", stderr)
	}
}

func TestResolveRunningTypoInstallReportsExecutableError(t *testing.T) {
	origExecutable := executable
	defer func() { executable = origExecutable }()

	executable = func() (string, error) {
		return "", errors.New("exec path unavailable")
	}

	_, err := resolveRunningTypoInstall()
	if err == nil || !strings.Contains(err.Error(), "cannot determine running typo binary") {
		t.Fatalf("expected executable error to be wrapped, got %v", err)
	}
}

func TestCmdUpdateReportsExecutableError(t *testing.T) {
	origExecutable := executable
	defer func() { executable = origExecutable }()

	executable = func() (string, error) {
		return "", errors.New("exec path unavailable")
	}

	code, _, stderr := captureUpdateOutput(t, func() int {
		return cmdUpdate(nil)
	})
	if code == 0 {
		t.Fatalf("expected executable error")
	}
	if !strings.Contains(stderr, "cannot determine running typo binary") {
		t.Fatalf("missing executable error, got %q", stderr)
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.1", -1},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0", "1.0.0", 0},
		{"2.0.0", "1.9.9", 1},
		{"1.10.0", "1.9.0", 1},
		{"v1.0.0", "v1.0.1", -1},
		{"dev", "1.0.0", -1},
		{"1.0.0", "dev", 1},
		{"unknown", "1.0.0", -1},
		{"", "1.0.0", -1},
		{"1.2.3", "1.2.3", 0},
		{"1.2", "1.2.0", 0},
		{"1.2.0", "1.2", 0},
		{"10.0.0", "9.0.0", 1},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			got := compareVersions(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestSplitVersion(t *testing.T) {
	tests := []struct {
		input string
		want  []int
	}{
		{"1.2.3", []int{1, 2, 3}},
		{"10.20.30", []int{10, 20, 30}},
		{"1", []int{1}},
		{"1.2", []int{1, 2}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitVersion(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitVersion(%q) = %#v, want %#v", tt.input, got, tt.want)
			}
		})
	}
}

func TestDetectDoctorInstallMethodUpdateSupport(t *testing.T) {
	tmpHome := t.TempDir()
	oldUserHomeDir := userHomeDir
	defer func() { userHomeDir = oldUserHomeDir }()
	userHomeDir = func() (string, error) {
		return tmpHome, nil
	}
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", filepath.Join(tmpHome, "go"))
	t.Setenv("HOMEBREW_PREFIX", "")
	t.Setenv("TYPO_INSTALL_DIR", "")

	tests := []struct {
		name            string
		path            string
		wantKind        doctorInstallKind
		updateSupported bool
	}{
		{
			name:            "go install",
			path:            filepath.Join(tmpHome, "go", "bin", "typo"),
			wantKind:        doctorInstallGo,
			updateSupported: false,
		},
		{
			name:            "script local bin",
			path:            filepath.Join(tmpHome, ".local", "bin", "typo"),
			wantKind:        doctorInstallScript,
			updateSupported: true,
		},
		{
			name:            "homebrew cellar",
			path:            "/opt/homebrew/Cellar/typo/1.1.0/bin/typo",
			wantKind:        doctorInstallHomebrew,
			updateSupported: true,
		},
		{
			name:            "manual release",
			path:            filepath.Join(tmpHome, "bin", "typo"),
			wantKind:        doctorInstallManual,
			updateSupported: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectDoctorInstallMethod(tt.path)
			if got.kind != tt.wantKind {
				t.Fatalf("kind = %v, want %v", got.kind, tt.wantKind)
			}
			if got.updateSupported != tt.updateSupported {
				t.Fatalf("updateSupported = %v, want %v", got.updateSupported, tt.updateSupported)
			}
		})
	}
}

func TestCmdUpdateScriptInstallBuildsMainByDefault(t *testing.T) {
	typoPath := filepath.Join(t.TempDir(), ".local", "bin", "typo")
	calls := withUpdateTestHooks(t, typoPath)

	code, stdout, stderr := captureUpdateOutput(t, func() int {
		return cmdUpdate(nil)
	})

	if code != 0 {
		t.Fatalf("cmdUpdate failed: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if len(*calls) != 1 {
		t.Fatalf("expected 1 command call, got %#v", *calls)
	}
	call := (*calls)[0]
	if call.name != "bash" {
		t.Fatalf("command = %q, want bash", call.name)
	}
	if len(call.args) != 2 || filepath.Base(call.args[0]) != "install.sh" || call.args[1] != "-b" {
		t.Fatalf("unexpected args: %#v", call.args)
	}
	wantEnv := "TYPO_INSTALL_DIR=" + filepath.Dir(typoPath)
	if len(call.extraEnv) != 1 || call.extraEnv[0] != wantEnv {
		t.Fatalf("extraEnv = %#v, want %q", call.extraEnv, wantEnv)
	}
	if !strings.Contains(stdout, "Updated typo from the main branch") {
		t.Fatalf("missing success output: %q", stdout)
	}
}

func TestCmdUpdateScriptInstallSpecificRelease(t *testing.T) {
	typoPath := filepath.Join(t.TempDir(), ".local", "bin", "typo")
	calls := withUpdateTestHooks(t, typoPath)

	code, stdout, stderr := captureUpdateOutput(t, func() int {
		return cmdUpdate([]string{"--version", "1.1.0"})
	})

	if code != 0 {
		t.Fatalf("cmdUpdate failed: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if len(*calls) != 1 {
		t.Fatalf("expected 1 command call, got %#v", *calls)
	}
	call := (*calls)[0]
	if len(call.args) != 3 || call.args[1] != "-s" || call.args[2] != "1.1.0" {
		t.Fatalf("unexpected args: %#v", call.args)
	}
	if !strings.Contains(stdout, "Updated typo to v1.1.0") {
		t.Fatalf("missing release success output: %q", stdout)
	}
}

func TestCmdUpdateScriptInstallMainAliases(t *testing.T) {
	for _, alias := range []string{"main", "latest"} {
		t.Run(alias, func(t *testing.T) {
			typoPath := filepath.Join(t.TempDir(), ".local", "bin", "typo")
			calls := withUpdateTestHooks(t, typoPath)

			code, stdout, stderr := captureUpdateOutput(t, func() int {
				return cmdUpdate([]string{"--version", alias})
			})

			if code != 0 {
				t.Fatalf("cmdUpdate failed: code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			if len(*calls) != 1 {
				t.Fatalf("expected 1 command call, got %#v", *calls)
			}
			call := (*calls)[0]
			if len(call.args) != 2 || filepath.Base(call.args[0]) != "install.sh" || call.args[1] != "-b" {
				t.Fatalf("unexpected args for %q: %#v", alias, call.args)
			}
			if !strings.Contains(stdout, "Updated typo from the main branch") {
				t.Fatalf("missing main success output: %q", stdout)
			}
		})
	}
}

func TestCmdUpdateRejectsGoStyleLatestVersion(t *testing.T) {
	typoPath := filepath.Join(t.TempDir(), ".local", "bin", "typo")
	calls := withUpdateTestHooks(t, typoPath)

	code, _, stderr := captureUpdateOutput(t, func() int {
		return cmdUpdate([]string{"--version", "@latest"})
	})

	if code == 0 {
		t.Fatalf("expected @latest to fail")
	}
	if len(*calls) != 0 {
		t.Fatalf("@latest should not run commands, got %#v", *calls)
	}
	if !strings.Contains(stderr, "unsupported --version \"@latest\"") ||
		!strings.Contains(stderr, "use 'typo update' for main") {
		t.Fatalf("missing @latest guidance, got %q", stderr)
	}
}

func TestCmdUpdateRejectsGoStyleLatestVersionBeforeInstallMethod(t *testing.T) {
	tmpHome := t.TempDir()
	typoPath := filepath.Join(tmpHome, "go", "bin", "typo")
	calls := withUpdateTestHooks(t, typoPath)
	t.Setenv("GOPATH", filepath.Join(tmpHome, "go"))

	code, _, stderr := captureUpdateOutput(t, func() int {
		return cmdUpdate([]string{"--version", "@latest"})
	})

	if code == 0 {
		t.Fatalf("expected @latest to fail")
	}
	if len(*calls) != 0 {
		t.Fatalf("@latest should not run commands, got %#v", *calls)
	}
	if !strings.Contains(stderr, "unsupported --version \"@latest\"") {
		t.Fatalf("missing @latest guidance, got %q", stderr)
	}
}

func TestCmdUpdateRejectsInvalidReleaseVersionBeforeInstallMethod(t *testing.T) {
	tmpHome := t.TempDir()
	typoPath := filepath.Join(tmpHome, "go", "bin", "typo")
	calls := withUpdateTestHooks(t, typoPath)
	t.Setenv("GOPATH", filepath.Join(tmpHome, "go"))

	code, _, stderr := captureUpdateOutput(t, func() int {
		return cmdUpdate([]string{"--version", "abc"})
	})

	if code == 0 {
		t.Fatalf("expected invalid release version to fail")
	}
	if len(*calls) != 0 {
		t.Fatalf("invalid release version should not run commands, got %#v", *calls)
	}
	if !strings.Contains(stderr, "invalid --version \"abc\"") ||
		!strings.Contains(stderr, "use '--version <release>' such as '--version 1.1.0'") {
		t.Fatalf("missing invalid version guidance, got %q", stderr)
	}
}

func TestCmdUpdateDryRunDoesNotRunCommand(t *testing.T) {
	typoPath := filepath.Join(t.TempDir(), ".local", "bin", "typo")
	calls := withUpdateTestHooks(t, typoPath)

	code, stdout, stderr := captureUpdateOutput(t, func() int {
		return cmdUpdate([]string{"--dry-run"})
	})

	if code != 0 {
		t.Fatalf("cmdUpdate dry-run failed: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if len(*calls) != 0 {
		t.Fatalf("dry-run should not run commands, got %#v", *calls)
	}
	if !strings.Contains(stdout, "Would run: TYPO_INSTALL_DIR=") {
		t.Fatalf("missing dry-run command output: %q", stdout)
	}
}

func TestCmdUpdateScriptInstallCheckRequiresGoForMain(t *testing.T) {
	typoPath := filepath.Join(t.TempDir(), ".local", "bin", "typo")
	calls := withUpdateTestHooks(t, typoPath)
	lookPath = func(file string) (string, error) {
		switch file {
		case "typo":
			return typoPath, nil
		case "bash":
			return "/bin/bash", nil
		case "go":
			return "", os.ErrNotExist
		default:
			return "", os.ErrNotExist
		}
	}

	code, _, stderr := captureUpdateOutput(t, func() int {
		return cmdUpdate([]string{"--check"})
	})

	if code == 0 {
		t.Fatalf("expected missing Go to fail")
	}
	if len(*calls) != 0 {
		t.Fatalf("check should not run commands, got %#v", *calls)
	}
	if !strings.Contains(stderr, "go is required to build typo from main") {
		t.Fatalf("missing Go error, got %q", stderr)
	}
}

func TestCmdUpdateHomebrewRunsBrewUpgrade(t *testing.T) {
	typoPath := "/opt/homebrew/Cellar/typo/1.1.0/bin/typo"
	calls := withUpdateTestHooks(t, typoPath)

	code, stdout, stderr := captureUpdateOutput(t, func() int {
		return cmdUpdate(nil)
	})

	if code != 0 {
		t.Fatalf("cmdUpdate Homebrew failed: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if len(*calls) != 2 {
		t.Fatalf("expected 2 brew command calls, got %#v", *calls)
	}
	if (*calls)[0].name != "brew" || !reflect.DeepEqual((*calls)[0].args, []string{"update"}) {
		t.Fatalf("unexpected first call: %#v", (*calls)[0])
	}
	if (*calls)[1].name != "brew" || !reflect.DeepEqual((*calls)[1].args, []string{"upgrade", "typo"}) {
		t.Fatalf("unexpected second call: %#v", (*calls)[1])
	}
	if !strings.Contains(stdout, "Updated typo with Homebrew") {
		t.Fatalf("missing Homebrew success output: %q", stdout)
	}
}

func TestCmdUpdateHomebrewDryRunDoesNotRunCommand(t *testing.T) {
	typoPath := "/opt/homebrew/Cellar/typo/1.1.0/bin/typo"
	calls := withUpdateTestHooks(t, typoPath)

	code, stdout, stderr := captureUpdateOutput(t, func() int {
		return cmdUpdate([]string{"--dry-run"})
	})

	if code != 0 {
		t.Fatalf("cmdUpdate Homebrew dry-run failed: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if len(*calls) != 0 {
		t.Fatalf("dry-run should not run commands, got %#v", *calls)
	}
	for _, want := range []string{"Would run: brew update", "Would run: brew upgrade typo"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout missing %q: %q", want, stdout)
		}
	}
}

func TestCmdUpdateHomebrewCheckReportsUpToDateAndAvailable(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		wantOutput string
	}{
		{
			name:       "up to date",
			output:     "",
			wantOutput: "typo is up to date according to Homebrew",
		},
		{
			name:       "available",
			output:     "typo\n",
			wantOutput: "Homebrew update available for typo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typoPath := "/opt/homebrew/Cellar/typo/1.1.0/bin/typo"
			calls := withUpdateTestHooks(t, typoPath)
			updateCommandOutput = func(name string, args []string, extraEnv []string) (string, error) {
				*calls = append(*calls, updateCommandCall{
					name:     name,
					args:     append([]string(nil), args...),
					extraEnv: append([]string(nil), extraEnv...),
				})
				return tt.output, nil
			}

			code, stdout, stderr := captureUpdateOutput(t, func() int {
				return cmdUpdate([]string{"--check"})
			})

			if code != 0 {
				t.Fatalf("cmdUpdate Homebrew check failed: code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			if len(*calls) != 1 || (*calls)[0].name != "brew" ||
				!reflect.DeepEqual((*calls)[0].args, []string{"outdated", "--quiet", "typo"}) {
				t.Fatalf("unexpected check calls: %#v", *calls)
			}
			if !strings.Contains(stdout, tt.wantOutput) {
				t.Fatalf("stdout missing %q: %q", tt.wantOutput, stdout)
			}
		})
	}
}

func TestCmdUpdateHomebrewReportsCheckAndUpgradeFailures(t *testing.T) {
	t.Run("check failure", func(t *testing.T) {
		typoPath := "/opt/homebrew/Cellar/typo/1.1.0/bin/typo"
		withUpdateTestHooks(t, typoPath)
		updateCommandOutput = func(name string, args []string, extraEnv []string) (string, error) {
			return "", errors.New("brew unavailable")
		}

		code, _, stderr := captureUpdateOutput(t, func() int {
			return cmdUpdate([]string{"--check"})
		})

		if code == 0 {
			t.Fatalf("expected Homebrew check failure")
		}
		if !strings.Contains(stderr, "failed to check Homebrew updates: brew unavailable") {
			t.Fatalf("missing check failure, got %q", stderr)
		}
	})

	t.Run("brew update failure", func(t *testing.T) {
		typoPath := "/opt/homebrew/Cellar/typo/1.1.0/bin/typo"
		withUpdateTestHooks(t, typoPath)
		updateRunCommand = func(name string, args []string, extraEnv []string) error {
			return errors.New("update failed")
		}

		code, _, stderr := captureUpdateOutput(t, func() int {
			return cmdUpdate(nil)
		})

		if code == 0 {
			t.Fatalf("expected brew update failure")
		}
		if !strings.Contains(stderr, "brew update failed: update failed") {
			t.Fatalf("missing brew update failure, got %q", stderr)
		}
	})

	t.Run("brew upgrade failure", func(t *testing.T) {
		typoPath := "/opt/homebrew/Cellar/typo/1.1.0/bin/typo"
		calls := withUpdateTestHooks(t, typoPath)
		updateRunCommand = func(name string, args []string, extraEnv []string) error {
			*calls = append(*calls, updateCommandCall{
				name: name,
				args: append([]string(nil), args...),
			})
			if len(*calls) == 2 {
				return errors.New("upgrade failed")
			}
			return nil
		}

		code, _, stderr := captureUpdateOutput(t, func() int {
			return cmdUpdate(nil)
		})

		if code == 0 {
			t.Fatalf("expected brew upgrade failure")
		}
		if !strings.Contains(stderr, "brew upgrade typo failed: upgrade failed") {
			t.Fatalf("missing brew upgrade failure, got %q", stderr)
		}
	})
}

func TestCmdUpdateHomebrewRejectsVersion(t *testing.T) {
	typoPath := "/opt/homebrew/Cellar/typo/1.1.0/bin/typo"
	calls := withUpdateTestHooks(t, typoPath)

	code, _, stderr := captureUpdateOutput(t, func() int {
		return cmdUpdate([]string{"--version", "1.0.0"})
	})

	if code == 0 {
		t.Fatalf("expected Homebrew --version to fail")
	}
	if len(*calls) != 0 {
		t.Fatalf("Homebrew --version should not run commands, got %#v", *calls)
	}
	if !strings.Contains(stderr, "homebrew updates do not support --version") {
		t.Fatalf("missing Homebrew version error, got %q", stderr)
	}
}

func TestCmdUpdateRejectsGoInstallTarget(t *testing.T) {
	tmpHome := t.TempDir()
	typoPath := filepath.Join(tmpHome, "go", "bin", "typo")
	calls := withUpdateTestHooks(t, typoPath)
	t.Setenv("GOPATH", filepath.Join(tmpHome, "go"))

	code, _, stderr := captureUpdateOutput(t, func() int {
		return cmdUpdate(nil)
	})

	if code == 0 {
		t.Fatalf("expected go install target to be rejected")
	}
	if len(*calls) != 0 {
		t.Fatalf("go install target should not run commands, got %#v", *calls)
	}
	if !strings.Contains(stderr, "go install github.com/yuluo-yx/typo/cmd/typo@latest") {
		t.Fatalf("missing go install guidance, got %q", stderr)
	}
}

func TestCmdUpdateCheckReportsGoInstallUnsupportedWithoutError(t *testing.T) {
	tmpHome := t.TempDir()
	typoPath := filepath.Join(tmpHome, "go", "bin", "typo")
	calls := withUpdateTestHooks(t, typoPath)
	t.Setenv("GOPATH", filepath.Join(tmpHome, "go"))

	code, stdout, stderr := captureUpdateOutput(t, func() int {
		return cmdUpdate([]string{"--check"})
	})

	if code != 0 {
		t.Fatalf("check should not fail for unsupported go install target: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if len(*calls) != 0 {
		t.Fatalf("check should not run commands, got %#v", *calls)
	}
	if stderr != "" {
		t.Fatalf("check should not print an error, got stderr=%q", stderr)
	}
	for _, want := range []string{
		"Install method: go install",
		"Update supported: no",
		"Current version: 1.0.0",
		"Latest Release: v1.1.0",
		"Release status: update available (1.0.0 -> v1.1.0)",
		"Latest main commit: abcdef1",
		"Main status: current commit unavailable",
		"go install @latest installs the latest Release",
		"go install github.com/yuluo-yx/typo/cmd/typo@latest",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout missing %q: %q", want, stdout)
		}
	}
}

func TestCmdUpdateCheckReportsUnsupportedInstallWithoutAction(t *testing.T) {
	withUpdateTestHooks(t, filepath.Join(t.TempDir(), ".local", "bin", "typo"))

	stdout := captureStdout(t, func() {
		printUnsupportedUpdateCheck(doctorInstallMethod{})
	})

	if !strings.Contains(stdout, "Update supported: no") ||
		!strings.Contains(stdout, "Run typo doctor for install method details.") {
		t.Fatalf("missing unsupported check output: %q", stdout)
	}
}

func TestPrintUpstreamCheckStatusHandlesUnavailableRemote(t *testing.T) {
	typoPath := filepath.Join(t.TempDir(), ".local", "bin", "typo")
	withUpdateTestHooks(t, typoPath)
	updateLatestRelease = func() (string, error) {
		return "", errors.New("release API down")
	}
	updateMainCommit = func() (string, error) {
		return "", errors.New("commit API down")
	}

	code, stdout, stderr := captureUpdateOutput(t, func() int {
		return cmdUpdate([]string{"--check"})
	})

	if code != 0 {
		t.Fatalf("check should not fail when upstream status is unavailable: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	for _, want := range []string{
		"Current version: 1.0.0",
		"Latest Release: unavailable (release API down)",
		"Latest main commit: unavailable (commit API down)",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout missing %q: %q", want, stdout)
		}
	}
}

func TestPrintUpstreamCheckStatusPrintsCommitAndBuildDate(t *testing.T) {
	typoPath := filepath.Join(t.TempDir(), ".local", "bin", "typo")
	withUpdateTestHooks(t, typoPath)
	commit = "abcdef123"
	date = "2026-05-07"

	stdout := captureStdout(t, printUpstreamCheckStatus)
	for _, want := range []string{
		"Current commit: abcdef123",
		"Current build date: 2026-05-07",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout missing %q: %q", want, stdout)
		}
	}
}

func TestCmdUpdateScriptInstallReleaseCheckComparesVersions(t *testing.T) {
	tests := []struct {
		name       string
		current    string
		args       []string
		wantOutput string
	}{
		{
			name:       "same version",
			current:    "1.1.0",
			args:       []string{"--check", "--version", "1.1.0"},
			wantOutput: "typo v1.1.0 is already installed",
		},
		{
			name:       "same version with force",
			current:    "1.1.0",
			args:       []string{"--check", "--force", "--version", "1.1.0"},
			wantOutput: "typo v1.1.0 is already installed; --force would reinstall it",
		},
		{
			name:       "older installed version",
			current:    "1.0.0",
			args:       []string{"--check", "--version", "1.1.0"},
			wantOutput: "Update available: 1.0.0 -> v1.1.0",
		},
		{
			name:       "newer installed version",
			current:    "1.2.0",
			args:       []string{"--check", "--version", "1.1.0"},
			wantOutput: "Installed version 1.2.0 is newer than v1.1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typoPath := filepath.Join(t.TempDir(), ".local", "bin", "typo")
			calls := withUpdateTestHooks(t, typoPath)
			version = tt.current

			code, stdout, stderr := captureUpdateOutput(t, func() int {
				return cmdUpdate(tt.args)
			})

			if code != 0 {
				t.Fatalf("cmdUpdate check failed: code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			if len(*calls) != 0 {
				t.Fatalf("check should not run commands, got %#v", *calls)
			}
			if !strings.Contains(stdout, tt.wantOutput) {
				t.Fatalf("stdout missing %q: %q", tt.wantOutput, stdout)
			}
		})
	}
}

func TestCmdUpdateScriptInstallReleaseNoopStopsBeforePrerequisites(t *testing.T) {
	typoPath := filepath.Join(t.TempDir(), ".local", "bin", "typo")
	calls := withUpdateTestHooks(t, typoPath)
	version = "1.1.0"
	lookPath = func(file string) (string, error) {
		return "", os.ErrNotExist
	}

	code, stdout, stderr := captureUpdateOutput(t, func() int {
		return cmdUpdate([]string{"--version", "1.1.0"})
	})

	if code != 0 {
		t.Fatalf("same release should stop without error: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if len(*calls) != 0 {
		t.Fatalf("release noop should not run commands, got %#v", *calls)
	}
	if !strings.Contains(stdout, "typo v1.1.0 is already installed") {
		t.Fatalf("missing noop output: %q", stdout)
	}
}

func TestCmdUpdateScriptInstallPrerequisiteFailures(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		missing    string
		wantOutput string
	}{
		{
			name:       "missing bash",
			args:       nil,
			missing:    "bash",
			wantOutput: "bash is required to run install.sh",
		},
		{
			name:       "missing curl",
			args:       []string{"--version", "1.1.0"},
			missing:    "curl",
			wantOutput: "curl is required to download install.sh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typoPath := filepath.Join(t.TempDir(), ".local", "bin", "typo")
			withUpdateTestHooks(t, typoPath)
			lookPath = func(file string) (string, error) {
				if file == tt.missing {
					return "", os.ErrNotExist
				}
				return "/usr/bin/" + file, nil
			}

			code, _, stderr := captureUpdateOutput(t, func() int {
				return cmdUpdate(tt.args)
			})

			if code == 0 {
				t.Fatalf("expected missing prerequisite to fail")
			}
			if !strings.Contains(stderr, tt.wantOutput) {
				t.Fatalf("stderr missing %q: %q", tt.wantOutput, stderr)
			}
		})
	}
}

func TestScriptInstallTargetDirRejectsBareCommand(t *testing.T) {
	if _, err := scriptInstallTargetDir("typo"); err == nil {
		t.Fatalf("expected bare command path to be rejected")
	}
}

func TestUpdateScriptInstallRejectsBareInstallPath(t *testing.T) {
	err := updateScriptInstall(updateFlags{}, doctorInstallMethod{path: "typo"})
	if err == nil || !strings.Contains(err.Error(), "cannot determine install directory") {
		t.Fatalf("expected bare install path error, got %v", err)
	}
}

func TestUnsupportedUpdateErrorVariants(t *testing.T) {
	tests := []struct {
		name    string
		install doctorInstallMethod
		want    string
	}{
		{
			name: "windows quick",
			install: doctorInstallMethod{
				kind:   doctorInstallWindowsQuick,
				action: "iwr install.ps1",
			},
			want: "does not support Windows quick install yet",
		},
		{
			name: "manual release",
			install: doctorInstallMethod{
				kind:   doctorInstallManual,
				detail: "/usr/local/bin/typo",
			},
			want: "does not replace manual Release binaries",
		},
		{
			name:    "unknown",
			install: doctorInstallMethod{},
			want:    "cannot determine a supported install method",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := unsupportedUpdateError(tt.install)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestPrintUpdateComparisons(t *testing.T) {
	tests := []struct {
		name string
		run  func()
		want string
	}{
		{
			name: "release up to date",
			run: func() {
				printReleaseComparison("1.1.0", "v1.1.0")
			},
			want: "Release status: up to date",
		},
		{
			name: "release newer installed",
			run: func() {
				printReleaseComparison("1.2.0", "v1.1.0")
			},
			want: "Release status: installed version 1.2.0 is newer than v1.1.0",
		},
		{
			name: "main matches",
			run: func() {
				printMainCommitComparison("abcdef1", "abcdef123456")
			},
			want: "Main status: current commit matches main",
		},
		{
			name: "main differs",
			run: func() {
				printMainCommitComparison("1111111", "abcdef1")
			},
			want: "Main status: current commit differs from main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := captureStdout(t, tt.run)
			if !strings.Contains(stdout, tt.want) {
				t.Fatalf("stdout missing %q: %q", tt.want, stdout)
			}
		})
	}
}

func TestCmdUpdateReportsCommandFailure(t *testing.T) {
	typoPath := filepath.Join(t.TempDir(), ".local", "bin", "typo")
	withUpdateTestHooks(t, typoPath)
	updateRunCommand = func(name string, args []string, extraEnv []string) error {
		return errors.New("boom")
	}

	code, _, stderr := captureUpdateOutput(t, func() int {
		return cmdUpdate(nil)
	})

	if code == 0 {
		t.Fatalf("expected command failure")
	}
	if !strings.Contains(stderr, "install script failed: boom") {
		t.Fatalf("missing command failure, got %q", stderr)
	}
}

func TestDownloadUpdateFileReportsCurlFailure(t *testing.T) {
	origRunCommand := updateRunCommand
	defer func() { updateRunCommand = origRunCommand }()

	updateRunCommand = func(name string, args []string, extraEnv []string) error {
		return errors.New("curl failed")
	}

	err := downloadUpdateFile(installScriptURL, filepath.Join(t.TempDir(), "install.sh"))
	if err == nil || !strings.Contains(err.Error(), "curl download failed: curl failed") {
		t.Fatalf("expected curl failure, got %v", err)
	}
}

func TestDownloadUpdateFileUsesCurl(t *testing.T) {
	origRunCommand := updateRunCommand
	defer func() { updateRunCommand = origRunCommand }()

	dst := filepath.Join(t.TempDir(), "install.sh")
	var got updateCommandCall
	updateRunCommand = func(name string, args []string, extraEnv []string) error {
		got = updateCommandCall{
			name:     name,
			args:     append([]string(nil), args...),
			extraEnv: append([]string(nil), extraEnv...),
		}
		return os.WriteFile(dst, []byte("install script"), 0755)
	}

	if err := downloadUpdateFile(installScriptURL, dst); err != nil {
		t.Fatalf("downloadUpdateFile failed: %v", err)
	}
	if got.name != "curl" {
		t.Fatalf("download command = %q, want curl", got.name)
	}
	wantArgs := []string{"-fsSL", "--retry", "1", "--retry-delay", "2", "-o", dst, installScriptURL}
	if !reflect.DeepEqual(got.args, wantArgs) {
		t.Fatalf("curl args = %#v, want %#v", got.args, wantArgs)
	}
}

func TestNormalizeVersionTagEmpty(t *testing.T) {
	if got := normalizeVersionTag(" "); got != "" {
		t.Fatalf("normalizeVersionTag empty = %q", got)
	}
}

func TestFetchUpdateJSONUsesCurlAndToken(t *testing.T) {
	origCommandOutput := updateCommandOutput
	defer func() { updateCommandOutput = origCommandOutput }()

	t.Setenv("GITHUB_TOKEN", "secret-token")
	var got updateCommandCall
	updateCommandOutput = func(name string, args []string, extraEnv []string) (string, error) {
		got = updateCommandCall{
			name:     name,
			args:     append([]string(nil), args...),
			extraEnv: append([]string(nil), extraEnv...),
		}
		return `{"tag_name":"v1.2.3"}`, nil
	}

	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := fetchUpdateJSON("https://example.test/releases/latest", &payload); err != nil {
		t.Fatalf("fetchUpdateJSON failed: %v", err)
	}
	if payload.TagName != "v1.2.3" {
		t.Fatalf("tag_name = %q", payload.TagName)
	}
	if got.name != "curl" || !reflect.DeepEqual(got.extraEnv, []string(nil)) {
		t.Fatalf("unexpected curl call metadata: %#v", got)
	}
	if !reflect.DeepEqual(got.args, []string{
		"-fsSL", "--retry", "1", "--retry-delay", "2",
		"-H", "Accept: application/vnd.github+json",
		"-H", "Authorization: Bearer secret-token",
		"https://example.test/releases/latest",
	}) {
		t.Fatalf("curl args = %#v", got.args)
	}
}

func TestFetchUpdateJSONReportsCommandAndJSONErrors(t *testing.T) {
	origCommandOutput := updateCommandOutput
	defer func() { updateCommandOutput = origCommandOutput }()

	t.Run("command error", func(t *testing.T) {
		updateCommandOutput = func(name string, args []string, extraEnv []string) (string, error) {
			return "", errors.New("network down")
		}
		err := fetchUpdateJSON("https://example.test", &struct{}{})
		if err == nil || !strings.Contains(err.Error(), "curl request failed: network down") {
			t.Fatalf("expected wrapped curl error, got %v", err)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		updateCommandOutput = func(name string, args []string, extraEnv []string) (string, error) {
			return "{", nil
		}
		err := fetchUpdateJSON("https://example.test", &struct{}{})
		if err == nil || !strings.Contains(err.Error(), "unexpected end of JSON input") {
			t.Fatalf("expected JSON error, got %v", err)
		}
	})
}

func TestFetchLatestReleaseTagAndMainCommit(t *testing.T) {
	origCommandOutput := updateCommandOutput
	defer func() { updateCommandOutput = origCommandOutput }()

	responses := map[string]string{
		latestReleaseAPIURL: `{"tag_name":"v2.0.0"}`,
		mainCommitAPIURL:    `{"sha":"abcdef1234567890"}`,
	}
	updateCommandOutput = func(name string, args []string, extraEnv []string) (string, error) {
		url := args[len(args)-1]
		return responses[url], nil
	}

	tag, err := fetchLatestReleaseTag()
	if err != nil || tag != "v2.0.0" {
		t.Fatalf("fetchLatestReleaseTag = %q, %v", tag, err)
	}
	sha, err := fetchMainCommit()
	if err != nil || sha != "abcdef1234567890" {
		t.Fatalf("fetchMainCommit = %q, %v", sha, err)
	}
}

func TestFetchLatestReleaseTagAndMainCommitRejectEmptyFields(t *testing.T) {
	origCommandOutput := updateCommandOutput
	defer func() { updateCommandOutput = origCommandOutput }()

	updateCommandOutput = func(name string, args []string, extraEnv []string) (string, error) {
		return `{}`, nil
	}

	if _, err := fetchLatestReleaseTag(); err == nil || !strings.Contains(err.Error(), "empty tag_name") {
		t.Fatalf("expected empty tag_name error, got %v", err)
	}
	if _, err := fetchMainCommit(); err == nil || !strings.Contains(err.Error(), "empty sha") {
		t.Fatalf("expected empty sha error, got %v", err)
	}
}

func TestRunUpdateCommandRejectsUnexpectedCommand(t *testing.T) {
	if err := runUpdateCommand("git", nil, nil); err == nil {
		t.Fatalf("expected runUpdateCommand to reject unexpected command")
	}
	if _, err := runUpdateCommandOutput("git", nil, nil); err == nil {
		t.Fatalf("expected runUpdateCommandOutput to reject unexpected command")
	}
}

func TestRunUpdateCommandExecutesWhitelistedBash(t *testing.T) {
	if _, err := newUpdateCommand("bash", []string{"-c", "exit 0"}); err != nil {
		t.Skipf("bash command unavailable: %v", err)
	}

	if err := runUpdateCommand("bash", []string{"-c", "exit 0"}, []string{"TYPO_TEST_VALUE=ok"}); err != nil {
		t.Fatalf("runUpdateCommand bash failed: %v", err)
	}
	output, err := runUpdateCommandOutput("bash", []string{"-c", "printf %s \"$TYPO_TEST_VALUE\""}, []string{"TYPO_TEST_VALUE=ok"})
	if err != nil {
		t.Fatalf("runUpdateCommandOutput bash failed: %v", err)
	}
	if output != "ok" {
		t.Fatalf("runUpdateCommandOutput = %q, want ok", output)
	}
}

func TestNewUpdateCommandAllowsWhitelistedCommands(t *testing.T) {
	for _, name := range []string{"bash", "brew", "curl"} {
		t.Run(name, func(t *testing.T) {
			cmd, err := newUpdateCommand(name, []string{"--version"})
			if err != nil {
				t.Fatalf("newUpdateCommand(%q) failed: %v", name, err)
			}
			if filepath.Base(cmd.Path) != name {
				t.Fatalf("command path = %q, want basename %q", cmd.Path, name)
			}
		})
	}
}

func TestNewUpdateCommandRejectsUnexpectedCommand(t *testing.T) {
	if _, err := newUpdateCommand("sh", nil); err == nil {
		t.Fatalf("expected unexpected update command to be rejected")
	}
}
