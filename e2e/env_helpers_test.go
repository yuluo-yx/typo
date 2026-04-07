package e2e

import (
	"os"
	"path/filepath"
	"strings"
)

// filteredCommandEnv 过滤掉会污染测试隔离环境的变量。
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

// appendWindowsHomeEnv 在 Windows 风格路径下补齐 home 相关变量。
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
