package e2e

import (
	"os"
	"path/filepath"
	"strings"
)

// filteredCommandEnv removes variables that can leak into isolated test environments.
func filteredCommandEnv(excludedPrefixes []string, extraCapacity int) []string {
	filtered := make([]string, 0, len(os.Environ())+extraCapacity)
	for _, item := range os.Environ() {
		if hasEnvPrefix(item, excludedPrefixes) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func hasEnvPrefix(item string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(item, prefix) {
			return true
		}
	}
	return false
}

// appendWindowsHomeEnv fills home-related variables for Windows-style paths.
func appendWindowsHomeEnv(env []string, home string) []string {
	volume := filepath.VolumeName(home)
	if volume == "" {
		return env
	}

	homePath := strings.TrimPrefix(home, volume)
	if homePath == "" {
		homePath = string(os.PathSeparator)
	}

	return append(env,
		"HOMEDRIVE="+volume,
		"HOMEPATH="+homePath,
	)
}
