package engine

import (
	"strings"

	"github.com/shown/typo/internal/parser"
)

// FixResult represents the result of a fix attempt.
type FixResult struct {
	Fixed   bool   // Whether a fix was found
	Command string // The corrected command
	Source  string // Where the fix came from (history, rule, parser, distance)
	Message string // Optional message to display
}

// Engine is the main correction engine.
type Engine struct {
	keyboard KeyboardWeights
	rules    *Rules
	history  *History
	parser   *parser.Registry
	commands []string // Known commands from $PATH
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

	// 4. Try edit distance
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
	return tryMatch(cmd, "history", func(s string) (string, bool) {
		entry, ok := e.history.Lookup(s)
		if ok {
			return entry.To, true
		}
		return "", false
	})
}

func (e *Engine) tryRules(cmd string) FixResult {
	return tryMatch(cmd, "rule", func(s string) (string, bool) {
		rule, ok := e.rules.Match(s)
		if ok {
			return rule.To, true
		}
		return "", false
	})
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

			return result
		}
	}

	return FixResult{Fixed: false}
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
