package e2e

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

type windowsQuickInstallEnv struct {
	root         string
	base         string
	home         string
	localAppData string
	installDir   string
}

type recordedRequests struct {
	mu       sync.Mutex
	requests []string
}

func newWindowsQuickInstallEnv(t *testing.T) *windowsQuickInstallEnv {
	t.Helper()

	base := t.TempDir()
	home := filepath.Join(base, "home")
	localAppData := filepath.Join(base, "LocalAppData")
	installDir := filepath.Join(localAppData, "Programs", "typo", "bin")

	for _, dir := range []string{home, localAppData, installDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create test directory: %v", err)
		}
	}

	return &windowsQuickInstallEnv{
		root:         repoRoot(t),
		base:         base,
		home:         home,
		localAppData: localAppData,
		installDir:   installDir,
	}
}

func (e *windowsQuickInstallEnv) commandEnv(extra ...string) []string {
	filtered := filteredCommandEnv([]string{
		"HOME=",
		"USERPROFILE=",
		"HOMEDRIVE=",
		"HOMEPATH=",
		"LOCALAPPDATA=",
		"TYPO_INSTALL_",
	}, len(extra)+8)

	filtered = append(filtered,
		"HOME="+e.home,
		"USERPROFILE="+e.home,
		"LOCALAPPDATA="+e.localAppData,
	)
	filtered = appendWindowsHomeEnv(filtered, e.home)
	filtered = append(filtered, extra...)
	return filtered
}

func (e *windowsQuickInstallEnv) run(t *testing.T, extraEnv []string, args ...string) e2eResult {
	t.Helper()

	if runtime.GOOS != "windows" {
		t.Skip("quick-install.ps1 e2e is only supported on Windows hosts")
	}
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("pwsh is not available")
	}

	scriptPath := filepath.Join(e.root, "tools", "scripts", "quick-install.ps1")
	cmdArgs := append([]string{"-NoLogo", "-NoProfile", "-File", scriptPath}, args...)
	cmd := exec.Command("pwsh", cmdArgs...)
	cmd.Dir = e.root
	cmd.Env = e.commandEnv(extraEnv...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to execute quick-install.ps1: %v", err)
		}
	}

	return e2eResult{
		stdout: stdout.String(),
		stderr: stderr.String(),
		code:   code,
	}
}

func TestWindowsQuickInstallInstallsLatestRelease(t *testing.T) {
	env := newWindowsQuickInstallEnv(t)

	binaryContent := []byte("fake-typo-windows-binary")
	expectedHashBytes := sha256.Sum256(binaryContent)
	expectedHash := hex.EncodeToString(expectedHashBytes[:])

	recorder := &recordedRequests{}
	server := newLatestReleaseServer(recorder, binaryContent, expectedHash)
	defer server.Close()

	result := env.run(t, []string{
		"TYPO_INSTALL_GITHUB_API=" + server.URL + "/repos/yuluo-yx/typo",
		"TYPO_INSTALL_RELEASE_BASE_URL=" + server.URL + "/releases/download",
		"TYPO_INSTALL_SKIP_PATH_UPDATE=1",
	})
	if result.code != 0 {
		t.Fatalf("quick install failed: stdout=%q stderr=%q code=%d", result.stdout, result.stderr, result.code)
	}

	installedBinary := filepath.Join(env.installDir, "typo.exe")
	assertInstalledBinaryMatches(t, installedBinary, binaryContent)
	assertQuickInstallOutput(t, result.stdout, installedBinary)
	assertRequestedPaths(t, recorder.snapshot(),
		"/repos/yuluo-yx/typo/releases?per_page=1",
		"/releases/download/v9.9.9/typo-windows-amd64.exe?",
		"/releases/download/v9.9.9/checksums.txt?",
	)
}

func TestWindowsQuickInstallFailsOnChecksumMismatch(t *testing.T) {
	env := newWindowsQuickInstallEnv(t)
	customInstallDir := filepath.Join(env.base, "custom-bin")

	recorder := &recordedRequests{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder.add(r.URL.Path + "?" + r.URL.RawQuery)

		switch r.URL.Path {
		case "/releases/download/v1.2.3/typo-windows-amd64.exe":
			_, _ = w.Write([]byte("bad-binary"))
		case "/releases/download/v1.2.3/checksums.txt":
			_, _ = w.Write([]byte(strings.Repeat("0", 64) + "  typo-windows-amd64.exe\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	result := env.run(t, []string{
		"TYPO_INSTALL_RELEASE_BASE_URL=" + server.URL + "/releases/download",
		"TYPO_INSTALL_SKIP_PATH_UPDATE=1",
	}, "-Version", "1.2.3", "-InstallDir", customInstallDir)
	if result.code == 0 {
		t.Fatalf("expected checksum mismatch to fail: stdout=%q stderr=%q", result.stdout, result.stderr)
	}
	if !strings.Contains(result.stderr, "Checksum verification failed") {
		t.Fatalf("expected checksum failure in stderr, got: %q", result.stderr)
	}

	if _, err := os.Stat(filepath.Join(customInstallDir, "typo.exe")); !os.IsNotExist(err) {
		t.Fatalf("binary should not be installed on checksum mismatch: %v", err)
	}

	requests := recorder.snapshot()
	if containsRequest(requests, "/repos/yuluo-yx/typo/releases?per_page=1") {
		t.Fatalf("latest release lookup should not be requested for explicit version: %v", requests)
	}
	assertRequestedPaths(t, requests,
		"/releases/download/v1.2.3/typo-windows-amd64.exe?",
		"/releases/download/v1.2.3/checksums.txt?",
	)
}

func containsRequest(requests []string, target string) bool {
	for _, request := range requests {
		if request == target {
			return true
		}
	}

	return false
}

func firstLineWithPrefix(text, prefix string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}

	return ""
}

func equalFoldPath(left, right string) bool {
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}

func sameInstalledFile(left, right string) bool {
	leftInfo, leftErr := os.Stat(left)
	rightInfo, rightErr := os.Stat(right)
	if leftErr == nil && rightErr == nil {
		return os.SameFile(leftInfo, rightInfo)
	}

	return equalFoldPath(left, right)
}

func (r *recordedRequests) add(request string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = append(r.requests, request)
}

func (r *recordedRequests) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.requests...)
}

func newLatestReleaseServer(recorder *recordedRequests, binaryContent []byte, expectedHash string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder.add(r.URL.Path + "?" + r.URL.RawQuery)

		switch {
		case r.URL.Path == "/repos/yuluo-yx/typo/releases" && r.URL.RawQuery == "per_page=1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"tag_name":"v9.9.9"}]`))
		case r.URL.Path == "/releases/download/v9.9.9/typo-windows-amd64.exe":
			_, _ = w.Write(binaryContent)
		case r.URL.Path == "/releases/download/v9.9.9/checksums.txt":
			_, _ = w.Write([]byte(expectedHash + "  typo-windows-amd64.exe\n"))
		default:
			http.NotFound(w, r)
		}
	}))
}

func assertInstalledBinaryMatches(t *testing.T, installedBinary string, want []byte) {
	t.Helper()

	data, err := os.ReadFile(installedBinary)
	if err != nil {
		t.Fatalf("failed to read installed binary: %v", err)
	}
	if !bytes.Equal(data, want) {
		t.Fatalf("installed binary content mismatch: got=%q want=%q", data, want)
	}
}

func assertQuickInstallOutput(t *testing.T, stdout, installedBinary string) {
	t.Helper()

	installedLine := firstLineWithPrefix(stdout, "Installed typo to ")
	if installedLine == "" {
		t.Fatalf("expected install line in stdout, got: %q", stdout)
	}
	reportedPath := strings.TrimPrefix(installedLine, "Installed typo to ")
	if !sameInstalledFile(reportedPath, installedBinary) {
		t.Fatalf("unexpected installed path in stdout: got=%q want=%q", installedLine, installedBinary)
	}
	if !strings.Contains(stdout, "Invoke-Expression (& typo init powershell | Out-String)") {
		t.Fatalf("expected PowerShell init hint in stdout, got: %q", stdout)
	}
	if !strings.Contains(stdout, "typo doctor") {
		t.Fatalf("expected doctor hint in stdout, got: %q", stdout)
	}
	if !strings.Contains(stdout, "$PROFILE.CurrentUserCurrentHost") {
		t.Fatalf("expected profile hint in stdout, got: %q", stdout)
	}
}

func assertRequestedPaths(t *testing.T, requests []string, want ...string) {
	t.Helper()

	for _, target := range want {
		if !containsRequest(requests, target) {
			t.Fatalf("expected request %q, got: %v", target, requests)
		}
	}
}
