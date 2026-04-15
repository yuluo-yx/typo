package commands

import itypes "github.com/yuluo-yx/typo/internal/types"

// CommandTreeRegistry stores builtin command trees.
type CommandTreeRegistry struct {
	trees  []*itypes.CommandTree
	byRoot map[string]*itypes.CommandTree
}

// NewCommandTreeRegistry creates a registry populated with builtin command trees.
func NewCommandTreeRegistry() *CommandTreeRegistry {
	registry := &CommandTreeRegistry{
		trees:  make([]*itypes.CommandTree, 0, len(builtinCommandTrees)),
		byRoot: make(map[string]*itypes.CommandTree, len(builtinCommandTrees)),
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
func (r *CommandTreeRegistry) Trees() []*itypes.CommandTree {
	if r == nil || len(r.trees) == 0 {
		return nil
	}

	return append([]*itypes.CommandTree(nil), r.trees...)
}

// HasRoot reports whether a command tree exists for the given root token.
func (r *CommandTreeRegistry) HasRoot(root string) bool {
	if r == nil || root == "" {
		return false
	}

	_, ok := r.byRoot[root]
	return ok
}

func commandLeaf() *itypes.CommandTreeNode {
	return &itypes.CommandTreeNode{StopAfterMatch: true}
}

func commandBranch(children map[string]*itypes.CommandTreeNode) *itypes.CommandTreeNode {
	return &itypes.CommandTreeNode{Children: children}
}

// builtinCommandTrees contains builtin CLI grammars that are stable enough to model as
// static command trees. It currently only includes typo because its multi-level
// command surface is explicit and stable, which makes conservative local correction
// feasible. External tools such as git and docker still use dynamic subcommand
// discovery and caching instead of being hard-coded here.
var builtinCommandTrees = []*itypes.CommandTree{
	{
		Root: "typo",
		Node: commandBranch(map[string]*itypes.CommandTreeNode{
			"config": commandBranch(map[string]*itypes.CommandTreeNode{
				"gen":   commandLeaf(),
				"get":   commandLeaf(),
				"list":  commandLeaf(),
				"reset": commandLeaf(),
				"set":   commandLeaf(),
			}),
			"doctor": commandLeaf(),
			"fix":    commandLeaf(),
			"help":   commandLeaf(),
			"history": commandBranch(map[string]*itypes.CommandTreeNode{
				"clear": commandLeaf(),
				"list":  commandLeaf(),
			}),
			"init": commandBranch(map[string]*itypes.CommandTreeNode{
				"bash": commandLeaf(),
				"fish": commandLeaf(),
				"zsh":  commandLeaf(),
			}),
			"learn": commandLeaf(),
			"rules": commandBranch(map[string]*itypes.CommandTreeNode{
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
