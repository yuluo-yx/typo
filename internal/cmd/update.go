package cmd

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const installScriptURL = "https://raw.githubusercontent.com/yuluo-yx/typo/main/tools/scripts/install.sh"

// Injectable for testing.
var (
	updateHTTPDownload  func(req *http.Request) (*http.Response, error)
	updateRunCommand    func(name string, args []string, extraEnv []string) error
	updateCommandOutput func(name string, args []string, extraEnv []string) (string, error)
	updateDownloadFile  func(url, dst string) error
	updateLatestRelease func() (string, error)
	updateMainCommit    func() (string, error)
	updateSleep         func(d time.Duration)
)

// errStop is a sentinel that signals the chain to stop without printing an error.
var errStop = errors.New("stop")

type updateFlags struct {
	checkOnly bool
	force     bool
	dryRun    bool
	targetVer string
}

type updateMode int

const (
	updateModeMain updateMode = iota
	updateModeRelease
)

func init() {
	downloadClient := &http.Client{Timeout: 10 * time.Minute}
	updateHTTPDownload = downloadClient.Do
	updateRunCommand = runUpdateCommand
	updateCommandOutput = runUpdateCommandOutput
	updateDownloadFile = downloadUpdateFile
	updateLatestRelease = fetchLatestReleaseTag
	updateMainCommit = fetchMainCommit
	updateSleep = time.Sleep
}

func cmdUpdate(args []string) int {
	flags, err := parseUpdateFlags(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if err := runUpdate(flags); err != nil {
		if errors.Is(err, errStop) {
			return 0
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

func parseUpdateFlags(args []string) (updateFlags, error) {
	var flags updateFlags
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.BoolVar(&flags.checkOnly, "check", false, "only check update support, do not install")
	fs.BoolVar(&flags.force, "force", false, "reinstall even if versions match")
	fs.BoolVar(&flags.dryRun, "dry-run", false, "simulate without making changes")
	fs.StringVar(&flags.targetVer, "version", "", "install a specific release version; latest builds from main")

	if err := fs.Parse(args); err != nil {
		return flags, err
	}
	return flags, nil
}

func runUpdate(flags updateFlags) error {
	if _, _, err := normalizeUpdateTarget(flags.targetVer); err != nil {
		return err
	}

	install, err := resolveRunningTypoInstall()
	if err != nil {
		return err
	}

	switch install.kind {
	case doctorInstallHomebrew:
		return updateHomebrew(flags, install)
	case doctorInstallScript:
		return updateScriptInstall(flags, install)
	default:
		if flags.checkOnly {
			printUnsupportedUpdateCheck(install)
			return nil
		}
		return unsupportedUpdateError(install)
	}
}

func resolveRunningTypoInstall() (doctorInstallMethod, error) {
	typoPath, err := executable()
	if err != nil {
		return doctorInstallMethod{}, fmt.Errorf("cannot determine running typo binary: %w", err)
	}
	return detectDoctorInstallMethod(typoPath), nil
}

func updateHomebrew(flags updateFlags, install doctorInstallMethod) error {
	mode, targetVersion, err := normalizeUpdateTarget(flags.targetVer)
	if err != nil {
		return err
	}
	if mode == updateModeRelease {
		return fmt.Errorf("homebrew updates do not support --version %s; use Homebrew directly if you need version pinning", targetVersion)
	}

	fmt.Printf("Update target: %s\n", install.detail)
	fmt.Println("Install method: Homebrew")

	if flags.dryRun {
		fmt.Println("Would run: brew update")
		fmt.Println("Would run: brew upgrade typo")
		return nil
	}

	if flags.checkOnly {
		output, err := updateCommandOutput("brew", []string{"outdated", "--quiet", "typo"}, nil)
		if err != nil {
			return fmt.Errorf("failed to check Homebrew updates: %w", err)
		}
		if strings.TrimSpace(output) == "" {
			fmt.Println("typo is up to date according to Homebrew")
			return nil
		}
		fmt.Println("Homebrew update available for typo")
		return nil
	}

	if err := updateRunCommand("brew", []string{"update"}, nil); err != nil {
		return fmt.Errorf("brew update failed: %w", err)
	}
	if err := updateRunCommand("brew", []string{"upgrade", "typo"}, nil); err != nil {
		return fmt.Errorf("brew upgrade typo failed: %w", err)
	}

	fmt.Println("Updated typo with Homebrew")
	return nil
}

func updateScriptInstall(flags updateFlags, install doctorInstallMethod) error {
	mode, targetVersion, err := normalizeUpdateTarget(flags.targetVer)
	if err != nil {
		return err
	}
	targetDir, err := scriptInstallTargetDir(install.path)
	if err != nil {
		return err
	}

	fmt.Printf("Update target: %s\n", install.path)
	fmt.Println("Install method: curl/install.sh")

	if err = validateScriptUpdatePrerequisites(mode); err != nil {
		return err
	}

	if flags.checkOnly {
		return checkScriptInstall(flags, mode, targetVersion)
	}

	scriptArgs, stop, err := scriptUpdateArgs(flags, mode, targetVersion)
	if err != nil || stop {
		return err
	}

	if flags.dryRun {
		fmt.Printf("Would download: %s\n", installScriptURL)
		fmt.Printf("Would run: TYPO_INSTALL_DIR=%s bash install.sh %s\n", targetDir, strings.Join(scriptArgs, " "))
		return nil
	}

	if err = runScriptUpdate(scriptArgs, targetDir); err != nil {
		return err
	}

	if mode == updateModeMain {
		fmt.Println("Updated typo from the main branch")
	} else {
		fmt.Printf("Updated typo to %s\n", normalizeVersionTag(targetVersion))
	}
	return nil
}

func scriptInstallTargetDir(installPath string) (string, error) {
	targetDir := filepath.Dir(filepath.Clean(installPath))
	if targetDir == "." || targetDir == "" {
		return "", fmt.Errorf("cannot determine install directory for %s", installPath)
	}
	return targetDir, nil
}

func validateScriptUpdatePrerequisites(mode updateMode) error {
	if _, lookupErr := lookPath("bash"); lookupErr != nil {
		return fmt.Errorf("bash is required to run install.sh: %w", lookupErr)
	}
	if mode != updateModeMain {
		return nil
	}
	if _, lookupErr := lookPath("go"); lookupErr != nil {
		return fmt.Errorf("go is required to build typo from main: %w", lookupErr)
	}
	return nil
}

func scriptUpdateArgs(flags updateFlags, mode updateMode, targetVersion string) ([]string, bool, error) {
	if mode == updateModeMain {
		return []string{"-b"}, false, nil
	}

	currentVer, _, _ := resolveVersionInfo()
	cmp := compareVersions(currentVer, targetVersion)
	if !flags.force && cmp >= 0 {
		printReleaseNoop(currentVer, targetVersion, cmp)
		return nil, true, nil
	}
	return []string{"-s", trimVersionPrefix(targetVersion)}, false, nil
}

func printReleaseNoop(currentVer, targetVersion string, cmp int) {
	if cmp == 0 {
		fmt.Printf("typo %s is already installed\n", normalizeVersionTag(targetVersion))
		return
	}
	fmt.Printf("Installed version %s is newer than %s\n", currentVer, normalizeVersionTag(targetVersion))
}

func runScriptUpdate(scriptArgs []string, targetDir string) error {
	tmpDir, err := os.MkdirTemp("", "typo-update-*")
	if err != nil {
		return fmt.Errorf("cannot create temporary update directory: %w", err)
	}
	defer func() { _ = removeAll(tmpDir) }()

	scriptPath := filepath.Join(tmpDir, "install.sh")
	if err := updateDownloadFile(installScriptURL, scriptPath); err != nil {
		return fmt.Errorf("failed to download install script: %w", err)
	}

	args := append([]string{scriptPath}, scriptArgs...)
	if err := updateRunCommand("bash", args, []string{"TYPO_INSTALL_DIR=" + targetDir}); err != nil {
		return fmt.Errorf("install script failed: %w", err)
	}
	return nil
}

func checkScriptInstall(flags updateFlags, mode updateMode, targetVersion string) error {
	if mode == updateModeMain {
		fmt.Println("Update supported: yes")
		fmt.Println("Target: main branch source build")
		fmt.Println("Go: available")
		printUpstreamCheckStatus()
		return nil
	}

	currentVer, _, _ := resolveVersionInfo()
	cmp := compareVersions(currentVer, targetVersion)
	targetTag := normalizeVersionTag(targetVersion)
	if cmp == 0 {
		if flags.force {
			fmt.Printf("typo %s is already installed; --force would reinstall it\n", targetTag)
		} else {
			fmt.Printf("typo %s is already installed\n", targetTag)
		}
	} else if cmp < 0 {
		fmt.Printf("Update available: %s -> %s\n", currentVer, targetTag)
	} else {
		fmt.Printf("Installed version %s is newer than %s\n", currentVer, targetTag)
	}
	return nil
}

func unsupportedUpdateError(install doctorInstallMethod) error {
	switch install.kind {
	case doctorInstallGo:
		return fmt.Errorf("typo update does not replace go install binaries at %s\nUse: %s", install.detail, install.action)
	case doctorInstallWindowsQuick:
		return fmt.Errorf("typo update does not support Windows quick install yet\nUse: %s", install.action)
	case doctorInstallManual:
		return fmt.Errorf("typo update does not replace manual Release binaries at %s\nUse the install script or Homebrew for managed updates", install.detail)
	default:
		return fmt.Errorf("typo update cannot determine a supported install method; run typo doctor for details")
	}
}

func printUnsupportedUpdateCheck(install doctorInstallMethod) {
	if install.detail != "" {
		fmt.Printf("Update target: %s\n", install.detail)
	}
	if install.name != "" {
		fmt.Printf("Install method: %s\n", install.name)
	}
	fmt.Println("Update supported: no")
	printUpstreamCheckStatus()
	if install.kind == doctorInstallGo {
		fmt.Println("Note: go install @latest installs the latest Release, not the main branch commit.")
	}
	if install.action != "" {
		fmt.Println("Use:")
		fmt.Printf("  %s\n", install.action)
		return
	}
	fmt.Println("Run typo doctor for install method details.")
}

func printUpstreamCheckStatus() {
	currentVer, currentCommit, currentDate := resolveVersionInfo()
	fmt.Printf("Current version: %s\n", currentVer)
	if currentCommit != "" && currentCommit != "none" {
		fmt.Printf("Current commit: %s\n", currentCommit)
	}
	if currentDate != "" && currentDate != UnknownValue {
		fmt.Printf("Current build date: %s\n", currentDate)
	}

	latestRelease, err := updateLatestRelease()
	if err != nil {
		fmt.Printf("Latest Release: unavailable (%v)\n", err)
	} else {
		fmt.Printf("Latest Release: %s\n", latestRelease)
		printReleaseComparison(currentVer, latestRelease)
	}

	mainCommit, err := updateMainCommit()
	if err != nil {
		fmt.Printf("Latest main commit: unavailable (%v)\n", err)
		return
	}
	shortMainCommit := shortRevision(mainCommit)
	fmt.Printf("Latest main commit: %s\n", shortMainCommit)
	printMainCommitComparison(currentCommit, shortMainCommit)
}

func printReleaseComparison(currentVer, latestRelease string) {
	switch compareVersions(currentVer, latestRelease) {
	case -1:
		fmt.Printf("Release status: update available (%s -> %s)\n", currentVer, latestRelease)
	case 0:
		fmt.Println("Release status: up to date")
	case 1:
		fmt.Printf("Release status: installed version %s is newer than %s\n", currentVer, latestRelease)
	}
}

func printMainCommitComparison(currentCommit, mainCommit string) {
	currentCommit = strings.TrimSpace(currentCommit)
	if currentCommit == "" || currentCommit == "none" || currentCommit == UnknownValue {
		fmt.Println("Main status: current commit unavailable")
		return
	}
	if strings.HasPrefix(mainCommit, currentCommit) || strings.HasPrefix(currentCommit, mainCommit) {
		fmt.Println("Main status: current commit matches main")
		return
	}
	fmt.Println("Main status: current commit differs from main")
}

func normalizeUpdateTarget(target string) (updateMode, string, error) {
	target = strings.TrimSpace(target)
	if target == "" || strings.EqualFold(target, "latest") || strings.EqualFold(target, "main") {
		return updateModeMain, "", nil
	}
	if strings.HasPrefix(target, "@") {
		return updateModeMain, "", fmt.Errorf("unsupported --version %q; use 'typo update' for main, or '--version <release>' such as '--version 1.1.0'", target)
	}
	return updateModeRelease, normalizeVersionTag(target), nil
}

func normalizeVersionTag(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return version
	}
	if strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}

func trimVersionPrefix(version string) string {
	return strings.TrimPrefix(strings.TrimSpace(version), "v")
}

func downloadUpdateFile(url, dst string) error {
	const maxAttempts = 2
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := downloadUpdateFileOnce(url, dst)
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt == maxAttempts || !shouldRetryUpdateDownload(err) {
			break
		}
		updateSleep(updateDownloadBackoff(err))
	}
	return lastErr
}

func fetchLatestReleaseTag() (string, error) {
	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := fetchUpdateJSON("https://api.github.com/repos/yuluo-yx/typo/releases/latest", &payload); err != nil {
		return "", err
	}
	if payload.TagName == "" {
		return "", fmt.Errorf("empty tag_name")
	}
	return payload.TagName, nil
}

func fetchMainCommit() (string, error) {
	var payload struct {
		SHA string `json:"sha"`
	}
	if err := fetchUpdateJSON("https://api.github.com/repos/yuluo-yx/typo/commits/main", &payload); err != nil {
		return "", err
	}
	if payload.SHA == "" {
		return "", fmt.Errorf("empty sha")
	}
	return payload.SHA, nil
}

func fetchUpdateJSON(url string, out any) error {
	const maxAttempts = 2
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := fetchUpdateJSONOnce(url, out)
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt == maxAttempts || !shouldRetryUpdateDownload(err) {
			break
		}
		updateSleep(updateDownloadBackoff(err))
	}
	return lastErr
}

func fetchUpdateJSONOnce(url string, out any) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "typo")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := updateHTTPDownload(req)
	if err != nil {
		return fmt.Errorf("network error: %w (check your connection or proxy settings)", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return updateHTTPStatusError{code: resp.StatusCode}
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func downloadUpdateFileOnce(url, dst string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "typo")

	resp, err := updateHTTPDownload(req)
	if err != nil {
		return fmt.Errorf("network error: %w (check your connection or proxy settings)", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return updateHTTPStatusError{code: resp.StatusCode}
	}

	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}
	return os.Chmod(dst, 0755)
}

type updateHTTPStatusError struct {
	code int
}

func (e updateHTTPStatusError) Error() string {
	return fmt.Sprintf("HTTP %d", e.code)
}

func shouldRetryUpdateDownload(err error) bool {
	var statusErr updateHTTPStatusError
	if errors.As(err, &statusErr) {
		return statusErr.code == http.StatusTooManyRequests || statusErr.code >= http.StatusInternalServerError
	}
	return isTimeoutError(err)
}

func updateDownloadBackoff(err error) time.Duration {
	var statusErr updateHTTPStatusError
	if errors.As(err, &statusErr) && statusErr.code == http.StatusTooManyRequests {
		return 20 * time.Second
	}
	return 2 * time.Second
}

func isTimeoutError(err error) bool {
	if os.IsTimeout(err) {
		return true
	}
	var timeout interface {
		Timeout() bool
	}
	return errors.As(err, &timeout) && timeout.Timeout()
}

func runUpdateCommand(name string, args []string, extraEnv []string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	return cmd.Run()
}

func runUpdateCommandOutput(name string, args []string, extraEnv []string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	output, err := cmd.Output()
	return string(output), err
}

// --- version comparison ---

func compareVersions(a, b string) int {
	a = trimVersionPrefix(a)
	b = trimVersionPrefix(b)

	if a == "dev" || a == "unknown" || a == "" {
		return -1
	}
	if b == "dev" || b == "unknown" || b == "" {
		return 1
	}

	aParts := splitVersion(a)
	bParts := splitVersion(b)

	for i := range max(len(aParts), len(bParts)) {
		aNum := partAt(aParts, i)
		bNum := partAt(bParts, i)
		if aNum < bNum {
			return -1
		}
		if aNum > bNum {
			return 1
		}
	}

	return 0
}

func splitVersion(v string) []int {
	parts := strings.Split(v, ".")
	nums := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			n = 0
		}
		nums[i] = n
	}
	return nums
}

func partAt(parts []int, i int) int {
	if i >= len(parts) {
		return 0
	}
	return parts[i]
}
