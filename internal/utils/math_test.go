package utils

import "testing"

func TestAbs(t *testing.T) {
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
			if got := Abs(tt.value); got != tt.want {
				t.Fatalf("Abs(%d) = %d, want %d", tt.value, got, tt.want)
			}
		})
	}
}