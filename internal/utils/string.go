package utils

// IsSingleAdjacentTransposition reports whether two strings differ
// by exactly one swap of adjacent characters.
func IsSingleAdjacentTransposition(original, candidate string) bool {
	originalRunes := []rune(original)
	candidateRunes := []rune(candidate)
	if len(originalRunes) != len(candidateRunes) || len(originalRunes) < 2 {
		return false
	}

	diffIdx := make([]int, 0, 2)
	for i := range originalRunes {
		if originalRunes[i] == candidateRunes[i] {
			continue
		}
		diffIdx = append(diffIdx, i)
		if len(diffIdx) > 2 {
			return false
		}
	}

	if len(diffIdx) != 2 || diffIdx[1] != diffIdx[0]+1 {
		return false
	}

	i := diffIdx[0]
	j := diffIdx[1]
	return originalRunes[i] == candidateRunes[j] && originalRunes[j] == candidateRunes[i]
}
