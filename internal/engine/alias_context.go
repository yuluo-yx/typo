package engine

import (
	"sort"
	"strings"

	itypes "github.com/yuluo-yx/typo/internal/types"
)

const maxAliasExpansionDepth = 8

type aliasResolver struct {
	entries map[string]itypes.AliasContextEntry
}

type aliasRewriteRecord struct {
	lineOrdinal int
	alias       string
	tokens      []string
	canRealias  bool
}

type rawRangeReplacement struct {
	start int
	end   int
	value string
}

func (e *Engine) fixWithAliasContext(input itypes.ParserContext) itypes.FixResult {
	expanded, records, ok := expandCommandAliases(input.Command, input.AliasContext)
	if !ok {
		return e.fixWithoutAliasContext(input)
	}
	e.markDebugFeature("alias")
	if debug := e.debugTrace(); debug != nil && expanded != input.Command {
		debug.Events = append(debug.Events, itypes.FixDebugEvent{
			Pass:    0,
			Stage:   "alias",
			Before:  input.Command,
			After:   expanded,
			Message: "expanded shell alias context",
		})
	}

	expandedInput := input
	expandedInput.Command = expanded

	result := e.fixWithoutAliasContext(expandedInput)
	if !result.Fixed {
		return itypes.FixResult{Fixed: false}
	}

	rewritten := rewriteCommandAliases(result.Command, records)
	if rewritten == strings.TrimSpace(input.Command) {
		return itypes.FixResult{Fixed: false}
	}
	if rewritten != result.Command && result.Message != "" {
		result.Message = strings.ReplaceAll(result.Message, result.Command, rewritten)
	}
	result.Command = rewritten

	return result
}

func expandCommandAliases(raw string, entries []itypes.AliasContextEntry) (string, []aliasRewriteRecord, bool) {
	resolver := newAliasResolver(entries)
	if resolver == nil {
		return raw, nil, false
	}

	lines, err := parseShellCommandLines(raw)
	if err != nil {
		return raw, nil, false
	}

	replacements := make([]rawRangeReplacement, 0)
	records := make([]aliasRewriteRecord, 0)
	for lineOrdinal, line := range lines {
		alias := line.commandWord()
		expansion, tokens, ok := resolver.resolve(alias)
		if !ok {
			continue
		}

		start, end := wordRange(line.args[line.commandIdx], len(raw))
		replacements = append(replacements, rawRangeReplacement{
			start: start,
			end:   end,
			value: expansion,
		})
		records = append(records, aliasRewriteRecord{
			lineOrdinal: lineOrdinal,
			alias:       alias,
			tokens:      tokens,
			canRealias:  line.commandIdx == 0,
		})
	}

	if len(replacements) == 0 {
		return raw, nil, false
	}

	return applyRawRangeReplacements(raw, replacements), records, true
}

func newAliasResolver(entries []itypes.AliasContextEntry) *aliasResolver {
	if len(entries) == 0 {
		return nil
	}

	resolver := &aliasResolver{entries: make(map[string]itypes.AliasContextEntry, len(entries))}
	for _, entry := range entries {
		if entry.Kind != "alias" && entry.Kind != "abbr" && entry.Kind != "function" {
			continue
		}
		entry.Name = strings.TrimSpace(entry.Name)
		entry.Expansion = strings.TrimSpace(entry.Expansion)
		if entry.Name == "" || entry.Expansion == "" {
			continue
		}
		if _, tokens := splitAliasExpansion(entry.Expansion); len(tokens) == 0 {
			continue
		}
		resolver.entries[entry.Name] = entry
	}
	if len(resolver.entries) == 0 {
		return nil
	}

	return resolver
}

func (r *aliasResolver) resolve(name string) (string, []string, bool) {
	entry, ok := r.entries[name]
	if !ok {
		return "", nil, false
	}

	expansion := entry.Expansion
	seen := map[string]bool{name: true}
	for depth := 0; depth < maxAliasExpansionDepth; depth++ {
		_, tokens := splitAliasExpansion(expansion)
		if len(tokens) == 0 {
			return "", nil, false
		}

		next, ok := r.entries[tokens[0]]
		if !ok {
			return strings.Join(tokens, " "), tokens, true
		}

		if seen[tokens[0]] {
			if tokens[0] == name && len(tokens) > 1 {
				return strings.Join(tokens, " "), tokens, true
			}
			return "", nil, false
		}

		seen[tokens[0]] = true
		nextTokens := append([]string{next.Expansion}, tokens[1:]...)
		expansion = strings.Join(nextTokens, " ")
	}

	return "", nil, false
}

func splitAliasExpansion(expansion string) (string, []string) {
	lines, err := parseShellCommandLines(expansion)
	if err == nil {
		if len(lines) != 1 || lines[0].hasRedirection {
			return "", nil
		}
		line := lines[0]
		tokens := make([]string, 0, len(line.args))
		for i := range line.args {
			token := line.args[i].Lit()
			if token == "" {
				return "", nil
			}
			tokens = append(tokens, token)
		}
		return strings.Join(tokens, " "), tokens
	}

	tokens := strings.Fields(expansion)
	if len(tokens) == 0 {
		return "", nil
	}
	return strings.Join(tokens, " "), tokens
}

func rewriteCommandAliases(raw string, records []aliasRewriteRecord) string {
	if len(records) == 0 {
		return raw
	}

	lines, err := parseShellCommandLines(raw)
	if err != nil {
		return raw
	}

	replacements := make([]rawRangeReplacement, 0, len(records))
	for _, record := range records {
		if !record.canRealias || record.lineOrdinal >= len(lines) {
			continue
		}

		line := lines[record.lineOrdinal]
		if line.commandIdx != 0 || !lineHasAliasExpansionPrefix(line, record.tokens) {
			continue
		}

		start, _ := wordRange(line.args[line.commandIdx], len(raw))
		_, end := wordRange(line.args[line.commandIdx+len(record.tokens)-1], len(raw))
		replacements = append(replacements, rawRangeReplacement{
			start: start,
			end:   end,
			value: record.alias,
		})
	}

	if len(replacements) == 0 {
		return raw
	}

	return applyRawRangeReplacements(raw, replacements)
}

func lineHasAliasExpansionPrefix(line *shellCommandLine, tokens []string) bool {
	if line == nil || len(tokens) == 0 || line.commandIdx+len(tokens) > len(line.args) {
		return false
	}

	for i, token := range tokens {
		if line.args[line.commandIdx+i].Lit() != token {
			return false
		}
	}

	return true
}

func applyRawRangeReplacements(raw string, replacements []rawRangeReplacement) string {
	if len(replacements) == 0 {
		return raw
	}

	sorted := append([]rawRangeReplacement(nil), replacements...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].start > sorted[j].start
	})

	result := raw
	for _, replacement := range sorted {
		if replacement.start < 0 || replacement.end > len(result) || replacement.start > replacement.end {
			continue
		}
		result = result[:replacement.start] + replacement.value + result[replacement.end:]
	}

	return result
}
