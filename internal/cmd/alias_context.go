package cmd

import (
	"bufio"
	"os"
	"regexp"
	"strings"

	itypes "github.com/yuluo-yx/typo/internal/types"
)

const (
	aliasContextMaxFileSize = 256 * 1024
	aliasContextMaxEntries  = 1024
)

var aliasContextKinds = map[string]bool{
	"alias":    true,
	"abbr":     true,
	"function": true,
	"env":      true,
}

var safeEnvNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func loadAliasContext(path string) []itypes.AliasContextEntry {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}

	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() > aliasContextMaxFileSize {
		return nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), aliasContextMaxFileSize)

	entries := make([]itypes.AliasContextEntry, 0)
	seen := make(map[string]int)
	for scanner.Scan() {
		entry, ok := parseAliasContextLine(scanner.Text())
		if !ok {
			continue
		}

		key := contextEntryKey(entry)
		if _, exists := seen[key]; exists {
			continue
		}
		if len(entries) >= aliasContextMaxEntries {
			break
		}
		seen[key] = len(entries)
		entries = append(entries, entry)
	}

	return entries
}

func parseAliasContextLine(line string) (itypes.AliasContextEntry, bool) {
	fields := strings.SplitN(line, "\t", 4)
	if len(fields) != 4 {
		return itypes.AliasContextEntry{}, false
	}

	entry := itypes.AliasContextEntry{
		Shell:     strings.TrimSpace(fields[0]),
		Kind:      strings.TrimSpace(fields[1]),
		Name:      strings.TrimSpace(fields[2]),
		Expansion: strings.TrimSpace(fields[3]),
	}
	if entry.Shell == "" || !aliasContextKinds[entry.Kind] {
		return itypes.AliasContextEntry{}, false
	}
	if entry.Kind == "env" {
		if !isSafeEnvName(entry.Name) {
			return itypes.AliasContextEntry{}, false
		}
		if entry.Expansion == "" {
			entry.Expansion = entry.Name
		}
		if !isSafeEnvName(entry.Expansion) {
			return itypes.AliasContextEntry{}, false
		}
		return entry, true
	}
	if !isSafeAliasName(entry.Name) || !isSafeAliasExpansion(entry.Expansion) {
		return itypes.AliasContextEntry{}, false
	}

	return entry, true
}

func contextEntryKey(entry itypes.AliasContextEntry) string {
	return entry.Kind + "\x00" + entry.Name
}

func isSafeEnvName(name string) bool {
	return safeEnvNamePattern.MatchString(strings.TrimSpace(name))
}

func isSafeAliasName(name string) bool {
	if name == "" || strings.ContainsAny(name, " \t\r\n\x00") {
		return false
	}
	return !strings.ContainsAny(name, "|&;<>`$(){}[]")
}

func isSafeAliasExpansion(expansion string) bool {
	if expansion == "" || strings.ContainsAny(expansion, "\t\r\n\x00") {
		return false
	}
	return !strings.ContainsAny(expansion, "|&;<>`$(){}[]")
}
