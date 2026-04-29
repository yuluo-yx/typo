package engine

import (
	"sort"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	itypes "github.com/yuluo-yx/typo/internal/types"
	"github.com/yuluo-yx/typo/internal/utils"
)

const envSimilarityThreshold = 0.8

type envVarCandidate struct {
	name        string
	distance    int
	similarity  float64
	lengthDelta int
	transposed  bool
}

func (e *Engine) tryEnvVarFix(cmd string, entries []itypes.AliasContextEntry) itypes.FixResult {
	envNames := envContextNames(entries)
	if len(envNames) == 0 {
		return itypes.FixResult{Fixed: false}
	}

	lines, err := parseShellCommandLines(cmd)
	if err != nil {
		return itypes.FixResult{Fixed: false}
	}

	replacements := make([]rawRangeReplacement, 0)
	for _, line := range lines {
		for _, word := range line.args {
			replacements = append(replacements, collectEnvVarWordReplacements(word, cmd, envNames, e.envDistanceMatchConfig())...)
		}
	}

	if len(replacements) == 0 {
		return itypes.FixResult{Fixed: false}
	}

	fixedCommand := applyRawRangeReplacements(cmd, replacements)
	if fixedCommand == cmd {
		return itypes.FixResult{Fixed: false}
	}

	return itypes.FixResult{
		Fixed:   true,
		Command: fixedCommand,
		Source:  "env",
	}
}

func (e *Engine) envDistanceMatchConfig() distanceMatchConfig {
	return distanceMatchConfig{
		keyboard:            e.keyboard,
		maxEditDistance:     e.maxEditDistance,
		similarityThreshold: envSimilarityThreshold,
	}
}

func envContextNames(entries []itypes.AliasContextEntry) []string {
	if len(entries) == 0 {
		return nil
	}

	names := make([]string, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if entry.Kind != "env" {
			continue
		}

		name := strings.TrimSpace(entry.Name)
		if name == "" {
			name = strings.TrimSpace(entry.Expansion)
		}
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}

		seen[name] = struct{}{}
		names = append(names, name)
	}

	sort.Strings(names)
	return names
}

func collectEnvVarWordReplacements(word *syntax.Word, raw string, envNames []string, cfg distanceMatchConfig) []rawRangeReplacement {
	if word == nil || len(word.Parts) == 0 {
		return nil
	}

	replacements := make([]rawRangeReplacement, 0)
	for _, part := range word.Parts {
		replacements = append(replacements, collectEnvVarPartReplacements(part, raw, envNames, cfg)...)
	}
	return replacements
}

func collectEnvVarPartReplacements(part syntax.WordPart, raw string, envNames []string, cfg distanceMatchConfig) []rawRangeReplacement {
	switch part := part.(type) {
	case *syntax.ParamExp:
		if replacement, ok := envVarReplacement(part, raw, envNames, cfg); ok {
			return []rawRangeReplacement{replacement}
		}
	case *syntax.DblQuoted:
		replacements := make([]rawRangeReplacement, 0)
		for _, nested := range part.Parts {
			replacements = append(replacements, collectEnvVarPartReplacements(nested, raw, envNames, cfg)...)
		}
		return replacements
	}

	return nil
}

func envVarReplacement(part *syntax.ParamExp, raw string, envNames []string, cfg distanceMatchConfig) (rawRangeReplacement, bool) {
	name, start, end, ok := simpleParamNameRange(part, len(raw))
	if !ok {
		return rawRangeReplacement{}, false
	}

	match, _, unique := closestEnvVar(name, envNames, cfg)
	if !unique || match == "" || match == name {
		return rawRangeReplacement{}, false
	}

	return rawRangeReplacement{
		start: start,
		end:   end,
		value: match,
	}, true
}

func simpleParamNameRange(part *syntax.ParamExp, rawLen int) (string, int, int, bool) {
	if part == nil || part.Param == nil || part.NestedParam != nil || hasUnsupportedParamFlags(part) || hasUnsupportedParamExpansions(part) {
		return "", 0, 0, false
	}

	name := strings.TrimSpace(part.Param.Value)
	if !isSimpleEnvName(name) {
		return "", 0, 0, false
	}

	start, end := utils.ShellNodeRange(part.Param, rawLen)
	return name, start, end, true
}

func hasUnsupportedParamFlags(part *syntax.ParamExp) bool {
	return part.Flags != nil || part.Excl || part.Length || part.Width || part.IsSet
}

func hasUnsupportedParamExpansions(part *syntax.ParamExp) bool {
	return part.Index != nil || len(part.Modifiers) > 0 || part.Slice != nil || part.Repl != nil || part.Names != 0 || part.Exp != nil
}

func isSimpleEnvName(name string) bool {
	if name == "" {
		return false
	}
	if name[0] != '_' && (name[0] < 'A' || name[0] > 'Z') && (name[0] < 'a' || name[0] > 'z') {
		return false
	}
	for i := 1; i < len(name); i++ {
		ch := name[i]
		if ch == '_' {
			continue
		}
		if ch >= 'A' && ch <= 'Z' {
			continue
		}
		if ch >= 'a' && ch <= 'z' {
			continue
		}
		if ch >= '0' && ch <= '9' {
			continue
		}
		return false
	}
	return true
}

func closestEnvVar(name string, envNames []string, cfg distanceMatchConfig) (string, int, bool) {
	candidates := make([]envVarCandidate, 0, len(envNames))
	seen := make(map[string]struct{}, len(envNames))

	for _, envName := range envNames {
		if envName == "" {
			continue
		}
		if _, ok := seen[envName]; ok {
			continue
		}
		seen[envName] = struct{}{}

		distance := Distance(name, envName, cfg.keyboard)
		candidates = append(candidates, envVarCandidate{
			name:        envName,
			distance:    distance,
			similarity:  SimilarityFromDistance(len(name), len(envName), distance),
			lengthDelta: utils.Abs(len(name) - len(envName)),
			transposed:  utils.IsSingleAdjacentTransposition(name, envName),
		})
	}

	if len(candidates) == 0 {
		return "", 999, false
	}

	sort.Slice(candidates, func(i, j int) bool {
		if cmp := compareFuzzyCandidateOrder(
			candidates[i].distance, candidates[j].distance,
			candidates[i].transposed, candidates[j].transposed,
			candidates[i].similarity, candidates[j].similarity,
		); cmp != 0 {
			return cmp < 0
		}
		if candidates[i].lengthDelta != candidates[j].lengthDelta {
			return candidates[i].lengthDelta < candidates[j].lengthDelta
		}
		return candidates[i].name < candidates[j].name
	})

	best := candidates[0]
	if !isGoodCommandDistanceMatch(name, best.name, best.distance, cfg) {
		return "", best.distance, false
	}
	if len(candidates) > 1 && envCandidatesConflict(best, candidates[1]) {
		return "", best.distance, false
	}

	return best.name, best.distance, true
}

func envCandidatesConflict(a, b envVarCandidate) bool {
	return a.transposed == b.transposed &&
		a.distance == b.distance &&
		a.similarity == b.similarity &&
		a.lengthDelta == b.lengthDelta
}
