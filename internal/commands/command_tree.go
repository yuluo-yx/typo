package commands

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

// CommandTreeRegistry stores builtin command trees.
type CommandTreeRegistry struct {
	trees  []*CommandTree
	byRoot map[string]*CommandTree
}

// NewCommandTreeRegistry creates a registry populated with builtin command trees.
func NewCommandTreeRegistry() *CommandTreeRegistry {
	registry := &CommandTreeRegistry{
		trees:  make([]*CommandTree, 0, len(builtinCommandTrees)),
		byRoot: make(map[string]*CommandTree, len(builtinCommandTrees)),
	}

	for _, tree := range builtinCommandTrees {
		if tree == nil || tree.Root == "" || tree.Node == nil {
			continue
		}
		registry.trees = append(registry.trees, tree)
		registry.byRoot[tree.Root] = tree
	}

	return registry
}

// Trees returns all registered trees.
func (r *CommandTreeRegistry) Trees() []*CommandTree {
	if r == nil || len(r.trees) == 0 {
		return nil
	}

	return append([]*CommandTree(nil), r.trees...)
}

// HasRoot reports whether a command tree exists for the given root token.
func (r *CommandTreeRegistry) HasRoot(root string) bool {
	if r == nil || root == "" {
		return false
	}

	_, ok := r.byRoot[root]
	return ok
}

func commandLeaf() *CommandTreeNode {
	return &CommandTreeNode{StopAfterMatch: true}
}

func commandBranch(children map[string]*CommandTreeNode) *CommandTreeNode {
	return &CommandTreeNode{Children: children}
}

// builtinCommandTrees contains builtin CLI grammars that are stable enough to model as
// static command trees. It currently only includes typo because its multi-level
// command surface is explicit and stable, which makes conservative local correction
// feasible. External tools such as git and docker still use dynamic subcommand
// discovery and caching instead of being hard-coded here.
var builtinCommandTrees = []*CommandTree{
	{
		Root: "typo",
		Node: commandBranch(map[string]*CommandTreeNode{
			"config": commandBranch(map[string]*CommandTreeNode{
				"gen":   commandLeaf(),
				"get":   commandLeaf(),
				"list":  commandLeaf(),
				"reset": commandLeaf(),
				"set":   commandLeaf(),
			}),
			"doctor": commandLeaf(),
			"fix":    commandLeaf(),
			"help":   commandLeaf(),
			"history": commandBranch(map[string]*CommandTreeNode{
				"clear": commandLeaf(),
				"list":  commandLeaf(),
			}),
			"init": commandBranch(map[string]*CommandTreeNode{
				"bash": commandLeaf(),
				"fish": commandLeaf(),
				"zsh":  commandLeaf(),
			}),
			"learn": commandLeaf(),
			"rules": commandBranch(map[string]*CommandTreeNode{
				"add":     commandLeaf(),
				"disable": commandLeaf(),
				"enable":  commandLeaf(),
				"list":    commandLeaf(),
				"remove":  commandLeaf(),
			}),
			"uninstall": commandLeaf(),
			"version":   commandLeaf(),
		}),
	},
}
