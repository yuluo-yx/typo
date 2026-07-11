package cmd

import (
	"fmt"
	"os"
	"time"
)

func cmdVersion(args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "Error: version does not accept arguments")
		return 1
	}

	resolvedVersion, resolvedCommit, resolvedDate := resolveVersionInfo()
	fmt.Printf("typo %s (commit: %s, built: %s)\n", resolvedVersion, resolvedCommit, resolvedDate)
	return 0
}

// Prefer release metadata injected by the build pipeline; fall back to VCS metadata embedded in the Go binary when needed.
func resolveVersionInfo() (string, string, string) {
	resolvedVersion := version
	resolvedCommit := commit
	resolvedDate := date

	info, ok := readBuildInfo()
	if !ok || info == nil {
		return resolvedVersion, resolvedCommit, resolvedDate
	}

	if (resolvedVersion == "" || resolvedVersion == "dev") && info.Main.Version != "" && info.Main.Version != "(devel)" {
		resolvedVersion = info.Main.Version
	}

	settings := make(map[string]string, len(info.Settings))
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}

	if resolvedCommit == "" || resolvedCommit == "none" {
		if revision := settings["vcs.revision"]; revision != "" {
			resolvedCommit = shortRevision(revision)
		}
	}

	if resolvedDate == "" || resolvedDate == UnknownValue {
		if vcsTime := settings["vcs.time"]; vcsTime != "" {
			resolvedDate = formatBuildDate(vcsTime)
		}
	}

	return resolvedVersion, resolvedCommit, resolvedDate
}

func shortRevision(revision string) string {
	if len(revision) <= 7 {
		return revision
	}

	return revision[:7]
}

func formatBuildDate(raw string) string {
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return raw
	}

	return parsed.UTC().Format("2006-01-02")
}
