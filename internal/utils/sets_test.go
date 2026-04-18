package utils

import "testing"

func TestStringSet(t *testing.T) {
	tests := []struct {
		name  string
		items []string
		want  map[string]bool
	}{
		{"empty slice", []string{}, map[string]bool{}},
		{"single item", []string{"git"}, map[string]bool{"git": true}},
		{"multiple items", []string{"git", "docker", "npm"}, map[string]bool{"git": true, "docker": true, "npm": true}},
		{"duplicate items", []string{"git", "git", "docker"}, map[string]bool{"git": true, "docker": true}},
		{"empty string element", []string{""}, map[string]bool{"": true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StringSet(tt.items)
			if len(got) != len(tt.want) {
				t.Fatalf("StringSet(%v) returned %d entries, want %d", tt.items, len(got), len(tt.want))
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Fatalf("StringSet(%v)[%q] = %v, want %v", tt.items, k, got[k], v)
				}
			}
		})
	}
}