package utils

import "math"

// OffsetToIndex converts a parser offset to a string index capped by rawLen.
func OffsetToIndex(offset uint, rawLen int) int {
	if offset > uint(math.MaxInt) {
		return rawLen
	}

	index := int(offset)
	if index > rawLen {
		return rawLen
	}
	return index
}
