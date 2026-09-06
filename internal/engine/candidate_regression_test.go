package engine

import (
	"strings"
	"testing"

	itypes "github.com/yuluo-yx/typo/internal/types"
)

func TestCandidatesPreserveKnownCommands(t *testing.T) {
	for _, command := range []string{"tool status", "tool 'unfinished", "tool status && gti status"} {
		t.Run(command, func(t *testing.T) {
			eng := NewEngine(WithCommands([]string{"tool", "took", "git", "get"}))
			candidates := eng.FixCandidatesWithContext(itypes.ParserContext{Command: command}, 5)
			for _, candidate := range candidates {
				if !strings.HasPrefix(candidate.Command, "tool ") {
					t.Errorf("known executable changed: %+v", candidate)
				}
			}
			if command == "tool status" && len(candidates) != 0 {
				t.Fatalf("valid command has candidates: %+v", candidates)
			}
		})
	}
}

func TestCandidatesPreserveAliasInAlternatives(t *testing.T) {
	eng := NewEngine(WithCommands([]string{"tool", "took", "git", "get"}), WithSimilarityThreshold(0.2))
	candidates := eng.FixCandidatesWithContext(itypes.ParserContext{
		Command:      "too status && gti status",
		AliasContext: []itypes.AliasContextEntry{{Shell: "bash", Kind: "alias", Name: "too", Expansion: "tool"}},
	}, 5)
	if len(candidates) < 2 {
		t.Fatalf("expected alternative corrections, got %+v", candidates)
	}
	for _, candidate := range candidates {
		if !strings.HasPrefix(candidate.Command, "too status && ") {
			t.Errorf("alias changed in candidate: %+v", candidate)
		}
	}
}
