package engine

import (
	"testing"

	itypes "github.com/yuluo-yx/typo/internal/types"
)

func TestCommandTreeChildTokens(t *testing.T) {
	node := &itypes.CommandTreeNode{
		Children: map[string]*itypes.CommandTreeNode{
			"zsh":  {},
			"bash": {},
			"fish": {},
		},
	}

	got := commandTreeChildTokens(node)
	want := []string{"bash", "fish", "zsh"}
	if len(got) != len(want) {
		t.Fatalf("commandTreeChildTokens() length = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("commandTreeChildTokens()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCommandTreeChildTokensEmpty(t *testing.T) {
	if got := commandTreeChildTokens(nil); got != nil {
		t.Fatalf("nil commandTreeChildTokens() = %v, want nil", got)
	}

	if got := commandTreeChildTokens(&itypes.CommandTreeNode{}); got != nil {
		t.Fatalf("empty commandTreeChildTokens() = %v, want nil", got)
	}
}

func TestCommandTreeChild(t *testing.T) {
	child := &itypes.CommandTreeNode{StopAfterMatch: true}
	node := &itypes.CommandTreeNode{
		Children: map[string]*itypes.CommandTreeNode{
			"version": child,
		},
	}

	got, ok := commandTreeChild(node, "version")
	if !ok {
		t.Fatal("commandTreeChild() did not find existing token")
	}
	if got != child {
		t.Fatalf("commandTreeChild() = %p, want %p", got, child)
	}

	if got, ok := commandTreeChild(node, "missing"); ok || got != nil {
		t.Fatalf("commandTreeChild() missing = (%p, %v), want (nil, false)", got, ok)
	}
}

func TestCommandTreeChildEmpty(t *testing.T) {
	if got, ok := commandTreeChild(nil, "anything"); ok || got != nil {
		t.Fatalf("nil commandTreeChild() = (%p, %v), want (nil, false)", got, ok)
	}

	if got, ok := commandTreeChild(&itypes.CommandTreeNode{}, "anything"); ok || got != nil {
		t.Fatalf("empty commandTreeChild() = (%p, %v), want (nil, false)", got, ok)
	}
}
