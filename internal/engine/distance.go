package engine

import (
	"math"
	"sync"
)

const maxPooledDistanceBufferLen = 4096

var distanceBufferPool = sync.Pool{
	New: func() any {
		return make([]float64, 0, 64)
	},
}

// Distance calculates the Levenshtein distance between two strings.
// It uses keyboard-aware weights for substitution operations.
// Only two rows of the DP matrix are kept at a time to reduce allocations.
func Distance(a, b string, weights KeyboardWeights) int {
	return distanceRunes([]rune(a), []rune(b), weights)
}

func distanceRunes(runesA, runesB []rune, weights KeyboardWeights) int {
	lenA := len(runesA)
	lenB := len(runesB)
	if lenA == 0 {
		return lenB
	}
	if lenB == 0 {
		return lenA
	}

	bufLen := 2 * (lenB + 1)
	buf := distanceBufferPool.Get().([]float64)
	if cap(buf) < bufLen {
		buf = make([]float64, bufLen)
	} else {
		buf = buf[:bufLen]
	}
	prev := buf[:lenB+1]
	curr := buf[lenB+1:]

	for j := 0; j <= lenB; j++ {
		prev[j] = float64(j)
	}

	for i := 1; i <= lenA; i++ {
		curr[0] = float64(i)
		for j := 1; j <= lenB; j++ {
			cost := substitutionCost(runesA[i-1], runesB[j-1], weights)
			curr[j] = min(
				prev[j]+1.0,    // deletion
				curr[j-1]+1.0,  // insertion
				prev[j-1]+cost, // substitution
			)
		}
		prev, curr = curr, prev
	}

	result := int(math.Round(prev[lenB]))
	if cap(buf) <= maxPooledDistanceBufferLen {
		distanceBufferPool.Put(buf[:0])
	}
	return result
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
	return SimilarityFromDistance(len(a), len(b), Distance(a, b, weights))
}

// SimilarityFromDistance derives a 0-1 similarity ratio from a precomputed
// edit distance, avoiding a redundant Distance() call.
func SimilarityFromDistance(lenA, lenB, distance int) float64 {
	maxLen := max(lenA, lenB)
	if maxLen == 0 {
		return 1.0
	}
	return 1.0 - float64(distance)/float64(maxLen)
}
