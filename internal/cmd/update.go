package cmd

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	installScriptURL    = "https://raw.githubusercontent.com/yuluo-yx/typo/main/tools/scripts/install.sh"
	latestReleaseAPIURL = "https://api.github.com/repos/yuluo-yx/typo/releases/latest"
	mainCommitAPIURL    = "https://api.github.com/repos/yuluo-yx/typo/commits/main"
)

var (
	updateRunCommand    func(name string, args []string, extraEnv []string) error
	updateCommandOutput func(name string, args []string, extraEnv []string) (string, error)
	updateDownloadFile  func(url, dst string) error
	updateLatestRelease func() (string, error)
	updateMainCommit    func() (string, error)
)

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
	updateRunCommand = runUpdateCommand
	updateCommandOutput = runUpdateCommandOutput
	updateDownloadFile = downloadUpdateFile
	updateLatestRelease = fetchLatestReleaseTag
	updateMainCommit = fetchMainCommit
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

	scriptArgs, stop, err := scriptUpdateArgs(flags, mode, targetVersion)
	if err != nil || stop {
		return err
	}
	if flags.dryRun {
		fmt.Printf("Would download: %s\n", installScriptURL)
		fmt.Printf("Would run: TYPO_INSTALL_DIR=%s bash install.sh %s\n", targetDir, strings.Join(scriptArgs, " "))
		return nil
	}

	if err = validateScriptUpdatePrerequisites(mode); err != nil {
		return err
	}

	if flags.checkOnly {
		return checkScriptInstall(flags, mode, targetVersion)
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
	if mode == updateModeMain {
		if _, lookupErr := lookPath("go"); lookupErr != nil {
			return fmt.Errorf("go is required to build typo from main: %w", lookupErr)
		}
	}
	if _, lookupErr := lookPath("curl"); lookupErr != nil {
		return fmt.Errorf("curl is required to download install.sh: %w", lookupErr)
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
	if !isReleaseVersionSelector(target) {
		return updateModeMain, "", fmt.Errorf("invalid --version %q; use '--version <release>' such as '--version 1.1.0'", target)
	}
	return updateModeRelease, normalizeVersionTag(target), nil
}

func isReleaseVersionSelector(target string) bool {
	target = trimVersionPrefix(target)
	if target == "" {
		return false
	}

	parts := strings.Split(target, ".")
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
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
	args := []string{"-fsSL", "--retry", "1", "--retry-delay", "2", "-o", dst, url}
	if err := updateRunCommand("curl", args, nil); err != nil {
		return fmt.Errorf("curl download failed: %w", err)
	}
	return os.Chmod(dst, 0755)
}

func fetchLatestReleaseTag() (string, error) {
	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := fetchUpdateJSON(latestReleaseAPIURL, &payload); err != nil {
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
	if err := fetchUpdateJSON(mainCommitAPIURL, &payload); err != nil {
		return "", err
	}
	if payload.SHA == "" {
		return "", fmt.Errorf("empty sha")
	}
	return payload.SHA, nil
}

func fetchUpdateJSON(url string, out any) error {
	args := []string{"-fsSL", "--retry", "1", "--retry-delay", "2", "-H", "Accept: application/vnd.github+json"}
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		args = append(args, "-H", "Authorization: Bearer " + token)
	}
	args = append(args, url)

	output, err := updateCommandOutput("curl", args, nil)
	if err != nil {
		return fmt.Errorf("curl request failed: %w", err)
	}
	if err := json.Unmarshal([]byte(output), out); err != nil {
		return err
	}
	return nil
}

func runUpdateCommand(name string, args []string, extraEnv []string) error {
	cmd, err := newUpdateCommand(name, args)
	if err != nil {
		return err
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	return cmd.Run()
}

func runUpdateCommandOutput(name string, args []string, extraEnv []string) (string, error) {
	cmd, err := newUpdateCommand(name, args)
	if err != nil {
		return "", err
	}
	cmd.Stderr = os.Stderr
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	output, err := cmd.Output()
	return string(output), err
}

func newUpdateCommand(name string, args []string) (*exec.Cmd, error) {
	switch name {
	case "bash":
		// #nosec G702 -- update commands are restricted to this whitelist and args are constructed by update flow.
		return exec.Command("bash", args...), nil
	case "brew":
		// #nosec G702 -- update commands are restricted to this whitelist and args are constructed by update flow.
		return exec.Command("brew", args...), nil
	case "curl":
		// #nosec G702 -- update commands are restricted to this whitelist and args are constructed by update flow.
		return exec.Command("curl", args...), nil
	default:
		return nil, fmt.Errorf("unsupported update command %q", name)
	}
}

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
