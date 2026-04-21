package cmd

import (
	"bufio"
	"os"
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
}

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

		if _, exists := seen[entry.Name]; exists {
			continue
		}
		if len(entries) >= aliasContextMaxEntries {
			break
		}
		seen[entry.Name] = len(entries)
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
	if !isSafeAliasName(entry.Name) || !isSafeAliasExpansion(entry.Expansion) {
		return itypes.AliasContextEntry{}, false
	}

	return entry, true
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
