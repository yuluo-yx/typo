package commands

import "testing"

func TestPreSubcommandOptionTakesValue(t *testing.T) {
	tests := []struct {
		tool, option string
		want         bool
	}{
		{"git", "-C", true},
		{"git", "--version", false},
		{"cargo", "--config", true},
		{"kubectl", "-n", true},
		{"unknown", "-C", false},
	}
	for _, tt := range tests {
		if got := PreSubcommandOptionTakesValue(tt.tool, tt.option); got != tt.want {
			t.Errorf("PreSubcommandOptionTakesValue(%q, %q) = %v, want %v", tt.tool, tt.option, got, tt.want)
		}
	}
}
