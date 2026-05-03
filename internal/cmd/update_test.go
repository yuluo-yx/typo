package cmd

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"strings"
	"testing"
	"time"
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
	origSleep := updateSleep
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
	updateSleep = func(d time.Duration) {}

	t.Cleanup(func() {
		lookPath = origLookPath
		executable = origExecutable
		userHomeDir = origUserHomeDir
		updateDownloadFile = origDownloadFile
		updateLatestRelease = origLatestRelease
		updateMainCommit = origMainCommit
		updateRunCommand = origRunCommand
		updateCommandOutput = origCommandOutput
		updateSleep = origSleep
		version = origVersion
		commit = origCommit
		date = origDate
		readBuildInfo = origReadBuildInfo
	})

	return &calls
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

func TestDownloadUpdateFileRetriesRetryableHTTPStatuses(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		wantSleep time.Duration
	}{
		{name: "rate limited", status: http.StatusTooManyRequests, wantSleep: 20 * time.Second},
		{name: "server error", status: http.StatusBadGateway, wantSleep: 2 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origDownload := updateHTTPDownload
			origSleep := updateSleep
			defer func() {
				updateHTTPDownload = origDownload
				updateSleep = origSleep
			}()

			calls := 0
			sleeps := make([]time.Duration, 0)
			updateHTTPDownload = func(req *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return &http.Response{
						StatusCode: tt.status,
						Body:       io.NopCloser(strings.NewReader("")),
					}, nil
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("install script")),
				}, nil
			}
			updateSleep = func(d time.Duration) {
				sleeps = append(sleeps, d)
			}

			dst := filepath.Join(t.TempDir(), "install.sh")
			if err := downloadUpdateFile(installScriptURL, dst); err != nil {
				t.Fatalf("downloadUpdateFile failed: %v", err)
			}
			if calls != 2 {
				t.Fatalf("calls = %d, want 2", calls)
			}
			if !reflect.DeepEqual(sleeps, []time.Duration{tt.wantSleep}) {
				t.Fatalf("sleeps = %#v, want %#v", sleeps, []time.Duration{tt.wantSleep})
			}
			data, err := os.ReadFile(dst)
			if err != nil {
				t.Fatalf("read downloaded file: %v", err)
			}
			if string(data) != "install script" {
				t.Fatalf("downloaded content = %q", string(data))
			}
		})
	}
}

func TestDownloadUpdateFileRetriesTimeout(t *testing.T) {
	origDownload := updateHTTPDownload
	origSleep := updateSleep
	defer func() {
		updateHTTPDownload = origDownload
		updateSleep = origSleep
	}()

	calls := 0
	sleeps := make([]time.Duration, 0)
	updateHTTPDownload = func(req *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return nil, timeoutTestError{}
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("install script")),
		}, nil
	}
	updateSleep = func(d time.Duration) {
		sleeps = append(sleeps, d)
	}

	dst := filepath.Join(t.TempDir(), "install.sh")
	if err := downloadUpdateFile(installScriptURL, dst); err != nil {
		t.Fatalf("downloadUpdateFile failed: %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
	if !reflect.DeepEqual(sleeps, []time.Duration{2 * time.Second}) {
		t.Fatalf("sleeps = %#v, want %#v", sleeps, []time.Duration{2 * time.Second})
	}
}

type timeoutTestError struct{}

func (timeoutTestError) Error() string   { return "timeout" }
func (timeoutTestError) Timeout() bool   { return true }
func (timeoutTestError) Temporary() bool { return true }
