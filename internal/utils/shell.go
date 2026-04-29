package utils

import "mvdan.cc/sh/v3/syntax"

// ShellNodeRange returns a shell syntax node range capped by the raw input length.
func ShellNodeRange(node syntax.Node, rawLen int) (int, int) {
	return OffsetToIndex(node.Pos().Offset(), rawLen), OffsetToIndex(node.End().Offset(), rawLen)
}
