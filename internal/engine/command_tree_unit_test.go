package engine

import (
	"testing"

	"github.com/yuluo-yx/typo/internal/commands"
	itypes "github.com/yuluo-yx/typo/internal/types"
)

func newCommandTreeTestEngine() *Engine {
	return NewEngine(
		WithCommands([]string{"typo", "type"}),
		WithKeyboard(NewQWERTYKeyboard()),
		WithCommandTrees(commands.NewCommandTreeRegistry()),
	)
}

func TestTryCommandTreeFixFallsBackToFieldsWhenShellParseFails(t *testing.T) {
	eng := newCommandTreeTestEngine()

	got := eng.tryCommandTreeFix("typ doctro '")
	if !got.Fixed || got.Command != "typo doctor '" || got.Source != "tree" {
		t.Fatalf("tryCommandTreeFix fallback = %+v", got)
	}
}

func TestTryCommandTreeFixFallbackGuardrails(t *testing.T) {
	eng := newCommandTreeTestEngine()

	tests := []struct {
		name string
		cmd  string
	}{
		{name: "empty fields after parse failure", cmd: "'"},
		{name: "unknown root", cmd: "zzzz nope '"},
		{name: "root change without matching child", cmd: "typ nope '"},
		{name: "no actual change", cmd: "typo doctor '"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := eng.tryCommandTreeFix(tt.cmd); got.Fixed {
				t.Fatalf("expected no fix, got %+v", got)
			}
		})
	}

	eng.commandTrees = nil
	if got := eng.tryCommandTreeFix("typ doctro"); got.Fixed {
		t.Fatalf("nil command tree registry should not fix, got %+v", got)
	}
}

func TestCommandTreeReplacementsForLineEdges(t *testing.T) {
	eng := newCommandTreeTestEngine()

	if got := eng.commandTreeReplacementsForLine(nil); got != nil {
		t.Fatalf("nil line replacements = %#v", got)
	}

	lines, err := parseShellCommandLines("typ nope")
	if err != nil {
		t.Fatalf("parseShellCommandLines failed: %v", err)
	}
	if got := eng.commandTreeReplacementsForLine(lines[0]); got != nil {
		t.Fatalf("root change without child match should not replace, got %#v", got)
	}
}

func TestFindCommandTreeRootForArgsBranches(t *testing.T) {
	eng := newCommandTreeTestEngine()

	if got := eng.findCommandTreeRootForArgs("typo", nil); got != "" {
		t.Fatalf("same root replacement should be empty, got %q", got)
	}
	if got := eng.findCommandTreeRootForArgs("typ", nil); got != "typo" {
		t.Fatalf("root replacement without args = %q", got)
	}
	if got := eng.findCommandTreeRootForArgs("typ", []string{"doctor"}); got != "typo" {
		t.Fatalf("root replacement with matching child = %q", got)
	}
	if got := eng.findCommandTreeRootForArgs("typ", []string{"unknown"}); got != "" {
		t.Fatalf("root replacement with unknown child = %q", got)
	}

	eng.commandTrees = nil
	if got := eng.findCommandTreeRootForArgs("typ", []string{"doctor"}); got != "" {
		t.Fatalf("nil registry root replacement = %q", got)
	}
}

func TestMatchCommandTreeTokensRejectsEmptyInputs(t *testing.T) {
	eng := newCommandTreeTestEngine()

	if tokens, count := eng.matchCommandTreeTokens(nil, &itypes.CommandTreeNode{}); tokens != nil || count != 0 {
		t.Fatalf("empty tokens = %#v, %d", tokens, count)
	}
	if tokens, count := eng.matchCommandTreeTokens([]string{"doctor"}, nil); tokens != nil || count != 0 {
		t.Fatalf("nil node = %#v, %d", tokens, count)
	}
}

func TestMatchCommandTreeChildRejectsEmptyInputs(t *testing.T) {
	eng := newCommandTreeTestEngine()

	if got, child, ok := eng.matchCommandTreeChild("", &itypes.CommandTreeNode{}); ok || got != "" || child != nil {
		t.Fatalf("empty child token = %q, %#v, %v", got, child, ok)
	}
	if got, child, ok := eng.matchCommandTreeChild("doctor", nil); ok || got != "" || child != nil {
		t.Fatalf("nil child node = %q, %#v, %v", got, child, ok)
	}
	if got, child, ok := eng.matchCommandTreeChild("doctor", &itypes.CommandTreeNode{}); ok || got != "" || child != nil {
		t.Fatalf("empty child map = %q, %#v, %v", got, child, ok)
	}
}

func TestMatchCommandTreeTokensStopsAfterMatchingChild(t *testing.T) {
	eng := newCommandTreeTestEngine()

	node := &itypes.CommandTreeNode{
		Children: map[string]*itypes.CommandTreeNode{
			"doctor": {StopAfterMatch: true},
		},
	}
	if tokens, count := eng.matchCommandTreeTokens([]string{"doctro", "ignored"}, node); count != 1 || len(tokens) != 1 || tokens[0] != "doctor" {
		t.Fatalf("matchCommandTreeTokens = %#v, %d", tokens, count)
	}
}

func TestCommandTreeCandidateOrderingAndBoundaries(t *testing.T) {
	eng := newCommandTreeTestEngine()
	cfg := eng.distanceMatchConfig()

	if _, ok := newCommandTreeTokenCandidate("abcdef", "ghijkl", nil, cfg, eng.keyboard); ok {
		t.Fatalf("distant candidate should be rejected")
	}
	if candidate, ok := newCommandTreeTokenCandidate("doctro", "doctor", nil, cfg, eng.keyboard); !ok || candidate.token != "doctor" {
		t.Fatalf("transposition candidate = %+v, %v", candidate, ok)
	}

	current := commandTreeTokenCandidate{token: "z", distance: 1, similarity: 0.7, lengthDelta: 1}
	tests := []struct {
		name      string
		candidate commandTreeTokenCandidate
		want      bool
	}{
		{
			name:      "lower distance",
			candidate: commandTreeTokenCandidate{token: "z", distance: 0, similarity: 0.1, lengthDelta: 9},
			want:      true,
		},
		{
			name:      "higher similarity",
			candidate: commandTreeTokenCandidate{token: "z", distance: 1, similarity: 0.9, lengthDelta: 9},
			want:      true,
		},
		{
			name:      "lower length delta",
			candidate: commandTreeTokenCandidate{token: "z", distance: 1, similarity: 0.7, lengthDelta: 0},
			want:      true,
		},
		{
			name:      "lexicographic tie",
			candidate: commandTreeTokenCandidate{token: "a", distance: 1, similarity: 0.7, lengthDelta: 1},
			want:      true,
		},
		{
			name:      "worse candidate",
			candidate: commandTreeTokenCandidate{token: "z", distance: 2, similarity: 1, lengthDelta: 0},
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasBetterCommandTreeCandidate(tt.candidate, current); got != tt.want {
				t.Fatalf("hasBetterCommandTreeCandidate = %v, want %v", got, tt.want)
			}
		})
	}

	if isShortBoundaryPreservingMatch("ab", "ac", 1) {
		t.Fatalf("two-rune boundary match should be rejected")
	}
	if isShortBoundaryPreservingMatch("abcde", "abxde", 1) {
		t.Fatalf("long boundary match should be rejected")
	}
	if !isShortBoundaryPreservingMatch("abcd", "abxd", 1) {
		t.Fatalf("short same-boundary match should be accepted")
	}
}
