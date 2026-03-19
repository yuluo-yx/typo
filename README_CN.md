# Typo - 命令自动修正工具

[![Build Status](https://github.com/yuluo-yx/typo/actions/workflows/ci.yml/badge.svg)](https://github.com/yuluo-yx/typo/actions/workflows/ci.yml) [![codecov](https://codecov.io/gh/yuluo-yx/typo/branch/main/graph/badge.svg)](https://codecov.io/gh/yuluo-yx/typo) [![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://golang.org)

**[English](README.md)** | 简体中文

按两次 `Esc` 键自动修正输错的命令。

![Typo Demo](typo-demo.gif)

## 快速开始

```bash
# 安装
go install github.com/yuluo-yx/typo/cmd/typo@latest

# 添加到 ~/.zshrc
eval "$(typo init zsh)"
```

重启终端后，输错命令按 `Esc` `Esc` 即可修正。

## 安装

<details>
<summary>从 Release 下载</summary>

从 [Releases](https://github.com/yuluo-yx/typo/releases) 页面下载对应平台的二进制文件：

| 平台 | 文件 |
|------|------|
| Linux AMD64 | `typo-linux-amd64` |
| Linux ARM64 | `typo-linux-arm64` |
| macOS AMD64 | `typo-darwin-amd64` |
| macOS ARM64 | `typo-darwin-arm64` |
| Windows AMD64 | `typo-windows-amd64.exe` |

```bash
# 以 macOS ARM64 为例
curl -LO https://github.com/yuluo-yx/typo/releases/latest/download/typo-darwin-arm64
chmod +x typo-darwin-arm64
sudo mv typo-darwin-arm64 /usr/local/bin/typo
```

</details>

<details>
<summary>从源码编译</summary>

```bash
git clone https://github.com/yuluo-yx/typo.git
cd typo
make install
```

</details>

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
2. **历史记录** - 使用之前学习过的修正
3. **规则匹配** - 内置和用户自定义规则
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
├── history.json       # 修正历史
├── rules.json         # 用户规则
└── subcommands.json   # 子命令缓存
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