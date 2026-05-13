package engine

import (
	"sort"

	itypes "github.com/yuluo-yx/typo/internal/types"
)

func commandTreeChildTokens(node *itypes.CommandTreeNode) []string {
	if node == nil || len(node.Children) == 0 {
		return nil
	}

	tokens := make([]string, 0, len(node.Children))
	for token := range node.Children {
		tokens = append(tokens, token)
	}
	sort.Strings(tokens)
	return tokens
}

func commandTreeChild(node *itypes.CommandTreeNode, token string) (*itypes.CommandTreeNode, bool) {
	if node == nil || len(node.Children) == 0 {
		return nil, false
	}

	child, ok := node.Children[token]
	return child, ok
}
