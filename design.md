# 多层子命令纠正设计

## Issue 参考

GitHub Issue: https://github.com/yuluo-yx/typo/issues/60

## 问题分析

### 当前已有的能力

现有引擎**已支持多层纠正**，通过以下机制实现：
- `GetChildren(tool, prefix []string)` 遍历前缀路径
- `collectSubcommandReplacements` 在每个层级进行模糊匹配
- 已有测试验证 `aws`、`gcloud`、`az` 的多层纠正

参考：`internal/engine/engine.go:919-963` 和 `internal/commands/subcommands.go:61-92`

### 真正的差距

问题在于**数据缺失**，而非架构：

| 工具 | 当前状态 | 需要的数据 |
|------|----------|------------|
| `git` | 动态发现可能获取 `stash`、`remote` | 嵌套：`stash → {save, list, pop}`、`remote → {add, remove}` |
| `docker` | 动态发现可能获取 `image`、`container` | 嵌套：`image → {build, list, pull}`、`container → {start, stop}` |
| `kubectl` | 动态发现可能获取 `get`、`describe` | 资源列表：`get → {pods, deployments}`（数百种资源） |
| `aws` | 多层动态发现已支持 | 工作正常，但超时敏感 |
| `gcloud` | 多层动态发现已支持 | 工作正常，但有 3 层限制 |

**关键说明**：`builtinSubcommands`（`internal/commands/subcommands.go:621-634`）只提供**扁平的一级数据**，不包含 `stash`、`image`、`container` 等嵌套父节点。这些只有在动态发现成功时才会出现。

### 根因

1. **`builtinSubcommands` 是扁平数组**：没有 git/docker/kubectl 的嵌套结构
2. **动态发现受限**：只有 `aws/gcloud/az` 支持 `fetchSubcommands(tool, prefix...)`
3. **解析器返回扁平列表**：`parseGitHelp` 返回 `[]string`，不是树结构
4. **缺少 fallback**：如果超时，用户无法获得嵌套纠正

---

## 设计目标

1. **统一树结构**：所有工具使用与 typo CLI 相同的 `CommandTreeNode`
2. **多层纠正**：每个层级独立进行模糊匹配
3. **完整语义**：支持 `terminal`、`passthrough`、`alias` 等节点类型
4. **破坏性变更**：旧缓存直接隔离，重新生成
5. **性能保持**：保持懒加载和缓存效率

---

## 新存储格式

### 文件：`~/.typo/subcommands.json`

```json
{
  "schema_version": 2,
  "tools": [
    {
      "tool": "git",
      "tree": {
        "children": {
          "stash": {
            "terminal": false,
            "children": {
              "save": {"terminal": true},
              "list": {"terminal": true},
              "pop": {"terminal": true},
              "push": {"terminal": true},
              "show": {"terminal": true},
              "drop": {"terminal": true},
              "clear": {"terminal": true}
            }
          },
          "remote": {
            "terminal": false,
            "children": {
              "add": {"terminal": true},
              "remove": {"terminal": true},
              "rename": {"terminal": true},
              "set-url": {"terminal": true},
              "show": {"terminal": true},
              "prune": {"terminal": true}
            }
          },
          "submodule": {
            "terminal": false,
            "children": {
              "add": {"terminal": true},
              "update": {"terminal": true},
              "init": {"terminal": true},
              "deinit": {"terminal": true},
              "status": {"terminal": true},
              "sync": {"terminal": true}
            }
          },
          "branch": {"terminal": true, "passthrough": true},
          "checkout": {"terminal": true, "passthrough": true},
          "commit": {"terminal": true, "passthrough": true},
          "status": {"terminal": true}
        }
      },
      "updated_at": "2026-04-14T12:00:00Z"
    },
    {
      "tool": "docker",
      "tree": {
        "children": {
          "image": {
            "terminal": false,
            "children": {
              "build": {"terminal": true},
              "history": {"terminal": true},
              "ls": {"terminal": true},
              "load": {"terminal": true},
              "prune": {"terminal": true},
              "pull": {"terminal": true},
              "push": {"terminal": true},
              "rm": {"terminal": true},
              "save": {"terminal": true},
              "tag": {"terminal": true}
            }
          },
          "container": {
            "terminal": false,
            "children": {
              "create": {"terminal": true},
              "start": {"terminal": true},
              "stop": {"terminal": true},
              "rm": {"terminal": true},
              "exec": {"terminal": true, "passthrough": true},
              "logs": {"terminal": true, "passthrough": true},
              "ls": {"terminal": true}
            }
          },
          "compose": {"terminal": true, "passthrough": true},
          "build": {"terminal": true, "passthrough": true},
          "pull": {"terminal": true}
        }
      },
      "updated_at": "2026-04-14T12:00:00Z"
    },
    {
      "tool": "kubectl",
      "tree": {
        "children": {
          "get": {
            "terminal": false,
            "children": {
              "pods": {"terminal": true, "alias": null},
              "po": {"terminal": true, "alias": "pods"},
              "deployments": {"terminal": true, "alias": null},
              "deploy": {"terminal": true, "alias": "deployments"},
              "services": {"terminal": true, "alias": null},
              "svc": {"terminal": true, "alias": "services"},
              "nodes": {"terminal": true},
              "configmaps": {"terminal": true},
              "secrets": {"terminal": true},
              "namespaces": {"terminal": true, "alias": null},
              "ns": {"terminal": true, "alias": "namespaces"}
            }
          },
          "describe": {
            "terminal": false,
            "children": {
              "pods": {"terminal": true},
              "deployments": {"terminal": true},
              "services": {"terminal": true},
              "nodes": {"terminal": true}
            }
          },
          "delete": {
            "terminal": false,
            "children": {
              "pods": {"terminal": true},
              "deployments": {"terminal": true}
            }
          },
          "apply": {"terminal": true, "passthrough": true},
          "create": {"terminal": true, "passthrough": true},
          "logs": {"terminal": true, "passthrough": true},
          "exec": {"terminal": true, "passthrough": true}
        }
      },
      "updated_at": "2026-04-14T12:00:00Z"
    }
  ]
}
```

### 节点语义说明

| 属性 | 含义 | 示例 |
|------|------|------|
| `terminal` | 是否可作为有效命令结束于此层级 | `git stash save` 的 `save` 是 terminal |
| `passthrough` | 匹配后是否接受任意操作数 | `git commit somefile` 的 `commit` 需要接受文件名 |
| `alias` | 指向另一个命令的别名 | `kubectl get po` → `kubectl get pods` |
| `children` | 子命令节点映射 | `stash` 有 `{save, list, pop}` 子节点 |

---

## 数据结构定义

### Go 类型定义

```go
// CacheHeader 用于检测 schema 版本
type CacheHeader struct {
    SchemaVersion int `json:"schema_version"`
}

// ToolTreeCache 单个工具的缓存树
type ToolTreeCache struct {
    Tool      string       `json:"tool"`
    Tree      *TreeNode    `json:"tree"`
    UpdatedAt time.Time    `json:"updated_at"`
}

// TreeNode JSON 序列化的树节点
type TreeNode struct {
    Children    map[string]*TreeNode `json:"children,omitempty"`
    Terminal    bool                 `json:"terminal,omitempty"`
    Passthrough bool                 `json:"passthrough,omitempty"`
    Alias       string               `json:"alias,omitempty"`
}

// ToCommandTreeNode 转换为引擎使用的 CommandTreeNode
func (n *TreeNode) ToCommandTreeNode() *CommandTreeNode {
    if n == nil {
        return nil
    }

    node := &CommandTreeNode{
        StopAfterMatch: n.Terminal && !n.Passthrough,
    }

    if len(n.Children) > 0 {
        node.Children = make(map[string]*CommandTreeNode, len(n.Children))
        for name, child := range n.Children {
            node.Children[name] = child.ToCommandTreeNode()
        }
    }

    return node
}

// GetEffectiveName 获取别名指向的实际命令名
func (n *TreeNode) GetEffectiveName() string {
    if n.Alias != "" {
        return n.Alias
    }
    return "" // 返回空表示无别名
}
```

### 统一的 ToolTreeRegistry

```go
// ToolTreeRegistry 管理所有工具的命令树
// 合并现有的 SubcommandRegistry 和 CommandTreeRegistry
type ToolTreeRegistry struct {
    mu             sync.RWMutex
    saveMu         sync.Mutex
    staticTrees    map[string]*CommandTree   // typo CLI 等静态树
    dynamicTrees   map[string]*CommandTree   // 动态发现的工具树
    cacheDir       string
    cacheExpiry    time.Duration
    discovery      DiscoveryRegistry
}

// GetTree 返回工具的命令树
func (r *ToolTreeRegistry) GetTree(tool string) *CommandTree {
    // 1. 先查静态树（typo CLI）
    if tree, ok := r.staticTrees[tool]; ok {
        return tree
    }

    // 2. 查缓存/动态树
    r.mu.RLock()
    if tree, ok := r.dynamicTrees[tool]; ok {
        r.mu.RUnlock()
        return tree
    }
    r.mu.RUnlock()

    // 3. 未找到，尝试动态发现
    return r.discoverAndCache(tool)
}

// GetChildren 返回指定路径的子命令列表
// 保持与现有 API 兼容
func (r *ToolTreeRegistry) GetChildren(tool string, prefix []string) []string {
    tree := r.GetTree(tool)
    if tree == nil {
        return nil
    }

    node := tree.Node
    for _, p := range prefix {
        child, ok := node.Child(p)
        if !ok {
            return nil
        }
        node = child
    }

    return node.ChildTokens()
}
```

---

## 内置命令树定义

扩展 `builtinCommandTrees`，添加常用工具的已知结构：

```go
var builtinCommandTrees = []*CommandTree{
    // typo CLI（保持不变）
    {Root: "typo", Node: typoTree},

    // git 嵌套结构
    {Root: "git", Node: commandBranch(map[string]*CommandTreeNode{
        "stash": commandBranch(map[string]*CommandTreeNode{
            "save":  commandLeaf(),
            "list":  commandLeaf(),
            "pop":   commandLeaf(),
            "push":  commandLeaf(),
            "show":  commandLeaf(),
            "drop":  commandLeaf(),
            "clear": commandLeaf(),
        }),
        "remote": commandBranch(map[string]*CommandTreeNode{
            "add":     commandLeaf(),
            "remove":  commandLeaf(),
            "rename":  commandLeaf(),
            "set-url": commandLeaf(),
            "show":    commandLeaf(),
            "prune":   commandLeaf(),
        }),
        "submodule": commandBranch(map[string]*CommandTreeNode{
            "add":    commandLeaf(),
            "update": commandLeaf(),
            "init":   commandLeaf(),
            "deinit": commandLeaf(),
            "status": commandLeaf(),
            "sync":   commandLeaf(),
        }),
        "branch":   commandLeafPassthrough(), // 接受分支名
        "checkout": commandLeafPassthrough(),
        "commit":   commandLeafPassthrough(),
        "status":   commandLeaf(),
        "add":      commandLeafPassthrough(),
        "push":     commandLeafPassthrough(),
        "pull":     commandLeafPassthrough(),
        "diff":     commandLeafPassthrough(),
        "log":      commandLeafPassthrough(),
        "reset":    commandLeafPassthrough(),
        "rebase":   commandLeafPassthrough(),
        "merge":    commandLeafPassthrough(),
        "switch":   commandLeafPassthrough(),
        "restore":  commandLeafPassthrough(),
    })},

    // docker 嵌套结构
    {Root: "docker", Node: commandBranch(map[string]*CommandTreeNode{
        "image": commandBranch(map[string]*CommandTreeNode{
            "build":   commandLeaf(),
            "history": commandLeaf(),
            "ls":      commandLeaf(),
            "load":    commandLeaf(),
            "prune":   commandLeaf(),
            "pull":    commandLeaf(),
            "push":    commandLeaf(),
            "rm":      commandLeaf(),
            "save":    commandLeaf(),
            "tag":     commandLeaf(),
        }),
        "container": commandBranch(map[string]*CommandTreeNode{
            "create":  commandLeaf(),
            "start":   commandLeaf(),
            "stop":    commandLeaf(),
            "rm":      commandLeaf(),
            "exec":    commandLeafPassthrough(),
            "logs":    commandLeafPassthrough(),
            "ls":      commandLeaf(),
            "inspect": commandLeaf(),
            "kill":    commandLeaf(),
            "pause":   commandLeaf(),
            "unpause": commandLeaf(),
            "rename":  commandLeaf(),
            "restart": commandLeaf(),
            "run":     commandLeafPassthrough(),
            "top":     commandLeaf(),
            "update":  commandLeaf(),
            "wait":    commandLeaf(),
        }),
        "network": commandBranch(map[string]*CommandTreeNode{
            "connect":    commandLeaf(),
            "create":     commandLeaf(),
            "disconnect": commandLeaf(),
            "inspect":    commandLeaf(),
            "ls":         commandLeaf(),
            "prune":      commandLeaf(),
            "rm":         commandLeaf(),
        }),
        "volume": commandBranch(map[string]*CommandTreeNode{
            "create":  commandLeaf(),
            "inspect": commandLeaf(),
            "ls":      commandLeaf(),
            "prune":   commandLeaf(),
            "rm":      commandLeaf(),
        }),
        "compose":  commandLeafPassthrough(),
        "build":    commandLeafPassthrough(),
        "pull":     commandLeaf(),
        "push":     commandLeaf(),
        "run":      commandLeafPassthrough(),
        "ps":       commandLeaf(),
        "exec":     commandLeafPassthrough(),
        "logs":     commandLeafPassthrough(),
        "inspect":  commandLeaf(),
    })},

    // kubectl 资源结构
    {Root: "kubectl", Node: commandBranch(map[string]*CommandTreeNode{
        "get": commandBranch(map[string]*CommandTreeNode{
            "pods":       commandLeaf(),
            "po":         commandLeafAlias("pods"),
            "deployments": commandLeaf(),
            "deploy":     commandLeafAlias("deployments"),
            "services":   commandLeaf(),
            "svc":        commandLeafAlias("services"),
            "nodes":      commandLeaf(),
            "no":         commandLeafAlias("nodes"),
            "configmaps": commandLeaf(),
            "cm":         commandLeafAlias("configmaps"),
            "secrets":    commandLeaf(),
            "namespaces": commandLeaf(),
            "ns":         commandLeafAlias("namespaces"),
            "ingresses":  commandLeaf(),
            "ing":        commandLeafAlias("ingresses"),
            "jobs":       commandLeaf(),
            "cronjobs":   commandLeaf(),
            "pv":         commandLeaf(),
            "pvc":        commandLeaf(),
        }),
        "describe": commandBranch(map[string]*CommandTreeNode{
            "pods":       commandLeaf(),
            "deployments": commandLeaf(),
            "services":   commandLeaf(),
            "nodes":      commandLeaf(),
            "configmaps": commandLeaf(),
            "secrets":    commandLeaf(),
            "namespaces": commandLeaf(),
        }),
        "delete": commandBranch(map[string]*CommandTreeNode{
            "pods":       commandLeaf(),
            "deployments": commandLeaf(),
            "services":   commandLeaf(),
            "nodes":      commandLeaf(),
            "configmaps": commandLeaf(),
            "secrets":    commandLeaf(),
            "namespaces": commandLeaf(),
        }),
        "apply":   commandLeafPassthrough(),
        "create":  commandLeafPassthrough(),
        "edit":    commandLeafPassthrough(),
        "logs":    commandLeafPassthrough(),
        "exec":    commandLeafPassthrough(),
        "rollout": commandBranch(map[string]*CommandTreeNode{
            "status":  commandLeaf(),
            "undo":    commandLeaf(),
            "restart": commandLeaf(),
        }),
        "config": commandBranch(map[string]*CommandTreeNode{
            "view":    commandLeaf(),
            "set":     commandLeaf(),
            "use-context": commandLeaf(),
        }),
    })},
}

// 辅助构造函数
func commandLeaf() *CommandTreeNode {
    return &CommandTreeNode{StopAfterMatch: true}
}

func commandLeafPassthrough() *CommandTreeNode {
    return &CommandTreeNode{StopAfterMatch: false} // 不停止，接受参数
}

func commandLeafAlias(target string) *CommandTreeNode {
    // 别名节点：标记别名关系，但结构上与 leaf 相同
    // 实际别名处理在匹配逻辑中完成
    return &CommandTreeNode{StopAfterMatch: true}
}
```

---

## 动态发现契约

### 可执行的发现接口

```go
// DiscoveryContract 定义工具的嵌套发现规则
type DiscoveryContract struct {
    Tool            string        // 工具名称
    MaxDepth        int           // 最大嵌套层级（1=仅根，2=根+一层）
    MaxCommands     int           // 最多探索的一级命令数
    TimeoutPerCmd   time.Duration // 每个子进程的超时
    MaxTotalTimeout time.Duration // 全局超时

    // 根级发现
    RootArgs   []string      // 获取帮助的参数
    RootParser func(string) []string // 解析函数

    // 嵌套发现
    NestedArgs   func(prefix []string) []string // 嵌套帮助参数
    NestedParser func(string) []string          // 嵌套解析函数

    // Fallback：发现失败时使用的内置结构
    BuiltinTree *TreeNode
}

var discoveryContracts = map[string]DiscoveryContract{
    "git": {
        Tool:            "git",
        MaxDepth:        2,
        MaxCommands:     15,
        TimeoutPerCmd:   500 * time.Millisecond,
        MaxTotalTimeout: 2 * time.Second,
        RootArgs:        []string{"help", "-a"},
        RootParser:      parseGitHelp,
        NestedArgs: func(prefix []string) []string {
            return append(prefix, "-h") // git stash -h
        },
        NestedParser: parseGitNestedHelp,
        BuiltinTree:  gitBuiltinTree, // 使用 builtinCommandTrees 中的 git 结构
    },

    "docker": {
        Tool:            "docker",
        MaxDepth:        2,
        MaxCommands:     10,
        TimeoutPerCmd:   500 * time.Millisecond,
        MaxTotalTimeout: 2 * time.Second,
        RootArgs:        []string{"--help"},
        RootParser:      parseDockerHelp,
        NestedArgs: func(prefix []string) []string {
            return append(prefix, "--help") // docker image --help
        },
        NestedParser: parseDockerNestedHelp,
        BuiltinTree:  dockerBuiltinTree,
    },

    "kubectl": {
        Tool:            "kubectl",
        MaxDepth:        2,
        MaxCommands:     8, // get, describe, delete, create, apply, logs, exec, edit
        TimeoutPerCmd:   500 * time.Millisecond,
        MaxTotalTimeout: 2 * time.Second,
        RootArgs:        []string{"--help"},
        RootParser:      parseKubectlHelp,
        // kubectl 资源不动态发现（数百种资源，超时不可靠）
        NestedArgs:   nil,
        NestedParser: nil,
        BuiltinTree:  kubectlBuiltinTree, // 仅使用内置资源列表
    },
}
```

### 带预算的发现实现

```go
func (r *ToolTreeRegistry) discoverNested(ctx context.Context, tool string) *TreeNode {
    contract, ok := discoveryContracts[tool]
    if !ok {
        return nil
    }

    // 如果没有嵌套发现能力（如 kubectl），直接返回内置树
    if contract.NestedArgs == nil {
        return contract.BuiltinTree
    }

    // 获取根级命令（带上下文超时）
    rootOutput, err := r.fetchHelpWithContext(ctx, tool, contract.RootArgs)
    if err != nil || rootOutput == "" {
        return contract.BuiltinTree // fallback
    }

    rootCommands := contract.RootParser(rootOutput)

    // 筛选：只发现内置已知的嵌套候选
    candidates := filterBuiltinCandidates(rootCommands, contract.BuiltinTree, contract.MaxCommands)

    // 构建树
    tree := &TreeNode{Children: make(map[string]*TreeNode)}

    for _, cmd := range candidates {
        // 每次检查上下文截止时间
        if ctx.Err() != nil {
            break
        }

        // 创建每命令超时上下文（从父上下文继承）
        cmdCtx, cancel := context.WithTimeout(ctx, contract.TimeoutPerCmd)
        defer cancel()

        nestedArgs := contract.NestedArgs([]string{cmd})
        nestedOutput, err := r.fetchHelpWithContext(cmdCtx, tool, nestedArgs)

        if err != nil || nestedOutput == "" {
            // 使用内置 fallback
            if builtinChild := getBuiltinChild(contract.BuiltinTree, cmd); builtinChild != nil {
                tree.Children[cmd] = builtinChild
            }
            continue
        }

        nested := contract.NestedParser(nestedOutput)
        if len(nested) > 0 {
            tree.Children[cmd] = &TreeNode{
                Children:    buildChildNodes(nested),
                Terminal:    false,
            }
        }
    }

    // 合入未发现的内置嵌套命令
    mergeBuiltinMissing(tree, contract.BuiltinTree, candidates)

    return tree
}

// fetchHelpWithContext 运行帮助命令，保持进程组清理契约
func (r *ToolTreeRegistry) fetchHelpWithContext(ctx context.Context, tool string, args []string) (string, error) {
    cmd := exec.Command(tool, args...)
    configureHelpCommand(cmd) // 在 Unix 上设置进程组

    var output bytes.Buffer
    cmd.Stdout = &output
    cmd.Stderr = &output

    if err := cmd.Start(); err != nil {
        return "", err
    }

    // 使用 select 模式等待，保持进程组清理
    waitCh := make(chan error, 1)
    go func() {
        waitCh <- cmd.Wait()
    }()

    select {
    case err := <-waitCh:
        return output.String(), err
    case <-ctx.Done():
        _ = killHelpCommand(cmd) // 杀掉整个进程组
        <-waitCh                 // 等待进程完全终止
        return "", ctx.Err()
    }
}
```

---

## 缓存加载与迁移

### 加载逻辑

```go
func (r *ToolTreeRegistry) loadCache() {
    cacheFile := filepath.Join(r.cacheDir, "subcommands.json")
    data, err := os.ReadFile(cacheFile)
    if err != nil {
        return // 无缓存，首次使用时发现
    }

    // 检测 schema 版本
    var header CacheHeader
    if err := json.Unmarshal(data, &header); err != nil {
        storage.QuarantineInvalidJSON(cacheFile, err)
        return
    }

    // 只接受 schema_version == 2（树格式）
    if header.SchemaVersion == 2 {
        r.loadTreeFormat(data)
        return
    }

    // 其他版本（包括旧版 v1 或无版本）隔离并重新发现
    storage.QuarantineInvalidJSON(cacheFile, 
        fmt.Errorf("schema_version=%d not supported, requires version 2", header.SchemaVersion))
}

func (r *ToolTreeRegistry) loadTreeFormat(data []byte) {
    var wrapper struct {
        SchemaVersion int            `json:"schema_version"`
        Tools         []*ToolTreeCache `json:"tools"`
    }

    if err := json.Unmarshal(data, &wrapper); err != nil {
        storage.QuarantineInvalidJSON(cacheFile, err)
        return
    }

    r.mu.Lock()
    for _, cache := range wrapper.Tools {
        tree := &CommandTree{
            Root: cache.Tool,
            Node: cache.Tree.ToCommandTreeNode(),
        }
        r.dynamicTrees[cache.Tool] = tree
    }
    r.mu.Unlock()
}

func (r *ToolTreeRegistry) saveCache() {
    wrapper := struct {
        SchemaVersion int            `json:"schema_version"`
        Tools         []*ToolTreeCache `json:"tools"`
    }{
        SchemaVersion: 2,
        Tools:         r.getAllCaches(),
    }

    data, _ := json.MarshalIndent(wrapper, "", "  ")
    _ = storage.WriteFileAtomic(cacheFile, data, 0600)
}
```

### 迁移行为

| 场景 | 行为 |
|------|------|
| 无缓存文件 | 首次使用时动态发现，保存为 `schema_version: 2` |
| 旧版缓存（无 `schema_version` 或 v1） | 隔离为 `.corrupt-<时间戳>`，重新发现 |
| `schema_version: 2` | 直接加载 |
| `schema_version >= 3` | 隔离，提示需要升级 |

---

## 引擎集成

### 保持 Shell 解析行为

**核心原则**：只修改数据查找后端，保持现有 shell 解析和选项处理逻辑。

```go
// collectSubcommandReplacements 保持不变
// 只有 GetChildren 调用的后端从 SubcommandRegistry 改为 ToolTreeRegistry

func (e *Engine) collectSubcommandReplacements(mainCmd string, tokens []string, startIdx int) ([]subcommandReplacement, bool) {
    cfg := e.distanceMatchConfig()
    prefix := make([]string, 0, len(tokens)-startIdx)
    replacements := make([]subcommandReplacement, 0)
    changed := false
    expectValue := false

    for i := startIdx; i < len(tokens); i++ {
        token := tokens[i]
        if expectValue {
            expectValue = false
            continue
        }

        // 通过 GetChildren 获取当前层级的子命令候选
        // API 不变，但后端现在使用树结构
        subcommands := e.toolTrees.GetChildren(mainCmd, prefix)
        if len(subcommands) == 0 {
            break
        }

        if token == "--" {
            break
        }

        // 选项处理逻辑保持不变
        if handled, needsValue := subcommandOptionBehavior(mainCmd, token, subcommands, ...); handled {
            expectValue = needsValue
            continue
        }

        if containsString(subcommands, token) {
            prefix = append(prefix, token)
            continue
        }

        match, distance := closestSubcommand(token, subcommands, cfg)
        if !isGoodDistanceMatch(token, match, distance, cfg) {
            break
        }

        replacements = append(replacements, subcommandReplacement{index: i, value: match})
        prefix = append(prefix, match)
        changed = true
    }

    return replacements, changed
}
```

### 别名处理

```go
// matchTreeChild 在匹配时处理别名
func (e *Engine) matchTreeChild(token string, node *CommandTreeNode) (string, *CommandTreeNode, bool) {
    if node == nil || token == "" {
        return "", nil, false
    }

    // 先尝试精确匹配
    if child, ok := node.Child(token); ok {
        return token, child, true
    }

    // 尝试别名匹配
    for name, child := range node.Children {
        if childNode := child; childNode != nil {
            // 检查是否是别名节点且别名指向的目标匹配 token
            // 这需要 TreeNode 保留别名信息
        }
    }

    // 模糊匹配
    // ... 现有逻辑
}
```

---

## 预期行为

| 输入 | 输出 | 工作原理 |
|------|------|----------|
| `git stsh save` | `git stash save` | `GetChildren("git", [])` → `stash`，`GetChildren("git", ["stash"])` → `save` |
| `git stash sve` | `git stash save` | 第二层级纠正 `sve` → `save` |
| `docker imge list` | `docker image list` | 第一层级纠正 `imge` → `image`，保留 `list` |
| `docker container stp` | `docker container stop` | 两层遍历 |
| `kubectl get pds` | `kubectl get pods` | 工具名纠正 + 资源纠正 |
| `kubectl get po` | `kubectl get pods` | 别名 `po` → `pods` |
| `git commit somefile` | `git commit somefile` | `passthrough` 允许接受任意参数 |

---

## 性能分析

### 发现成本

| 工具 | 子进程数 | 总超时 | 说明 |
|------|----------|--------|------|
| `git` | 1 + ≤15 嵌套 | ≤2s | 只探索内置已知的嵌套候选 |
| `docker` | 1 + ≤10 嵌套 | ≤2s | 同上 |
| `kubectl` | 1（仅根） | ≤500ms | 资源仅用内置，无动态发现 |
| `aws` | 现有逻辑 | 不变 | 已支持多层 |
| `gcloud` | 现有逻辑 | 不变 | 已支持多层 |

### 匹配复杂度

- 每层级：扫描所有子节点 → `O(n)`，其中 `n` = 子节点数
- 总复杂度：`O(depth × n)`，与现有逻辑相同

### 缓存大小

| 格式 | git 示例大小 |
|------|--------------|
| 树格式（v2） | ~6KB |

每个工具 <10KB，总体影响很小。

---

## 测试覆盖

### 必须覆盖的测试场景

| 分类 | 测试 |
|------|------|
| 多层纠正 | `git stsh save`, `git stash sve`, `docker imge list`, `docker container stp`, `kubectl get pds` |
| 选项处理 | `git -C /path stash sve`, `docker --context prod imge list`, `kubectl get pds -n prod` |
| 选项在层级间 | `git stash --all sve`, `docker container --help stp` |
| `--` 分隔符 | `git stash -- sve`, `kubectl get -- pods` |
| Shell 包装器 | `sudo git stsh save`, `sudo docker imge list` |
| 复合命令 | `git stsh save && git stsh list` |
| 别名 | `kubectl get po` → `kubectl get pods` |
| Passthrough | `git commit somefile`（不纠正文件名） |
| 空嵌套 | `git logs sve`（无嵌套结构，不应纠正） |
| 缓存迁移 | 旧版 v1 缓存被隔离，新版 v2 正常加载 |
| 发现超时 | 部分发现成功，fallback 到内置 |

### 基准目标

| 指标 | 目标 |
|------|------|
| 首次发现延迟 | ≤2s |
| 缓存加载时间 | ≤10ms |
| 每次纠正延迟 | ≤5ms |
| 每工具内存占用 | ≤10KB |

---

## 实现步骤

1. **阶段 1：数据结构**
   - 定义 `TreeNode` 和 `ToolTreeCache`
   - 添加转换方法 `ToCommandTreeNode`
   - 实现 JSON 序列化

2. **阶段 2：Registry 统一**
   - 创建 `ToolTreeRegistry` 合并静态和动态树
   - 实现 `GetTree(tool)` 和 `GetChildren(tool, prefix)`
   - 保持 API 兼容

3. **阶段 3：内置树扩展**
   - 扩展 `builtinCommandTrees` 添加 git/docker/kubectl
   - 添加 `commandLeafPassthrough()` 构造函数
   - 实现别名标记

4. **阶段 4：发现增强**
   - 定义 `DiscoveryContract` 契约
   - 实现带预算的 `discoverNested`
   - 保持进程组清理契约

5. **阶段 5：引擎集成**
   - 替换 `SubcommandRegistry` 为 `ToolTreeRegistry`
   - 保持 shell 解析逻辑不变
   - 添加别名处理

6. **阶段 6：迁移与测试**
   - 实现缓存加载逻辑（只接受 v2）
   - 添加隔离旧版缓存的处理
   - 补充测试覆盖

---

## 风险与缓解

| 风险 | 概率 | 影响 | 缓解措施 |
|------|------|------|----------|
| 发现超时 | 中 | 高 | 1s 单命令超时；内置树作为 fallback |
| 帮助输出解析错误 | 低 | 中 | 内置验证；用户可手动清理缓存 |
| 缓存迁移困惑 | 中 | 低 | 清晰的升级文档；自动隔离 |
| 内存占用 | 低 | 低 | 树结构紧凑；每工具 <10KB |

---

## 总结

本设计通过统一树结构解决 Issue #60 的多层子命令纠正需求：

1. **新缓存格式**：`schema_version: 2` + 递归 `TreeNode`
2. **统一 Registry**：`ToolTreeRegistry` 合并静态和动态树
3. **完整语义**：`terminal`、`passthrough`、`alias`
4. **内置 fallback**：发现失败时使用预定义结构
5. **破坏性迁移**：旧缓存隔离，重新发现

实现保持 typo 核心原则：即时反馈、保守纠正、Shell-aware 解析、最小延迟。
