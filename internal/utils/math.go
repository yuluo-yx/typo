package utils

// Abs returns the absolute value of an integer.
// Note: Abs(math.MinInt) returns a negative value due to two's complement overflow,
// matching the behavior of the original inline helper.
func Abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
