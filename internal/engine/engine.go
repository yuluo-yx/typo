package engine

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/yuluo-yx/typo/internal/commands"
	"github.com/yuluo-yx/typo/internal/parser"
	itypes "github.com/yuluo-yx/typo/internal/types"
	"github.com/yuluo-yx/typo/internal/utils"
)

// Engine is the main correction engine.
type Engine struct {
	keyboard             KeyboardWeights
	similarityThreshold  float64
	maxEditDistance      int
	maxFixPasses         int
	autoLearnThreshold   int
	longOptionFixEnabled bool
	pathFixEnabled       bool
	disabledCommands     map[string]bool
	rules                *Rules
	history              *History
	parser               *parser.Registry
	commands             []string // Loaded command set, seeded first and expanded on demand.
	availableCmds        []string // Cached command set after disabled-command filtering.
	availableCmdRunes    []commandRuneCandidate
	availableCmdIndex    commandCandidateIndex
	availableCmdsSet     map[string]struct{} // O(1) lookup mirror of availableCmds.
	commandsVersion      uint64              // Version of commands; increments whenever the loaded command set changes.
	availableCmdsVersion uint64              // Commands version represented by the available-command cache.
	commandLoader        func() []string
	commandLookup        func(string) bool // Optional single-command PATH lookup used before a full scan.
	commandLookupCache   map[string]bool   // Memoized commandLookup answers for this engine.
	commandsLoadOnce     sync.Once
	commandsFullyLoad    bool
	toolTrees            *commands.ToolTreeRegistry
	commandTrees         *commands.CommandTreeRegistry
	debugEnabled         bool
	currentDebug         *itypes.FixDebugInfo
}

type distanceMatchConfig struct {
	keyboard            KeyboardWeights
	maxEditDistance     int
	similarityThreshold float64
}

// Option configures an Engine during construction.
type Option func(*Engine)

// WithKeyboard sets the keyboard weights.
func WithKeyboard(kb KeyboardWeights) Option {
	return func(e *Engine) { e.keyboard = kb }
}

// WithSimilarityThreshold sets the minimum similarity threshold for edit-distance candidates.
func WithSimilarityThreshold(threshold float64) Option {
	return func(e *Engine) { e.similarityThreshold = threshold }
}

// WithMaxEditDistance sets the maximum allowed edit distance.
func WithMaxEditDistance(distance int) Option {
	return func(e *Engine) { e.maxEditDistance = distance }
}

// WithMaxFixPasses sets the maximum number of passes allowed in one fix chain.
func WithMaxFixPasses(passes int) Option {
	return func(e *Engine) { e.maxFixPasses = passes }
}

// WithAutoLearnThreshold sets the minimum history count needed before promoting a pair to a user rule.
func WithAutoLearnThreshold(threshold int) Option {
	return func(e *Engine) { e.autoLearnThreshold = threshold }
}

// WithExperimentalLongOptionFix enables the experimental long-option fix stage.
func WithExperimentalLongOptionFix(enabled bool) Option {
	return func(e *Engine) { e.longOptionFixEnabled = enabled }
}

// WithPathCorrection enables rewriting missing path arguments to the closest existing path.
func WithPathCorrection(enabled bool) Option {
	return func(e *Engine) { e.pathFixEnabled = enabled }
}

// WithDisabledCommands excludes commands from the candidate command set.
func WithDisabledCommands(commands []string) Option {
	return func(e *Engine) {
		if e.disabledCommands == nil {
			e.disabledCommands = make(map[string]bool, len(commands))
		}
		for _, command := range commands {
			if command == "" {
				continue
			}
			e.disabledCommands[command] = true
		}
	}
}

// WithRules sets the rules.
func WithRules(rules *Rules) Option {
	return func(e *Engine) { e.rules = rules }
}

// WithHistory sets the history.
func WithHistory(history *History) Option {
	return func(e *Engine) { e.history = history }
}

// WithParser sets the parser registry.
func WithParser(p *parser.Registry) Option {
	return func(e *Engine) { e.parser = p }
}

// WithCommands sets the known commands.
func WithCommands(cmds []string) Option {
	return func(e *Engine) { e.setCommands(cmds) }
}

// WithCommandLoader sets the lazy loader for discovered commands.
func WithCommandLoader(loader func() []string) Option {
	return func(e *Engine) { e.commandLoader = loader }
}

// WithCommandLookup sets a cheap existence check for one command, used instead of
// loading the whole PATH when only a yes/no answer is needed.
func WithCommandLookup(lookup func(string) bool) Option {
	return func(e *Engine) { e.commandLookup = lookup }
}

// WithToolTrees sets the external tool command tree registry.
func WithToolTrees(trees *commands.ToolTreeRegistry) Option {
	return func(e *Engine) { e.toolTrees = trees }
}

// WithCommandTrees sets the command tree registry.
func WithCommandTrees(trees *commands.CommandTreeRegistry) Option {
	return func(e *Engine) { e.commandTrees = trees }
}

// NewEngine creates a new correction engine.
func NewEngine(opts ...Option) *Engine {
	e := &Engine{
		keyboard:            DefaultKeyboard,
		similarityThreshold: 0.6,
		maxEditDistance:     2,
		maxFixPasses:        32,
		autoLearnThreshold:  3,
		disabledCommands:    make(map[string]bool),
		rules:               NewRules(""),
		history:             NewHistory(""),
		commands:            []string{},
	}

	for _, opt := range opts {
		opt(e)
	}

	e.refreshAvailableCommands()

	return e
}
func (e *Engine) tryParser(input itypes.ParserContext) itypes.FixResult {
	if e.parser == nil {
		e.parser = parser.NewRegistry()
	}

	lines, err := parseShellCommandLines(input.Command)
	if err == nil {
		hasMultipleCommands := len(lines) > 1
		usableLine := false
		var matchedLine *shellCommandLine
		var matchedResult itypes.ParserResult

		for _, line := range lines {
			command := line.commandSuffixRaw()
			if command == "" {
				continue
			}
			usableLine = true
			if matchedLine != nil &&
				!e.parser.CanParseCommand(matchedResult.Parser, line.commandWord()) {
				continue
			}
			result := e.parser.Parse(itypes.ParserContext{
				Command:             command,
				Stderr:              input.Stderr,
				ExitCode:            input.ExitCode,
				HasMultipleCommands: hasMultipleCommands,
				HasRedirection:      line.hasRedirection,
				HasPrivilegeWrapper: line.hasWrapper("sudo"),
			})
			if result.Fixed {
				if matchedLine != nil {
					return itypes.FixResult{Fixed: false}
				}
				matchedLine = line
				matchedResult = result
			}
		}

		if matchedLine != nil {
			return itypes.FixResult{
				Fixed:   true,
				Command: matchedLine.replaceCommandSuffix(matchedResult.Command),
				Source:  fixSourceParser,
				Message: matchedResult.Message,
				Kind:    matchedResult.Kind,
			}
		}
		if usableLine {
			return itypes.FixResult{Fixed: false}
		}
	}

	result := e.parser.Parse(itypes.ParserContext{
		Command:             input.Command,
		Stderr:              input.Stderr,
		ExitCode:            input.ExitCode,
		HasPrivilegeWrapper: strings.HasPrefix(strings.TrimSpace(input.Command), "sudo "),
		ShellParseFailed:    true,
	})
	if result.Fixed {
		return itypes.FixResult{
			Fixed:   true,
			Command: result.Command,
			Source:  fixSourceParser,
			Message: result.Message,
			Kind:    result.Kind,
		}
	}
	return itypes.FixResult{Fixed: false}
}

func (e *Engine) tryHistory(cmd string) itypes.FixResult {
	result := e.tryMatchOnCommand(cmd, fixSourceHistory, func(s string) (string, bool) {
		if e.shouldSkipHistoryLookup(s) {
			return "", false
		}

		entry, ok := e.history.Lookup(s)
		if !ok || e.shouldSkipHistoryEntry(s, entry.To) {
			return "", false
		}
		return entry.To, true
	})

	// If history fixed main command, also try to fix subcommand
	if result.Fixed && e.toolTrees != nil {
		result = e.fixSubcommandInResult(result)
	}

	return result
}

// shouldSkipHistoryLookup keeps implicit history from rewriting a single token
// that is already known. User rules still run earlier and remain the explicit
// way to override a valid command.
func (e *Engine) shouldSkipHistoryLookup(cmd string) bool {
	parts := strings.Fields(strings.TrimSpace(cmd))
	if len(parts) != 1 {
		return false
	}

	return e.commandExists(parts[0]) || commands.IsShellBuiltin(parts[0])
}

// shouldSkipHistoryEntry rejects stale history when the input is a likely
// adjacent transposition of a common command but history points somewhere else.
func (e *Engine) shouldSkipHistoryEntry(from, to string) bool {
	candidate := e.commonTranspositionCandidate(from)
	return candidate != "" && candidate != to
}

// commonTranspositionCandidate finds the built-in common command that would be
// reached by swapping one adjacent character pair in a single-word input.
func (e *Engine) commonTranspositionCandidate(cmd string) string {
	parts := strings.Fields(strings.TrimSpace(cmd))
	if len(parts) != 1 {
		return ""
	}

	cmdWord := parts[0]
	// Only common commands qualify, so scan that small set instead of all of PATH.
	for _, candidate := range commands.DiscoverCommon() {
		if candidate == cmdWord || !e.isAvailableCommand(candidate) {
			continue
		}
		if isSingleAdjacentTransposition(cmdWord, candidate) {
			return candidate
		}
	}

	return ""
}

func (e *Engine) tryUserRules(cmd string) itypes.FixResult {
	result := e.tryMatchOnCommand(cmd, "rule", func(s string) (string, bool) {
		rule, ok := e.rules.MatchUser(s)
		if ok {
			return rule.To, true
		}
		return "", false
	})

	// If rule fixed main command, also try to fix subcommand
	if result.Fixed && e.toolTrees != nil {
		result = e.fixSubcommandInResult(result)
	}

	return result
}

func (e *Engine) tryBuiltinRules(cmd string) itypes.FixResult {
	result := e.tryMatchOnCommand(cmd, "rule", func(s string) (string, bool) {
		rule, ok := e.rules.MatchBuiltin(s)
		if ok && isSingleCommandWord(s) && e.commandExists(s) {
			return "", false
		}
		if ok {
			return rule.To, true
		}
		return "", false
	})

	// If rule fixed main command, also try to fix subcommand
	if result.Fixed && e.toolTrees != nil {
		result = e.fixSubcommandInResult(result)
	}

	return result
}

func (e *Engine) fixSubcommandInResult(result itypes.FixResult) itypes.FixResult {
	if fixed := e.trySubcommandFixWithSource(result.Command, result.Source); fixed.Fixed {
		e.markDebugFeature("subcommand")
		return fixed
	}
	return result
}

type matchFunc func(string) (string, bool)

func (e *Engine) tryMatchOnCommand(cmd, source string, match matchFunc) itypes.FixResult {
	for _, line := range commandLinesOrFields(cmd) {
		if replacement, ok := match(line.commandSuffixRaw()); ok {
			return itypes.FixResult{
				Fixed:   true,
				Command: line.replaceCommandSuffixDedup(replacement),
				Source:  source,
			}
		}

		if replacement, ok := match(line.commandWord()); ok {
			return itypes.FixResult{
				Fixed:   true,
				Command: line.replaceCommandWord(replacement),
				Source:  source,
			}
		}
	}

	return itypes.FixResult{Fixed: false}
}

func (e *Engine) tryDistance(cmd string) itypes.FixResult {
	matchCfg := e.distanceMatchConfig()

	for _, line := range commandLinesOrFields(cmd) {
		cmdWord := line.commandWord()
		if e.isProtectedCommandWord(cmdWord) {
			continue
		}

		bestMatch, bestDistance := e.closestKnownCommand(cmdWord)
		if !isGoodCommandDistanceMatch(cmdWord, bestMatch, bestDistance, matchCfg) {
			if bestMatch != "" && bestMatch != cmdWord {
				bestSimilarity := SimilarityFromDistance(runeCount(cmdWord), runeCount(bestMatch), bestDistance)
				e.recordRejectedCandidate(fixSourceDistance, cmdWord, bestMatch, bestDistance, bestSimilarity, "did not pass command distance threshold")
			}
			continue
		}
		if bestMatch == cmdWord {
			continue
		}

		result := itypes.FixResult{
			Fixed:   true,
			Command: line.replaceCommandWord(bestMatch),
			Source:  fixSourceDistance,
		}

		if e.toolTrees != nil {
			result = e.fixSubcommandInResult(result)
		}

		return result
	}

	return itypes.FixResult{Fixed: false}
}

func (e *Engine) distanceFixCandidates(cmd string, limit int) []itypes.FixResult {
	if limit < 1 {
		return nil
	}

	results := make([]itypes.FixResult, 0, limit)
	for _, line := range commandLinesOrFields(cmd) {
		cmdWord := line.commandWord()
		if e.isProtectedCommandWord(cmdWord) || e.commandExists(cmdWord) {
			continue
		}
		for _, candidate := range e.rankedKnownCommandCandidates(cmdWord) {
			if candidate.name == cmdWord {
				continue
			}
			result := itypes.FixResult{
				Fixed:   true,
				Command: line.replaceCommandWord(candidate.name),
				Source:  fixSourceDistance,
			}
			if e.toolTrees != nil {
				result = e.fixSubcommandInResult(result)
			}
			if isMeaningfulFix(cmd, result) {
				results = append(results, result)
			}
			if len(results) >= limit {
				return results
			}
		}
	}

	return results
}

func (e *Engine) tryToolOptionFix(cmd string) itypes.FixResult {
	if e.toolTrees == nil {
		return itypes.FixResult{Fixed: false}
	}

	for _, line := range commandLinesOrFields(cmd) {
		mainCmd, resolvedCmd, resolveErr := e.resolveShellCommandLine(line)
		if resolveErr != nil {
			continue
		}

		args := make([]string, 0, len(line.args)-line.commandIdx-1)
		for i := line.commandIdx + 1; i < len(line.args); i++ {
			args = append(args, line.args[i].Lit())
		}

		idx, replacement, ok := e.findLongOptionFix(mainCmd, args)
		if !ok {
			continue
		}

		replacements := []shellWordReplacement{{
			index: line.commandIdx + 1 + idx,
			value: replacement,
		}}
		if resolvedCmd != "" {
			replacements = append(replacements, shellWordReplacement{index: line.commandIdx, value: resolvedCmd})
		}

		name, _, _ := splitLongOptionToken(replacement)
		return itypes.FixResult{
			Fixed:   true,
			Command: line.replaceWords(replacements...),
			Source:  "option",
			Message: fmt.Sprintf("did you mean: %s?", name),
		}
	}

	return itypes.FixResult{Fixed: false}
}

func (e *Engine) findLongOptionFix(mainCmd string, args []string) (int, string, bool) {
	if e == nil || e.toolTrees == nil || len(args) == 0 {
		return -1, "", false
	}

	matchCfg := e.longOptionMatchConfig()
	prefix := make([]string, 0, 3)
	expectValue := false

	for i, arg := range args {
		if expectValue {
			expectValue = false
			continue
		}

		if arg == "--" {
			break
		}

		if fixIndex, replacement, needsValue, handled := e.tryLongOptionToken(mainCmd, prefix, args, i, matchCfg); handled {
			if fixIndex >= 0 {
				return fixIndex, replacement, true
			}
			expectValue = needsValue
			continue
		}

		if needsValue, isShortOption := longOptionScanShortOption(mainCmd, arg); isShortOption {
			expectValue = needsValue
			continue
		}

		if canonical, ok := e.toolTrees.ResolveChild(mainCmd, prefix, arg); ok {
			prefix = append(prefix, canonical)
		}
	}

	return -1, "", false
}

func (e *Engine) tryLongOptionToken(
	mainCmd string,
	prefix []string,
	args []string,
	idx int,
	matchCfg distanceMatchConfig,
) (int, string, bool, bool) {
	arg := args[idx]
	name, suffix, isLongOption := splitLongOptionToken(arg)
	if !isLongOption {
		return -1, "", false, false
	}

	if e.toolTrees.HasLongOptionInScope(mainCmd, prefix, name) {
		return -1, "", e.toolTrees.LongOptionTakesValue(mainCmd, prefix, name) && suffix == "", true
	}

	replacement := closestLongOption(e.toolTrees.LongOptionsInScope(mainCmd, prefix), name, matchCfg)
	if replacement != "" && e.canApplyLongOptionReplacement(mainCmd, prefix, args, idx, replacement, suffix, matchCfg) {
		return idx, replacement + suffix, false, true
	}

	return -1, "", e.toolTrees.LongOptionTakesValue(mainCmd, prefix, name) && suffix == "", true
}

func longOptionScanShortOption(mainCmd, arg string) (bool, bool) {
	if !isShortToolOption(arg) {
		return false, false
	}

	return optionTakesValue(mainCmd, arg), true
}

func (e *Engine) canApplyLongOptionReplacement(
	mainCmd string,
	prefix []string,
	args []string,
	idx int,
	replacement string,
	suffix string,
	matchCfg distanceMatchConfig,
) bool {
	if e == nil || e.toolTrees == nil || replacement == "" {
		return false
	}
	if suffix != "" || !e.toolTrees.LongOptionTakesValue(mainCmd, prefix, replacement) {
		return true
	}

	nextToken := tokenAt(args, idx+1)
	if nextToken == "" || nextToken == "--" || strings.HasPrefix(nextToken, "-") {
		return false
	}

	subcommands := e.toolTrees.GetChildren(mainCmd, prefix)
	if len(subcommands) > 0 && tokenAt(args, idx+2) == "" {
		return false
	}
	return !isSubcommandCandidate(nextToken, subcommands, matchCfg)
}

func (e *Engine) trySubcommandFix(cmd string) itypes.FixResult {
	if e.toolTrees == nil {
		return itypes.FixResult{Fixed: false}
	}

	for _, line := range commandLinesOrFields(cmd) {
		mainCmd, resolvedCmd, resolveErr := e.resolveShellCommandLine(line)
		if resolveErr != nil {
			continue
		}

		subcmdIdx := findSubcommandWordIndex(mainCmd, line)
		if subcmdIdx == -1 {
			continue
		}

		replacements, changed := e.fixSubcommandWords(mainCmd, line, subcmdIdx)
		if !changed {
			continue
		}

		if resolvedCmd != "" {
			replacements = append(replacements, shellWordReplacement{index: line.commandIdx, value: resolvedCmd})
		}

		fixedCommand := line.replaceWords(replacements...)
		return itypes.FixResult{
			Fixed:   true,
			Command: fixedCommand,
			Source:  "subcommand",
			Message: fmt.Sprintf("did you mean: %s?", fixedCommand),
		}
	}

	return itypes.FixResult{Fixed: false}
}

func (e *Engine) trySubcommandFixWithSource(cmd, source string) itypes.FixResult {
	result := e.trySubcommandFix(cmd)
	if result.Fixed && source != "" {
		result.Source = source
	}
	return result
}

type subcommandReplacement struct {
	index int
	value string
}

func (e *Engine) fixSubcommandWords(mainCmd string, line *shellCommandLine, startIdx int) ([]shellWordReplacement, bool) {
	tokens := make([]string, len(line.args))
	for i, arg := range line.args {
		tokens[i] = arg.Lit()
	}

	updates, changed := e.collectSubcommandReplacements(mainCmd, tokens, startIdx)
	replacements := make([]shellWordReplacement, 0, len(updates))
	for _, update := range updates {
		replacements = append(replacements, shellWordReplacement(update))
	}

	return replacements, changed
}

func (e *Engine) collectSubcommandReplacements(mainCmd string, tokens []string, startIdx int) ([]subcommandReplacement, bool) {
	cfg := e.distanceMatchConfig()
	prefix := make([]string, 0, len(tokens)-startIdx)
	replacements := make([]subcommandReplacement, 0)
	changed := false
	expectValue := false

	for i := startIdx; i < len(tokens); i++ {
		token := tokens[i]
		if expectValue {
			expectValue = false
			continue
		}

		subcommands := e.toolTrees.GetChildren(mainCmd, prefix)
		if len(subcommands) == 0 {
			break
		}

		if token == "--" {
			break
		}

		if handled, needsValue := subcommandOptionBehavior(mainCmd, token, subcommands, tokenAt(tokens, i+1), tokenAt(tokens, i+2), cfg); handled {
			expectValue = needsValue
			continue
		}

		if canonical, ok := e.toolTrees.ResolveChild(mainCmd, prefix, token); ok {
			if canonical != token {
				replacements = append(replacements, subcommandReplacement{index: i, value: canonical})
				changed = true
			}
			prefix = append(prefix, canonical)
			continue
		}

		match, distance := closestSubcommand(token, subcommands, cfg)
		if !isGoodSubcommandMatch(token, match, distance, cfg) {
			if match != "" && match != token {
				similarity := SimilarityFromDistance(runeCount(token), runeCount(match), distance)
				e.recordRejectedCandidate("subcommand", token, match, distance, similarity, "did not pass subcommand distance threshold")
			}
			break
		}

		canonical := match
		if resolved, ok := e.toolTrees.ResolveChild(mainCmd, prefix, match); ok {
			canonical = resolved
		}

		replacements = append(replacements, subcommandReplacement{index: i, value: canonical})
		prefix = append(prefix, canonical)
		changed = true
	}

	return replacements, changed
}

func subcommandOptionBehavior(mainCmd, token string, subcommands []string, nextToken, nextNextToken string, cfg distanceMatchConfig) (bool, bool) {
	if token == "--" {
		return false, false
	}

	name, suffix, isOption := splitToolOptionToken(token)
	if isOption {
		if suffix != "" {
			return true, false
		}

		if optionTakesValue(mainCmd, name) {
			return true, true
		}

		if shouldTreatNextTokenAsOptionValue(nextToken, nextNextToken, subcommands, cfg) {
			return true, true
		}

		return true, false
	}

	if !strings.HasPrefix(token, "-") || token == "-" {
		return false, false
	}

	if optionTakesValue(mainCmd, token) {
		return true, true
	}

	if shouldTreatNextTokenAsOptionValue(nextToken, nextNextToken, subcommands, cfg) {
		return true, true
	}

	return true, false
}

func shouldTreatNextTokenAsOptionValue(nextToken, nextNextToken string, subcommands []string, cfg distanceMatchConfig) bool {
	if nextToken == "" || nextNextToken == "" {
		return false
	}

	if nextToken == "--" || strings.HasPrefix(nextToken, "-") {
		return false
	}

	if isSubcommandCandidate(nextToken, subcommands, cfg) {
		return false
	}

	// Hierarchical subcommands can contain `--flag value`; use next-level candidates as a conservative boundary.
	return isSubcommandCandidate(nextNextToken, subcommands, cfg)
}

func isSubcommandCandidate(token string, subcommands []string, cfg distanceMatchConfig) bool {
	if slices.Contains(subcommands, token) {
		return true
	}

	match, distance := closestSubcommand(token, subcommands, cfg)
	return isGoodSubcommandMatch(token, match, distance, cfg)
}

func tokenAt(tokens []string, idx int) string {
	if idx < 0 || idx >= len(tokens) {
		return ""
	}

	return tokens[idx]
}

func (e *Engine) resolveShellCommandLine(line *shellCommandLine) (string, string, error) {
	mainCmd := line.commandWord()
	if e.commandExists(mainCmd) || e.isProtectedCommandWord(mainCmd) {
		return mainCmd, "", nil
	}

	resolved := e.findClosestCommand(mainCmd)
	if resolved == "" {
		return "", "", fmt.Errorf("unknown command: %s", mainCmd)
	}
	return resolved, resolved, nil
}

func (e *Engine) distanceMatchConfig() distanceMatchConfig {
	return distanceMatchConfig{
		keyboard:            e.keyboard,
		maxEditDistance:     e.maxEditDistance,
		similarityThreshold: e.similarityThreshold,
	}
}

func (e *Engine) longOptionMatchConfig() distanceMatchConfig {
	cfg := e.distanceMatchConfig()
	if cfg.similarityThreshold < longOptionSimilarityThreshold {
		cfg.similarityThreshold = longOptionSimilarityThreshold
	}
	return cfg
}

func (e *Engine) isProtectedCommandWord(cmd string) bool {
	return e.rules.IsTarget(cmd) || e.history.IsTarget(cmd)
}

func closestSubcommand(subcmd string, knownSubcommands []string, cfg distanceMatchConfig) (string, int) {
	bestMatch := ""
	bestDistance := 999
	bestLengthDelta := 999
	bestSimilarity := -1.0
	bestTransposition := false

	for _, known := range knownSubcommands {
		d := Distance(subcmd, known, cfg.keyboard)
		if !isGoodSubcommandMatch(subcmd, known, d, cfg) {
			continue
		}
		lengthDelta := absInt(runeCount(subcmd) - runeCount(known))
		similarity := SimilarityFromDistance(runeCount(subcmd), runeCount(known), d)
		transposed := isSingleAdjacentTransposition(subcmd, known)
		if d < bestDistance ||
			(d == bestDistance && transposed && !bestTransposition) ||
			(d == bestDistance && transposed == bestTransposition && lengthDelta < bestLengthDelta) ||
			(d == bestDistance && transposed == bestTransposition && lengthDelta == bestLengthDelta && similarity > bestSimilarity) {
			bestDistance = d
			bestLengthDelta = lengthDelta
			bestMatch = known
			bestSimilarity = similarity
			bestTransposition = transposed
		}
	}

	return bestMatch, bestDistance
}

func isGoodDistanceMatch(original, candidate string, distance int, cfg distanceMatchConfig) bool {
	if candidate == "" || distance > cfg.maxEditDistance {
		return false
	}

	return SimilarityFromDistance(runeCount(original), runeCount(candidate), distance) >= cfg.similarityThreshold
}

func isGoodSubcommandMatch(original, candidate string, distance int, cfg distanceMatchConfig) bool {
	if isGoodDistanceMatch(original, candidate, distance, cfg) {
		return true
	}

	if candidate == "" || distance > cfg.maxEditDistance {
		return false
	}

	if isSingleAdjacentTransposition(original, candidate) {
		return true
	}

	if isShortPluralTranspositionMatch(original, candidate) {
		return true
	}

	return isShortBoundaryPreservingMatch(original, candidate, distance)
}

func isShortPluralTranspositionMatch(original, candidate string) bool {
	originalRunes := []rune(original)
	candidateRunes := []rune(candidate)
	if len(originalRunes) < 3 || len(originalRunes) > 4 || len(candidateRunes) != len(originalRunes)+1 {
		return false
	}
	if candidateRunes[len(candidateRunes)-1] != 's' {
		return false
	}

	return isSingleAdjacentTransposition(original, string(candidateRunes[:len(candidateRunes)-1]))
}

func isGoodCommandDistanceMatch(original, candidate string, distance int, cfg distanceMatchConfig) bool {
	if isGoodDistanceMatch(original, candidate, distance, cfg) {
		return true
	}

	if candidate == "" || distance > cfg.maxEditDistance {
		return false
	}

	return isSingleAdjacentTransposition(original, candidate)
}

func isMeaningfulFix(original string, result itypes.FixResult) bool {
	if !result.Fixed {
		return false
	}

	return !commandsEquivalent(original, result.Command)
}

func commandsEquivalent(a, b string) bool {
	aParts := strings.Fields(strings.TrimSpace(a))
	bParts := strings.Fields(strings.TrimSpace(b))

	if len(aParts) != len(bParts) {
		return false
	}

	for i := range aParts {
		if aParts[i] != bParts[i] {
			return false
		}
	}

	return true
}

func findSubcommandWordIndex(mainCmd string, line *shellCommandLine) int {
	if line.commandIdx+1 >= len(line.args) {
		return -1
	}

	expectValue := false
	for i := line.commandIdx + 1; i < len(line.args); i++ {
		arg := line.args[i].Lit()
		if expectValue {
			expectValue = false
			continue
		}

		if arg == "--" {
			if i+1 < len(line.args) {
				return i + 1
			}
			return -1
		}

		if strings.HasPrefix(arg, "--") {
			name, _, hasInlineValue := utils.SplitInlineValue(arg)
			if hasInlineValue {
				if optionTakesValue(mainCmd, name) {
					continue
				}
			}
			if optionTakesValue(mainCmd, name) {
				expectValue = true
			}
			continue
		}

		if strings.HasPrefix(arg, "-") && arg != "-" {
			if optionTakesValue(mainCmd, arg) {
				expectValue = true
			}
			continue
		}

		return i
	}

	return -1
}

// optionTakesValue reports whether a global option before the subcommand consumes the next token.
func optionTakesValue(mainCmd, option string) bool {
	return commands.PreSubcommandOptionTakesValue(mainCmd, option)
}

func splitToolOptionToken(arg string) (name string, suffix string, isOption bool) {
	if strings.HasPrefix(arg, "--") {
		if name, suffix, ok := utils.SplitInlineValue(arg); ok {
			return name, suffix, true
		}
		return arg, "", true
	}

	if strings.HasPrefix(arg, "-") && arg != "-" {
		if len(arg) > 2 {
			return "", "", false
		}
		return arg, "", true
	}

	return "", "", false
}

func splitLongOptionToken(arg string) (name string, suffix string, isOption bool) {
	if !strings.HasPrefix(arg, "--") || arg == "--" {
		return "", "", false
	}
	if name, suffix, ok := utils.SplitInlineValue(arg); ok {
		return name, suffix, true
	}
	return arg, "", true
}

func isShortToolOption(arg string) bool {
	return strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && arg != "-"
}

func closestLongOption(candidates []string, option string, cfg distanceMatchConfig) string {
	if len(candidates) == 0 || !strings.HasPrefix(option, "--") || option == "--" {
		return ""
	}

	bestMatch := ""
	bestDistance := 999
	for _, candidate := range candidates {
		if !strings.HasPrefix(candidate, "--") || candidate == "--" {
			continue
		}

		distance := Distance(option, candidate, cfg.keyboard)
		if distance < bestDistance {
			bestDistance = distance
			bestMatch = candidate
		}
	}

	if !isGoodLongOptionMatch(option, bestMatch, bestDistance, cfg) {
		return ""
	}

	return bestMatch
}

func isGoodLongOptionMatch(original, candidate string, distance int, cfg distanceMatchConfig) bool {
	if isGoodDistanceMatch(original, candidate, distance, cfg) {
		return true
	}

	if candidate == "" || cfg.maxEditDistance < 1 {
		return false
	}

	if isSingleAdjacentTransposition(original, candidate) {
		return cfg.maxEditDistance >= 1
	}

	heuristicDistance := longOptionHeuristicDistance(original, candidate)
	if heuristicDistance <= 0 || heuristicDistance > cfg.maxEditDistance {
		return false
	}

	return isLongBoundaryPreservingMatch(original, candidate)
}

func longOptionHeuristicDistance(original, candidate string) int {
	original = strings.TrimPrefix(original, "--")
	candidate = strings.TrimPrefix(candidate, "--")

	if original == candidate {
		return 0
	}
	if isSingleAdjacentTransposition(original, candidate) {
		return 1
	}

	originalRunes := []rune(original)
	candidateRunes := []rune(candidate)
	if absInt(len(originalRunes)-len(candidateRunes)) != 1 {
		return 999
	}

	longer := candidateRunes
	shorter := originalRunes
	if len(originalRunes) > len(candidateRunes) {
		longer = originalRunes
		shorter = candidateRunes
	}

	for i := range longer {
		reduced := make([]rune, 0, len(longer)-1)
		reduced = append(reduced, longer[:i]...)
		reduced = append(reduced, longer[i+1:]...)

		reducedText := string(reduced)
		shorterText := string(shorter)
		if reducedText == shorterText {
			return 1
		}
		if isSingleAdjacentTransposition(reducedText, shorterText) {
			return 2
		}
	}

	return 999
}

func isLongBoundaryPreservingMatch(original, candidate string) bool {
	original = strings.TrimPrefix(original, "--")
	candidate = strings.TrimPrefix(candidate, "--")

	originalRunes := []rune(original)
	candidateRunes := []rune(candidate)
	if len(originalRunes) < 6 || len(candidateRunes) < 6 {
		return false
	}
	if absInt(len(originalRunes)-len(candidateRunes)) > 1 {
		return false
	}

	originalLast := len(originalRunes) - 1
	candidateLast := len(candidateRunes) - 1
	return originalRunes[0] == candidateRunes[0] &&
		originalRunes[1] == candidateRunes[1] &&
		originalRunes[originalLast] == candidateRunes[candidateLast]
}

// Learn stores a user-taught correction as a rule instead of history.
func (e *Engine) Learn(from, to string) error {
	return e.AddRule(from, to)
}

// AddRule adds a user rule.
func (e *Engine) AddRule(from, to string) error {
	return e.withPersistentState(context.Background(), func() error {
		return e.storeUserRule(from, to)
	})
}

func (e *Engine) storeUserRule(from, to string) error {
	rule, err := normalizeUserRule(itypes.Rule{From: from, To: to})
	if err != nil {
		return err
	}

	previous, existed := e.rules.MatchUser(rule.From)
	if err := e.rules.AddUserRule(rule); err != nil {
		return err
	}
	if err := e.clearConflictingHistory(rule.From); err != nil {
		var rollbackErr error
		if existed {
			rollbackErr = e.rules.AddUserRule(previous)
		} else {
			rollbackErr = e.rules.RemoveUserRule(rule.From)
		}
		if rollbackErr != nil {
			return errors.Join(err, fmt.Errorf("roll back user rule: %w", rollbackErr))
		}
		return err
	}
	return nil
}

// RemoveRule removes a user rule and clears its related correction history in the same operation.
func (e *Engine) RemoveRule(from string) error {
	return e.withPersistentState(context.Background(), func() error {
		return e.removeRule(from)
	})
}

func (e *Engine) removeRule(from string) error {
	from = strings.TrimSpace(from)
	previous, existed := e.rules.MatchUser(from)
	if !existed {
		return ErrRuleNotFound
	}
	if err := e.rules.RemoveUserRule(from); err != nil {
		return err
	}
	if err := e.clearConflictingHistory(from); err != nil {
		if rollbackErr := e.rules.AddUserRule(previous); rollbackErr != nil {
			return errors.Join(err, fmt.Errorf("roll back removed user rule: %w", rollbackErr))
		}
		return err
	}
	return nil
}

// ListRules returns all rules.
func (e *Engine) ListRules() []itypes.Rule {
	return e.rules.ListRules()
}

// ListHistory returns all history entries.
func (e *Engine) ListHistory() []itypes.HistoryEntry {
	return e.history.List()
}

// RecordHistory records a correction that actually happened.
func (e *Engine) RecordHistory(from, to string) error {
	return e.withPersistentState(context.Background(), func() error {
		return e.history.Record(from, to)
	})
}

// RecordHistoryAndAutoLearn records an accepted correction and, inside the same
// locked transaction, promotes the pair to a user rule once it reaches the
// threshold. Sharing one lock avoids reloading and rewriting state twice per fix.
// ctx bounds only the auto-learn step; the history write is never skipped.
func (e *Engine) RecordHistoryAndAutoLearn(ctx context.Context, from, to string) (itypes.AutoLearnDebugInfo, error) {
	var info itypes.AutoLearnDebugInfo
	err := e.withPersistentState(context.Background(), func() error {
		if err := e.history.Record(from, to); err != nil {
			return err
		}
		info = toAutoLearnDebugInfo(e.maybeAutoLearnFromHistory(ctx, from, to))
		info.Attempted = true
		return nil
	})
	return info, err
}

func (e *Engine) clearConflictingHistory(from string) error {
	if e.history == nil {
		return nil
	}

	return e.history.RemoveConflictsForRule(from)
}
