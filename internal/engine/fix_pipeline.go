package engine

import (
	"slices"
	"strings"
	"time"

	itypes "github.com/yuluo-yx/typo/internal/types"
)

// Fix attempts to fix the given command.
// stderr is optional and used for error parsing.
func (e *Engine) Fix(cmd, stderr string) itypes.FixResult {
	return e.FixWithContext(itypes.ParserContext{
		Command: cmd,
		Stderr:  stderr,
	})
}

// FixWithContext attempts to fix the given command with parser context.

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

// FixCandidatesWithContext returns selectable correction candidates for the input.

// FixCandidatesWithContext returns selectable correction candidates for the input.
func (e *Engine) FixCandidatesWithContext(input itypes.ParserContext, limit int) []itypes.FixCandidate {
	input.Command = strings.TrimSpace(input.Command)
	if e == nil || input.Command == "" || limit < 1 {
		return nil
	}

	candidates := make([]itypes.FixCandidate, 0, limit)
	seen := make(map[string]bool, limit)
	addResult := func(result itypes.FixResult) {
		if len(candidates) >= limit || !isMeaningfulFix(input.Command, result) || seen[result.Command] {
			return
		}
		seen[result.Command] = true
		candidates = append(candidates, itypes.FixCandidate{
			Command: result.Command,
			Source:  result.Source,
			Message: result.Message,
		})
	}

	addResult(e.FixWithContext(input))
	for _, result := range e.distanceFixCandidates(input.Command, limit) {
		addResult(result)
		if len(candidates) >= limit {
			break
		}
	}

	return candidates
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
