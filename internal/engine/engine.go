package engine

import (
	"fmt"
	"strings"

	"github.com/yuluo-yx/typo/internal/commands"
	"github.com/yuluo-yx/typo/internal/parser"
)

// FixResult represents the result of a fix attempt.
type FixResult struct {
	Fixed   bool   // Whether a fix was found
	Command string // The corrected command
	Source  string // Where the fix came from (history, rule, parser, distance, subcommand)
	Message string // Optional message to display
}

// Engine is the main correction engine.
type Engine struct {
	keyboard    KeyboardWeights
	rules       *Rules
	history     *History
	parser      *parser.Registry
	commands    []string // Known commands from $PATH
	subcommands *commands.SubcommandRegistry
}

// Option is a functional option for Engine.
type Option func(*Engine)

// WithKeyboard sets the keyboard weights.
func WithKeyboard(kb KeyboardWeights) Option {
	return func(e *Engine) { e.keyboard = kb }
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
	return func(e *Engine) { e.commands = cmds }
}

// WithSubcommands sets the subcommand registry.
func WithSubcommands(s *commands.SubcommandRegistry) Option {
	return func(e *Engine) { e.subcommands = s }
}

// NewEngine creates a new correction engine.
func NewEngine(opts ...Option) *Engine {
	e := &Engine{
		keyboard: DefaultKeyboard,
		rules:    NewRules(""),
		history:  NewHistory(""),
		parser:   parser.NewRegistry(),
		commands: []string{},
	}

	for _, opt := range opts {
		opt(e)
	}

	return e
}

// Fix attempts to fix the given command.
// stderr is optional and used for error parsing.
func (e *Engine) Fix(cmd, stderr string) FixResult {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return FixResult{Fixed: false}
	}

	currentCmd := cmd
	messages := make([]string, 0)
	lastSource := ""

	for range 32 {
		result := e.fixOnePass(currentCmd, stderr)
		if !isMeaningfulFix(currentCmd, result) {
			break
		}

		currentCmd = result.Command
		lastSource = result.Source
		if result.Message != "" && !containsString(messages, result.Message) {
			messages = append(messages, result.Message)
		}
	}

	if currentCmd != cmd {
		return FixResult{
			Fixed:   true,
			Command: currentCmd,
			Source:  lastSource,
			Message: strings.Join(messages, "; "),
		}
	}

	return FixResult{Fixed: false}
}

func (e *Engine) fixOnePass(cmd, stderr string) FixResult {
	// 1. Try error parser first (if stderr provided)
	if stderr != "" {
		if result := e.tryParser(cmd, stderr); isMeaningfulFix(cmd, result) {
			return result
		}
	}

	// 2. Try history
	if result := e.tryHistory(cmd); isMeaningfulFix(cmd, result) {
		return result
	}

	// 3. Try rules
	if result := e.tryRules(cmd); isMeaningfulFix(cmd, result) {
		return result
	}

	// 4. Try subcommand fix
	if result := e.trySubcommandFix(cmd); isMeaningfulFix(cmd, result) {
		return result
	}

	// 5. Try edit distance
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
	if result := e.tryHistory(cmdWord); result.Fixed {
		rebuilt := e.rebuildCommand(result.Command, args, "history")
		if isMeaningfulFix(cmd, rebuilt) {
			return rebuilt
		}
	}

	if result := e.tryRules(cmdWord); result.Fixed {
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

		if rule, ok := e.rules.Match(cmdWord); ok {
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

func (e *Engine) tryParser(cmd, stderr string) FixResult {
	lines, err := parseShellCommandLines(cmd)
	if err == nil {
		for _, line := range lines {
			result := e.parser.Parse(line.commandSuffixRaw(), stderr)
			if result.Fixed {
				return FixResult{
					Fixed:   true,
					Command: line.replaceCommandSuffix(result.Command),
					Source:  "parser",
					Message: result.Message,
				}
			}
		}

		return FixResult{Fixed: false}
	}

	result := e.parser.Parse(cmd, stderr)
	if result.Fixed {
		return FixResult{
			Fixed:   true,
			Command: result.Command,
			Source:  "parser",
			Message: result.Message,
		}
	}
	return FixResult{Fixed: false}
}

func (e *Engine) tryHistory(cmd string) FixResult {
	result := e.tryMatchOnCommand(cmd, "history", func(s string) (string, bool) {
		entry, ok := e.history.Lookup(s)
		if ok {
			return entry.To, true
		}
		return "", false
	})

	// If history fixed main command, also try to fix subcommand
	if result.Fixed && e.subcommands != nil {
		result = e.fixSubcommandInResult(result)
	}

	return result
}

func (e *Engine) tryRules(cmd string) FixResult {
	result := e.tryMatchOnCommand(cmd, "rule", func(s string) (string, bool) {
		rule, ok := e.rules.Match(s)
		if ok {
			return rule.To, true
		}
		return "", false
	})

	// If rule fixed main command, also try to fix subcommand
	if result.Fixed && e.subcommands != nil {
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
					Command: line.replaceCommandSuffix(replacement),
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
	bestMatch := ""
	bestDistance := 999

	for _, known := range e.commands {
		d := Distance(cmdWord, known, e.keyboard)
		if d < bestDistance {
			bestDistance = d
			bestMatch = known
		}
	}

	// Check if match is good enough
	// Threshold: distance <= 2 and similarity > 60%
	if bestMatch != "" && bestMatch != cmdWord && bestDistance <= 2 {
		similarity := Similarity(cmdWord, bestMatch, e.keyboard)
		if similarity >= 0.6 {
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
			if e.subcommands != nil {
				result = e.fixSubcommandInResult(result)
			}

			return result
		}
	}

	return FixResult{Fixed: false}
}

func (e *Engine) tryDistanceWithShell(cmd string) (FixResult, bool) {
	lines, err := parseShellCommandLines(cmd)
	if err != nil {
		return FixResult{Fixed: false}, false
	}

	for _, line := range lines {
		if e.isProtectedCommandWord(line.commandWord()) {
			continue
		}

		bestMatch, bestDistance := e.closestKnownCommand(line.commandWord())
		if !isGoodDistanceMatch(line.commandWord(), bestMatch, bestDistance, e.keyboard) {
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

		if e.subcommands != nil {
			result = e.fixSubcommandInResult(result)
		}

		return result, true
	}

	return FixResult{Fixed: false}, true
}

func (e *Engine) trySubcommandFix(cmd string) FixResult {
	if e.subcommands == nil {
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
	if !containsString(e.commands, mainCmd) {
		resolved := e.findClosestCommand(mainCmd)
		if resolved == "" {
			return FixResult{Fixed: false}
		}
		mainCmd = resolved
	}

	// Get subcommands for this tool
	subcommands := e.subcommands.Get(mainCmd)
	if len(subcommands) == 0 {
		return FixResult{Fixed: false}
	}

	subcmdIdx := findSubcommandIndex(mainCmd, parts)
	if subcmdIdx == -1 {
		return FixResult{Fixed: false}
	}

	subcmd := parts[subcmdIdx]

	// Check if subcommand is already valid
	if containsString(subcommands, subcmd) {
		return FixResult{Fixed: false}
	}

	// Try to find closest subcommand
	bestMatch := ""
	bestDistance := 999

	for _, known := range subcommands {
		d := Distance(subcmd, known, e.keyboard)
		if d < bestDistance {
			bestDistance = d
			bestMatch = known
		}
	}

	// Threshold: distance <= 2 and similarity > 60%
	if bestMatch != "" && bestDistance <= 2 {
		similarity := Similarity(subcmd, bestMatch, e.keyboard)
		if similarity >= 0.6 {
			// Update main command if it was resolved
			parts[0] = mainCmd
			parts[subcmdIdx] = bestMatch
			return FixResult{
				Fixed:   true,
				Command: strings.Join(parts, " "),
				Source:  "subcommand",
				Message: fmt.Sprintf("did you mean: %s?", bestMatch),
			}
		}
	}

	return FixResult{Fixed: false}
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

		subcommands := e.subcommands.Get(mainCmd)
		if len(subcommands) == 0 {
			continue
		}

		subcmdIdx := findSubcommandWordIndex(mainCmd, line)
		if subcmdIdx == -1 {
			continue
		}

		subcmd := line.args[subcmdIdx].Lit()
		if containsString(subcommands, subcmd) {
			continue
		}

		bestMatch, bestDistance := closestSubcommand(subcmd, subcommands, e.keyboard)
		if !isGoodDistanceMatch(subcmd, bestMatch, bestDistance, e.keyboard) {
			continue
		}

		replacements := []shellWordReplacement{{index: subcmdIdx, value: bestMatch}}
		if resolvedCmd != "" {
			replacements = append(replacements, shellWordReplacement{index: line.commandIdx, value: resolvedCmd})
		}

		return FixResult{
			Fixed:   true,
			Command: line.replaceWords(replacements...),
			Source:  "subcommand",
			Message: fmt.Sprintf("did you mean: %s?", bestMatch),
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

func (e *Engine) resolveShellCommandLine(line *shellCommandLine) (string, string, error) {
	mainCmd := line.commandWord()
	if containsString(e.commands, mainCmd) || e.isProtectedCommandWord(mainCmd) {
		return mainCmd, "", nil
	}

	resolved := e.findClosestCommand(mainCmd)
	if resolved == "" {
		return "", "", fmt.Errorf("unknown command: %s", mainCmd)
	}
	return resolved, resolved, nil
}

func (e *Engine) findClosestCommand(cmd string) string {
	bestMatch, bestDistance := e.closestKnownCommand(cmd)
	if isGoodDistanceMatch(cmd, bestMatch, bestDistance, e.keyboard) {
		return bestMatch
	}
	return ""
}

func (e *Engine) closestKnownCommand(cmd string) (string, int) {
	bestMatch := ""
	bestDistance := 999

	for _, known := range e.commands {
		d := Distance(cmd, known, e.keyboard)
		if d < bestDistance {
			bestDistance = d
			bestMatch = known
		}
	}

	return bestMatch, bestDistance
}

func (e *Engine) isProtectedCommandWord(cmd string) bool {
	for _, rule := range e.rules.ListRules() {
		if rule.To == cmd {
			return true
		}
	}

	for _, entry := range e.history.List() {
		if entry.To == cmd {
			return true
		}
	}

	return false
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
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

func closestSubcommand(subcmd string, knownSubcommands []string, keyboard KeyboardWeights) (string, int) {
	bestMatch := ""
	bestDistance := 999

	for _, known := range knownSubcommands {
		d := Distance(subcmd, known, keyboard)
		if d < bestDistance {
			bestDistance = d
			bestMatch = known
		}
	}

	return bestMatch, bestDistance
}

func isGoodDistanceMatch(original, candidate string, distance int, keyboard KeyboardWeights) bool {
	if candidate == "" || distance > 2 {
		return false
	}

	return Similarity(original, candidate, keyboard) >= 0.6
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
}

// Learn stores a user-taught correction as a rule instead of history.
func (e *Engine) Learn(from, to string) error {
	return e.rules.AddUserRule(Rule{From: from, To: to})
}

// AddRule adds a user rule.
func (e *Engine) AddRule(from, to string) error {
	return e.rules.AddUserRule(Rule{From: from, To: to})
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
