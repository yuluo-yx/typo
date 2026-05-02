package types

import "testing"

func TestCommandTreeNodeChildTokens(t *testing.T) {
	node := &CommandTreeNode{
		Children: map[string]*CommandTreeNode{
			"zsh":  {},
			"bash": {},
			"fish": {},
		},
	}

	got := node.ChildTokens()
	want := []string{"bash", "fish", "zsh"}
	if len(got) != len(want) {
		t.Fatalf("ChildTokens() length = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ChildTokens()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCommandTreeNodeChildTokensEmpty(t *testing.T) {
	var nilNode *CommandTreeNode
	if got := nilNode.ChildTokens(); got != nil {
		t.Fatalf("nil ChildTokens() = %v, want nil", got)
	}

	if got := (&CommandTreeNode{}).ChildTokens(); got != nil {
		t.Fatalf("empty ChildTokens() = %v, want nil", got)
	}
}

func TestCommandTreeNodeChild(t *testing.T) {
	child := &CommandTreeNode{StopAfterMatch: true}
	node := &CommandTreeNode{
		Children: map[string]*CommandTreeNode{
			"version": child,
		},
	}

	got, ok := node.Child("version")
	if !ok {
		t.Fatal("Child() did not find existing token")
	}
	if got != child {
		t.Fatalf("Child() = %p, want %p", got, child)
	}

	if got, ok := node.Child("missing"); ok || got != nil {
		t.Fatalf("Child() missing = (%p, %v), want (nil, false)", got, ok)
	}
}

func TestCommandTreeNodeChildEmpty(t *testing.T) {
	var nilNode *CommandTreeNode
	if got, ok := nilNode.Child("anything"); ok || got != nil {
		t.Fatalf("nil Child() = (%p, %v), want (nil, false)", got, ok)
	}

	if got, ok := (&CommandTreeNode{}).Child("anything"); ok || got != nil {
		t.Fatalf("empty Child() = (%p, %v), want (nil, false)", got, ok)
	}
}
