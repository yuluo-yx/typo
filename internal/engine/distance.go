package engine

import (
	"math"
	"sync"
	"unicode/utf8"
)

const maxPooledDistanceBufferLen = 4096

var distanceBufferPool = sync.Pool{
	New: func() any {
		buf := make([]float64, 0, 64)
		return &buf
	},
}

// Distance calculates the Levenshtein distance between two strings.
// It uses keyboard-aware weights for substitution operations.
// Only two rows of the DP matrix are kept at a time to reduce allocations.
func Distance(a, b string, weights KeyboardWeights) int {
	return distanceRunes([]rune(a), []rune(b), weights)
}

func distanceRunes(runesA, runesB []rune, weights KeyboardWeights) int {
	distance, _ := weightedDistance(runesA, runesB, weights, math.Inf(1))
	return int(math.Round(distance))
}

// distanceRunesWithin returns the distance when it is at most limit. It stops as
// soon as every cell of a DP row exceeds the limit, which skips most of the work
// for unrelated candidates; ok=false means the distance is larger than limit.
func distanceRunesWithin(runesA, runesB []rune, weights KeyboardWeights, limit int) (int, bool) {
	if limit < 0 {
		return 0, false
	}
	// Any raw value at or above limit+0.5 rounds to more than limit.
	distance, ok := weightedDistance(runesA, runesB, weights, float64(limit)+0.5)
	if !ok {
		return 0, false
	}
	rounded := int(math.Round(distance))
	return rounded, rounded <= limit
}

// weightedDistance computes the raw keyboard-weighted Levenshtein distance. It
// returns ok=false early once the distance is known to be >= cutoff.
func weightedDistance(runesA, runesB []rune, weights KeyboardWeights, cutoff float64) (float64, bool) {
	runesA, runesB = trimSharedBoundaries(runesA, runesB)
	lenA := len(runesA)
	lenB := len(runesB)
	if lenA == 0 || lenB == 0 {
		distance := float64(lenA + lenB)
		return distance, distance < cutoff
	}
	// Each length difference costs at least one insertion or deletion.
	if float64(absInt(lenA-lenB)) >= cutoff {
		return 0, false
	}

	bufLen := 2 * (lenB + 1)
	bufPtr := distanceBufferPool.Get().(*[]float64)
	buf := *bufPtr
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

	release := func() {
		if cap(buf) <= maxPooledDistanceBufferLen {
			*bufPtr = buf[:0]
			distanceBufferPool.Put(bufPtr)
		}
	}

	for i := 1; i <= lenA; i++ {
		// Row minima never decrease, so the final distance is at least rowMin.
		if rowMin := fillDistanceRow(prev, curr, i, runesA[i-1], runesB, weights); rowMin >= cutoff {
			release()
			return 0, false
		}
		prev, curr = curr, prev
	}

	result := prev[lenB]
	release()
	return result, result < cutoff
}

// trimSharedBoundaries drops the common prefix and suffix, which never
// contribute to the weighted Levenshtein distance.
func trimSharedBoundaries(runesA, runesB []rune) ([]rune, []rune) {
	for len(runesA) > 0 && len(runesB) > 0 && runesA[0] == runesB[0] {
		runesA, runesB = runesA[1:], runesB[1:]
	}
	for len(runesA) > 0 && len(runesB) > 0 && runesA[len(runesA)-1] == runesB[len(runesB)-1] {
		runesA, runesB = runesA[:len(runesA)-1], runesB[:len(runesB)-1]
	}
	return runesA, runesB
}

// fillDistanceRow computes DP row i from prev into curr and returns the row minimum.
func fillDistanceRow(prev, curr []float64, i int, charA rune, runesB []rune, weights KeyboardWeights) float64 {
	curr[0] = float64(i)
	rowMin := curr[0]
	for j := 1; j <= len(runesB); j++ {
		cost := substitutionCost(charA, runesB[j-1], weights)
		curr[j] = min(
			prev[j]+1.0,    // deletion
			curr[j-1]+1.0,  // insertion
			prev[j-1]+cost, // substitution
		)
		rowMin = min(rowMin, curr[j])
	}
	return rowMin
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
	runesA := []rune(a)
	runesB := []rune(b)
	if len(runesA) == 0 && len(runesB) == 0 {
		return 1.0
	}
	return SimilarityFromDistance(len(runesA), len(runesB), distanceRunes(runesA, runesB, weights))
}

func runeCount(value string) int {
	return utf8.RuneCountInString(value)
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
