# 稳定性与兼容性契约

[English](stability.md) | 简体中文

本文档定义了用户和打包者在 v1.x 版本系列中可以依赖的内容。
Typo 遵循[语义化版本](https://semver.org/lang/zh-CN/)。在 v1.x 系列中，
**稳定**接口不会发生不兼容的变更，除非进行主版本号升级。

## 稳定性级别

每个公共接口属于以下两个级别之一：

| 级别 | 含义 |
|------|------|
| **稳定** | 在 v1.x 期间不会以不兼容的方式变更。弃用将在未来主版本移除前至少提前一个次版本公告。 |
| **实验性** | 可能在任何次版本中变更或移除。实验性功能会在出现的地方明确标注。 |

## 配置目录（`~/.typo`）

**级别：稳定**

- Typo 将所有用户数据存储在 `~/.typo/` 下。
- 该目录及其内容在整个 v1.x 期间将保持在此路径。
- 文件以 `0600` 权限创建，目录以 `0755` 权限创建。
- `typo uninstall` 会删除整个 `~/.typo/` 目录，此行为将保持不变。

### 稳定文件

| 文件 | 用途 |
|------|------|
| `config.json` | 由 `typo config` 管理的运行时设置 |
| `rules.json` | 学习和用户定义的规则 |
| `usage_history.json` | 已接受的修正历史 |
| `subcommands.json` | 缓存的已发现子命令 |

次版本可能会向 `~/.typo/` 添加新文件，但现有文件不会被重命名、移动或改变用途。

### 迁移

如果次版本更改了配置文件的内部结构，Typo 将在首次加载时自动迁移文件并保留备份
（`<file>.backup-<timestamp>`）。在 v1.x 期间永远不需要手动迁移。

## 配置文件格式

**级别：稳定**

以下 `config.json` 键在 v1.x 期间不会被重命名或移除：

- `similarity-threshold`
- `max-edit-distance`
- `max-fix-passes`
- `keyboard`
- `history.enabled`
- `rules.<scope>.enabled`

次版本可能会添加新键。无法识别的键会被忽略，因此由较新 v1.x 版本写入的配置文件
仍然可以在较旧的 v1.x 二进制文件中加载（未知键会被静默跳过）。

### 支持的键盘布局

以下 `keyboard` 值是稳定的：

- `qwerty`
- `dvorak`
- `colemak`

次版本可能会添加更多布局。

### 内建规则范围

以下 `rules.<scope>.enabled` 的范围是稳定的：

`git`、`docker`、`npm`、`yarn`、`kubectl`、`cargo`、`brew`、`helm`、
`terraform`、`python`、`pip`、`go`、`java`、`system`

次版本可能会添加更多范围。现有范围不会被重命名或移除。

## CLI 标志和子命令

### 稳定的子命令

以下顶级子命令及其文档中记录的标志是稳定的：

| 命令 | 稳定的标志 |
|------|-----------|
| `typo fix <cmd>` | `-s <file>`、`--exit-code <n>`、`--no-history` |
| `typo learn <from> <to>` | *（无）* |
| `typo config list` | *（无）* |
| `typo config get <key>` | *（无）* |
| `typo config set <key> <value>` | *（无）* |
| `typo config reset` | *（无）* |
| `typo config gen` | `--force` |
| `typo rules list` | *（无）* |
| `typo rules add <from> <to>` | *（无）* |
| `typo rules remove <from>` | *（无）* |
| `typo rules enable <scope>` | *（无）* |
| `typo rules disable <scope>` | *（无）* |
| `typo history list` | *（无）* |
| `typo history clear` | *（无）* |
| `typo init <shell>` | *（无）* |
| `typo doctor` | *（无）* |
| `typo version` | *（无）* |
| `typo uninstall` | *（无）* |

稳定的子命令和标志在 v1.x 期间不会被移除或以不兼容的方式更改行为。次版本可能会
向现有命令添加新标志。

### 实验性功能

目前没有实验性子命令或标志。如果在未来的次版本中引入实验性功能，将在命令帮助输出
和本文档中明确标注。

## Shell 集成 API

**级别：稳定**

`typo init <shell>` 生成的 Shell 脚本可以依赖以下契约：

- **支持的 Shell**：`zsh`、`bash`、`fish`、`powershell`（包括 `pwsh` 别名）。
  这些将在整个 v1.x 期间保持支持。
- **触发绑定**：`Esc` `Esc` 是默认键绑定，将保持为默认值。用户可以按照故障排除
  指南中的说明重新绑定到其他键。
- **环境变量**：当 Shell 集成激活时会设置 `TYPO_SHELL_INTEGRATION=1`。脚本可以
  检查此变量以检测 Typo 集成是否已加载。
- **Stderr 缓存**：zsh、bash 和 PowerShell 集成会创建临时 `typo-stderr-*` 文件，
  用于将真实错误输出传递给 `typo fix -s`。此机制在整个 v1.x 期间将保持可用，
  但内部文件命名可能会更改。

### 可能变更的内容

- Shell 初始化脚本的内部实现（函数名、内部变量）不是公共 API 的一部分，可能在
  任何版本中更改。
- `typo doctor` 输出的确切格式是信息性的，可能在次版本中更改。不要以编程方式解析它。

## 退出码

**级别：稳定**

| 代码 | 含义 |
|------|------|
| `0` | 成功 |
| `1` | 一般错误 |

次版本可能会引入更多退出码以表示更具体的错误条件。现有代码的含义不会更改。

## 本契约不涵盖的内容

- **构建工具链版本**：最低 Go 版本可能在次版本中更新。
- **内部包 API**：`internal/` Go 包不是公共 API 的一部分。
- **CI 工作流和 Makefile 目标**：这些是开发工具，可能随时更改。

## 相关文档

- CLI 用法请参阅[命令参考](commands_CN.md)。
- 配置和本地文件请参阅[Typo 工作原理](how-it-works_CN.md)。
- 安装说明请参阅[快速开始](../getting-started/quick-start_CN.md)。
