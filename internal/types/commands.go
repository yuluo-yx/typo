package types

import "sort"

// CommandTreeNode describes the valid command tokens below one grammar node.
type CommandTreeNode struct {
	Children       map[string]*CommandTreeNode
	StopAfterMatch bool
	Alias          string
}

// ChildTokens returns the canonical child tokens in deterministic order.
func (n *CommandTreeNode) ChildTokens() []string {
	if n == nil || len(n.Children) == 0 {
		return nil
	}

	tokens := make([]string, 0, len(n.Children))
	for token := range n.Children {
		tokens = append(tokens, token)
	}
	sort.Strings(tokens)
	return tokens
}

// Child returns the child node for the given canonical token.
func (n *CommandTreeNode) Child(token string) (*CommandTreeNode, bool) {
	if n == nil || len(n.Children) == 0 {
		return nil, false
	}

	child, ok := n.Children[token]
	return child, ok
}

// CommandTree defines one canonical CLI grammar rooted at a command word.
type CommandTree struct {
	Root string
	Node *CommandTreeNode
}
