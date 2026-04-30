package engine

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

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
	disabledCommands     map[string]bool
	rules                *Rules
	history              *History
	parser               *parser.Registry
	commands             []string // Loaded command set, seeded first and expanded on demand.
	availableCmds        []string // Cached command set after disabled-command filtering.
	availableCmdRunes    []commandRuneCandidate
	availableCmdsSet     map[string]struct{} // O(1) lookup mirror of availableCmds.
	availableCmdsLen     int                 // Original commands length for `availableCmds`; detects stale cache after direct appends.
	commandLoader        func() []string
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

type commandRuneCandidate struct {
	name  string
	runes []rune
}

const (
	fixSourceParser               = "parser"
	fixSourceHistory              = "history"
	fixSourceDistance             = "distance"
	longOptionSimilarityThreshold = 0.75
)

// Option is a functional option for Engine.
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
	return func(e *Engine) { e.commands = append([]string(nil), cmds...) }
}

// WithCommandLoader sets the lazy loader for discovered commands.
func WithCommandLoader(loader func() []string) Option {
	return func(e *Engine) { e.commandLoader = loader }
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
		parser:              parser.NewRegistry(),
		commands:            []string{},
	}

	for _, opt := range opts {
		opt(e)
	}

	e.refreshAvailableCommands()

	return e
}

// EnableDebug enables per-fix debug tracing.
func (e *Engine) EnableDebug() {
	if e != nil {
		e.debugEnabled = true
	}
}

// DisableDebug disables per-fix debug tracing.
func (e *Engine) DisableDebug() {
	if e != nil {
		e.debugEnabled = false
		e.currentDebug = nil
	}
}

// Fix attempts to fix the given command.
// stderr is optional and used for error parsing.
func (e *Engine) Fix(cmd, stderr string) itypes.FixResult {
	return e.FixWithContext(itypes.ParserContext{
		Command: cmd,
		Stderr:  stderr,
	})
}

// FixWithContext attempts to fix the given command with parser context.
func (e *Engine) FixWithContext(input itypes.ParserContext) itypes.FixResult {
	input.Command = strings.TrimSpace(input.Command)
	debugInfo := e.beginDebugTrace(input)
	startedAt := time.Now()
	defer e.clearDebugTrace()

	if input.Command == "" {
		return e.attachDebug(itypes.FixResult{Fixed: false}, debugInfo, startedAt)
	}

	if len(input.AliasContext) > 0 {
		return e.attachDebug(e.fixWithAliasContext(input), debugInfo, startedAt)
	}

	return e.attachDebug(e.fixWithoutAliasContext(input), debugInfo, startedAt)
}

func (e *Engine) beginDebugTrace(input itypes.ParserContext) *itypes.FixDebugInfo {
	if e == nil || !e.debugEnabled {
		return nil
	}

	debugInfo := &itypes.FixDebugInfo{
		InputCommand:         input.Command,
		AliasContextProvided: len(input.AliasContext) > 0,
		AliasContextEntries:  len(input.AliasContext),
	}
	e.currentDebug = debugInfo
	return debugInfo
}

func (e *Engine) clearDebugTrace() {
	if e != nil {
		e.currentDebug = nil
	}
}

func (e *Engine) attachDebug(result itypes.FixResult, debugInfo *itypes.FixDebugInfo, startedAt time.Time) itypes.FixResult {
	if debugInfo != nil {
		debugInfo.EngineDuration = time.Since(startedAt)
		result.Debug = debugInfo
	}
	return result
}

func (e *Engine) debugTrace() *itypes.FixDebugInfo {
	if e == nil {
		return nil
	}
	return e.currentDebug
}

func (e *Engine) markDebugFeature(stage string) {
	debug := e.debugTrace()
	if debug == nil {
		return
	}

	switch stage {
	case "alias":
		debug.UsedAlias = true
		debug.AliasContextUsed = true
	case fixSourceParser:
		debug.UsedParser = true
	case fixSourceHistory:
		debug.UsedHistory = true
	case "rule":
		debug.UsedRule = true
	case "tree":
		debug.UsedCommandTree = true
	case "subcommand":
		debug.UsedSubcommand = true
	case fixSourceDistance:
		debug.UsedDistance = true
	case "env":
		debug.UsedEnv = true
	case "option":
		debug.UsedOption = true
	}
}

func (e *Engine) recordAcceptedFix(pass int, before string, result itypes.FixResult) {
	debug := e.debugTrace()
	if debug == nil || !result.Fixed {
		return
	}

	e.markDebugFeature(result.Source)
	if result.UsedParser {
		debug.UsedParser = true
	}

	debug.Events = append(debug.Events, itypes.FixDebugEvent{
		Pass:    pass,
		Stage:   result.Source,
		Before:  before,
		After:   result.Command,
		Message: result.Message,
	})
}

func (e *Engine) recordRejectedCandidate(stage, input, candidate string, distance int, similarity float64, reason string) {
	debug := e.debugTrace()
	if debug == nil || input == "" || candidate == "" || reason == "" {
		return
	}

	for _, existing := range debug.RejectedCandidates {
		if existing.Stage == stage && existing.Input == input && existing.Candidate == candidate {
			return
		}
	}
	if len(debug.RejectedCandidates) >= 5 {
		return
	}

	debug.RejectedCandidates = append(debug.RejectedCandidates, itypes.FixDebugCandidate{
		Stage:      stage,
		Input:      input,
		Candidate:  candidate,
		Distance:   distance,
		Similarity: similarity,
		Reason:     reason,
	})
}

func (e *Engine) fixWithoutAliasContext(input itypes.ParserContext) itypes.FixResult {
	input.Command = strings.TrimSpace(input.Command)
	if input.Command == "" {
		return itypes.FixResult{Fixed: false}
	}

	originalCmd := input.Command
	currentCmd := input.Command
	messages := make([]string, 0)
	lastSource := ""
	resultKind := ""
	usedParser := false

	passes := e.maxFixPasses
	if passes < 1 {
		passes = 1
	}

	for pass := 1; pass <= passes; pass++ {
		input.Command = currentCmd
		result := e.fixOnePass(input)
		if !isMeaningfulFix(currentCmd, result) {
			break
		}

		e.recordAcceptedFix(pass, currentCmd, result)
		currentCmd = result.Command
		lastSource = result.Source
		if result.Message != "" && !slices.Contains(messages, result.Message) {
			messages = append(messages, result.Message)
		}
		if resultKind == "" && result.Kind != "" {
			resultKind = result.Kind
		}
		if result.Source == fixSourceParser {
			usedParser = true
			// stderr only belongs to the failed command that triggered this fix.
			// Once a parser fix lands, later passes must not consume the same stderr again.
			input.Stderr = ""
		}
	}

	if currentCmd != originalCmd {
		return itypes.FixResult{
			Fixed:      true,
			Command:    currentCmd,
			Source:     lastSource,
			Message:    strings.Join(messages, "; "),
			Kind:       resultKind,
			UsedParser: usedParser,
		}
	}

	return itypes.FixResult{Fixed: false}
}

func (e *Engine) fixOnePass(input itypes.ParserContext) itypes.FixResult {
	cmd := input.Command

	// 1. Try error parser first (if stderr provided)
	if input.Stderr != "" {
		if result := e.tryParser(input); isMeaningfulFix(cmd, result) {
			return result
		}
	}

	// 2. Try explicit user rules
	if result := e.tryUserRules(cmd); isMeaningfulFix(cmd, result) {
		return result
	}

	// 3. Try history
	if result := e.tryHistory(cmd); isMeaningfulFix(cmd, result) {
		return result
	}

	// 4. Try command tree fix
	if result := e.tryCommandTreeFix(cmd); isMeaningfulFix(cmd, result) {
		return result
	}

	// 5. Try builtin rules
	if result := e.tryBuiltinRules(cmd); isMeaningfulFix(cmd, result) {
		return result
	}

	// 6. Try subcommand fix
	if result := e.trySubcommandFix(cmd); isMeaningfulFix(cmd, result) {
		return result
	}

	// 7. Try edit distance
	if result := e.tryDistance(cmd); isMeaningfulFix(cmd, result) {
		return result
	}

	// 8. Try environment variable fix
	if result := e.tryEnvVarFix(cmd, input.AliasContext); isMeaningfulFix(cmd, result) {
		return result
	}

	// 9. Try tool global option fix
	if e.longOptionFixEnabled {
		if result := e.tryToolOptionFix(cmd); isMeaningfulFix(cmd, result) {
			return result
		}
	}

	return itypes.FixResult{Fixed: false}
}

// FixCommand attempts to fix only the command word, preserving arguments.
func (e *Engine) FixCommand(cmd string) itypes.FixResult {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return itypes.FixResult{Fixed: false}
	}

	if result := e.fixCommandWordWithShell(cmd); result.Fixed {
		return result
	}

	// Split into command and args
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return itypes.FixResult{Fixed: false}
	}

	cmdWord := parts[0]
	args := parts[1:]

	// Try to fix just the command word
	if result := e.tryUserRules(cmdWord); result.Fixed {
		rebuilt := e.rebuildCommand(result.Command, args, "rule")
		if isMeaningfulFix(cmd, rebuilt) {
			return rebuilt
		}
	}

	if result := e.tryHistory(cmdWord); result.Fixed {
		rebuilt := e.rebuildCommand(result.Command, args, fixSourceHistory)
		if isMeaningfulFix(cmd, rebuilt) {
			return rebuilt
		}
	}

	if replacement := e.findCommandTreeRootForArgs(cmdWord, args); replacement != "" {
		rebuilt := e.rebuildCommand(replacement, args, "tree")
		if isMeaningfulFix(cmd, rebuilt) {
			return rebuilt
		}
	}

	if result := e.tryBuiltinRules(cmdWord); result.Fixed {
		rebuilt := e.rebuildCommand(result.Command, args, "rule")
		if isMeaningfulFix(cmd, rebuilt) {
			return rebuilt
		}
	}

	if result := e.tryDistance(cmdWord); result.Fixed {
		rebuilt := e.rebuildCommand(result.Command, args, fixSourceDistance)
		if isMeaningfulFix(cmd, rebuilt) {
			return rebuilt
		}
	}

	return itypes.FixResult{Fixed: false}
}

func (e *Engine) fixCommandWordWithShell(cmd string) itypes.FixResult {
	lines, err := parseShellCommandLines(cmd)
	if err != nil {
		return itypes.FixResult{Fixed: false}
	}

	for _, line := range lines {
		cmdWord := line.commandWord()

		if rule, ok := e.rules.MatchUser(cmdWord); ok {
			result := itypes.FixResult{
				Fixed:   true,
				Command: line.replaceCommandWord(rule.To),
				Source:  "rule",
			}
			if isMeaningfulFix(cmd, result) {
				return result
			}
		}

		if entry, ok := e.history.Lookup(cmdWord); ok {
			result := itypes.FixResult{
				Fixed:   true,
				Command: line.replaceCommandWord(entry.To),
				Source:  fixSourceHistory,
			}
			if isMeaningfulFix(cmd, result) {
				return result
			}
		}

		args := make([]string, 0, len(line.args)-line.commandIdx-1)
		for i := line.commandIdx + 1; i < len(line.args); i++ {
			args = append(args, line.args[i].Lit())
		}

		if replacement := e.findCommandTreeRootForArgs(cmdWord, args); replacement != "" {
			result := itypes.FixResult{
				Fixed:   true,
				Command: line.replaceCommandWord(replacement),
				Source:  "tree",
			}
			if isMeaningfulFix(cmd, result) {
				return result
			}
		}

		if rule, ok := e.rules.MatchBuiltin(cmdWord); ok {
			result := itypes.FixResult{
				Fixed:   true,
				Command: line.replaceCommandWord(rule.To),
				Source:  "rule",
			}
			if isMeaningfulFix(cmd, result) {
				return result
			}
		}

		if replacement := e.findClosestCommand(cmdWord); replacement != "" {
			result := itypes.FixResult{
				Fixed:   true,
				Command: line.replaceCommandWord(replacement),
				Source:  fixSourceDistance,
			}
			if isMeaningfulFix(cmd, result) {
				return result
			}
		}
	}

	return itypes.FixResult{Fixed: false}
}

func (e *Engine) tryParser(input itypes.ParserContext) itypes.FixResult {
	lines, err := parseShellCommandLines(input.Command)
	if err == nil {
		hasMultipleCommands := len(lines) > 1

		for _, line := range lines {
			result := e.parser.Parse(itypes.ParserContext{
				Command:             line.commandSuffixRaw(),
				Stderr:              input.Stderr,
				ExitCode:            input.ExitCode,
				HasMultipleCommands: hasMultipleCommands,
				HasRedirection:      line.hasRedirection,
				HasPrivilegeWrapper: line.hasWrapper("sudo"),
			})
			if result.Fixed {
				return itypes.FixResult{
					Fixed:   true,
					Command: line.replaceCommandSuffix(result.Command),
					Source:  fixSourceParser,
					Message: result.Message,
					Kind:    result.Kind,
				}
			}
		}

		return itypes.FixResult{Fixed: false}
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

	return e.isAvailableCommand(parts[0]) || commands.IsShellBuiltin(parts[0])
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
	for _, candidate := range e.availableCommands() {
		if candidate == cmdWord || !commands.IsCommonCommand(candidate) {
			continue
		}
		if utils.IsSingleAdjacentTransposition(cmdWord, candidate) {
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

func tryMatch(cmd, source string, match matchFunc) itypes.FixResult {
	// Try full command first
	if replacement, ok := match(cmd); ok {
		return itypes.FixResult{
			Fixed:   true,
			Command: replacement,
			Source:  source,
		}
	}

	// Try command word only
	parts := strings.Fields(cmd)
	if len(parts) > 1 {
		if replacement, ok := match(parts[0]); ok {
			return itypes.FixResult{
				Fixed:   true,
				Command: replacement + " " + strings.Join(parts[1:], " "),
				Source:  source,
			}
		}
	}

	return itypes.FixResult{Fixed: false}
}

func (e *Engine) tryMatchOnCommand(cmd, source string, match matchFunc) itypes.FixResult {
	lines, err := parseShellCommandLines(cmd)
	if err == nil {
		for _, line := range lines {
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

	return tryMatch(cmd, source, match)
}

func (e *Engine) tryDistance(cmd string) itypes.FixResult {
	if result, parsed := e.tryDistanceWithShell(cmd); parsed {
		return result
	}

	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return itypes.FixResult{Fixed: false}
	}

	cmdWord := parts[0]
	if e.isProtectedCommandWord(cmdWord) {
		return itypes.FixResult{Fixed: false}
	}

	// Find best match from known commands
	bestMatch, bestDistance := e.closestKnownCommand(cmdWord)
	bestSimilarity := SimilarityFromDistance(len(cmdWord), len(bestMatch), bestDistance)

	// Check if match is good enough
	// Threshold: distance <= 2 and similarity > 60%
	if bestMatch != "" && bestMatch != cmdWord && isGoodCommandDistanceMatch(cmdWord, bestMatch, bestDistance, e.distanceMatchConfig()) {
		result := itypes.FixResult{
			Fixed:   true,
			Command: bestMatch,
			Source:  fixSourceDistance,
		}

		// Add original args
		if len(parts) > 1 {
			result.Command = bestMatch + " " + strings.Join(parts[1:], " ")
		}

		// Also try to fix subcommand
		if e.toolTrees != nil {
			result = e.fixSubcommandInResult(result)
		}

		return result
	}
	if bestMatch != "" && bestMatch != cmdWord {
		e.recordRejectedCandidate(fixSourceDistance, cmdWord, bestMatch, bestDistance, bestSimilarity, "did not pass command distance threshold")
	}

	return itypes.FixResult{Fixed: false}
}

func (e *Engine) tryDistanceWithShell(cmd string) (itypes.FixResult, bool) {
	lines, err := parseShellCommandLines(cmd)
	if err != nil {
		return itypes.FixResult{Fixed: false}, false
	}

	matchCfg := e.distanceMatchConfig()

	for _, line := range lines {
		if e.isProtectedCommandWord(line.commandWord()) {
			continue
		}

		bestMatch, bestDistance := e.closestKnownCommand(line.commandWord())
		bestSimilarity := SimilarityFromDistance(len(line.commandWord()), len(bestMatch), bestDistance)
		if !isGoodCommandDistanceMatch(line.commandWord(), bestMatch, bestDistance, matchCfg) {
			if bestMatch != "" && bestMatch != line.commandWord() {
				e.recordRejectedCandidate(fixSourceDistance, line.commandWord(), bestMatch, bestDistance, bestSimilarity, "did not pass command distance threshold")
			}
			continue
		}
		if bestMatch == line.commandWord() {
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

		return result, true
	}

	return itypes.FixResult{Fixed: false}, true
}

func (e *Engine) tryToolOptionFix(cmd string) itypes.FixResult {
	if e.toolTrees == nil {
		return itypes.FixResult{Fixed: false}
	}

	if result, parsed := e.tryToolOptionFixWithShell(cmd); parsed {
		return result
	}

	parts := strings.Fields(cmd)
	if len(parts) < 2 {
		return itypes.FixResult{Fixed: false}
	}

	mainCmd := parts[0]
	resolvedCmd := ""
	if !e.isAvailableCommand(mainCmd) {
		resolved := e.findClosestCommand(mainCmd)
		if resolved == "" {
			return itypes.FixResult{Fixed: false}
		}
		resolvedCmd = resolved
		mainCmd = resolved
		parts[0] = resolved
	}

	idx, replacement, ok := e.findLongOptionFix(mainCmd, parts[1:])
	if !ok {
		return itypes.FixResult{Fixed: false}
	}

	parts[idx+1] = replacement
	if resolvedCmd != "" {
		parts[0] = resolvedCmd
	}

	name, _, _ := splitLongOptionToken(replacement)
	return itypes.FixResult{
		Fixed:   true,
		Command: strings.Join(parts, " "),
		Source:  "option",
		Message: fmt.Sprintf("did you mean: %s?", name),
	}
}

func (e *Engine) tryToolOptionFixWithShell(cmd string) (itypes.FixResult, bool) {
	if e.toolTrees == nil {
		return itypes.FixResult{Fixed: false}, true
	}

	lines, err := parseShellCommandLines(cmd)
	if err != nil {
		return itypes.FixResult{Fixed: false}, false
	}

	for _, line := range lines {
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
		}, true
	}

	return itypes.FixResult{Fixed: false}, true
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

	return optionTakesValue(mainCmd, arg) || toolOptionTakesValue(mainCmd, arg), true
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

	if result, parsed := e.trySubcommandFixWithShell(cmd); parsed {
		return result
	}

	parts := strings.Fields(cmd)
	if len(parts) < 2 {
		return itypes.FixResult{Fixed: false}
	}

	mainCmd := parts[0]

	// If main command is not known, try to resolve it first
	if !e.isAvailableCommand(mainCmd) {
		resolved := e.findClosestCommand(mainCmd)
		if resolved == "" {
			return itypes.FixResult{Fixed: false}
		}
		mainCmd = resolved
	}

	// Get subcommands for this tool
	subcmdIdx := findSubcommandIndex(mainCmd, parts)
	if subcmdIdx == -1 {
		return itypes.FixResult{Fixed: false}
	}

	fixedParts, changed := e.fixSubcommandParts(mainCmd, parts, subcmdIdx)
	if !changed {
		return itypes.FixResult{Fixed: false}
	}

	fixedParts[0] = mainCmd
	fixedCommand := strings.Join(fixedParts, " ")
	return itypes.FixResult{
		Fixed:   true,
		Command: fixedCommand,
		Source:  "subcommand",
		Message: fmt.Sprintf("did you mean: %s?", fixedCommand),
	}
}

func (e *Engine) trySubcommandFixWithShell(cmd string) (itypes.FixResult, bool) {
	lines, err := parseShellCommandLines(cmd)
	if err != nil {
		return itypes.FixResult{Fixed: false}, false
	}

	for _, line := range lines {
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
		}, true
	}

	return itypes.FixResult{Fixed: false}, true
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

func (e *Engine) fixSubcommandParts(mainCmd string, parts []string, startIdx int) ([]string, bool) {
	fixed := append([]string(nil), parts...)
	replacements, changed := e.collectSubcommandReplacements(mainCmd, fixed, startIdx)
	for _, replacement := range replacements {
		fixed[replacement.index] = replacement.value
	}

	return fixed, changed
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
				similarity := SimilarityFromDistance(len(token), len(match), distance)
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

		if optionTakesValue(mainCmd, name) || toolOptionTakesValue(mainCmd, name) {
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

	if optionTakesValue(mainCmd, token) || toolOptionTakesValue(mainCmd, token) {
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
	if e.isAvailableCommand(mainCmd) || e.isProtectedCommandWord(mainCmd) {
		return mainCmd, "", nil
	}

	resolved := e.findClosestCommand(mainCmd)
	if resolved == "" {
		return "", "", fmt.Errorf("unknown command: %s", mainCmd)
	}
	return resolved, resolved, nil
}

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
	bestMatch, bestDistance := e.closestKnownCommandFromCandidates(cmd, e.availableCommandCandidates())
	if isGoodCommandDistanceMatch(cmd, bestMatch, bestDistance, matchCfg) || e.commandLoader == nil || e.commandsFullyLoad {
		return bestMatch, bestDistance
	}

	// Only scan PATH on demand when builtin or seeded commands cannot produce a good candidate.
	e.loadCommands()
	return e.closestKnownCommandFromCandidates(cmd, e.availableCommandCandidates())
}

func (e *Engine) closestKnownCommandFromSlice(cmd string, knownCommands []string) (string, int) {
	return e.closestKnownCommandFromCandidates(cmd, commandRuneCandidatesFromStrings(knownCommands))
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
			transposed: utils.IsSingleAdjacentTransposition(cmd, known.name),
		})
	}

	if len(candidates) == 0 {
		return "", 999
	}

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

	return candidates[0].name, candidates[0].distance
}

func (e *Engine) availableCommands() []string {
	if len(e.commands) != e.availableCmdsLen {
		e.refreshAvailableCommands()
	}
	return e.availableCmds
}

func (e *Engine) availableCommandCandidates() []commandRuneCandidate {
	if len(e.commands) != e.availableCmdsLen {
		e.refreshAvailableCommands()
	}
	return e.availableCmdRunes
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
		e.commands = utils.MergeUniqueStrings(e.commands, e.filterDisabledCommands(loaded)...)
		e.refreshAvailableCommands()
		e.commandsFullyLoad = true
	})
}

func (e *Engine) refreshAvailableCommands() {
	e.availableCmds = e.filterDisabledCommands(e.commands)
	e.availableCmdRunes = commandRuneCandidatesFromStrings(e.availableCmds)
	e.availableCmdsSet = make(map[string]struct{}, len(e.availableCmds))
	for _, cmd := range e.availableCmds {
		e.availableCmdsSet[cmd] = struct{}{}
	}
	e.availableCmdsLen = len(e.commands)
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

func (e *Engine) isProtectedCommandWord(cmd string) bool {
	return e.rules.IsTarget(cmd) || e.history.IsTarget(cmd)
}

func (e *Engine) rebuildCommand(cmdWord string, args []string, source string) itypes.FixResult {
	result := itypes.FixResult{
		Fixed:   true,
		Command: cmdWord,
		Source:  source,
	}

	if len(args) > 0 {
		result.Command = cmdWord + " " + strings.Join(args, " ")
	}

	return result
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
		lengthDelta := utils.Abs(len(subcmd) - len(known))
		similarity := SimilarityFromDistance(len(subcmd), len(known), d)
		transposed := utils.IsSingleAdjacentTransposition(subcmd, known)
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

	return SimilarityFromDistance(len(original), len(candidate), distance) >= cfg.similarityThreshold
}

func isGoodSubcommandMatch(original, candidate string, distance int, cfg distanceMatchConfig) bool {
	if isGoodDistanceMatch(original, candidate, distance, cfg) {
		return true
	}

	if candidate == "" || distance > cfg.maxEditDistance {
		return false
	}

	if utils.IsSingleAdjacentTransposition(original, candidate) {
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

	return utils.IsSingleAdjacentTransposition(original, string(candidateRunes[:len(candidateRunes)-1]))
}

func isGoodCommandDistanceMatch(original, candidate string, distance int, cfg distanceMatchConfig) bool {
	if isGoodDistanceMatch(original, candidate, distance, cfg) {
		return true
	}

	if candidate == "" || distance > cfg.maxEditDistance {
		return false
	}

	return utils.IsSingleAdjacentTransposition(original, candidate)
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

func findSubcommandIndex(mainCmd string, parts []string) int {
	if len(parts) < 2 {
		return -1
	}

	expectValue := false
	for i := 1; i < len(parts); i++ {
		arg := parts[i]
		if expectValue {
			expectValue = false
			continue
		}

		if arg == "--" {
			if i+1 < len(parts) {
				return i + 1
			}
			return -1
		}

		if strings.HasPrefix(arg, "--") {
			if strings.Contains(arg, "=") {
				continue
			}
			if optionTakesValue(mainCmd, arg) {
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

func optionTakesValue(mainCmd, option string) bool {
	options, ok := subcommandPreOptionsWithValues[mainCmd]
	if !ok {
		return false
	}

	return options[option]
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

func toolOptionTakesValue(mainCmd, option string) bool {
	options, ok := builtinToolOptionsWithValues[mainCmd]
	return ok && options[option]
}

func closestToolOption(mainCmd, option string, cfg distanceMatchConfig) string {
	candidates := builtinToolOptions[mainCmd]
	if len(candidates) == 0 {
		return ""
	}

	bestMatch := ""
	bestDistance := 999
	for _, candidate := range candidates {
		if strings.HasPrefix(option, "--") != strings.HasPrefix(candidate, "--") {
			continue
		}
		if strings.HasPrefix(option, "-") && !strings.HasPrefix(option, "--") {
			if !strings.HasPrefix(candidate, "-") || strings.HasPrefix(candidate, "--") {
				continue
			}
		}

		distance := Distance(option, candidate, cfg.keyboard)
		if distance < bestDistance {
			bestDistance = distance
			bestMatch = candidate
		}
	}

	if !isGoodDistanceMatch(option, bestMatch, bestDistance, cfg) {
		return ""
	}

	return bestMatch
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

	if utils.IsSingleAdjacentTransposition(original, candidate) {
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
	if utils.IsSingleAdjacentTransposition(original, candidate) {
		return 1
	}

	originalRunes := []rune(original)
	candidateRunes := []rune(candidate)
	if utils.Abs(len(originalRunes)-len(candidateRunes)) != 1 {
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
		if utils.IsSingleAdjacentTransposition(reducedText, shorterText) {
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
	if utils.Abs(len(originalRunes)-len(candidateRunes)) > 1 {
		return false
	}

	originalLast := len(originalRunes) - 1
	candidateLast := len(candidateRunes) - 1
	return originalRunes[0] == candidateRunes[0] &&
		originalRunes[1] == candidateRunes[1] &&
		originalRunes[originalLast] == candidateRunes[candidateLast]
}

var subcommandPreOptionsWithValues = map[string]map[string]bool{
	"git": {
		"-C":             true,
		"-c":             true,
		"--config-env":   true,
		"--exec-path":    true,
		"--git-dir":      true,
		"--namespace":    true,
		"--super-prefix": true,
		"--work-tree":    true,
	},
	"docker": {
		"--config":    true,
		"--context":   true,
		"--host":      true,
		"--log-level": true,
		"-H":          true,
		"-l":          true,
	},
	"go": {
		"-C": true,
	},
	"kubectl": {
		"--as":                    true,
		"--as-group":              true,
		"--as-uid":                true,
		"--cache-dir":             true,
		"--certificate-authority": true,
		"--client-certificate":    true,
		"--client-key":            true,
		"--cluster":               true,
		"--context":               true,
		"--kubeconfig":            true,
		"--namespace":             true,
		"--password":              true,
		"--profile":               true,
		"--request-timeout":       true,
		"--server":                true,
		"--token":                 true,
		"--user":                  true,
		"--username":              true,
		"-n":                      true,
		"-s":                      true,
	},
	"npm": {
		"--cache":      true,
		"--prefix":     true,
		"--userconfig": true,
		"-C":           true,
	},
	"cargo": {
		"--color":   true,
		"--config":  true,
		"--explain": true,
		"-C":        true,
		"-Z":        true,
	},
	"terraform": {
		"-chdir": true,
	},
	"helm": {
		"--burst-limit":       true,
		"--host":              true,
		"--kube-apiserver":    true,
		"--kube-as-group":     true,
		"--kube-as-user":      true,
		"--kube-ca-file":      true,
		"--kube-context":      true,
		"--kube-token":        true,
		"--kubeconfig":        true,
		"--namespace":         true,
		"--qps":               true,
		"--registry-config":   true,
		"--repository-cache":  true,
		"--repository-config": true,
		"-n":                  true,
	},
	"aws": {
		"--ca-bundle":           true,
		"--cli-binary-format":   true,
		"--cli-connect-timeout": true,
		"--cli-read-timeout":    true,
		"--color":               true,
		"--endpoint-url":        true,
		"--output":              true,
		"--profile":             true,
		"--query":               true,
		"--region":              true,
	},
	"gcloud": {
		"--access-token-file":            true,
		"--account":                      true,
		"--billing-project":              true,
		"--configuration":                true,
		"--filter":                       true,
		"--flags-file":                   true,
		"--flatten":                      true,
		"--format":                       true,
		"--impersonate-service-account":  true,
		"--project":                      true,
		"--trace-token":                  true,
		"--user-output-enabled-log-file": true,
		"--verbosity":                    true,
	},
	"az": {
		"--output":       true,
		"--query":        true,
		"--subscription": true,
		"--tenant":       true,
	},
}

var builtinToolOptions = map[string][]string{
	"cargo": {"-C", "-V", "-Z", "-h", "-q", "-v", "--color", "--config", "--explain", "--frozen", "--help", "--list", "--locked", "--offline", "--quiet", "--verbose", "--version"},
}

var builtinToolOptionsWithValues = map[string]map[string]bool{
	"cargo": {
		"--color":   true,
		"--config":  true,
		"--explain": true,
		"-C":        true,
		"-Z":        true,
	},
}

// Learn stores a user-taught correction as a rule instead of history.
func (e *Engine) Learn(from, to string) error {
	if err := e.rules.AddUserRule(itypes.Rule{From: from, To: to}); err != nil {
		return err
	}

	return e.clearConflictingHistory(from)
}

// AddRule adds a user rule.
func (e *Engine) AddRule(from, to string) error {
	if err := e.rules.AddUserRule(itypes.Rule{From: from, To: to}); err != nil {
		return err
	}

	return e.clearConflictingHistory(from)
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
	return e.history.Record(from, to)
}

type commandCandidate struct {
	name       string
	distance   int
	similarity float64
	priority   int
	transposed bool
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

func (e *Engine) clearConflictingHistory(from string) error {
	if e.history == nil {
		return nil
	}

	return e.history.RemoveConflictsForRule(from)
}
