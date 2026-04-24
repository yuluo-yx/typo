# 命令参考

[English](commands.md) | 简体中文

## `typo fix`

修正一条命令，并把修正结果输出到 stdout。

```bash
typo fix "gut stauts"
typo fix "gut status && dcoker ps"
typo fix "gut status | grep main"
typo fix "typ doctro"
typo fix "typo hsitory lsit"
```

常用参数：

- `-s <file>`：从 shell 集成保存的 `stderr` 文件中读取真实报错。
- `--exit-code <n>`：把上一条命令的退出码作为额外修正上下文。
- `--no-history`：本次修正不写入历史记录。
- `--alias-context <file>`：读取 shell 集成采集的修正上下文。
- `--debug`：把本次修正链路的调试信息输出到 stderr，不影响 stdout 上的修正命令。

`--alias-context` 主要由 `typo init <shell>` 生成的脚本内部使用。该上下文是
临时的、仅属于当前 shell 会话；Typo 会先展开 `k=kubectl` 这类别名，再把 `$VAR`
token 与当前会话里的环境变量名（例如 `$HOME`）进行匹配，最后在安全时把结果输出回
原始别名形态。

重复出现且被接受的修正会在达到阈值后静默提升为用户规则。可通过 `typo config set auto-learn-threshold 0` 关闭该行为。

`--debug` 输出当前会话内的可观测信息，包括：

- 命中的修正阶段和每一轮 pass 的前后命令。
- 是否使用了 alias 上下文、parser、history、subcommand 等链路。
- 是否按需加载了 PATH 命令集。
- 被拒绝的高分候选（如果有）。
- auto-learn 是否触发、是否超时，以及原因。
- 总耗时、引擎修正耗时，以及 auto-learn 等待耗时。
- 总耗时、引擎修正耗时，以及 auto-learn 等待耗时。

## `typo learn`

教会 Typo 一条个人习惯修正规则。

```bash
typo learn "gst" "git status"
```

日常添加规则优先用 `learn`。`typo learn` 和 `typo rules add` 都会写入同一类用户规则，持久化到 `~/.typo/rules.json`，并清理冲突历史；`learn` 更偏“教会 typo 一个习惯”。

当最短路径匹配难以推断特别离谱的拼写错误时，`learn` 也可以作为最后一道兜底，例如教会 `gitsss` -> `git`。对于 shell alias，优先使用 shell 集成：zsh、bash、fish 和 PowerShell 会自动传递当前会话的别名上下文。只有当你希望脱离当前 shell 会话持久化一条手动规则时，才需要用 `learn` 记录 alias。

## `typo config`

管理持久化运行配置，配置文件位于 `~/.typo/config.json`。

```bash
typo config list
typo config get keyboard
typo config set keyboard dvorak
typo config reset
typo config gen
typo config gen --force
```

当前可配置的 key 包括：

- `similarity-threshold`
- `max-edit-distance`
- `max-fix-passes`
- `auto-learn-threshold`
- `keyboard`
- `history.enabled`
- `experimental.long-option-correction.enabled` *（实验性；默认：`false`）*
- `rules.<scope>.enabled`

## `typo rules`

管理用户规则和内置规则作用域。

```bash
typo rules list
typo rules add "gst" "git status"
typo rules remove "gst"
typo rules disable git
typo rules enable docker
```

持久化行为说明：

- `typo rules add` 和 `typo rules remove` 会更新 `~/.typo/rules.json` 中的用户规则。
- `typo rules enable` 和 `typo rules disable` 会通过 `rules.<scope>.enabled` 更新 `~/.typo/config.json` 中的内置规则开关。

当前可通过 `rules.<scope>.enabled` 控制的内置作用域：

- `git`、`docker`、`npm`、`yarn`、`kubectl`、`cargo`、`brew`、`helm`
- `terraform`、`python`、`pip`、`go`、`java`、`system`

## `typo history`

查看或清空已接受的修正历史。

```bash
typo history list
typo history clear
```

## `typo stats`

以轻量摘要方式分析已接受的修正历史。

```bash
typo stats
typo stats --since 7
typo stats --top 5
```

常用参数：

- `--since <days>`：只统计“最后一次被接受的时间戳”落在最近 N 天内的历史 pair；展示的 `count` 仍然是该 pair 当前保存的累计次数。
- `--top <n>`：限制摘要里展示的 typo pair 数量。

## `typo init`

打印指定 shell 的集成脚本。

```bash
typo init zsh
typo init bash
typo init fish
typo init powershell
```

支持的 shell 名称：

- `zsh`
- `bash`
- `fish`
- `powershell`
- `pwsh` 可作为别名使用，内部会归一化为 `powershell`

## `typo doctor`

检查当前环境、生效配置和 shell 集成提示。

```bash
typo doctor
```

输出主要包括：

- shell 检测结果
- 二进制发现状态
- 配置目录状态
- shell integration 提示
- Homebrew、curl 安装脚本、手动 Release 二进制、Windows quick install 与 `go install` 的安装方式检测
- 常见 shell 配置错误提示，例如 fish 使用了错误的 init 命令风格
- 通过 `go install` 安装时的 Go bin `PATH` 提示

## `typo version`

输出当前版本、commit 和构建时间（如果可用）。

```bash
typo version
```

## `typo uninstall`

删除本地 Typo 配置，并输出剩余的手动清理步骤。

```bash
typo uninstall
```

## 相关文档

- 真实修正场景请看 [使用示例](../example/use_CN.md)。
- 修正策略、配置文件和构建命令请看 [工作原理](how-it-works_CN.md)。
- v1.x 稳定性承诺请看 [稳定性契约](stability_CN.md)。
- 安装和平台接入请看 [快速开始](../getting-started/quick-start_CN.md)。
