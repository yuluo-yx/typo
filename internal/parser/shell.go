package parser

import (
	"fmt"
	"strings"

	"github.com/yuluo-yx/typo/internal/utils"
	"mvdan.cc/sh/v3/syntax"
)

type shellCall struct {
	raw  string
	args []*syntax.Word
}

func parseShellCall(raw string) (*shellCall, error) {
	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	file, err := parser.Parse(strings.NewReader(raw+"\n"), "")
	if err != nil {
		return nil, err
	}

	for _, stmt := range file.Stmts {
		call, ok := stmt.Cmd.(*syntax.CallExpr)
		if !ok || len(call.Args) == 0 {
			continue
		}

		return &shellCall{
			raw:  raw,
			args: call.Args,
		}, nil
	}

	return nil, fmt.Errorf("unsupported command shape")
}

func (c *shellCall) replaceWord(index int, replacement string) string {
	start, end := parserWordRange(c.args[index], len(c.raw))
	return c.raw[:start] + replacement + c.raw[end:]
}

func (c *shellCall) replaceSubcommand(command, expected, replacement string, optionsWithValues map[string]bool) (string, bool) {
	index := findShellSubcommandIndex(c.args, command, optionsWithValues)
	if index == -1 {
		return "", false
	}
	if expected != "" && c.args[index].Lit() != expected {
		return "", false
	}

	return c.replaceWord(index, replacement), true
}

func parserWordRange(word *syntax.Word, rawLen int) (int, int) {
	return utils.OffsetToIndex(word.Pos().Offset(), rawLen), utils.OffsetToIndex(word.End().Offset(), rawLen)
}

func findShellSubcommandIndex(args []*syntax.Word, command string, optionsWithValues map[string]bool) int {
	if len(args) < 2 || args[0].Lit() != command {
		return -1
	}

	expectValue := false
	for i := 1; i < len(args); i++ {
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

		if strings.HasPrefix(arg, "--") {
			name := arg
			if eq := strings.IndexByte(arg, '='); eq >= 0 {
				name = arg[:eq]
				if optionsWithValues[name] {
					continue
				}
			}
			if optionsWithValues[name] {
				expectValue = true
				continue
			}
			continue
		}

		if strings.HasPrefix(arg, "-") && arg != "-" {
			if optionsWithValues[arg] {
				expectValue = true
			}
			continue
		}

		return i
	}

	return -1
}

var gitParserOptionsWithValues = map[string]bool{
	"--config-env":   true,
	"--exec-path":    true,
	"--git-dir":      true,
	"--namespace":    true,
	"--super-prefix": true,
	"--work-tree":    true,
	"-C":             true,
	"-c":             true,
}

var dockerParserOptionsWithValues = map[string]bool{
	"--config":    true,
	"--context":   true,
	"--host":      true,
	"--log-level": true,
	"-H":          true,
	"-l":          true,
}

var npmParserOptionsWithValues = map[string]bool{
	"--cache":      true,
	"--prefix":     true,
	"--userconfig": true,
	"-C":           true,
}
