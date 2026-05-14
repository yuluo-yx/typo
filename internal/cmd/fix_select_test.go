package cmd

import (
	"bytes"
	"errors"
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

func TestSelectedIndexWrapsAtBoundaries(t *testing.T) {
	tests := []struct {
		name      string
		selected  int
		count     int
		direction int
		want      int
	}{
		{name: "empty list keeps selection", selected: 2, count: 0, direction: 1, want: 2},
		{name: "zero direction keeps selection", selected: 1, count: 3, direction: 0, want: 1},
		{name: "up wraps from first to last", selected: 0, count: 3, direction: -1, want: 2},
		{name: "up moves to previous", selected: 2, count: 3, direction: -1, want: 1},
		{name: "down wraps from last to first", selected: 2, count: 3, direction: 1, want: 0},
		{name: "down moves to next", selected: 0, count: 3, direction: 1, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := selectedIndex(tt.selected, tt.count, tt.direction); got != tt.want {
				t.Fatalf("selectedIndex(%d, %d, %d) = %d, want %d", tt.selected, tt.count, tt.direction, got, tt.want)
			}
		})
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

func TestSelectFixResultFallsBackToFirstCandidateWhenTerminalFails(t *testing.T) {
	eng := engine.NewEngine(
		engine.WithCommands([]string{"git", "get"}),
		engine.WithMaxEditDistance(3),
		engine.WithSimilarityThreshold(0.2),
	)
	oldChoose := chooseFixCandidateFromTerminalFunc
	defer func() { chooseFixCandidateFromTerminalFunc = oldChoose }()
	chooseFixCandidateFromTerminalFunc = func(candidates []itypes.FixCandidate) (itypes.FixCandidate, bool, error) {
		return itypes.FixCandidate{}, false, errors.New("terminal unavailable")
	}

	result, ok := selectFixResult(eng, itypes.ParserContext{Command: "gti status"}, 2)
	if !ok {
		t.Fatal("selectFixResult should fall back when terminal selection fails")
	}
	if result.Command != "git status" {
		t.Fatalf("fallback command = %q, want git status", result.Command)
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

func TestFixSelectWarnsWhenCandidateSelectionDisabled(t *testing.T) {
	useTempHome(t)

	code, stdout, stderr := runCLI(t, []string{"typo", "fix", "--select", "gti", "status"})
	if code != 0 {
		t.Fatalf("fix --select should still return the best correction: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if strings.TrimSpace(stdout) != "git status" {
		t.Fatalf("stdout = %q, want git status", stdout)
	}
	if !strings.Contains(stderr, "candidates.enabled=false") {
		t.Fatalf("stderr should explain why --select did not open a menu, got %q", stderr)
	}
	if !strings.Contains(stderr, "typo config set candidates.enabled true") {
		t.Fatalf("stderr should include the enabling command, got %q", stderr)
	}
}
