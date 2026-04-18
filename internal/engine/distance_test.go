package engine

import (
	"testing"
)

func TestDistance(t *testing.T) {
	kb := NewQWERTYKeyboard()

	tests := []struct {
		name     string
		a        string
		b        string
		expected int
	}{
		{"identical strings", "git", "git", 0},
		{"empty strings", "", "", 0},
		{"one empty string", "git", "", 3},
		{"single substitution", "git", "got", 1},
		{"single insertion", "git", "gits", 1},
		{"single deletion", "gits", "git", 1},
		{"complete different", "abc", "xyz", 3},
		{"adjacent keys substitution", "git", "got", 1},
		{"typo: gut to git", "gut", "git", 1},
		{"typo: gti to git", "gti", "git", 2},
		{"typo: dcoker to docker", "dcoker", "docker", 2},
		{"unicode support", "日本", "日本語", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Distance(tt.a, tt.b, kb)
			if result != tt.expected {
				t.Errorf("Distance(%q, %q) = %d, want %d", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestDistanceWithAdjacentKeys(t *testing.T) {
	kb := NewQWERTYKeyboard()

	// 'i' and 'o' are adjacent on QWERTY keyboard
	distance := Distance("git", "got", kb)
	// g->g (0) + i->o (0.5, adjacent) + t->t (0) = 0.5, rounded to 1
	if distance != 1 {
		t.Errorf("Expected distance 1 for git->got with adjacent keys, got %d", distance)
	}
}

func TestSimilarity(t *testing.T) {
	kb := NewQWERTYKeyboard()

	tests := []struct {
		name   string
		a      string
		b      string
		minSim float64
		maxSim float64
	}{
		{"identical strings", "git", "git", 0.99, 1.0},
		{"similar strings", "git", "got", 0.5, 0.9},
		{"different strings", "abc", "xyz", 0.0, 0.4},
		{"empty strings", "", "", 1.0, 1.0},
		{"one empty string", "git", "", 0.0, 0.1},
		{"longer strings", "docker", "dcoker", 0.6, 0.9},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Similarity(tt.a, tt.b, kb)
			if result < tt.minSim || result > tt.maxSim {
				t.Errorf("Similarity(%q, %q) = %f, want between %f and %f",
					tt.a, tt.b, result, tt.minSim, tt.maxSim)
			}
		})
	}
}


func TestMax(t *testing.T) {
	tests := []struct {
		name     string
		a        int
		b        int
		expected int
	}{
		{"a greater", 5, 3, 5},
		{"b greater", 3, 5, 5},
		{"equal", 5, 5, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := max(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("max(%d, %d) = %d, want %d", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestSubstitutionCost(t *testing.T) {
	kb := NewQWERTYKeyboard()

	tests := []struct {
		name     string
		a        rune
		b        rune
		expected float64
	}{
		{"identical characters", 'a', 'a', 0.0},
		{"adjacent keys i and o", 'i', 'o', 0.5},
		{"adjacent keys a and s", 'a', 's', 0.5},
		{"non-adjacent keys a and p", 'a', 'p', 1.0},
		{"non-adjacent keys q and m", 'q', 'm', 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := substitutionCost(tt.a, tt.b, kb)
			if result != tt.expected {
				t.Errorf("substitutionCost(%q, %q) = %f, want %f", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}
