# 从 0.x 升级到 v1

[English](upgrade-from-0x.md) | 简体中文

本指南说明从 Typo 0.x 版本升级到 v1 时的破坏性变更和迁移步骤。

## 升级前准备

1. 确认当前版本：

   ```bash
   typo version
   ```

2. 备份本地数据目录：

   ```bash
   cp -r ~/.typo ~/.typo.backup
   ```

3. 记录当前使用的 shell 集成行。运行 `typo doctor` 确认。

## 破坏性变更

### 配置文件格式

Typo 0.1.x 没有配置文件。如果从 0.1.x 升级，无需迁移 — v1 会在首次使用时自动创建 `~/.typo/config.json` 并使用合理的默认值。

如果从 0.2.x 升级，配置文件格式（`~/.typo/config.json`）与 v1 向前兼容。以下键名保持不变：

- `similarity_threshold`
- `max_edit_distance`
- `max_fix_passes`
- `auto_learn_threshold`
- `keyboard`
- `history.enabled`
- `rules.<scope>.enabled`

v1 可能会引入额外的配置键。已有文件中的未知键会被保留但不会被当前版本使用。

如果配置文件包含无效 JSON，v1 会自动将其隔离（重命名为 `config.json.corrupt-<时间戳>`），并回退到默认配置。可以通过以下命令重新生成配置：

```bash
typo config gen --force
```

### CLI 变更

| 0.x 命令 | v1 对应命令 | 说明 |
|---|---|---|
| `typo fix <command>` | `typo fix <command>` | 不变 |
| `typo learn <from> <to>` | `typo learn <from> <to>` | 不变 |
| 无 | `typo config list\|get\|set\|reset\|gen` | 0.2.0 新增，v1 中稳定 |
| 无 | `typo rules list\|add\|remove\|enable\|disable` | 0.2.0 新增，v1 中稳定 |
| 无 | `typo history list\|clear` | 0.2.0 新增，v1 中稳定 |
| 无 | `typo doctor` | v1 新增 |
| 无 | `typo uninstall` | v1 新增 |

如果你的脚本调用了 `typo fix`，升级后无需修改。`-s <file>` 和 `--exit-code <n>` 参数保持不变。

### Shell 集成

Shell init 命令与 0.2.x 保持一致：

```bash
# zsh
eval "$(typo init zsh)"

# bash
eval "$(typo init bash)"

# fish
typo init fish | source

# PowerShell
Invoke-Expression (& typo init powershell | Out-String)
```

需要注意的变更：

- **fish 支持** 在 v1 中新增。如果你之前在 0.x 上使用 zsh 或 bash，想要切换到 fish，请添加上方的 fish init 行。
- **`TYPO_SHELL_INTEGRATION` 环境变量** 在 v1 中由所有 shell 集成设置为 `1`。`typo doctor` 会检查此变量。如果你有自定义脚本需要检测 typo 是否存在，可以使用此变量。
- **`TYPO_ACTIVE_SHELL` 环境变量** 由 fish 集成设置，帮助 typo 识别当前 shell。其他 shell 通过 `$SHELL` 检测。
- Shell 集成脚本现在设置了 owner 追踪变量（`TYPO_STDERR_CACHE_OWNER`、`TYPO_ORIG_STDERR_FD_OWNER`），用于安全处理嵌套 shell 会话。这些是内部变量，不应手动设置。

### 目录结构

`~/.typo/` 目录布局已扩展：

```text
~/.typo/
├── config.json          （0.2.0 新增）
├── rules.json           （用户规则，之前学习的纠正）
├── usage_history.json   （纠正历史）
└── subcommands.json     （缓存的子命令发现结果）
```

- 如果从 0.1.x 升级，`rules.json` 是可能已存在的唯一文件。其他文件会自动创建。
- `subcommands.json` 是缓存文件，删除是安全的 — 下次使用时会重新生成。
- 目录权限应为 `0755`，文件权限应为 `0600`。

### 子命令缓存格式

当前 v1 版本写入的 `subcommands.json` 使用 `schema_version: 3`。该格式保存树形子命令和长选项元数据，因此 Typo 可以修正 `aws cloudformation wait stack-create-complete`、`gcloud container clusters list` 这类多层命令路径，也可以复用缓存中的选项候选来支持实验性 `--long-option` 修正。

旧版本生成的缓存可能没有 `schema_version` 字段，也可能使用扁平列表格式，或使用上一版仅包含树形子命令的 v2 格式。Typo 首次加载时会把这些旧缓存隔离为 `subcommands.json.corrupt-<时间戳>`，并在下一次执行子命令发现时重新生成缓存。

不需要手动迁移。该缓存不保存用户规则、历史记录或配置。如果需要强制刷新，可以删除 `~/.typo/subcommands.json`，然后运行会触发子命令修正的命令。

### 规则作用域

v1 引入了比 0.2.0 更多的内置规则作用域：

| 作用域 | 起始版本 |
|---|---|
| `git`、`docker`、`npm` | 0.1.0 |
| `yarn`、`kubectl`、`cargo`、`brew` | 0.2.0 |
| `helm`、`terraform`、`python`、`pip`、`go`、`java`、`system` | 0.2.0 |

所有作用域默认启用。如果你之前禁用了某个作用域，设置会被保留。

### 安装方式

v1 支持以下安装方式（均可通过 `typo doctor` 验证）：

- `curl` 安装脚本（macOS / Linux）
- Windows PowerShell 快速安装脚本
- Homebrew
- 手动下载 GitHub Release 二进制文件

如果使用 Homebrew 安装，请通过 Homebrew 升级：

```bash
typo update
```

如果使用安装脚本，请使用 `typo update`。默认行为是本地构建 `main` 分支，因此需要 Go：

```bash
typo update
```

`typo update --version main` 和 `typo update --version latest` 是同一套 main 分支源码构建的
别名。不要使用 `--version @latest`；`@latest` 是 Go module 的版本语法。

如果要通过脚本托管路径安装指定 Release：

```bash
typo update --version 1.1.0
```

`typo update` 会拒绝 `go install`、手动 Release 和 Windows quick-install 安装的二进制。
这些安装方式请使用 `typo doctor` 输出的对应命令。

## 升级后检查清单

升级完成后：

1. 重启终端（或重新 source shell 配置文件）。
2. 运行 `typo doctor` 验证新版本已加载且所有检查通过。
3. 运行 `typo version` 确认版本号。
4. 运行 `typo config list` 验证配置已正确继承。
5. 运行 `typo rules list` 验证自定义规则完好。

## 回退方法

如果需要恢复到 0.x：

1. 恢复备份：

   ```bash
   rm -rf ~/.typo
   mv ~/.typo.backup ~/.typo
   ```

2. 安装指定的 0.x 版本：

   ```bash
   curl -fsSL https://raw.githubusercontent.com/yuluo-yx/typo/main/tools/scripts/install.sh | bash -s -- -s 0.2.0
   ```

3. 重启终端。

## 相关文档

- [问题排查](troubleshooting_CN.md)
- [命令参考](../reference/commands_CN.md)
- [工作原理](../reference/how-it-works_CN.md)
- [快速开始](../getting-started/quick-start_CN.md)
