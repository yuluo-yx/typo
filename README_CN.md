<p align="center">
  <img src="docs/logo.svg" alt="Typo" width="280">
</p>

<p align="center"><strong>在终端里直接修正输错的命令。</strong></p>

[![Build Status](https://github.com/yuluo-yx/typo/actions/workflows/build-and-test.yml/badge.svg)](https://github.com/yuluo-yx/typo/actions/workflows/build-and-test.yml) [![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://golang.org) [![Version](https://img.shields.io/github/v/tag/yuluo-yx/typo)](https://github.com/yuluo-yx/typo/releases) [![License](https://img.shields.io/github/license/yuluo-yx/typo)](LICENSE) [![Stars](https://img.shields.io/github/stars/yuluo-yx/typo)](https://github.com/yuluo-yx/typo)

[English](README.md) | 简体中文

Typo 是用 Go 编写的命令自动修正工具。输入命令后按两次 `Esc`，Typo 会把当前命令行替换成更可能正确的命令。

<p align="center">
  <img src="docs/typo-demo.gif" alt="Typo Demo">
</p>

## TheFuck？

不是有 thefuck 了吗，为什么还要编写 typo？

有下面几个原因：

- theFuck 不在维护了，issue，pr 没人处理（这也是最大的原因；
- theFuck 和 Python 版本有绑定关系，我安装的时候废了点功夫～；
- theFuck 对包含 "" 的处理并不好。

基于上面的原因，我用 Go 写了 Typo，它不是 TheFuck 的翻译。而是从头开始的！


## 功能亮点

- 支持在 zsh、bash、fish 和 PowerShell 中原地修正命令。
- 支持主命令、子命令、`&&` 连接命令、管道命令和运行时报错。
- 将当前 shell 会话中的 alias 和简单包装函数作为修正上下文，例如 `k=kubectl`。
- 支持修正当前 shell 上下文里的环境变量名拼写错误，例如 `$HOEM` -> `$HOME`。
- 支持用 `typo learn` 添加个人修正规则；用户规则和历史记录保存在 `~/.typo`。
- 支持 macOS、Linux、WSL 和 Windows PowerShell 7+ 的原生二进制安装。
- 使用 Go 语言编写，二进制安装不依赖外部环境。

## 快速开始

使用 Homebrew 安装：

```bash
brew tap yuluo-yx/typo https://github.com/yuluo-yx/typo
brew install typo
```

或者在 macOS / Linux 上使用脚本安装：

```bash
curl -fsSL https://raw.githubusercontent.com/yuluo-yx/typo/main/tools/scripts/install.sh | bash
```

Windows PowerShell 7+ 使用下面的命令安装：

```powershell
iwr -useb https://raw.githubusercontent.com/yuluo-yx/typo/main/tools/scripts/quick-install.ps1 | iex
```

升级、校验 checksum 和平台差异说明请看 [快速开始](docs/getting-started/quick-start_CN.md)。

## Shell 集成

请先安装 Typo，再把对应的初始化命令添加到 shell 配置文件。

| Shell | 配置文件 | 初始化命令 |
|-------|----------|------------|
| zsh | `~/.zshrc` | `eval "$(typo init zsh)"` |
| bash | `~/.bashrc` | `eval "$(typo init bash)"` |
| fish | `~/.config/fish/config.fish` | `typo init fish \| source` |
| PowerShell 7+ | `$PROFILE.CurrentUserCurrentHost` | `Invoke-Expression (& typo init powershell \| Out-String)` |

重启终端后，运行下面的命令检查配置：

```bash
typo doctor
```

然后输入一条带拼写错误的命令，并按两次 `Esc`：

```shell
gti stauts

# 按两次 Esc 后
git status
```

Shell 集成也能识别当前会话中的 alias：

```shell
alias k=kubectl
k lgo

# 按两次 Esc 后
k logs
```

Shell 集成也能根据当前会话里的环境变量名修正 `$VAR` 拼写：

```shell
cd $HOEM/project

# 按两次 Esc 后
cd $HOME/project
```

## CLI 命令

需要直接输出结果或管理个人规则时，可以使用 CLI 命令：

```bash
typo fix "gut status && dcoker ps"
typo learn "gst" "git status"
typo config list
typo rules list
typo history list
```

完整命令和参数说明请看 [命令参考](docs/reference/commands_CN.md)。

## 文档导航

- [快速开始](docs/getting-started/quick-start_CN.md)
- [命令参考](docs/reference/commands_CN.md)
- [使用示例](docs/example/use_CN.md)
- [稳定性契约](docs/reference/stability_CN.md)
- [问题排查](docs/troubleshooting/troubleshooting_CN.md)
- [从 0.x 升级](docs/troubleshooting/upgrade-from-0x_CN.md)
- [工作原理](docs/reference/how-it-works_CN.md)

## 本地开发

开发环境需要 Go 1.25+ 和 GNU Make。

```bash
make setup
make test
make ci
```

提交变更前请确保 Git precommit 通过。代码风格、测试要求和提交信息格式请看 [贡献指南](CONTRIBUTING.md)。

## Release 完整性

当前 GitHub Release 流程会额外发布一个 `checksums.txt`，其中包含所有平台二进制的 SHA-256 摘要；部分历史 Release 可能缺少该文件。
如果你是直接使用 Release 资产安装，并且对应 Release 提供 `checksums.txt`，请先对照该文件校验后再放到 `PATH` 中。
完整校验步骤请看 [快速开始](docs/getting-started/quick-start_CN.md)。

## 社区贡献者

感谢所有参与 Typo 构建的贡献者。

<p align="center">
  <img src=".github/CONTRIBUTORS.svg" alt="Typo 贡献者">
</p>

## 许可证

MIT
