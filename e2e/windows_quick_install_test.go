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

	var (
		mu       sync.Mutex
		requests []string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, r.URL.Path+"?"+r.URL.RawQuery)
		mu.Unlock()

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
	data, err := os.ReadFile(installedBinary)
	if err != nil {
		t.Fatalf("failed to read installed binary: %v", err)
	}
	if !bytes.Equal(data, binaryContent) {
		t.Fatalf("installed binary content mismatch: got=%q want=%q", data, binaryContent)
	}

	if !strings.Contains(result.stdout, "Installed typo to "+installedBinary) {
		t.Fatalf("expected install path in stdout, got: %q", result.stdout)
	}
	if !strings.Contains(result.stdout, "Invoke-Expression (& typo init powershell | Out-String)") {
		t.Fatalf("expected PowerShell init hint in stdout, got: %q", result.stdout)
	}
	if !strings.Contains(result.stdout, "typo doctor") {
		t.Fatalf("expected doctor hint in stdout, got: %q", result.stdout)
	}

	mu.Lock()
	defer mu.Unlock()
	if !containsRequest(requests, "/repos/yuluo-yx/typo/releases?per_page=1") {
		t.Fatalf("latest release lookup was not requested: %v", requests)
	}
	if !containsRequest(requests, "/releases/download/v9.9.9/typo-windows-amd64.exe?") {
		t.Fatalf("binary download was not requested: %v", requests)
	}
	if !containsRequest(requests, "/releases/download/v9.9.9/checksums.txt?") {
		t.Fatalf("checksums download was not requested: %v", requests)
	}
}

func TestWindowsQuickInstallFailsOnChecksumMismatch(t *testing.T) {
	env := newWindowsQuickInstallEnv(t)
	customInstallDir := filepath.Join(env.base, "custom-bin")

	var (
		mu       sync.Mutex
		requests []string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, r.URL.Path+"?"+r.URL.RawQuery)
		mu.Unlock()

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

	mu.Lock()
	defer mu.Unlock()
	if containsRequest(requests, "/repos/yuluo-yx/typo/releases?per_page=1") {
		t.Fatalf("latest release lookup should not be requested for explicit version: %v", requests)
	}
	if !containsRequest(requests, "/releases/download/v1.2.3/typo-windows-amd64.exe?") {
		t.Fatalf("binary download was not requested: %v", requests)
	}
	if !containsRequest(requests, "/releases/download/v1.2.3/checksums.txt?") {
		t.Fatalf("checksums download was not requested: %v", requests)
	}
}

func containsRequest(requests []string, target string) bool {
	for _, request := range requests {
		if request == target {
			return true
		}
	}

	return false
}
