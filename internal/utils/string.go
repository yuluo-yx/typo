package utils

import "strings"

// SplitInlineValue splits a token like "--flag=value" into "--flag" and "=value".
func SplitInlineValue(value string) (name, suffix string, ok bool) {
	if eq := strings.IndexByte(value, '='); eq >= 0 {
		return value[:eq], value[eq:], true
	}

	return value, "", false
}
