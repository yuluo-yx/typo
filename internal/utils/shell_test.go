package utils

import (
	"strings"
	"testing"

	"mvdan.cc/sh/v3/syntax"
)

func TestShellNodeRange(t *testing.T) {
	raw := "git status"
	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	file, err := parser.Parse(strings.NewReader(raw+"\n"), "")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	call := file.Stmts[0].Cmd.(*syntax.CallExpr)
	start, end := ShellNodeRange(call.Args[1], len(raw))
	if got := raw[start:end]; got != "status" {
		t.Fatalf("ShellNodeRange() slice = %q, want status", got)
	}
}
