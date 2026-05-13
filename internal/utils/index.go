package utils

import "math"

func offsetToIndex(offset uint, rawLen int) int {
	if offset > uint(math.MaxInt) {
		return rawLen
	}

	index := int(offset)
	if index > rawLen {
		return rawLen
	}
	return index
}
