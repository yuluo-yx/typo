package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/yuluo-yx/typo/internal/engine"
	itypes "github.com/yuluo-yx/typo/internal/types"
)

func TestSelectFixCandidateByNumber(t *testing.T) {
	candidates := []itypes.FixCandidate{
		{Command: "git status"},
		{Command: "get status"},
		{Command: "go status"},
	}

	selected, ok, err := selectFixCandidate(candidates, strings.NewReader("2"), &bytes.Buffer{})
	if err != nil {
		t.Fatalf("selectFixCandidate failed: %v", err)
	}
	if !ok {
		t.Fatal("selectFixCandidate should select a candidate")
	}
	if selected.Command != "get status" {
		t.Fatalf("selected command = %q, want get status", selected.Command)
	}
}

func TestSelectFixCandidateWithArrowKeysAndEnter(t *testing.T) {
	candidates := []itypes.FixCandidate{
		{Command: "git status"},
		{Command: "get status"},
		{Command: "go status"},
	}
	input := strings.NewReader("\x1b[B\x1b[B\r")

	selected, ok, err := selectFixCandidate(candidates, input, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("selectFixCandidate failed: %v", err)
	}
	if !ok {
		t.Fatal("selectFixCandidate should select a candidate")
	}
	if selected.Command != "go status" {
		t.Fatalf("selected command = %q, want go status", selected.Command)
	}
}

func TestSelectFixCandidateDefaultsToFirstOnEnter(t *testing.T) {
	candidates := []itypes.FixCandidate{
		{Command: "git status"},
		{Command: "get status"},
	}

	selected, ok, err := selectFixCandidate(candidates, strings.NewReader("\n"), &bytes.Buffer{})
	if err != nil {
		t.Fatalf("selectFixCandidate failed: %v", err)
	}
	if !ok {
		t.Fatal("selectFixCandidate should select a candidate")
	}
	if selected.Command != "git status" {
		t.Fatalf("selected command = %q, want git status", selected.Command)
	}
}

func TestSelectFixCandidateCancel(t *testing.T) {
	candidates := []itypes.FixCandidate{
		{Command: "git status"},
		{Command: "get status"},
	}

	if _, ok, err := selectFixCandidate(candidates, strings.NewReader("q"), &bytes.Buffer{}); err != nil || ok {
		t.Fatalf("selectFixCandidate(q) = ok %v err %v, want cancel without error", ok, err)
	}
	if _, ok, err := selectFixCandidate(candidates, strings.NewReader("\x1b"), &bytes.Buffer{}); err != nil || ok {
		t.Fatalf("selectFixCandidate(Esc) = ok %v err %v, want cancel without error", ok, err)
	}
	if _, ok, err := selectFixCandidate(candidates, strings.NewReader("\x03"), &bytes.Buffer{}); err != nil || ok {
		t.Fatalf("selectFixCandidate(Ctrl+C) = ok %v err %v, want cancel without error", ok, err)
	}
}

func TestSelectFixCandidateDrawsNumberedMenu(t *testing.T) {
	candidates := []itypes.FixCandidate{
		{Command: "git status"},
		{Command: "get status"},
	}
	var out bytes.Buffer

	_, _, err := selectFixCandidate(candidates, strings.NewReader("1"), &out)
	if err != nil {
		t.Fatalf("selectFixCandidate failed: %v", err)
	}

	text := out.String()
	for _, want := range []string{"typo: choose a correction", "1) git status", "2) get status"} {
		if !strings.Contains(text, want) {
			t.Fatalf("menu output missing %q: %q", want, text)
		}
	}
}

func TestSelectFixResultUsesTerminalSelection(t *testing.T) {
	eng := engine.NewEngine(
		engine.WithCommands([]string{"git", "get"}),
		engine.WithMaxEditDistance(3),
		engine.WithSimilarityThreshold(0.2),
	)
	oldChoose := chooseFixCandidateFromTerminalFunc
	defer func() { chooseFixCandidateFromTerminalFunc = oldChoose }()
	chooseFixCandidateFromTerminalFunc = func(candidates []itypes.FixCandidate) (itypes.FixCandidate, bool, error) {
		if len(candidates) != 2 {
			t.Fatalf("terminal candidates = %+v, want two candidates", candidates)
		}
		return candidates[1], true, nil
	}

	result, ok := selectFixResult(eng, itypes.ParserContext{Command: "gti status"}, 2)
	if !ok {
		t.Fatal("selectFixResult should accept terminal selection")
	}
	if result.Command != "get status" {
		t.Fatalf("selected command = %q, want get status", result.Command)
	}
}

func TestSelectFixResultCancel(t *testing.T) {
	eng := engine.NewEngine(
		engine.WithCommands([]string{"git", "get"}),
		engine.WithMaxEditDistance(3),
		engine.WithSimilarityThreshold(0.2),
	)
	oldChoose := chooseFixCandidateFromTerminalFunc
	defer func() { chooseFixCandidateFromTerminalFunc = oldChoose }()
	chooseFixCandidateFromTerminalFunc = func(candidates []itypes.FixCandidate) (itypes.FixCandidate, bool, error) {
		return itypes.FixCandidate{}, false, nil
	}

	if _, ok := selectFixResult(eng, itypes.ParserContext{Command: "gti status"}, 2); ok {
		t.Fatal("selectFixResult should report cancellation")
	}
}

func TestFixSelectRejectsDebugModes(t *testing.T) {
	useTempHome(t)

	code, stdout, stderr := runCLI(t, []string{"typo", "fix", "--select", "--debug", "gut", "status"})
	if code == 0 {
		t.Fatalf("fix --select --debug should fail: stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stderr, "--select cannot be combined") {
		t.Fatalf("expected select/debug error, got stdout=%q stderr=%q", stdout, stderr)
	}
}
