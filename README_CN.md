<p align="center">
  <img src="docs/logo.svg" alt="Typo" width="280">
</p>

<p align="center">命令自动修正工具</p>

[![Build Status](https://github.com/yuluo-yx/typo/actions/workflows/ci.yml/badge.svg)](https://github.com/yuluo-yx/typo/actions/workflows/ci.yml) [![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://golang.org) [![Version](https://img.shields.io/github/v/tag/yuluo-yx/typo)](https://github.com/yuluo-yx/typo/releases) [![License](https://img.shields.io/github/license/yuluo-yx/typo)](LICENSE) [![Stars](https://img.shields.io/github/stars/yuluo-yx/typo)](https://github.com/yuluo-yx/typo)

**[English](README.md)** | 简体中文

按两次 `Esc` 键自动修正输错的命令。

<p align="center">
  <img src="docs/typo-demo.gif" alt="Typo Demo">
</p>

## 快速开始

### 通过 Homebrew 安装

Coming soon.

### 或通过脚本安装

```bash
curl -fsSL https://raw.githubusercontent.com/yuluo-yx/typo/main/install.sh | bash
```

脚本默认下载预编译的 Release 二进制。只有在从 `main` 分支源码构建时才需要 `Go`。

可选参数：

```bash
# 显式安装最新 Release
curl -fsSL https://raw.githubusercontent.com/yuluo-yx/typo/main/install.sh | bash -s -- -s latest

# 安装指定 Release 版本
curl -fsSL https://raw.githubusercontent.com/yuluo-yx/typo/main/install.sh | bash -s -- -s 26.03.24

# 从 main 分支源码构建（需要 Go）
curl -fsSL https://raw.githubusercontent.com/yuluo-yx/typo/main/install.sh | bash -s -- -b
```

说明：安装脚本当前支持 macOS 和 Linux。

### 运行

```bash
# 添加到 ~/.zshrc
eval "$(typo init zsh)"
```

重启终端后，输错命令按 `Esc` `Esc` 即可修正。（不用回车也可！）

## 命令

### `typo fix` - 修正命令

```bash
typo fix "gut stauts"                 # → git status
typo fix "gut status && dcoker ps"    # → git status && docker ps
typo fix "gut status | grep main"     # → git status | grep main
```

### `typo learn` - 学习修正规则

```bash
typo learn "gst" "git status"         # 更适合日常把常见输入习惯教给 typo
```

日常使用优先选 `learn`。`typo learn` 和 `typo rules add` 都会写入同一类用户规则，并清理冲突的历史记录；区别主要在命令意图上：`learn` 更像“教会 typo 一个习惯”，`rules add` 更像“显式管理规则表”，适合和 `rules list`、`rules remove` 搭配使用。

### `typo rules` - 管理规则

```bash
typo rules list                    # 列出所有规则
typo rules add "gst" "git status"  # 效果与 `learn` 相同，更偏规则管理操作
typo rules remove "gst"            # 删除规则
```

### `typo history` - 查看修正历史

```bash
typo history list      # 显示历史修正
typo history clear     # 清除历史
```

### `typo doctor` - 诊断问题

```bash
typo doctor            # 检查配置状态
```

### 其他命令

```bash
typo init zsh          # 打印 shell 集成脚本
typo version           # 显示版本
typo uninstall         # 清理本地配置并提示剩余手动清理步骤
```

## 工作原理

Typo 按以下优先级修正命令：

1. **错误解析** - 在有 stderr 时优先提取命令自身给出的建议
2. **用户规则** - 优先应用 learn 结果和用户自定义规则
3. **历史记录** - 复用之前接受过的真实修正
4. **内置规则** - 应用程序内置的常见 typo 映射
5. **子命令修正** - 在继续回退前先尝试已知工具的子命令
6. **编辑距离** - 基于键盘布局的模糊匹配，相邻按键替换成本更低

### 支持的错误解析

- **git**: `did you mean...`、无 upstream 分支等
- **docker**: 未知命令建议
- **npm**: 命令未找到建议

`-s <file>` 表示让 `typo fix` 从文件里读取 stderr。它主要用于这类基于真实报错提示的修正场景，平时通常由 zsh 集成在命令失败后自动传入。

示例：

```bash
typo fix -s git.stderr "git remove -v"      # → git remote -v
typo fix -s git.stderr "git pull"           # → git pull --set-upstream origin main
typo fix -s docker.stderr "docker psa"      # → docker ps
typo fix -s npm.stderr "npm isntall react"  # → npm install react
```

### 子命令智能修正

自动解析工具子命令，智能修正：

```bash
typo fix "git stattus"   # → git status
typo fix "docker biuld"  # → docker build
```

支持：git, docker, npm, yarn, kubectl, cargo, go, pip, brew, terraform, helm

## 配置

文件存储在 `~/.typo/` 目录：

```
~/.typo/
├── rules.json                   # learn 结果与用户自定义规则
├── usage_history.json           # 已接受或直接执行修正的历史记录
└── subcommands.json             # 子命令缓存
```

## 编译

```bash
make build      # 编译当前平台
make build-all  # 编译所有平台
make install    # 安装到 Go BIN
make test       # 运行测试
make coverage   # 运行覆盖率测试
make lint       # 运行检查
```

## License

MIT
