package engine

import "unicode/utf8"

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func isSingleAdjacentTransposition(original, candidate string) bool {
	if utf8.RuneCountInString(original) != utf8.RuneCountInString(candidate) {
		return false
	}
	return isSingleAdjacentTranspositionRunes([]rune(original), []rune(candidate))
}

// isSingleAdjacentTranspositionRunes is the allocation-free form used in hot loops.
func isSingleAdjacentTranspositionRunes(originalRunes, candidateRunes []rune) bool {
	if len(originalRunes) != len(candidateRunes) || len(originalRunes) < 2 {
		return false
	}

	first, second, diffs := -1, -1, 0
	for i := range originalRunes {
		if originalRunes[i] == candidateRunes[i] {
			continue
		}
		diffs++
		if diffs > 2 {
			return false
		}
		if first == -1 {
			first = i
		} else {
			second = i
		}
	}

	if diffs != 2 || second != first+1 {
		return false
	}
	return originalRunes[first] == candidateRunes[second] && originalRunes[second] == candidateRunes[first]
}
