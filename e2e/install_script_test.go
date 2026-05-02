package e2e

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

type installScriptEnv struct {
	root       string
	home       string
	tmpDir     string
	binDir     string
	installDir string
	logFile    string
}

func newInstallScriptEnv(t *testing.T) *installScriptEnv {
	t.Helper()

	root := repoRoot(t)
	base := t.TempDir()
	home := filepath.Join(base, "home")
	tmpDir := filepath.Join(base, "tmp")
	binDir := filepath.Join(base, "bin")
	installDir := filepath.Join(base, "install")
	logFile := filepath.Join(base, "curl.log")

	for _, dir := range []string{home, tmpDir, binDir, installDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create install test directory: %v", err)
		}
	}

	return &installScriptEnv{
		root:       root,
		home:       home,
		tmpDir:     tmpDir,
		binDir:     binDir,
		installDir: installDir,
		logFile:    logFile,
	}
}

func (e *installScriptEnv) writeBinScript(t *testing.T, name, script string) {
	t.Helper()

	path := filepath.Join(e.binDir, name)
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("failed to write install helper %s: %v", name, err)
	}
}

func (e *installScriptEnv) commandEnv(extra ...string) []string {
	filtered := filteredCommandEnv([]string{
		"HOME=",
		"USERPROFILE=",
		"HOMEDRIVE=",
		"HOMEPATH=",
		"PATH=",
		"TMP=",
		"TEMP=",
		"TMPDIR=",
		"TYPO_INSTALL_DIR=",
		"TYPO_TEST_CURL_LOG=",
		"TYPO_TEST_RELEASE_BINARY=",
		"TYPO_TEST_RELEASE_SHA256=",
		"TYPO_TEST_SOURCE_ARCHIVE=",
	}, len(extra)+10)

	filtered = append(filtered,
		"HOME="+e.home,
		"USERPROFILE="+e.home,
		"PATH="+e.binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"TMP="+e.tmpDir,
		"TEMP="+e.tmpDir,
		"TMPDIR="+e.tmpDir,
		"TYPO_INSTALL_DIR="+e.installDir,
		"TYPO_TEST_CURL_LOG="+e.logFile,
	)
	filtered = appendWindowsHomeEnv(filtered, e.home)
	filtered = append(filtered, extra...)
	return filtered
}

func (e *installScriptEnv) runWithEnv(t *testing.T, extraEnv []string, args ...string) e2eResult {
	t.Helper()

	scriptPath := filepath.Join(e.root, "tools", "scripts", "install.sh")
	cmd := exec.Command("bash", append([]string{scriptPath}, args...)...)
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
			t.Fatalf("failed to execute tools/scripts/install.sh: %v", err)
		}
	}

	return e2eResult{
		stdout: stdout.String(),
		stderr: stderr.String(),
		code:   code,
	}
}

func TestInstallScriptInstallsLatestRelease(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("install.sh e2e is only supported on Unix hosts")
	}

	env := newInstallScriptEnv(t)

	releaseBinary := filepath.Join(env.tmpDir, "release-typo")
	releaseBinaryContent := []byte("#!/bin/sh\necho installed-from-release\n")
	if err := os.WriteFile(releaseBinary, releaseBinaryContent, 0755); err != nil {
		t.Fatalf("failed to write fake release binary: %v", err)
	}

	env.writeBinScript(t, "uname", `#!/bin/sh
case "${1:-}" in
  -s) echo Linux ;;
  -m) echo x86_64 ;;
  *) echo Linux ;;
esac
`)
	env.writeBinScript(t, "curl", `#!/bin/sh
set -eu
url=""
output=""
expect_output=0
for arg in "$@"; do
  if [ "$expect_output" -eq 1 ]; then
    output="$arg"
    expect_output=0
    continue
  fi
  if [ "$arg" = "-o" ]; then
    expect_output=1
    continue
  fi
  case "$arg" in
    http://*|https://*) url="$arg" ;;
  esac
done
printf '%s\n' "$url" >> "$TYPO_TEST_CURL_LOG"
case "$url" in
  "https://api.github.com/repos/yuluo-yx/typo/releases?per_page=1")
    printf '[\n  {\n    "tag_name": "v9.9.9"\n  }\n]\n'
    ;;
  "https://github.com/yuluo-yx/typo/releases/download/v9.9.9/typo-linux-amd64")
    cp "$TYPO_TEST_RELEASE_BINARY" "$output"
    chmod 755 "$output"
    ;;
  "https://github.com/yuluo-yx/typo/releases/download/v9.9.9/checksums.txt")
    printf '%s  typo-linux-amd64\n' "$TYPO_TEST_RELEASE_SHA256" > "$output"
    ;;
  *)
    echo "unexpected curl URL: $url" >&2
    exit 1
    ;;
esac
`)

	result := env.runWithEnv(t, []string{
		"TYPO_TEST_RELEASE_BINARY=" + releaseBinary,
		"TYPO_TEST_RELEASE_SHA256=" + sha256Hex(releaseBinaryContent),
	})
	if result.code != 0 {
		t.Fatalf("install latest release failed: stdout=%q stderr=%q code=%d", result.stdout, result.stderr, result.code)
	}

	installedBinary := filepath.Join(env.installDir, "typo")
	output, err := exec.Command(installedBinary).CombinedOutput()
	if err != nil {
		t.Fatalf("installed release binary failed to execute: %v\n%s", err, output)
	}
	if strings.TrimSpace(string(output)) != "installed-from-release" {
		t.Fatalf("unexpected installed release binary output: %q", output)
	}

	logData, err := os.ReadFile(env.logFile)
	if err != nil {
		t.Fatalf("failed to read curl log: %v", err)
	}
	logText := string(logData)
	if !strings.Contains(logText, "https://api.github.com/repos/yuluo-yx/typo/releases?per_page=1") {
		t.Fatalf("latest release lookup was not requested: %s", logText)
	}
	if !strings.Contains(logText, "https://github.com/yuluo-yx/typo/releases/download/v9.9.9/typo-linux-amd64") {
		t.Fatalf("release binary download was not requested: %s", logText)
	}
	if !strings.Contains(logText, "https://github.com/yuluo-yx/typo/releases/download/v9.9.9/checksums.txt") {
		t.Fatalf("checksum manifest download was not requested: %s", logText)
	}
}

func TestInstallScriptContinuesWhenChecksumManifestIsUnavailable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("install.sh e2e is only supported on Unix hosts")
	}

	env := newInstallScriptEnv(t)
	releaseBinary := filepath.Join(env.tmpDir, "release-typo")
	if err := os.WriteFile(releaseBinary, []byte("#!/bin/sh\necho installed-without-checksum\n"), 0755); err != nil {
		t.Fatalf("failed to write fake release binary: %v", err)
	}

	env.writeBinScript(t, "uname", installTestUnameScript())
	env.writeBinScript(t, "curl", `#!/bin/sh
set -eu
url=""
output=""
expect_output=0
for arg in "$@"; do
  if [ "$expect_output" -eq 1 ]; then
    output="$arg"
    expect_output=0
    continue
  fi
  if [ "$arg" = "-o" ]; then
    expect_output=1
    continue
  fi
  case "$arg" in
    http://*|https://*) url="$arg" ;;
  esac
done
case "$url" in
  "https://github.com/yuluo-yx/typo/releases/download/v1.2.3/typo-linux-amd64")
    cp "$TYPO_TEST_RELEASE_BINARY" "$output"
    chmod 755 "$output"
    ;;
  "https://github.com/yuluo-yx/typo/releases/download/v1.2.3/checksums.txt")
    printf '404'
    exit 22
    ;;
  *)
    echo "unexpected curl URL: $url" >&2
    exit 1
    ;;
esac
`)

	result := env.runWithEnv(t, []string{"TYPO_TEST_RELEASE_BINARY=" + releaseBinary}, "-s", "1.2.3")
	if result.code != 0 {
		t.Fatalf("install should continue without checksums: stdout=%q stderr=%q code=%d", result.stdout, result.stderr, result.code)
	}
	if !strings.Contains(result.stderr, "Checksum verification will be skipped") {
		t.Fatalf("expected checksum warning, got stdout=%q stderr=%q", result.stdout, result.stderr)
	}

	installedBinary := filepath.Join(env.installDir, "typo")
	output, err := exec.Command(installedBinary).CombinedOutput()
	if err != nil {
		t.Fatalf("installed release binary failed to execute: %v\n%s", err, output)
	}
	if strings.TrimSpace(string(output)) != "installed-without-checksum" {
		t.Fatalf("unexpected installed release binary output: %q", output)
	}
}

func TestInstallScriptFailsWhenChecksumManifestDownloadErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("install.sh e2e is only supported on Unix hosts")
	}

	env := newInstallScriptEnv(t)
	releaseBinary := filepath.Join(env.tmpDir, "release-typo")
	if err := os.WriteFile(releaseBinary, []byte("#!/bin/sh\necho unverified\n"), 0755); err != nil {
		t.Fatalf("failed to write fake release binary: %v", err)
	}

	env.writeBinScript(t, "uname", installTestUnameScript())
	env.writeBinScript(t, "curl", `#!/bin/sh
set -eu
url=""
output=""
expect_output=0
for arg in "$@"; do
  if [ "$expect_output" -eq 1 ]; then
    output="$arg"
    expect_output=0
    continue
  fi
  if [ "$arg" = "-o" ]; then
    expect_output=1
    continue
  fi
  case "$arg" in
    http://*|https://*) url="$arg" ;;
  esac
done
case "$url" in
  "https://github.com/yuluo-yx/typo/releases/download/v1.2.3/typo-linux-amd64")
    cp "$TYPO_TEST_RELEASE_BINARY" "$output"
    chmod 755 "$output"
    ;;
  "https://github.com/yuluo-yx/typo/releases/download/v1.2.3/checksums.txt")
    printf '500'
    exit 22
    ;;
  *)
    echo "unexpected curl URL: $url" >&2
    exit 1
    ;;
esac
`)

	result := env.runWithEnv(t, []string{"TYPO_TEST_RELEASE_BINARY=" + releaseBinary}, "-s", "1.2.3")
	if result.code == 0 {
		t.Fatalf("expected checksum manifest download error to fail: stdout=%q stderr=%q", result.stdout, result.stderr)
	}
	if !strings.Contains(result.stderr, "Refusing to install an unverified binary") {
		t.Fatalf("expected unverified binary refusal, got stdout=%q stderr=%q", result.stdout, result.stderr)
	}
	if _, err := os.Stat(filepath.Join(env.installDir, "typo")); !os.IsNotExist(err) {
		t.Fatalf("binary should not be installed when checksum manifest download fails: %v", err)
	}
}

func TestInstallScriptFailsOnChecksumMismatch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("install.sh e2e is only supported on Unix hosts")
	}

	env := newInstallScriptEnv(t)
	releaseBinary := filepath.Join(env.tmpDir, "release-typo")
	if err := os.WriteFile(releaseBinary, []byte("#!/bin/sh\necho tampered\n"), 0755); err != nil {
		t.Fatalf("failed to write fake release binary: %v", err)
	}

	env.writeBinScript(t, "uname", installTestUnameScript())
	env.writeBinScript(t, "curl", installTestReleaseCurlScript("0000000000000000000000000000000000000000000000000000000000000000  typo-linux-amd64\n"))

	result := env.runWithEnv(t, []string{"TYPO_TEST_RELEASE_BINARY=" + releaseBinary}, "-s", "1.2.3")
	if result.code == 0 {
		t.Fatalf("expected checksum mismatch to fail: stdout=%q stderr=%q", result.stdout, result.stderr)
	}
	if !strings.Contains(result.stderr, "Checksum verification failed") {
		t.Fatalf("expected checksum failure in stderr, got stdout=%q stderr=%q", result.stdout, result.stderr)
	}
	if _, err := os.Stat(filepath.Join(env.installDir, "typo")); !os.IsNotExist(err) {
		t.Fatalf("binary should not be installed on checksum mismatch: %v", err)
	}
}

func TestInstallScriptFailsWhenChecksumEntryIsMissing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("install.sh e2e is only supported on Unix hosts")
	}

	env := newInstallScriptEnv(t)
	releaseBinary := filepath.Join(env.tmpDir, "release-typo")
	if err := os.WriteFile(releaseBinary, []byte("#!/bin/sh\necho missing-entry\n"), 0755); err != nil {
		t.Fatalf("failed to write fake release binary: %v", err)
	}

	env.writeBinScript(t, "uname", installTestUnameScript())
	env.writeBinScript(t, "curl", installTestReleaseCurlScript("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff  typo-darwin-arm64\n"))

	result := env.runWithEnv(t, []string{"TYPO_TEST_RELEASE_BINARY=" + releaseBinary}, "-s", "1.2.3")
	if result.code == 0 {
		t.Fatalf("expected missing checksum entry to fail: stdout=%q stderr=%q", result.stdout, result.stderr)
	}
	if !strings.Contains(result.stderr, "Unable to find checksum entry for typo-linux-amd64") {
		t.Fatalf("expected missing checksum entry error, got stdout=%q stderr=%q", result.stdout, result.stderr)
	}
	if _, err := os.Stat(filepath.Join(env.installDir, "typo")); !os.IsNotExist(err) {
		t.Fatalf("binary should not be installed without checksum entry: %v", err)
	}
}

func TestInstallScriptBuildsFromSource(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("install.sh e2e is only supported on Unix hosts")
	}

	env := newInstallScriptEnv(t)

	sourceArchive := filepath.Join(env.tmpDir, "typo-main.tar.gz")
	writeTarGz(t, sourceArchive, map[string]string{
		"typo-main/go.mod":           "module example.com/typo-fixture\n\ngo 1.25.0\n",
		"typo-main/cmd/typo/main.go": "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"built-from-source\")\n}\n",
	})

	env.writeBinScript(t, "uname", `#!/bin/sh
case "${1:-}" in
  -s) echo Linux ;;
  -m) echo x86_64 ;;
  *) echo Linux ;;
esac
`)
	env.writeBinScript(t, "curl", `#!/bin/sh
set -eu
url=""
output=""
expect_output=0
for arg in "$@"; do
  if [ "$expect_output" -eq 1 ]; then
    output="$arg"
    expect_output=0
    continue
  fi
  if [ "$arg" = "-o" ]; then
    expect_output=1
    continue
  fi
  case "$arg" in
    http://*|https://*) url="$arg" ;;
  esac
done
printf '%s\n' "$url" >> "$TYPO_TEST_CURL_LOG"
case "$url" in
  "https://github.com/yuluo-yx/typo/archive/refs/heads/main.tar.gz")
    cp "$TYPO_TEST_SOURCE_ARCHIVE" "$output"
    ;;
  *)
    echo "unexpected curl URL: $url" >&2
    exit 1
    ;;
esac
`)

	result := env.runWithEnv(t, []string{"TYPO_TEST_SOURCE_ARCHIVE=" + sourceArchive}, "-b")
	if result.code != 0 {
		t.Fatalf("build from source failed: stdout=%q stderr=%q code=%d", result.stdout, result.stderr, result.code)
	}
	if !strings.Contains(result.stdout, "Building typo from the main branch") {
		t.Fatalf("missing build-from-source output: stdout=%q stderr=%q", result.stdout, result.stderr)
	}

	installedBinary := filepath.Join(env.installDir, "typo")
	output, err := exec.Command(installedBinary).CombinedOutput()
	if err != nil {
		t.Fatalf("installed source binary failed to execute: %v\n%s", err, output)
	}
	if strings.TrimSpace(string(output)) != "built-from-source" {
		t.Fatalf("unexpected installed source binary output: %q", output)
	}

	logData, err := os.ReadFile(env.logFile)
	if err != nil {
		t.Fatalf("failed to read curl log: %v", err)
	}
	if !strings.Contains(string(logData), "https://github.com/yuluo-yx/typo/archive/refs/heads/main.tar.gz") {
		t.Fatalf("source archive download was not requested: %s", logData)
	}
}

func writeTarGz(t *testing.T, archivePath string, files map[string]string) {
	t.Helper()

	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("failed to create tar.gz: %v", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Fatalf("failed to close tar.gz file: %v", closeErr)
		}
	}()

	gzipWriter := gzip.NewWriter(file)
	defer func() {
		if closeErr := gzipWriter.Close(); closeErr != nil {
			t.Fatalf("failed to close gzip writer: %v", closeErr)
		}
	}()

	tarWriter := tar.NewWriter(gzipWriter)
	defer func() {
		if closeErr := tarWriter.Close(); closeErr != nil {
			t.Fatalf("failed to close tar writer: %v", closeErr)
		}
	}()

	createdDirs := make(map[string]bool)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		dir := path.Dir(name)
		if dir != "." {
			writeTarDir(t, tarWriter, createdDirs, dir)
		}

		content := []byte(files[name])
		header := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("failed to write tar header for %s: %v", name, err)
		}
		if _, err := tarWriter.Write(content); err != nil {
			t.Fatalf("failed to write tar content for %s: %v", name, err)
		}
	}
}

func writeTarDir(t *testing.T, tarWriter *tar.Writer, createdDirs map[string]bool, dir string) {
	t.Helper()

	if dir == "." || dir == "" || createdDirs[dir] {
		return
	}

	parent := path.Dir(dir)
	if parent != "." && parent != dir {
		writeTarDir(t, tarWriter, createdDirs, parent)
	}

	header := &tar.Header{
		Name:     dir + "/",
		Typeflag: tar.TypeDir,
		Mode:     0755,
	}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatalf("failed to write tar directory %s: %v", dir, err)
	}
	createdDirs[dir] = true
}

func sha256Hex(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func installTestUnameScript() string {
	return `#!/bin/sh
case "${1:-}" in
  -s) echo Linux ;;
  -m) echo x86_64 ;;
  *) echo Linux ;;
esac
`
}

func installTestReleaseCurlScript(checksums string) string {
	return fmt.Sprintf(`#!/bin/sh
set -eu
url=""
output=""
expect_output=0
for arg in "$@"; do
  if [ "$expect_output" -eq 1 ]; then
    output="$arg"
    expect_output=0
    continue
  fi
  if [ "$arg" = "-o" ]; then
    expect_output=1
    continue
  fi
  case "$arg" in
    http://*|https://*) url="$arg" ;;
  esac
done
case "$url" in
  "https://github.com/yuluo-yx/typo/releases/download/v1.2.3/typo-linux-amd64")
    cp "$TYPO_TEST_RELEASE_BINARY" "$output"
    chmod 755 "$output"
    ;;
  "https://github.com/yuluo-yx/typo/releases/download/v1.2.3/checksums.txt")
    cat > "$output" <<'EOF'
%sEOF
    ;;
  *)
    echo "unexpected curl URL: $url" >&2
    exit 1
    ;;
esac
`, checksums)
}
