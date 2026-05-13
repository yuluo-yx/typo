package cmd

import (
	"os"
	"path/filepath"
	"strings"
)

// Install path detection functions.

func isGoInstallPath(typoPath string) bool {
	return sameDir(filepath.Dir(typoPath), getGoBinDir())
}

func isWindowsQuickInstallPath(typoPath string) bool {
	localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if localAppData == "" {
		return false
	}

	return sameDir(filepath.Dir(typoPath), filepath.Join(localAppData, "Programs", "typo", "bin"))
}

func isHomebrewInstallPath(typoPath string) bool {
	for _, candidate := range doctorInstallCandidatePaths(typoPath) {
		normalized := filepath.ToSlash(filepath.Clean(candidate))
		if strings.Contains(normalized, "/Cellar/typo/") || strings.Contains(normalized, "/opt/typo/") {
			return true
		}

		homebrewPrefix := strings.TrimSpace(os.Getenv("HOMEBREW_PREFIX"))
		if homebrewPrefix != "" && sameDir(filepath.Dir(candidate), filepath.Join(homebrewPrefix, "bin")) {
			return true
		}

		for _, prefix := range []string{"/opt/homebrew", "/usr/local", "/home/linuxbrew/.linuxbrew"} {
			if pathWithinDir(candidate, filepath.Join(prefix, "Cellar", "typo")) ||
				pathWithinDir(candidate, filepath.Join(prefix, "opt", "typo")) {
				return true
			}
		}
	}

	return false
}

func isScriptInstallPath(typoPath string) bool {
	installDir := strings.TrimSpace(os.Getenv("TYPO_INSTALL_DIR"))
	if installDir != "" && sameDir(filepath.Dir(typoPath), installDir) {
		return true
	}

	homeDir, err := userHomeDir()
	if err == nil && sameDir(filepath.Dir(typoPath), filepath.Join(homeDir, ".local", "bin")) {
		return true
	}

	for _, dir := range []string{"/usr/local/bin", "/opt/homebrew/bin"} {
		if sameDir(filepath.Dir(typoPath), dir) {
			return true
		}
	}

	return false
}
