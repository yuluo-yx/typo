package utils

import (
	"reflect"
	"testing"
)

func TestMergeUniqueStrings(t *testing.T) {
	base := []string{"git", "docker"}
	got := MergeUniqueStrings(base, "docker", "", "npm", "git", "kubectl")
	want := []string{"git", "docker", "npm", "kubectl"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MergeUniqueStrings() = %#v, want %#v", got, want)
	}
	if !reflect.DeepEqual(base, []string{"git", "docker"}) {
		t.Fatalf("MergeUniqueStrings() mutated base slice: %#v", base)
	}
}
