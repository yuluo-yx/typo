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

// builtinCommandTrees contains static CLI grammars used directly by the engine.
// External tool fallback trees are defined in this file too, but ToolTreeRegistry
// still consumes them lazily so discovery and cache writes keep their current behavior.
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
			"stats":     commandLeaf(),
			"uninstall": commandLeaf(),
			"version":   commandLeaf(),
		}),
	},
}

var builtinToolTrees = buildBuiltinToolTrees()

func buildBuiltinToolTrees() map[string]*TreeNode {
	return map[string]*TreeNode{
		"git":       gitBuiltinTree(),
		"docker":    dockerBuiltinTree(),
		"npm":       flatToolTree("ci", "install", "list", "login", "publish", "run", "test", "uninstall", "update"),
		"yarn":      flatToolTree("add", "build", "cache", "create", "exec", "info", "init", "install", "remove", "run", "test", "upgrade"),
		"kubectl":   kubectlBuiltinTree(),
		"cargo":     flatToolTree("bench", "build", "check", "clean", "doc", "fmt", "help", "run", "test", "update"),
		"go":        flatToolTree("build", "clean", "env", "fmt", "generate", "get", "install", "list", "mod", "run", "test", "tool"),
		"brew":      flatToolTree("cleanup", "doctor", "info", "install", "list", "search", "tap", "uninstall", "update", "upgrade"),
		"terraform": flatToolTree("apply", "destroy", "fmt", "import", "init", "output", "plan", "show", "state", "validate"),
		"helm":      flatToolTree("dependency", "get", "install", "lint", "list", "package", "pull", "repo", "search", "template", "upgrade"),
		"aws":       flatToolTree("cloudwatch", "configure", "dynamodb", "ec2", "iam", "lambda", "rds", "s3", "sns", "sqs", "sts"),
		"gcloud":    flatToolTree("bigquery", "compute", "config", "functions", "iam", "kubernetes", "pubsub", "services", "storage"),
		"az":        flatToolTree("account", "aks", "functionapp", "group", "network", "storage", "vm", "webapp"),
	}
}

var dynamicOnlySubcommandTools = map[string]bool{
	"pip":      true,
	"pip3":     true,
	"composer": true,
	"ansible":  true,
}

var prefetchSubcommandTools = []string{"git", "docker", "npm", "yarn", "kubectl", "cargo", "go", "aws", "gcloud", "az"}

func builtinTreeForTool(tool string) *TreeNode {
	return builtinToolTrees[tool]
}

func isKnownSubcommandTool(tool string) bool {
	if tool == "" {
		return false
	}
	return builtinTreeForTool(tool) != nil || dynamicOnlySubcommandTools[tool]
}

func prefetchSubcommandToolNames() []string {
	return append([]string(nil), prefetchSubcommandTools...)
}

func builtinNodeForPath(tool string, prefix []string) *TreeNode {
	root := builtinTreeForTool(tool)
	if root == nil {
		return nil
	}
	node, ok := treeNodeForPath(root, prefix)
	if !ok {
		return nil
	}
	return node
}

func builtinChildrenForPath(tool string, prefix []string) []string {
	node := builtinNodeForPath(tool, prefix)
	if node == nil {
		return nil
	}
	return node.childTokens()
}

func treeNodeForPath(root *TreeNode, prefix []string) (*TreeNode, bool) {
	if root == nil {
		return nil, false
	}

	node := root
	for _, token := range prefix {
		if node == nil || len(node.Children) == 0 {
			return nil, false
		}
		child, ok := node.Children[token]
		if !ok {
			return nil, false
		}
		node = child
	}
	return node, true
}

func treeBranch(children map[string]*TreeNode) *TreeNode {
	node := &TreeNode{Children: children}
	node.refreshChildTokens()
	return node
}

func treeLeaf() *TreeNode {
	return &TreeNode{Terminal: true}
}

func treeLeafPassthrough() *TreeNode {
	return &TreeNode{Terminal: true, Passthrough: true}
}

func treeLeafAlias(target string) *TreeNode {
	return &TreeNode{Terminal: true, Alias: target}
}

func flatToolTree(subcommands ...string) *TreeNode {
	children := make(map[string]*TreeNode, len(subcommands))
	for _, subcommand := range subcommands {
		if subcommand == "" {
			continue
		}
		children[subcommand] = &TreeNode{}
	}
	return treeBranch(children)
}

func gitBuiltinTree() *TreeNode {
	return treeBranch(map[string]*TreeNode{
		"add":      treeLeafPassthrough(),
		"branch":   treeLeafPassthrough(),
		"checkout": treeLeafPassthrough(),
		"clone":    treeLeafPassthrough(),
		"commit":   treeLeafPassthrough(),
		"diff":     treeLeafPassthrough(),
		"fetch":    treeLeafPassthrough(),
		"init":     treeLeaf(),
		"log":      treeLeafPassthrough(),
		"merge":    treeLeafPassthrough(),
		"pull":     treeLeafPassthrough(),
		"push":     treeLeafPassthrough(),
		"rebase":   treeLeafPassthrough(),
		"remote": treeBranch(map[string]*TreeNode{
			"add":     treeLeaf(),
			"remove":  treeLeaf(),
			"rename":  treeLeaf(),
			"set-url": treeLeaf(),
			"show":    treeLeaf(),
			"prune":   treeLeaf(),
		}),
		"restore": treeLeafPassthrough(),
		"stash": treeBranch(map[string]*TreeNode{
			"save":  treeLeaf(),
			"list":  treeLeaf(),
			"pop":   treeLeaf(),
			"push":  treeLeaf(),
			"show":  treeLeaf(),
			"drop":  treeLeaf(),
			"clear": treeLeaf(),
			"apply": treeLeaf(),
		}),
		"status": treeLeaf(),
		"submodule": treeBranch(map[string]*TreeNode{
			"add":    treeLeaf(),
			"update": treeLeaf(),
			"init":   treeLeaf(),
			"deinit": treeLeaf(),
			"status": treeLeaf(),
			"sync":   treeLeaf(),
		}),
		"switch": treeLeafPassthrough(),
	})
}

func dockerBuiltinTree() *TreeNode {
	return treeBranch(map[string]*TreeNode{
		"build":   treeLeafPassthrough(),
		"compose": treeLeafPassthrough(),
		"container": treeBranch(map[string]*TreeNode{
			"create":  treeLeaf(),
			"start":   treeLeaf(),
			"stop":    treeLeaf(),
			"rm":      treeLeaf(),
			"exec":    treeLeafPassthrough(),
			"logs":    treeLeafPassthrough(),
			"ls":      treeLeaf(),
			"inspect": treeLeaf(),
			"kill":    treeLeaf(),
			"pause":   treeLeaf(),
			"unpause": treeLeaf(),
			"rename":  treeLeaf(),
			"restart": treeLeaf(),
			"run":     treeLeafPassthrough(),
			"top":     treeLeaf(),
			"update":  treeLeaf(),
			"wait":    treeLeaf(),
		}),
		"exec":    treeLeafPassthrough(),
		"image":   dockerImageTree(),
		"images":  treeLeaf(),
		"inspect": treeLeaf(),
		"logs":    treeLeafPassthrough(),
		"network": treeBranch(map[string]*TreeNode{
			"connect":    treeLeaf(),
			"create":     treeLeaf(),
			"disconnect": treeLeaf(),
			"inspect":    treeLeaf(),
			"ls":         treeLeaf(),
			"prune":      treeLeaf(),
			"rm":         treeLeaf(),
		}),
		"ps":    treeLeaf(),
		"pull":  treeLeaf(),
		"push":  treeLeaf(),
		"rm":    treeLeaf(),
		"run":   treeLeafPassthrough(),
		"start": treeLeaf(),
		"stop":  treeLeaf(),
		"volume": treeBranch(map[string]*TreeNode{
			"create":  treeLeaf(),
			"inspect": treeLeaf(),
			"ls":      treeLeaf(),
			"prune":   treeLeaf(),
			"rm":      treeLeaf(),
		}),
	})
}

func dockerImageTree() *TreeNode {
	return treeBranch(map[string]*TreeNode{
		"build":   treeLeaf(),
		"history": treeLeaf(),
		"import":  treeLeaf(),
		"inspect": treeLeaf(),
		"list":    treeLeaf(),
		"ls":      treeLeaf(),
		"load":    treeLeaf(),
		"prune":   treeLeaf(),
		"pull":    treeLeaf(),
		"push":    treeLeaf(),
		"rm":      treeLeaf(),
		"save":    treeLeaf(),
		"tag":     treeLeaf(),
	})
}

func kubectlBuiltinTree() *TreeNode {
	resourceTree := kubectlResourceTree()
	return treeBranch(map[string]*TreeNode{
		"api-resources": treeLeaf(),
		"apply":         treeLeafPassthrough(),
		"config": treeBranch(map[string]*TreeNode{
			"view":        treeLeaf(),
			"set":         treeLeaf(),
			"use-context": treeLeaf(),
		}),
		"create":   treeLeafPassthrough(),
		"delete":   resourceTree.clone(),
		"describe": resourceTree.clone(),
		"edit":     treeLeafPassthrough(),
		"exec":     treeLeafPassthrough(),
		"get":      resourceTree,
		"logs":     treeLeafPassthrough(),
		"patch":    treeLeafPassthrough(),
		"rollout": treeBranch(map[string]*TreeNode{
			"status":  treeLeaf(),
			"undo":    treeLeaf(),
			"restart": treeLeaf(),
		}),
	})
}

func kubectlResourceTree() *TreeNode {
	return treeBranch(map[string]*TreeNode{
		"pods":        treeLeaf(),
		"po":          treeLeafAlias("pods"),
		"deployments": treeLeaf(),
		"deploy":      treeLeafAlias("deployments"),
		"services":    treeLeaf(),
		"svc":         treeLeafAlias("services"),
		"nodes":       treeLeaf(),
		"no":          treeLeafAlias("nodes"),
		"configmaps":  treeLeaf(),
		"cm":          treeLeafAlias("configmaps"),
		"secrets":     treeLeaf(),
		"namespaces":  treeLeaf(),
		"ns":          treeLeafAlias("namespaces"),
		"ingresses":   treeLeaf(),
		"ing":         treeLeafAlias("ingresses"),
		"jobs":        treeLeaf(),
		"cronjobs":    treeLeaf(),
		"pv":          treeLeaf(),
		"pvc":         treeLeaf(),
	})
}
