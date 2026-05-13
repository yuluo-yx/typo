package engine

import (
	"sort"

	"github.com/yuluo-yx/typo/internal/commands"
	"github.com/yuluo-yx/typo/internal/utils"
)

type commandRuneCandidate struct {
	name  string
	runes []rune
}

type commandCandidateIndex struct {
	all       []commandRuneCandidate
	byRuneLen map[int][]commandRuneCandidate
	minLen    int
	maxLen    int
}

const (
	fixSourceParser               = "parser"
	fixSourceHistory              = "history"
	fixSourceDistance             = "distance"
	longOptionSimilarityThreshold = 0.75
)

// Option is a functional option for Engine.

func (e *Engine) findClosestCommand(cmd string) string {
	matchCfg := e.distanceMatchConfig()
	bestMatch, bestDistance := e.closestKnownCommand(cmd)
	if isGoodCommandDistanceMatch(cmd, bestMatch, bestDistance, matchCfg) {
		return bestMatch
	}
	return ""
}

func (e *Engine) closestKnownCommand(cmd string) (string, int) {
	matchCfg := e.distanceMatchConfig()
	bestMatch, bestDistance := e.closestKnownCommandFromCandidates(cmd, e.availableCommandCandidates(cmd, matchCfg.maxEditDistance))
	if isGoodCommandDistanceMatch(cmd, bestMatch, bestDistance, matchCfg) || e.commandLoader == nil || e.commandsFullyLoad {
		return bestMatch, bestDistance
	}

	// Only scan PATH on demand when builtin or seeded commands cannot produce a good candidate.
	e.loadCommands()
	return e.closestKnownCommandFromCandidates(cmd, e.availableCommandCandidates(cmd, matchCfg.maxEditDistance))
}

func (e *Engine) rankedKnownCommandCandidates(cmd string) []commandCandidate {
	matchCfg := e.distanceMatchConfig()
	candidates := e.rankedKnownCommandCandidatesFrom(cmd, e.availableCommandCandidates(cmd, matchCfg.maxEditDistance), matchCfg)
	if len(candidates) > 0 || e.commandLoader == nil || e.commandsFullyLoad {
		return candidates
	}

	e.loadCommands()
	return e.rankedKnownCommandCandidatesFrom(cmd, e.availableCommandCandidates(cmd, matchCfg.maxEditDistance), matchCfg)
}

func (e *Engine) rankedKnownCommandCandidatesFrom(cmd string, knownCommands []commandRuneCandidate, matchCfg distanceMatchConfig) []commandCandidate {
	cmdRunes := []rune(cmd)
	candidates := make([]commandCandidate, 0, len(knownCommands))
	seen := make(map[string]bool, len(knownCommands))

	for _, known := range knownCommands {
		if known.name == "" || seen[known.name] {
			continue
		}
		seen[known.name] = true

		d := distanceRunes(cmdRunes, known.runes, e.keyboard)
		if !isGoodCommandDistanceMatch(cmd, known.name, d, matchCfg) {
			continue
		}
		candidates = append(candidates, commandCandidate{
			name:       known.name,
			distance:   d,
			similarity: SimilarityFromDistance(len(cmd), len(known.name), d),
			priority:   e.commandPriority(known.name),
			transposed: isSingleAdjacentTransposition(cmd, known.name),
		})
	}

	sortCommandCandidates(candidates)
	return candidates
}

func (e *Engine) closestKnownCommandFromCandidates(cmd string, knownCommands []commandRuneCandidate) (string, int) {
	cmdRunes := []rune(cmd)
	candidates := make([]commandCandidate, 0, len(knownCommands))
	seen := make(map[string]bool, len(knownCommands))

	for _, known := range knownCommands {
		if known.name == "" || seen[known.name] {
			continue
		}
		seen[known.name] = true

		d := distanceRunes(cmdRunes, known.runes, e.keyboard)
		candidates = append(candidates, commandCandidate{
			name:       known.name,
			distance:   d,
			similarity: SimilarityFromDistance(len(cmd), len(known.name), d),
			priority:   e.commandPriority(known.name),
			transposed: isSingleAdjacentTransposition(cmd, known.name),
		})
	}

	if len(candidates) == 0 {
		return "", 999
	}

	sortCommandCandidates(candidates)

	return candidates[0].name, candidates[0].distance
}

func (e *Engine) availableCommands() []string {
	e.ensureAvailableCommandsFresh()
	return e.availableCmds
}

func (e *Engine) availableCommandCandidates(cmd string, maxEditDistance int) []commandRuneCandidate {
	e.ensureAvailableCommandsFresh()
	return e.availableCmdIndex.candidatesFor(cmd, maxEditDistance)
}

func (e *Engine) isAvailableCommand(cmd string) bool {
	e.availableCommands() // ensure cache is fresh
	_, ok := e.availableCmdsSet[cmd]
	return ok
}

func (e *Engine) hasKnownCommand(cmd string) bool {
	if e.isAvailableCommand(cmd) {
		return true
	}
	if e.commandLoader == nil || e.commandsFullyLoad {
		return false
	}

	e.loadCommands()
	return e.isAvailableCommand(cmd)
}

func (e *Engine) loadCommands() {
	e.commandsLoadOnce.Do(func() {
		if e.commandLoader == nil {
			e.refreshAvailableCommands()
			e.commandsFullyLoad = true
			return
		}

		loaded := e.commandLoader()
		if debug := e.debugTrace(); debug != nil {
			debug.LoadedPATHCommands = true
			debug.LoadedPATHCommandCount = len(loaded)
		}
		e.setCommands(utils.MergeUniqueStrings(e.commands, e.filterDisabledCommands(loaded)...))
		e.refreshAvailableCommands()
		e.commandsFullyLoad = true
	})
}

func (e *Engine) setCommands(commands []string) {
	e.commands = append([]string(nil), commands...)
	e.commandsVersion++
}

func (e *Engine) addCommands(commands ...string) {
	if len(commands) == 0 {
		return
	}
	e.commands = append(e.commands, commands...)
	e.commandsVersion++
}

func (e *Engine) ensureAvailableCommandsFresh() {
	if e.availableCmdsVersion != e.commandsVersion {
		e.refreshAvailableCommands()
	}
}

func (e *Engine) refreshAvailableCommands() {
	e.availableCmds = e.filterDisabledCommands(e.commands)
	e.availableCmdRunes = commandRuneCandidatesFromStrings(e.availableCmds)
	e.availableCmdIndex = newCommandCandidateIndex(e.availableCmdRunes)
	e.availableCmdsSet = make(map[string]struct{}, len(e.availableCmds))
	for _, cmd := range e.availableCmds {
		e.availableCmdsSet[cmd] = struct{}{}
	}
	e.availableCmdsVersion = e.commandsVersion
}

func commandRuneCandidatesFromStrings(commands []string) []commandRuneCandidate {
	candidates := make([]commandRuneCandidate, 0, len(commands))
	for _, cmd := range commands {
		candidates = append(candidates, commandRuneCandidate{
			name:  cmd,
			runes: []rune(cmd),
		})
	}
	return candidates
}

func newCommandCandidateIndex(candidates []commandRuneCandidate) commandCandidateIndex {
	index := commandCandidateIndex{
		all:       candidates,
		byRuneLen: make(map[int][]commandRuneCandidate),
	}
	if len(candidates) == 0 {
		return index
	}

	index.minLen = len(candidates[0].runes)
	index.maxLen = index.minLen
	for _, candidate := range candidates {
		runeLen := len(candidate.runes)
		index.byRuneLen[runeLen] = append(index.byRuneLen[runeLen], candidate)
		if runeLen < index.minLen {
			index.minLen = runeLen
		}
		if runeLen > index.maxLen {
			index.maxLen = runeLen
		}
	}

	return index
}

func (index commandCandidateIndex) candidatesFor(cmd string, maxEditDistance int) []commandRuneCandidate {
	if len(index.all) == 0 {
		return nil
	}
	if maxEditDistance < 0 {
		return index.all
	}

	cmdLen := len([]rune(cmd))
	minLen := cmdLen - maxEditDistance
	if minLen < 0 {
		minLen = 0
	}
	maxLen := cmdLen + maxEditDistance
	if minLen <= index.minLen && maxLen >= index.maxLen {
		return index.all
	}

	candidates := make([]commandRuneCandidate, 0)
	for runeLen := minLen; runeLen <= maxLen; runeLen++ {
		candidates = append(candidates, index.byRuneLen[runeLen]...)
	}
	return candidates
}

func (e *Engine) filterDisabledCommands(commands []string) []string {
	if len(e.disabledCommands) == 0 {
		return commands
	}

	filtered := make([]string, 0, len(commands))
	for _, command := range commands {
		if !e.disabledCommands[command] {
			filtered = append(filtered, command)
		}
	}
	return filtered
}

type commandCandidate struct {
	name       string
	distance   int
	similarity float64
	priority   int
	transposed bool
}

func sortCommandCandidates(candidates []commandCandidate) {
	sort.Slice(candidates, func(i, j int) bool {
		// Exact matches must stay first; otherwise, prefer adjacent
		// transpositions over ordinary fuzzy matches from PATH.
		if cmp := compareFuzzyCandidateOrder(
			candidates[i].distance, candidates[j].distance,
			candidates[i].transposed, candidates[j].transposed,
			candidates[i].similarity, candidates[j].similarity,
		); cmp != 0 {
			return cmp < 0
		}
		if candidates[i].priority != candidates[j].priority {
			return candidates[i].priority > candidates[j].priority
		}
		return candidates[i].name < candidates[j].name
	})
}

func compareFuzzyCandidateOrder(distanceA, distanceB int, transposedA, transposedB bool, similarityA, similarityB float64) int {
	if distanceA == 0 || distanceB == 0 {
		switch {
		case distanceA < distanceB:
			return -1
		case distanceA > distanceB:
			return 1
		default:
			return 0
		}
	}
	if transposedA != transposedB {
		if transposedA {
			return -1
		}
		return 1
	}
	if distanceA != distanceB {
		if distanceA < distanceB {
			return -1
		}
		return 1
	}
	if similarityA != similarityB {
		if similarityA > similarityB {
			return -1
		}
		return 1
	}
	return 0
}

func (e *Engine) commandPriority(cmd string) int {
	score := e.rules.TargetPriority(cmd)

	if commands.IsCommonCommand(cmd) {
		score += 50
	}

	if commands.IsShellBuiltin(cmd) {
		score += 25
	}

	if e.toolTrees != nil && e.toolTrees.HasSubcommands(cmd) {
		score += 25
	}

	if e.commandTrees != nil && e.commandTrees.HasRoot(cmd) {
		score += 50
	}

	return score
}
