package engine

import (
	"math"
)

// Distance calculates the Levenshtein distance between two strings.
// It uses keyboard-aware weights for substitution operations.
func Distance(a, b string, weights KeyboardWeights) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	// Convert to runes for proper Unicode handling
	runesA := []rune(a)
	runesB := []rune(b)
	lenA := len(runesA)
	lenB := len(runesB)

	// Create distance matrix
	matrix := make([][]float64, lenA+1)
	for i := range matrix {
		matrix[i] = make([]float64, lenB+1)
	}

	// Initialize first column
	for i := 0; i <= lenA; i++ {
		matrix[i][0] = float64(i)
	}

	// Initialize first row
	for j := 0; j <= lenB; j++ {
		matrix[0][j] = float64(j)
	}

	// Fill the matrix
	for i := 1; i <= lenA; i++ {
		for j := 1; j <= lenB; j++ {
			cost := substitutionCost(runesA[i-1], runesB[j-1], weights)

			matrix[i][j] = minFloat64(
				matrix[i-1][j]+1.0,    // deletion
				matrix[i][j-1]+1.0,    // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return int(math.Round(matrix[lenA][lenB]))
}

// substitutionCost returns the cost of substituting one character for another.
// Adjacent keys on the keyboard have lower substitution cost.
func substitutionCost(a, b rune, weights KeyboardWeights) float64 {
	if a == b {
		return 0.0
	}

	// Check if characters are adjacent on keyboard
	if weights.IsAdjacent(a, b) {
		return 0.5
	}

	return 1.0
}

// Similarity calculates the similarity ratio between two strings (0-1).
func Similarity(a, b string, weights KeyboardWeights) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}

	maxLen := float64(max(len(a), len(b)))
	if maxLen == 0 {
		return 1.0
	}

	distance := Distance(a, b, weights)
	return 1.0 - float64(distance)/maxLen
}

func minFloat64(vals ...float64) float64 {
	result := vals[0]
	for _, v := range vals[1:] {
		if v < result {
			result = v
		}
	}
	return result
}
