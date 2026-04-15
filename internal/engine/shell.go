package engine

import (
	"fmt"
	"sort"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"github.com/yuluo-yx/typo/internal/utils"
)

type shellCommandLine struct {
	raw            string
	args           []*syntax.Word
	commandIdx     int
	hasRedirection bool
}

type shellWordReplacement struct {
	index int
	value string
}

func parseShellCommandLines(raw string) ([]*shellCommandLine, error) {
	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	file, err := parser.Parse(strings.NewReader(raw+"\n"), "")
	if err != nil {
		return nil, err
	}

	lines := make([]*shellCommandLine, 0)
	syntax.Walk(file, func(node syntax.Node) bool {
		stmt, ok := node.(*syntax.Stmt)
		if !ok {
			return true
		}

		call, ok := stmt.Cmd.(*syntax.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}

		commandIdx := findExecutableArgIndex(call.Args)
		if commandIdx == -1 {
			return true
		}

		lines = append(lines, &shellCommandLine{
			raw:            raw,
			args:           call.Args,
			commandIdx:     commandIdx,
			hasRedirection: len(stmt.Redirs) > 0,
		})

		return true
	})

	if len(lines) == 0 {
		return nil, fmt.Errorf("unsupported command shape")
	}

	sort.SliceStable(lines, func(i, j int) bool {
		return lines[i].args[lines[i].commandIdx].Pos().Offset() < lines[j].args[lines[j].commandIdx].Pos().Offset()
	})

	return lines, nil
}

func (c *shellCommandLine) commandWord() string {
	return c.args[c.commandIdx].Lit()
}

func (c *shellCommandLine) commandSuffixRaw() string {
	start, end := c.commandSuffixRange()
	return c.raw[start:end]
}

func (c *shellCommandLine) replaceCommandWord(replacement string) string {
	return c.replaceWord(c.commandIdx, replacement)
}

func (c *shellCommandLine) replaceCommandSuffix(replacement string) string {
	start, end := c.commandSuffixRange()
	return c.raw[:start] + replacement + c.raw[end:]
}

func (c *shellCommandLine) replaceCommandSuffixDedup(replacement string) string {
	normalized := trimShellWordPrefix(replacement, c.wrapperWords())
	if normalized == "" {
		normalized = replacement
	}
	return c.replaceCommandSuffix(normalized)
}

func (c *shellCommandLine) replaceWord(index int, replacement string) string {
	start, end := wordRange(c.args[index], len(c.raw))
	return c.raw[:start] + replacement + c.raw[end:]
}

func (c *shellCommandLine) replaceWords(replacements ...shellWordReplacement) string {
	if len(replacements) == 0 {
		return c.raw
	}

	sorted := append([]shellWordReplacement(nil), replacements...)
	sort.SliceStable(sorted, func(i, j int) bool {
		iStart, _ := wordRange(c.args[sorted[i].index], len(c.raw))
		jStart, _ := wordRange(c.args[sorted[j].index], len(c.raw))
		return iStart > jStart
	})

	result := c.raw
	for _, replacement := range sorted {
		start, end := wordRange(c.args[replacement.index], len(c.raw))
		result = result[:start] + replacement.value + result[end:]
	}

	return result
}

func (c *shellCommandLine) commandSuffixRange() (int, int) {
	start, _ := wordRange(c.args[c.commandIdx], len(c.raw))
	_, end := wordRange(c.args[len(c.args)-1], len(c.raw))
	return start, end
}

func (c *shellCommandLine) wrapperWords() []string {
	if c.commandIdx <= 0 {
		return nil
	}

	wrappers := make([]string, 0, c.commandIdx)
	for i := 0; i < c.commandIdx; i++ {
		wrappers = append(wrappers, c.args[i].Lit())
	}
	return wrappers
}

func (c *shellCommandLine) hasWrapper(name string) bool {
	for i := 0; i < c.commandIdx; i++ {
		if c.args[i].Lit() == name {
			return true
		}
	}
	return false
}

func wordRange(word *syntax.Word, rawLen int) (int, int) {
	return utils.OffsetToIndex(word.Pos().Offset(), rawLen), utils.OffsetToIndex(word.End().Offset(), rawLen)
}

func trimShellWordPrefix(raw string, prefixWords []string) string {
	if len(prefixWords) == 0 {
		return raw
	}

	lines, err := parseShellCommandLines(raw)
	if err != nil || len(lines) != 1 {
		return raw
	}

	line := lines[0]
	if len(line.args) < len(prefixWords) {
		return raw
	}

	for i, want := range prefixWords {
		if line.args[i].Lit() != want {
			return raw
		}
	}

	_, end := wordRange(line.args[len(prefixWords)-1], len(line.raw))
	return strings.TrimLeft(line.raw[end:], " \t")
}

func findExecutableArgIndex(args []*syntax.Word) int {
	idx := 0
	for idx < len(args) {
		word := args[idx].Lit()
		switch word {
		case "builtin", "nocorrect", "noglob":
			idx++
		case "command":
			idx++
			for idx < len(args) && commandWrapperOptions[args[idx].Lit()] {
				idx++
			}
		case "env":
			next := skipEnvWrapperArgs(args, idx+1)
			if next == -1 {
				return -1
			}
			idx = next
		case "sudo":
			next := skipWrapperOptions(args, idx+1, sudoWrapperOptionsWithValues, sudoWrapperOptions)
			if next == -1 {
				return -1
			}
			idx = next
		case "time":
			idx++
			for idx < len(args) && timeWrapperOptions[args[idx].Lit()] {
				idx++
			}
		default:
			return idx
		}
	}

	return -1
}

func skipWrapperOptions(args []*syntax.Word, start int, optionsWithValues, optionsWithoutValues map[string]bool) int {
	expectValue := false
	for i := start; i < len(args); i++ {
		arg := args[i].Lit()
		if expectValue {
			expectValue = false
			continue
		}

		if arg == "--" {
			if i+1 < len(args) {
				return i + 1
			}
			return -1
		}

		if handled, needsValue := handleLongWrapperOption(arg, optionsWithValues, optionsWithoutValues); handled {
			expectValue = needsValue
			continue
		}

		if handled, needsValue := handleShortWrapperOption(arg, optionsWithValues, optionsWithoutValues); handled {
			expectValue = needsValue
			continue
		}

		return i
	}

	return -1
}

func skipEnvWrapperArgs(args []*syntax.Word, start int) int {
	expectValue := false
	for i := start; i < len(args); i++ {
		arg := args[i].Lit()
		if expectValue {
			expectValue = false
			continue
		}

		if arg == "--" {
			if i+1 < len(args) {
				return i + 1
			}
			return -1
		}

		if handled, needsValue := handleLongWrapperOption(arg, envWrapperOptionsWithValues, envWrapperOptions); handled {
			expectValue = needsValue
			continue
		}

		if handled, needsValue := handleShortWrapperOption(arg, envWrapperOptionsWithValues, envWrapperOptions); handled {
			expectValue = needsValue
			continue
		}

		if isEnvAssignment(arg) {
			continue
		}

		return i
	}

	return -1
}

func handleLongWrapperOption(arg string, optionsWithValues, optionsWithoutValues map[string]bool) (bool, bool) {
	if !strings.HasPrefix(arg, "--") {
		return false, false
	}

	name := arg
	if eq := strings.IndexByte(arg, '='); eq >= 0 {
		name = arg[:eq]
		if optionsWithValues[name] {
			return true, false
		}
	}

	if optionsWithValues[name] {
		return true, true
	}
	if optionsWithoutValues[name] || name == arg {
		return true, false
	}

	return false, false
}

func handleShortWrapperOption(arg string, optionsWithValues, optionsWithoutValues map[string]bool) (bool, bool) {
	if !strings.HasPrefix(arg, "-") || arg == "-" || strings.HasPrefix(arg, "--") {
		return false, false
	}

	if optionsWithValues[arg] {
		return true, true
	}
	if optionsWithoutValues[arg] || len(arg) > 1 {
		return true, false
	}

	return false, false
}

func isEnvAssignment(arg string) bool {
	eq := strings.IndexByte(arg, '=')
	if eq <= 0 {
		return false
	}
	return syntax.ValidName(arg[:eq])
}

var commandWrapperOptions = map[string]bool{
	"-p": true,
	"-v": true,
	"-V": true,
}

var timeWrapperOptions = map[string]bool{
	"-p": true,
}

var sudoWrapperOptions = map[string]bool{
	"-A":                 true,
	"-b":                 true,
	"-E":                 true,
	"-e":                 true,
	"-H":                 true,
	"-K":                 true,
	"-k":                 true,
	"-n":                 true,
	"-P":                 true,
	"-S":                 true,
	"-s":                 true,
	"-v":                 true,
	"-V":                 true,
	"--askpass":          true,
	"--background":       true,
	"--edit":             true,
	"--help":             true,
	"--login":            true,
	"--non-interactive":  true,
	"--preserve-env":     true,
	"--remove-timestamp": true,
	"--reset-timestamp":  true,
	"--shell":            true,
	"--stdin":            true,
	"--validate":         true,
	"--version":          true,
}

var sudoWrapperOptionsWithValues = map[string]bool{
	"-C":                true,
	"-c":                true,
	"-D":                true,
	"-g":                true,
	"-h":                true,
	"-p":                true,
	"-R":                true,
	"-r":                true,
	"-t":                true,
	"-T":                true,
	"-u":                true,
	"-U":                true,
	"--chdir":           true,
	"--close-from":      true,
	"--command-timeout": true,
	"--group":           true,
	"--host":            true,
	"--other-user":      true,
	"--prompt":          true,
	"--role":            true,
	"--timeout":         true,
	"--user":            true,
}

var envWrapperOptions = map[string]bool{
	"-0":                   true,
	"-i":                   true,
	"--ignore-environment": true,
	"--null":               true,
}

var envWrapperOptionsWithValues = map[string]bool{
	"-C":             true,
	"-S":             true,
	"-u":             true,
	"--chdir":        true,
	"--split-string": true,
	"--unset":        true,
}
