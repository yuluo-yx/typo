package engine

import (
	"fmt"
	"strings"

	"github.com/shown/typo/internal/commands"
	"github.com/shown/typo/internal/parser"
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

	// 1. Try error parser first (if stderr provided)
	if stderr != "" {
		if result := e.tryParser(cmd, stderr); result.Fixed {
			return result
		}
	}

	// 2. Try history
	if result := e.tryHistory(cmd); result.Fixed {
		return result
	}

	// 3. Try rules
	if result := e.tryRules(cmd); result.Fixed {
		return result
	}

	// 4. Try subcommand fix
	if result := e.trySubcommandFix(cmd); result.Fixed {
		return result
	}

	// 5. Try edit distance
	if result := e.tryDistance(cmd); result.Fixed {
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

	// Split into command and args
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return FixResult{Fixed: false}
	}

	cmdWord := parts[0]
	args := parts[1:]

	// Try to fix just the command word
	if result := e.tryHistory(cmdWord); result.Fixed {
		return e.rebuildCommand(result.Command, args, "history")
	}

	if result := e.tryRules(cmdWord); result.Fixed {
		return e.rebuildCommand(result.Command, args, "rule")
	}

	if result := e.tryDistance(cmdWord); result.Fixed {
		return e.rebuildCommand(result.Command, args, "distance")
	}

	return FixResult{Fixed: false}
}

func (e *Engine) tryParser(cmd, stderr string) FixResult {
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
	result := tryMatch(cmd, "history", func(s string) (string, bool) {
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
	result := tryMatch(cmd, "rule", func(s string) (string, bool) {
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
	parts := strings.Fields(result.Command)
	if len(parts) < 2 {
		return result
	}

	mainCmd := parts[0]
	subcommands := e.subcommands.Get(mainCmd)
	if len(subcommands) == 0 {
		return result
	}

	// Find subcommand position (skip options)
	subcmdIdx := -1
	for i, arg := range parts[1:] {
		if !strings.HasPrefix(arg, "-") {
			subcmdIdx = i + 1
			break
		}
	}

	if subcmdIdx == -1 {
		return result
	}

	subcmd := parts[subcmdIdx]

	// Check if subcommand is already valid
	if containsString(subcommands, subcmd) {
		return result
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

	if bestMatch != "" && bestDistance <= 2 {
		similarity := Similarity(subcmd, bestMatch, e.keyboard)
		if similarity > 0.6 {
			parts[subcmdIdx] = bestMatch
			return FixResult{
				Fixed:   true,
				Command: strings.Join(parts, " "),
				Source:  result.Source,
				Message: fmt.Sprintf("did you mean: %s?", bestMatch),
			}
		}
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

func (e *Engine) tryDistance(cmd string) FixResult {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return FixResult{Fixed: false}
	}

	cmdWord := parts[0]

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
	if bestMatch != "" && bestDistance <= 2 {
		similarity := Similarity(cmdWord, bestMatch, e.keyboard)
		if similarity > 0.6 {
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

func (e *Engine) trySubcommandFix(cmd string) FixResult {
	if e.subcommands == nil {
		return FixResult{Fixed: false}
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

	// Find the subcommand position (skip options)
	subcmdIdx := -1
	for i, arg := range parts[1:] {
		if !strings.HasPrefix(arg, "-") {
			subcmdIdx = i + 1
			break
		}
	}

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
		if similarity > 0.6 {
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

func (e *Engine) findClosestCommand(cmd string) string {
	bestMatch := ""
	bestDistance := 999

	for _, known := range e.commands {
		d := Distance(cmd, known, e.keyboard)
		if d < bestDistance {
			bestDistance = d
			bestMatch = known
		}
	}

	if bestMatch != "" && bestDistance <= 2 {
		similarity := Similarity(cmd, bestMatch, e.keyboard)
		if similarity > 0.6 {
			return bestMatch
		}
	}
	return ""
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

// Learn records a correction in history.
func (e *Engine) Learn(from, to string) error {
	return e.history.Record(from, to)
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
