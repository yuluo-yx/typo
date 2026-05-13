package types

// CommandTreeNode describes the valid command tokens below one grammar node.
type CommandTreeNode struct {
	Children       map[string]*CommandTreeNode
	StopAfterMatch bool
	Alias          string
}

// CommandTree defines one canonical CLI grammar rooted at a command word.
type CommandTree struct {
	Root string
	Node *CommandTreeNode
}
