package engine

import "testing"

func TestAbsInt(t *testing.T) {
	tests := []struct {
		name  string
		value int
		want  int
	}{
		{"positive", 5, 5},
		{"negative", -5, 5},
		{"zero", 0, 0},
		{"large negative", -1000, 1000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := absInt(tt.value); got != tt.want {
				t.Fatalf("absInt(%d) = %d, want %d", tt.value, got, tt.want)
			}
		})
	}
}

func TestIsSingleAdjacentTransposition(t *testing.T) {
	tests := []struct {
		name      string
		original  string
		candidate string
		want      bool
	}{
		{"identical strings", "git", "git", false},
		{"adjacent swap gti->git", "gti", "git", true},
		{"adjacent swap form->from", "form", "from", true},
		{"single adjacent pair ab->ba", "ab", "ba", true},
		{"two non-adjacent diffs", "abcd", "adcb", false},
		{"three diffs", "abc", "def", false},
		{"different lengths", "git", "gi", false},
		{"single character", "a", "b", false},
		{"empty strings", "", "", false},
		{"duplicate runes aab->aba", "aab", "aba", true},
		{"unicode adjacent swap", "你好", "好你", true},
		{"unicode identical", "你好", "你好", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSingleAdjacentTransposition(tt.original, tt.candidate); got != tt.want {
				t.Fatalf("isSingleAdjacentTransposition(%q, %q) = %v, want %v",
					tt.original, tt.candidate, got, tt.want)
			}
		})
	}
}
