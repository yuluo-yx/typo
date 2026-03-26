<p align="center">
  <img src="docs/logo.svg" alt="Typo" width="280">
</p>

<p align="center">命令自动修正工具</p>

[![Build Status](https://github.com/yuluo-yx/typo/actions/workflows/ci.yml/badge.svg)](https://github.com/yuluo-yx/typo/actions/workflows/ci.yml) [![codecov](https://codecov.io/gh/yuluo-yx/typo/branch/main/graph/badge.svg)](https://codecov.io/gh/yuluo-yx/typo) [![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://golang.org) [![Version](https://img.shields.io/github/v/tag/yuluo-yx/typo)](https://github.com/yuluo-yx/typo/releases) [![License](https://img.shields.io/github/license/yuluo-yx/typo)](LICENSE) [![Stars](https://img.shields.io/github/stars/yuluo-yx/typo)](https://github.com/yuluo-yx/typo)

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
typo fix "gut stauts"     # → git status
typo fix "dcoker ps"      # → docker ps
```

### `typo learn` - 学习修正规则

```bash
typo learn "gut" "git"    # 保存规则供后续使用
```

### `typo rules` - 管理规则

```bash
typo rules list                    # 列出所有规则
typo rules add "gut" "git"         # 添加自定义规则
typo rules remove "gut"            # 删除规则
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
typo uninstall         # 卸载 typo
```

## 工作原理

Typo 按以下优先级修正命令：

1. **错误解析** - 从 stderr 提取 "did you mean" 建议
2. **历史记录** - 复用之前接受过的真实修正
3. **规则匹配** - 内置、learn 生成与用户自定义规则
4. **编辑距离** - 基于键盘布局的模糊匹配

### 支持的错误解析

- **git**: `did you mean...`、无 upstream 分支等
- **docker**: 未知命令建议
- **npm**: 命令未找到建议

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
├── rules.json                   # learn 结果与用户规则
├── usage_history.json           # 真实修正历史
└── subcommands.json             # 子命令缓存
```

## 编译

```bash
make build      # 编译当前平台
make build-all  # 编译所有平台
make test       # 运行测试
make lint       # 运行检查
```

## License

MIT
