package engine

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/yuluo-yx/typo/internal/commands"
	"github.com/yuluo-yx/typo/internal/parser"
)

// FixResult represents the result of a fix attempt.
type FixResult struct {
	Fixed      bool   // Whether a fix was found
	Command    string // The corrected command
	Source     string // Where the fix came from (history, rule, parser, distance, subcommand)
	Message    string // Optional message to display
	Kind       string // Internal result tag used for extra handling.
	UsedParser bool   // Whether parser context was consumed in the fix chain.
}

// Engine is the main correction engine.
type Engine struct {
	keyboard            KeyboardWeights
	similarityThreshold float64
	maxEditDistance     int
	maxFixPasses        int
	disabledCommands    map[string]bool
	rules               *Rules
	history             *History
	parser              *parser.Registry
	commands            []string            // Loaded command set, seeded first and expanded on demand.
	availableCmds       []string            // Cached command set after disabled-command filtering.
	availableCmdsSet    map[string]struct{} // O(1) lookup mirror of availableCmds.
	availableCmdsLen    int                 // `availableCmds` 对应的原始 commands 长度，用于检测直接追加后的缓存失效。
	commandLoader       func() []string
	commandsLoadOnce    sync.Once
	commandsFullyLoad   bool
	toolTrees           *commands.ToolTreeRegistry
	commandTrees        *commands.CommandTreeRegistry
}

type distanceMatchConfig struct {
	keyboard            KeyboardWeights
	maxEditDistance     int
	similarityThreshold float64
}

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

// WithToolTrees 设置外部工具命令树注册表。
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

// Fix attempts to fix the given command.
// stderr is optional and used for error parsing.
func (e *Engine) Fix(cmd, stderr string) FixResult {
	return e.FixWithContext(parser.Context{
		Command: cmd,
		Stderr:  stderr,
	})
}

// FixWithContext attempts to fix the given command with parser context.
func (e *Engine) FixWithContext(input parser.Context) FixResult {
	input.Command = strings.TrimSpace(input.Command)
	if input.Command == "" {
		return FixResult{Fixed: false}
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

	for range passes {
		input.Command = currentCmd
		result := e.fixOnePass(input)
		if !isMeaningfulFix(currentCmd, result) {
			break
		}

		currentCmd = result.Command
		lastSource = result.Source
		if result.Message != "" && !containsString(messages, result.Message) {
			messages = append(messages, result.Message)
		}
		if resultKind == "" && result.Kind != "" {
			resultKind = result.Kind
		}
		if result.Source == "parser" {
			usedParser = true
			// stderr only belongs to the failed command that triggered this fix.
			// Once a parser fix lands, later passes must not consume the same stderr again.
			input.Stderr = ""
		}
	}

	if currentCmd != originalCmd {
		return FixResult{
			Fixed:      true,
			Command:    currentCmd,
			Source:     lastSource,
			Message:    strings.Join(messages, "; "),
			Kind:       resultKind,
			UsedParser: usedParser,
		}
	}

	return FixResult{Fixed: false}
}

func (e *Engine) fixOnePass(input parser.Context) FixResult {
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

	// 6. Try tool global option fix
	if result := e.tryToolOptionFix(cmd); isMeaningfulFix(cmd, result) {
		return result
	}

	// 7. Try subcommand fix
	if result := e.trySubcommandFix(cmd); isMeaningfulFix(cmd, result) {
		return result
	}

	// 8. Try edit distance
	if result := e.tryDistance(cmd); isMeaningfulFix(cmd, result) {
		return result
	}

	return FixResult{Fixed: false}
}

// FixCommand attempts to fix only the command word, preserving arguments.
func (e *Engine) FixCommand(cmd string) FixResult {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return FixResult{Fixed: false}
	}

	if result := e.fixCommandWordWithShell(cmd); result.Fixed {
		return result
	}

	// Split into command and args
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return FixResult{Fixed: false}
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
		rebuilt := e.rebuildCommand(result.Command, args, "history")
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
		rebuilt := e.rebuildCommand(result.Command, args, "distance")
		if isMeaningfulFix(cmd, rebuilt) {
			return rebuilt
		}
	}

	return FixResult{Fixed: false}
}

func (e *Engine) fixCommandWordWithShell(cmd string) FixResult {
	lines, err := parseShellCommandLines(cmd)
	if err != nil {
		return FixResult{Fixed: false}
	}

	for _, line := range lines {
		cmdWord := line.commandWord()

		if rule, ok := e.rules.MatchUser(cmdWord); ok {
			result := FixResult{
				Fixed:   true,
				Command: line.replaceCommandWord(rule.To),
				Source:  "rule",
			}
			if isMeaningfulFix(cmd, result) {
				return result
			}
		}

		if entry, ok := e.history.Lookup(cmdWord); ok {
			result := FixResult{
				Fixed:   true,
				Command: line.replaceCommandWord(entry.To),
				Source:  "history",
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
			result := FixResult{
				Fixed:   true,
				Command: line.replaceCommandWord(replacement),
				Source:  "tree",
			}
			if isMeaningfulFix(cmd, result) {
				return result
			}
		}

		if rule, ok := e.rules.MatchBuiltin(cmdWord); ok {
			result := FixResult{
				Fixed:   true,
				Command: line.replaceCommandWord(rule.To),
				Source:  "rule",
			}
			if isMeaningfulFix(cmd, result) {
				return result
			}
		}

		if replacement := e.findClosestCommand(cmdWord); replacement != "" {
			result := FixResult{
				Fixed:   true,
				Command: line.replaceCommandWord(replacement),
				Source:  "distance",
			}
			if isMeaningfulFix(cmd, result) {
				return result
			}
		}
	}

	return FixResult{Fixed: false}
}

func (e *Engine) tryParser(input parser.Context) FixResult {
	lines, err := parseShellCommandLines(input.Command)
	if err == nil {
		hasMultipleCommands := len(lines) > 1

		for _, line := range lines {
			result := e.parser.Parse(parser.Context{
				Command:             line.commandSuffixRaw(),
				Stderr:              input.Stderr,
				ExitCode:            input.ExitCode,
				HasMultipleCommands: hasMultipleCommands,
				HasRedirection:      line.hasRedirection,
				HasPrivilegeWrapper: line.hasWrapper("sudo"),
			})
			if result.Fixed {
				return FixResult{
					Fixed:   true,
					Command: line.replaceCommandSuffix(result.Command),
					Source:  "parser",
					Message: result.Message,
					Kind:    result.Kind,
				}
			}
		}

		return FixResult{Fixed: false}
	}

	result := e.parser.Parse(parser.Context{
		Command:             input.Command,
		Stderr:              input.Stderr,
		ExitCode:            input.ExitCode,
		HasPrivilegeWrapper: strings.HasPrefix(strings.TrimSpace(input.Command), "sudo "),
		ShellParseFailed:    true,
	})
	if result.Fixed {
		return FixResult{
			Fixed:   true,
			Command: result.Command,
			Source:  "parser",
			Message: result.Message,
			Kind:    result.Kind,
		}
	}
	return FixResult{Fixed: false}
}

func (e *Engine) tryHistory(cmd string) FixResult {
	result := e.tryMatchOnCommand(cmd, "history", func(s string) (string, bool) {
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
		if isSingleAdjacentTransposition(cmdWord, candidate) {
			return candidate
		}
	}

	return ""
}

func (e *Engine) tryUserRules(cmd string) FixResult {
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

func (e *Engine) tryBuiltinRules(cmd string) FixResult {
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

func (e *Engine) fixSubcommandInResult(result FixResult) FixResult {
	if fixed := e.trySubcommandFixWithSource(result.Command, result.Source); fixed.Fixed {
		return fixed
	}
	return result
}

type matchFunc func(string) (string, bool)

func tryMatch(cmd, source string, match matchFunc) FixResult {
	// Try full command first
	if replacement, ok := match(cmd); ok {
		return FixResult{
			Fixed:   true,
			Command: replacement,
			Source:  source,
		}
	}

	// Try command word only
	parts := strings.Fields(cmd)
	if len(parts) > 1 {
		if replacement, ok := match(parts[0]); ok {
			return FixResult{
				Fixed:   true,
				Command: replacement + " " + strings.Join(parts[1:], " "),
				Source:  source,
			}
		}
	}

	return FixResult{Fixed: false}
}

func (e *Engine) tryMatchOnCommand(cmd, source string, match matchFunc) FixResult {
	lines, err := parseShellCommandLines(cmd)
	if err == nil {
		for _, line := range lines {
			if replacement, ok := match(line.commandSuffixRaw()); ok {
				return FixResult{
					Fixed:   true,
					Command: line.replaceCommandSuffixDedup(replacement),
					Source:  source,
				}
			}

			if replacement, ok := match(line.commandWord()); ok {
				return FixResult{
					Fixed:   true,
					Command: line.replaceCommandWord(replacement),
					Source:  source,
				}
			}
		}

		return FixResult{Fixed: false}
	}

	return tryMatch(cmd, source, match)
}

func (e *Engine) tryDistance(cmd string) FixResult {
	if result, parsed := e.tryDistanceWithShell(cmd); parsed {
		return result
	}

	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return FixResult{Fixed: false}
	}

	cmdWord := parts[0]
	if e.isProtectedCommandWord(cmdWord) {
		return FixResult{Fixed: false}
	}

	// Find best match from known commands
	bestMatch, bestDistance := e.closestKnownCommand(cmdWord)

	// Check if match is good enough
	// Threshold: distance <= 2 and similarity > 60%
	if bestMatch != "" && bestMatch != cmdWord && isGoodCommandDistanceMatch(cmdWord, bestMatch, bestDistance, e.distanceMatchConfig()) {
		result := FixResult{
			Fixed:   true,
			Command: bestMatch,
			Source:  "distance",
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

	return FixResult{Fixed: false}
}

func (e *Engine) tryDistanceWithShell(cmd string) (FixResult, bool) {
	lines, err := parseShellCommandLines(cmd)
	if err != nil {
		return FixResult{Fixed: false}, false
	}

	matchCfg := e.distanceMatchConfig()

	for _, line := range lines {
		if e.isProtectedCommandWord(line.commandWord()) {
			continue
		}

		bestMatch, bestDistance := e.closestKnownCommand(line.commandWord())
		if !isGoodCommandDistanceMatch(line.commandWord(), bestMatch, bestDistance, matchCfg) {
			continue
		}
		if bestMatch == line.commandWord() {
			continue
		}

		result := FixResult{
			Fixed:   true,
			Command: line.replaceCommandWord(bestMatch),
			Source:  "distance",
		}

		if e.toolTrees != nil {
			result = e.fixSubcommandInResult(result)
		}

		return result, true
	}

	return FixResult{Fixed: false}, true
}

func (e *Engine) tryToolOptionFix(cmd string) FixResult {
	if result, parsed := e.tryToolOptionFixWithShell(cmd); parsed {
		return result
	}

	matchCfg := e.distanceMatchConfig()

	parts := strings.Fields(cmd)
	if len(parts) < 2 {
		return FixResult{Fixed: false}
	}

	mainCmd := parts[0]
	if !e.isAvailableCommand(mainCmd) {
		resolved := e.findClosestCommand(mainCmd)
		if resolved == "" {
			return FixResult{Fixed: false}
		}
		mainCmd = resolved
		parts[0] = resolved
	}

	expectValue := false
	for i := 1; i < len(parts); i++ {
		arg := parts[i]
		if expectValue {
			expectValue = false
			continue
		}

		if arg == "--" {
			break
		}

		name, suffix, isOption := splitToolOptionToken(arg)
		if !isOption {
			break
		}

		if isKnownToolOption(mainCmd, name) {
			if toolOptionTakesValue(mainCmd, name) && suffix == "" {
				expectValue = true
			}
			continue
		}

		replacement := closestToolOption(mainCmd, name, matchCfg)
		if replacement == "" {
			continue
		}

		parts[i] = replacement + suffix
		return FixResult{
			Fixed:   true,
			Command: strings.Join(parts, " "),
			Source:  "option",
			Message: fmt.Sprintf("did you mean: %s?", replacement),
		}
	}

	return FixResult{Fixed: false}
}

func (e *Engine) tryToolOptionFixWithShell(cmd string) (FixResult, bool) {
	lines, err := parseShellCommandLines(cmd)
	if err != nil {
		return FixResult{Fixed: false}, false
	}

	matchCfg := e.distanceMatchConfig()

	for _, line := range lines {
		mainCmd, resolvedCmd, resolveErr := e.resolveShellCommandLine(line)
		if resolveErr != nil {
			continue
		}

		expectValue := false
		for i := line.commandIdx + 1; i < len(line.args); i++ {
			arg := line.args[i].Lit()
			if expectValue {
				expectValue = false
				continue
			}

			if arg == "--" {
				break
			}

			name, suffix, isOption := splitToolOptionToken(arg)
			if !isOption {
				break
			}

			if isKnownToolOption(mainCmd, name) {
				if toolOptionTakesValue(mainCmd, name) && suffix == "" {
					expectValue = true
				}
				continue
			}

			replacement := closestToolOption(mainCmd, name, matchCfg)
			if replacement == "" {
				continue
			}

			replacements := []shellWordReplacement{{index: i, value: replacement + suffix}}
			if resolvedCmd != "" {
				replacements = append(replacements, shellWordReplacement{index: line.commandIdx, value: resolvedCmd})
			}

			return FixResult{
				Fixed:   true,
				Command: line.replaceWords(replacements...),
				Source:  "option",
				Message: fmt.Sprintf("did you mean: %s?", replacement),
			}, true
		}
	}

	return FixResult{Fixed: false}, true
}

func (e *Engine) trySubcommandFix(cmd string) FixResult {
	if e.toolTrees == nil {
		return FixResult{Fixed: false}
	}

	if result, parsed := e.trySubcommandFixWithShell(cmd); parsed {
		return result
	}

	parts := strings.Fields(cmd)
	if len(parts) < 2 {
		return FixResult{Fixed: false}
	}

	mainCmd := parts[0]

	// If main command is not known, try to resolve it first
	if !e.isAvailableCommand(mainCmd) {
		resolved := e.findClosestCommand(mainCmd)
		if resolved == "" {
			return FixResult{Fixed: false}
		}
		mainCmd = resolved
	}

	// Get subcommands for this tool
	subcmdIdx := findSubcommandIndex(mainCmd, parts)
	if subcmdIdx == -1 {
		return FixResult{Fixed: false}
	}

	fixedParts, changed := e.fixSubcommandParts(mainCmd, parts, subcmdIdx)
	if !changed {
		return FixResult{Fixed: false}
	}

	fixedParts[0] = mainCmd
	fixedCommand := strings.Join(fixedParts, " ")
	return FixResult{
		Fixed:   true,
		Command: fixedCommand,
		Source:  "subcommand",
		Message: fmt.Sprintf("did you mean: %s?", fixedCommand),
	}
}

func (e *Engine) trySubcommandFixWithShell(cmd string) (FixResult, bool) {
	lines, err := parseShellCommandLines(cmd)
	if err != nil {
		return FixResult{Fixed: false}, false
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
		return FixResult{
			Fixed:   true,
			Command: fixedCommand,
			Source:  "subcommand",
			Message: fmt.Sprintf("did you mean: %s?", fixedCommand),
		}, true
	}

	return FixResult{Fixed: false}, true
}

func (e *Engine) trySubcommandFixWithSource(cmd, source string) FixResult {
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
		if !isGoodDistanceMatch(token, match, distance, cfg) {
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

	// 层级子命令之间允许插入 `--flag value`，这里用下一层子命令候选做保守判断。
	return isSubcommandCandidate(nextNextToken, subcommands, cfg)
}

func isSubcommandCandidate(token string, subcommands []string, cfg distanceMatchConfig) bool {
	if containsString(subcommands, token) {
		return true
	}

	match, distance := closestSubcommand(token, subcommands, cfg)
	return isGoodDistanceMatch(token, match, distance, cfg)
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
	bestMatch, bestDistance := e.closestKnownCommandFromSlice(cmd, e.availableCommands())
	if isGoodCommandDistanceMatch(cmd, bestMatch, bestDistance, matchCfg) || e.commandLoader == nil || e.commandsFullyLoad {
		return bestMatch, bestDistance
	}

	// Only scan PATH on demand when builtin or seeded commands cannot produce a good candidate.
	e.loadCommands()
	return e.closestKnownCommandFromSlice(cmd, e.availableCommands())
}

func (e *Engine) closestKnownCommandFromSlice(cmd string, knownCommands []string) (string, int) {
	candidates := make([]commandCandidate, 0, len(knownCommands))
	seen := make(map[string]bool, len(knownCommands))

	for _, known := range knownCommands {
		if known == "" || seen[known] {
			continue
		}
		seen[known] = true

		d := Distance(cmd, known, e.keyboard)
		candidates = append(candidates, commandCandidate{
			name:       known,
			distance:   d,
			similarity: SimilarityFromDistance(len(cmd), len(known), d),
			priority:   e.commandPriority(known),
			transposed: isSingleAdjacentTransposition(cmd, known),
		})
	}

	if len(candidates) == 0 {
		return "", 999
	}

	sort.Slice(candidates, func(i, j int) bool {
		// Exact matches must stay first; otherwise, prefer adjacent
		// transpositions over ordinary fuzzy matches from PATH.
		if candidates[i].distance == 0 || candidates[j].distance == 0 {
			return candidates[i].distance < candidates[j].distance
		}
		if candidates[i].transposed != candidates[j].transposed {
			return candidates[i].transposed
		}
		if candidates[i].distance != candidates[j].distance {
			return candidates[i].distance < candidates[j].distance
		}
		if candidates[i].similarity != candidates[j].similarity {
			return candidates[i].similarity > candidates[j].similarity
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

		e.commands = mergeUniqueStrings(e.commands, e.filterDisabledCommands(e.commandLoader())...)
		e.refreshAvailableCommands()
		e.commandsFullyLoad = true
	})
}

func (e *Engine) refreshAvailableCommands() {
	e.availableCmds = e.filterDisabledCommands(e.commands)
	e.availableCmdsSet = make(map[string]struct{}, len(e.availableCmds))
	for _, cmd := range e.availableCmds {
		e.availableCmdsSet[cmd] = struct{}{}
	}
	e.availableCmdsLen = len(e.commands)
}

func (e *Engine) distanceMatchConfig() distanceMatchConfig {
	return distanceMatchConfig{
		keyboard:            e.keyboard,
		maxEditDistance:     e.maxEditDistance,
		similarityThreshold: e.similarityThreshold,
	}
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

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func mergeUniqueStrings(base []string, extra ...string) []string {
	result := append([]string(nil), base...)
	seen := make(map[string]bool, len(result)+len(extra))
	for _, item := range result {
		seen[item] = true
	}

	for _, item := range extra {
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}

	return result
}

func (e *Engine) rebuildCommand(cmdWord string, args []string, source string) FixResult {
	result := FixResult{
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
		if !isGoodDistanceMatch(subcmd, known, d, cfg) {
			continue
		}
		lengthDelta := abs(len(subcmd) - len(known))
		similarity := SimilarityFromDistance(len(subcmd), len(known), d)
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

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func isGoodDistanceMatch(original, candidate string, distance int, cfg distanceMatchConfig) bool {
	if candidate == "" || distance > cfg.maxEditDistance {
		return false
	}

	return SimilarityFromDistance(len(original), len(candidate), distance) >= cfg.similarityThreshold
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

func isMeaningfulFix(original string, result FixResult) bool {
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
			name := arg
			if eq := strings.IndexByte(arg, '='); eq >= 0 {
				name = arg[:eq]
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
		if eq := strings.IndexByte(arg, '='); eq >= 0 {
			return arg[:eq], arg[eq:], true
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

func isKnownToolOption(mainCmd, option string) bool {
	options, ok := builtinToolOptionSet[mainCmd]
	return ok && options[option]
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

var builtinToolOptionSet = buildBuiltinToolOptionSet()

var builtinToolOptionsWithValues = map[string]map[string]bool{
	"cargo": {
		"--color":   true,
		"--config":  true,
		"--explain": true,
		"-C":        true,
		"-Z":        true,
	},
}

func buildBuiltinToolOptionSet() map[string]map[string]bool {
	result := make(map[string]map[string]bool, len(builtinToolOptions))
	for tool, options := range builtinToolOptions {
		set := make(map[string]bool, len(options))
		for _, option := range options {
			set[option] = true
		}
		result[tool] = set
	}

	return result
}

// Learn stores a user-taught correction as a rule instead of history.
func (e *Engine) Learn(from, to string) error {
	if err := e.rules.AddUserRule(Rule{From: from, To: to}); err != nil {
		return err
	}

	return e.clearConflictingHistory(from)
}

// AddRule adds a user rule.
func (e *Engine) AddRule(from, to string) error {
	if err := e.rules.AddUserRule(Rule{From: from, To: to}); err != nil {
		return err
	}

	return e.clearConflictingHistory(from)
}

// ListRules returns all rules.
func (e *Engine) ListRules() []Rule {
	return e.rules.ListRules()
}

// ListHistory returns all history entries.
func (e *Engine) ListHistory() []HistoryEntry {
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
