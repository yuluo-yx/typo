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
	if a == b {
		return 0
	}
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

// stackDistanceRowLen is the longest DP row kept on the stack. Command words and
// path segments almost always fit, so the common case never touches the pool.
const stackDistanceRowLen = 32

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

	if lenB < stackDistanceRowLen {
		var rows [2 * stackDistanceRowLen]float64
		return distanceRows(runesA, runesB, weights, cutoff, rows[:2*(lenB+1)])
	}

	bufPtr := distanceBufferPool.Get().(*[]float64)
	buf := *bufPtr
	if bufLen := 2 * (lenB + 1); cap(buf) < bufLen {
		buf = make([]float64, bufLen)
	} else {
		buf = buf[:bufLen]
	}
	distance, ok := distanceRows(runesA, runesB, weights, cutoff, buf)
	if cap(buf) <= maxPooledDistanceBufferLen {
		*bufPtr = buf[:0]
		distanceBufferPool.Put(bufPtr)
	}
	return distance, ok
}

// distanceRows runs the two-row DP over buf, which must hold 2*(len(runesB)+1) values.
func distanceRows(runesA, runesB []rune, weights KeyboardWeights, cutoff float64, buf []float64) (float64, bool) {
	lenB := len(runesB)
	prev := buf[:lenB+1]
	curr := buf[lenB+1:]
	for j := range prev {
		prev[j] = float64(j)
	}

	for i, charA := range runesA {
		curr[0] = float64(i + 1)
		// Row minima never decrease, so the final distance is at least rowMin.
		rowMin := curr[0]
		for j, charB := range runesB {
			cost := 0.0
			if charA != charB {
				cost = substitutionCost(charA, charB, weights)
			}
			value := min(
				prev[j+1]+1.0, // deletion
				curr[j]+1.0,   // insertion
				prev[j]+cost,  // substitution
			)
			curr[j+1] = value
			rowMin = min(rowMin, value)
		}
		if rowMin >= cutoff {
			return 0, false
		}
		prev, curr = curr, prev
	}

	result := prev[lenB]
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
	if a == b {
		return 1.0
	}
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
