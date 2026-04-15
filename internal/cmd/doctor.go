package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuluo-yx/typo/internal/config"
	"github.com/yuluo-yx/typo/internal/utils"
)

func cmdDoctor() int {
	fmt.Println("Checking typo configuration...")
	fmt.Println()

	hasError := false
	cfg := config.Load()
	shellName, shellRC := detectShellIntegrationTarget()

	// Check if typo is in PATH
	fmt.Print("[1/6] typo command: ")
	typoPath, err := lookPath("typo")
	if err == nil {
		fmt.Printf("✓ available in PATH (%s)\n", typoPath)
	} else {
		fmt.Println("✗ not found in PATH")
		// Check if typo exists in Go bin
		goBinPath := checkGoBinTypo()
		if goBinPath != "" {
			fmt.Println()
			fmt.Println("  Found typo in Go bin directory but not in PATH.")
			fmt.Printf("  Add the following to your %s:\n", shellRC)
			fmt.Printf("    export PATH=\"$PATH:%s\"\n", goBinPath)
			fmt.Println()
		}
		hasError = true
	}

	// Check config directory
	fmt.Print("[2/6] config directory: ")
	if info, err := statPath(cfg.ConfigDir); err == nil && info.IsDir() {
		fmt.Printf("✓ %s\n", cfg.ConfigDir)
	} else {
		fmt.Printf("⊘ %s (will be created on first use)\n", cfg.ConfigDir)
	}

	// Check config file and print effective settings
	fmt.Print("[3/6] config file: ")
	if configFile := cfg.ConfigFilePath(); configFile != "" {
		if info, err := statPath(configFile); err == nil && !info.IsDir() {
			fmt.Printf("✓ %s\n", configFile)
		} else {
			fmt.Printf("⊘ %s (using defaults; run 'typo config gen' to create it)\n", configFile)
		}
	} else {
		fmt.Println("⊘ unavailable")
	}

	printDoctorEffectiveConfig(cfg, shellName)
	hasError = checkDoctorShellIntegration(shellName, shellRC) || hasError
	checkDoctorInstallMethod(shellName, shellRC, doctorInstallPath(typoPath))
	hasError = checkDoctorGoBinPath(shellName, shellRC, typoPath) || hasError

	fmt.Println()
	if hasError {
		fmt.Println("Some checks failed. Please fix the issues above.")
		return 1
	}

	fmt.Println("All checks passed!")
	return 0
}

func printDoctorEffectiveConfig(cfg *config.Config, shellName string) {
	fmt.Println()
	fmt.Println("effective config:")
	fmt.Printf("  shell: %s\n", shellName)
	for _, setting := range cfg.ListSettings() {
		fmt.Printf("  %s=%s\n", setting.Key, setting.Value)
	}
	fmt.Println()
}

func checkDoctorShellIntegration(shellName, shellRC string) bool {
	fmt.Print("[4/6] shell integration: ")
	if os.Getenv("TYPO_SHELL_INTEGRATION") == "1" {
		fmt.Println("✓ loaded")
		return printDoctorShellMisconfiguration(shellName)
	}

	fmt.Println("✗ not loaded")
	fmt.Println()
	if shellName != "" {
		fmt.Printf("To enable shell integration, add to your %s:\n", shellRC)
		fmt.Printf("  %s\n", shellInitCommand(shellName))
		fmt.Println()
		fmt.Printf("Then restart your shell or run: %s\n", shellReloadCommand(shellName, shellRC))
		_ = printDoctorShellMisconfiguration(shellName)
		return true
	}

	fmt.Println("To enable shell integration, add one of the following:")
	fmt.Println("  # Zsh (~/.zshrc)")
	fmt.Println("  eval \"$(typo init zsh)\"")
	fmt.Println("  # Bash (~/.bashrc)")
	fmt.Println("  eval \"$(typo init bash)\"")
	fmt.Println("  # Fish (~/.config/fish/config.fish)")
	fmt.Println("  typo init fish | source")
	fmt.Println("  # PowerShell ($PROFILE.CurrentUserCurrentHost)")
	fmt.Println("  Invoke-Expression (& typo init powershell)")
	fmt.Println()
	fmt.Println("Then restart your shell or source the matching profile file.")
	return true
}

func checkDoctorInstallMethod(shellName, shellRC, typoPath string) {
	method := detectDoctorInstallMethod(typoPath)

	fmt.Print("[5/6] install method: ")
	if method.name == UnknownValue {
		fmt.Println("⊘ unable to determine")
	} else if method.detail != "" {
		fmt.Printf("✓ %s (%s)\n", method.name, method.detail)
	} else {
		fmt.Printf("✓ %s\n", method.name)
	}

	if method.action != "" {
		fmt.Println("  Install/update:")
		fmt.Printf("    %s\n", method.action)
	}
	if shellName != "" {
		fmt.Printf("  Shell config: %s\n", shellRC)
		fmt.Printf("  Shell init: %s\n", shellInitCommand(shellName))
	} else {
		fmt.Println("  Shell setup: run typo doctor from zsh, bash, fish, or PowerShell for shell-specific instructions.")
	}
}

func checkDoctorGoBinPath(shellName, shellRC, typoPath string) bool {
	fmt.Print("[6/6] Go bin PATH: ")
	goBinDir := getGoBinDir()
	typoInGoBin := false
	if typoPath != "" {
		typoInGoBin = utils.SameDir(filepath.Dir(typoPath), goBinDir)
	}

	if !typoInGoBin {
		fmt.Println("⊘ skipped (not a go install binary)")
		return false
	}
	if goBinDir == "" {
		fmt.Println("⊘ Go not installed or GOPATH not set")
		return false
	}
	if utils.PathContainsDir(os.Getenv("PATH"), goBinDir) {
		fmt.Println("✓ configured")
		return false
	}

	fmt.Printf("✗ %s not in PATH\n", goBinDir)
	fmt.Println()
	if shellName != "" {
		fmt.Printf("  If you installed typo with 'go install', add to your %s:\n", shellRC)
	} else {
		fmt.Println("  If you installed typo with 'go install', add to your shell config:")
	}
	fmt.Printf("    %s\n", shellPathExportCommand(shellName, goBinDir))
	return true
}

func doctorInstallPath(typoPath string) string {
	if typoPath != "" {
		return typoPath
	}

	if execPath, err := executable(); err == nil {
		return execPath
	}
	return ""
}

func printDoctorShellMisconfiguration(shellName string) bool {
	message := doctorShellMisconfiguration(shellName)
	if message == "" {
		return false
	}

	fmt.Println()
	fmt.Println("Common shell integration misconfiguration detected:")
	fmt.Printf("  %s\n", message)
	return true
}

func doctorShellMisconfiguration(shellName string) string {
	configPath := shellConfigFilePath(shellName)
	if configPath == "" {
		return ""
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}

	content := string(data)
	expected := shellInitCommand(shellName)
	if expected == "" || strings.Contains(content, expected) {
		return ""
	}
	if !strings.Contains(content, "typo init") {
		return ""
	}

	return fmt.Sprintf("%s contains a typo init command that does not match %s. Use: %s", shellConfigDisplayPath(shellName), shellName, expected)
}

// doctorInstallMethod represents detected install method details.
type doctorInstallMethod struct {
	name   string
	detail string
	action string
}

func detectDoctorInstallMethod(typoPath string) doctorInstallMethod {
	if typoPath == "" {
		return doctorInstallMethod{
			name:   UnknownValue,
			action: "Install with: curl -fsSL https://raw.githubusercontent.com/yuluo-yx/typo/main/tools/scripts/install.sh | bash",
		}
	}

	if isGoInstallPath(typoPath) {
		return doctorInstallMethod{
			name:   "go install",
			detail: filepath.Dir(filepath.Clean(typoPath)),
			action: "go install github.com/yuluo-yx/typo/cmd/typo@latest",
		}
	}
	if isWindowsQuickInstallPath(typoPath) {
		return doctorInstallMethod{
			name:   "Windows quick install",
			detail: filepath.Dir(filepath.Clean(typoPath)),
			action: "iwr -useb https://raw.githubusercontent.com/yuluo-yx/typo/main/tools/scripts/quick-install.ps1 | iex",
		}
	}
	if isHomebrewInstallPath(typoPath) {
		return doctorInstallMethod{
			name:   "Homebrew",
			detail: filepath.Dir(filepath.Clean(typoPath)),
			action: "brew upgrade typo",
		}
	}
	if isScriptInstallPath(typoPath) {
		return doctorInstallMethod{
			name:   "curl/install.sh",
			detail: filepath.Dir(filepath.Clean(typoPath)),
			action: "curl -fsSL https://raw.githubusercontent.com/yuluo-yx/typo/main/tools/scripts/install.sh | bash",
		}
	}

	return doctorInstallMethod{
		name:   "manual release",
		detail: filepath.Dir(filepath.Clean(typoPath)),
		action: "Download the matching GitHub Release binary, verify checksums.txt, then place typo in a PATH directory.",
	}
}

func doctorInstallCandidatePaths(typoPath string) []string {
	candidates := []string{filepath.Clean(typoPath)}
	if resolved, err := filepath.EvalSymlinks(typoPath); err == nil && !utils.SameDir(resolved, typoPath) {
		candidates = append(candidates, filepath.Clean(resolved))
	}
	return candidates
}
