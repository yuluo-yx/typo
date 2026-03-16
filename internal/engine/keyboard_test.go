package engine

import (
	"testing"
)

func TestQWERTYKeyboard_IsAdjacent(t *testing.T) {
	kb := NewQWERTYKeyboard()

	tests := []struct {
		name     string
		a        rune
		b        rune
		expected bool
	}{
		// Adjacent keys
		{"i and o", 'i', 'o', true},
		{"o and i", 'o', 'i', true},
		{"a and s", 'a', 's', true},
		{"s and d", 's', 'd', true},
		{"h and j", 'h', 'j', true},
		{"n and m", 'n', 'm', true},

		// Non-adjacent keys
		{"a and p", 'a', 'p', false},
		{"q and m", 'q', 'm', false},
		{"z and p", 'z', 'p', false},

		// Same key is not considered adjacent
		{"a and a", 'a', 'a', false},

		// Case insensitive
		{"A and s (case insensitive)", 'A', 's', true},
		{"a and S (case insensitive)", 'a', 'S', true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := kb.IsAdjacent(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("IsAdjacent(%q, %q) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestQWERTYKeyboard_Rows(t *testing.T) {
	kb := NewQWERTYKeyboard()

	// Test row 2 horizontal adjacency
	row2 := []rune{'q', 'w', 'e', 'r', 't', 'y', 'u', 'i', 'o', 'p'}
	for i := 0; i < len(row2)-1; i++ {
		if !kb.IsAdjacent(row2[i], row2[i+1]) {
			t.Errorf("Expected %q and %q to be adjacent", row2[i], row2[i+1])
		}
	}

	// Test row 3 horizontal adjacency
	row3 := []rune{'a', 's', 'd', 'f', 'g', 'h', 'j', 'k', 'l'}
	for i := 0; i < len(row3)-1; i++ {
		if !kb.IsAdjacent(row3[i], row3[i+1]) {
			t.Errorf("Expected %q and %q to be adjacent", row3[i], row3[i+1])
		}
	}

	// Test row 4 horizontal adjacency
	row4 := []rune{'z', 'x', 'c', 'v', 'b', 'n', 'm'}
	for i := 0; i < len(row4)-1; i++ {
		if !kb.IsAdjacent(row4[i], row4[i+1]) {
			t.Errorf("Expected %q and %q to be adjacent", row4[i], row4[i+1])
		}
	}
}

func TestQWERTYKeyboard_VerticalAdjacency(t *testing.T) {
	kb := NewQWERTYKeyboard()

	// Test vertical adjacency (between rows)
	verticalPairs := []struct {
		a rune
		b rune
	}{
		{'q', 'a'},
		{'w', 's'},
		{'e', 'd'},
		{'a', 'z'},
		{'s', 'x'},
		{'d', 'c'},
	}

	for _, pair := range verticalPairs {
		if !kb.IsAdjacent(pair.a, pair.b) {
			t.Errorf("Expected %q and %q to be adjacent vertically", pair.a, pair.b)
		}
	}
}

func TestDefaultKeyboard(t *testing.T) {
	if DefaultKeyboard == nil {
		t.Error("DefaultKeyboard should not be nil")
	}

	// Test that DefaultKeyboard works
	if !DefaultKeyboard.IsAdjacent('i', 'o') {
		t.Error("DefaultKeyboard should recognize i and o as adjacent")
	}
}

func TestNewQWERTYKeyboard(t *testing.T) {
	kb := NewQWERTYKeyboard()
	if kb == nil {
		t.Fatal("NewQWERTYKeyboard returned nil")
	}

	if kb.adjacentKeys == nil {
		t.Error("adjacentKeys map should be initialized")
	}

	// Verify some expected adjacencies exist
	if len(kb.adjacentKeys) == 0 {
		t.Error("adjacentKeys should not be empty after initialization")
	}
}
