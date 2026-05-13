package utils

import "testing"

func TestSplitInlineValue(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		wantName   string
		wantSuffix string
		wantOK     bool
	}{
		{"without inline value", "--flag", "--flag", "", false},
		{"with inline value", "--flag=value", "--flag", "=value", true},
		{"empty inline value", "--flag=", "--flag", "=", true},
		{"leading equals", "=value", "", "=value", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, suffix, ok := SplitInlineValue(tt.value)
			if name != tt.wantName || suffix != tt.wantSuffix || ok != tt.wantOK {
				t.Fatalf("SplitInlineValue(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tt.value, name, suffix, ok, tt.wantName, tt.wantSuffix, tt.wantOK)
			}
		})
	}
}
