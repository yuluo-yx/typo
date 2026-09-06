package engine

import (
	"fmt"
	"math"
	"strings"
	"testing"

	itypes "github.com/yuluo-yx/typo/internal/types"
)

func TestClosestCommandChoosesAnEligibleMatch(t *testing.T) {
	eng := NewEngine(WithCommands([]string{"z", "xyz"}), WithSimilarityThreshold(0.3))
	got := eng.findClosestCommand("x")
	if got != "xyz" {
		t.Fatalf("ineligible short match hid eligible candidate: got %q, want xyz", got)
	}
}

func TestCandidateIndexAcceptsMaximumDistance(t *testing.T) {
	eng := NewEngine(WithCommands([]string{"tool", "other"}))
	got := eng.availableCommandCandidates("tool", math.MaxInt)
	if len(got) != 2 {
		t.Fatalf("large distance overflowed the index bounds: %+v", got)
	}
}

func TestBuiltinRulePreservesExistingExecutable(t *testing.T) {
	eng := NewEngine(WithCommands([]string{"gti", "git"}))
	for _, command := range []string{"gti", "gti status", "gti 'unfinished"} {
		if result := eng.Fix(command, ""); result.Fixed {
			t.Errorf("builtin rule replaced existing executable: %q -> %+v", command, result)
		}
		if result := eng.FixCommand(command); result.Fixed {
			t.Errorf("command-only fix replaced existing executable: %q -> %+v", command, result)
		}
		if candidates := eng.FixCandidatesWithContext(itypes.ParserContext{Command: command}, 3); len(candidates) != 0 {
			t.Errorf("existing executable has unwanted candidates: %+v", candidates)
		}
	}
	if err := eng.AddRule("gti", "git"); err != nil {
		t.Fatal(err)
	}
	if result := eng.Fix("gti status", ""); !result.Fixed || result.Command != "git status" {
		t.Fatalf("explicit user rule must still override the executable: %+v", result)
	}
}

func BenchmarkCommandMatching(b *testing.B) {
	commands := make([]string, 10000)
	for i := range commands {
		commands[i] = fmt.Sprintf("custom%05d", i)
	}
	eng := NewEngine(WithCommands(commands))
	for _, input := range []string{"custom05000", "custom0500x", "unrelatedxx"} {
		b.Run(input, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				eng.closestKnownCommand(input)
			}
		})
	}
}

func TestDistancePrefixSuffixOptimizationPreservesWeights(t *testing.T) {
	words := []string{"", "a", "s", "as", "sa", "ab", "ba", "日本", "本日"}
	for _, layout := range []string{"qwerty", "dvorak", "colemak"} {
		weights, err := KeyboardByName(layout)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range words {
			for _, b := range words {
				for _, padding := range []string{"", "custom-", strings.Repeat("a", 20)} {
					left, right := padding+a+padding, padding+b+padding
					want := referenceWeightedDistance([]rune(left), []rune(right), weights)
					if got := Distance(left, right, weights); got != want {
						t.Fatalf("%s distance(%q, %q)=%d, want %d", layout, left, right, got, want)
					}
				}
			}
		}
	}
}

// The full matrix is an independent oracle for the optimized rolling-row implementation.
func referenceWeightedDistance(a, b []rune, weights KeyboardWeights) int {
	matrix := make([][]float64, len(a)+1)
	for i := range matrix {
		matrix[i] = make([]float64, len(b)+1)
		matrix[i][0] = float64(i)
	}
	for j := range matrix[0] {
		matrix[0][j] = float64(j)
	}
	for i, left := range a {
		for j, right := range b {
			cost := 1.0
			if left == right {
				cost = 0
			} else if weights.IsAdjacent(left, right) {
				cost = 0.5
			}
			matrix[i+1][j+1] = min(matrix[i][j+1]+1, matrix[i+1][j]+1, matrix[i][j]+cost)
		}
	}
	return int(math.Round(matrix[len(a)][len(b)]))
}
