package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Shell detection functions.

func currentShellName() string {
	if shell := normalizeShellName(os.Getenv("TYPO_ACTIVE_SHELL")); shell != "" {
		return shell
	}

	shellPath := strings.TrimSpace(os.Getenv("SHELL"))
	if shellPath != "" {
		shellBase := strings.ToLower(filepath.Base(shellPath))
		if shellBase != "" && shellBase != "." {
			if shell := normalizeShellName(shellBase); shell != "" {
				return shell
			}
		}
	}

	if detectPowerShellEnvironment() {
		return "powershell"
	}

	return UnknownValue
}

func detectShellIntegrationTarget() (string, string) {
	switch currentShellName() {
	case "bash":
		return "bash", "~/.bashrc"
	case "zsh":
		return "zsh", "~/.zshrc"
	case "fish":
		return "fish", "~/.config/fish/config.fish"
	case "powershell":
		return "powershell", "$PROFILE.CurrentUserCurrentHost"
	default:
		return "", "~/.zshrc or ~/.bashrc or ~/.config/fish/config.fish or $PROFILE.CurrentUserCurrentHost"
	}
}

func normalizeShellName(shell string) string {
	shell = strings.TrimSpace(strings.ToLower(shell))
	shell = strings.TrimSuffix(shell, ".exe")

	switch shell {
	case "bash", "fish", "zsh":
		return shell
	case "pwsh", "powershell":
		return "powershell"
	default:
		return ""
	}
}

func detectPowerShellEnvironment() bool {
	return strings.TrimSpace(os.Getenv("POWERSHELL_DISTRIBUTION_CHANNEL")) != "" ||
		strings.TrimSpace(os.Getenv("PSModulePath")) != "" ||
		strings.TrimSpace(os.Getenv("PSExecutionPolicyPreference")) != ""
}

func shellInitCommand(shellName string) string {
	switch shellName {
	case "powershell":
		return "Invoke-Expression (& typo init powershell)"
	case "fish":
		return "typo init fish | source"
	case "bash", "zsh":
		return fmt.Sprintf("eval \"$(typo init %s)\"", shellName)
	default:
		return ""
	}
}

func shellReloadCommand(shellName, shellRC string) string {
	switch shellName {
	case "powershell":
		return fmt.Sprintf(". %s", shellRC)
	case "bash", "fish", "zsh":
		return fmt.Sprintf("source %s", shellRC)
	default:
		return ""
	}
}

func shellPathExportCommand(shellName, dir string) string {
	switch shellName {
	case "powershell":
		return fmt.Sprintf("$env:PATH = \"$env:PATH%c%s\"", os.PathListSeparator, dir)
	case "fish":
		return fmt.Sprintf("set -gx PATH $PATH %s", dir)
	default:
		return fmt.Sprintf("export PATH=\"$PATH:%s\"", dir)
	}
}

func shellConfigFilePath(shellName string) string {
	homeDir, err := UserHomeDir()
	if err != nil {
		return ""
	}

	switch shellName {
	case "bash":
		return filepath.Join(homeDir, ".bashrc")
	case "zsh":
		return filepath.Join(homeDir, ".zshrc")
	case "fish":
		return filepath.Join(homeDir, ".config", "fish", "config.fish")
	default:
		return ""
	}
}

func shellConfigDisplayPath(shellName string) string {
	switch shellName {
	case "bash":
		return "~/.bashrc"
	case "zsh":
		return "~/.zshrc"
	case "fish":
		return "~/.config/fish/config.fish"
	default:
		return "shell config"
	}
}

func getGoBinDir() string {
	// Try GOPATH first
	goPath := os.Getenv("GOPATH")
	goBin := os.Getenv("GOBIN")
	if goBin != "" {
		return filepath.Clean(goBin)
	}
	if goPath == "" {
		// Try default GOPATH
		homeDir, err := UserHomeDir()
		if err != nil {
			return ""
		}
		goPath = homeDir + "/go"
	}
	return filepath.Join(goPath, "bin")
}

func checkGoBinTypo() string {
	goBinDir := getGoBinDir()
	if goBinDir == "" {
		return ""
	}
	typoPath := filepath.Join(goBinDir, "typo")
	if _, err := StatPath(typoPath); err == nil {
		return goBinDir
	}
	return ""
}