package utils

// StringSet creates a map[string]bool from a string slice,
// where each element maps to true.
func StringSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[item] = true
	}
	return set
}
