<p align="center">
  <img src="docs/logo.svg" alt="Typo" width="280">
</p>

<p align="center">命令自动修正工具</p>

[![Build Status](https://github.com/yuluo-yx/typo/actions/workflows/build-and-test.yml/badge.svg)](https://github.com/yuluo-yx/typo/actions/workflows/build-and-test.yml) [![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://golang.org) [![Version](https://img.shields.io/github/v/tag/yuluo-yx/typo)](https://github.com/yuluo-yx/typo/releases) [![License](https://img.shields.io/github/license/yuluo-yx/typo)](LICENSE) [![Stars](https://img.shields.io/github/stars/yuluo-yx/typo)](https://github.com/yuluo-yx/typo)

**[English](README.md)** | 简体中文

按两次 `Esc` 键自动修正输错的命令。

<p align="center">
  <img src="docs/typo-demo.gif" alt="Typo Demo">
</p>

## TheFuck？

不是有 thefuck 了吗，为什么还要编写 typo？

有下面几个原因：

1. theFuck 不在维护了，issue，pr 没人处理（这也是最大的原因；
2. theFuck 和 Python 版本有绑定关系，我安装的时候废了点功夫～；
3. theFuck 对包含 `""` 的处理并不好。

基于上面的原因，我用 Go 写了 Typo，它不是 TheFuck 的翻译。而是从头开始的！

## Shell 集成

安装与平台接入说明请看 [快速开始](docs/getting-started/quick-start_CN.md)。

| Shell 终端 | 支持状态  |
|-----------|----------|
| zsh       | ✅ 已支持 |
| bash      | ✅ 已支持 |
| fish      | ✅ 已支持 |
| PowerShell| ✅ 已支持 |

## 运行

```bash
# 添加到 ~/.zshrc
eval "$(typo init zsh)"

# 或添加到 ~/.bashrc
eval "$(typo init bash)"

# 或添加到 ~/.config/fish/config.fish
typo init fish | source

# 或添加到 $PROFILE.CurrentUserCurrentHost
# 注意: Powershell 版本需要大于等于 7.x. 你可以通过 `$PSVersionTable.PSVersion` 检查版本.
Invoke-Expression (& typo init powershell | Out-String)
```

重启终端后，先运行 `typo doctor`，然后输错命令按 `Esc` `Esc` 即可修正。

## 文档导航

- [快速开始](docs/getting-started/quick-start_CN.md)
- [命令参考](docs/reference/commands_CN.md)
- [使用示例](docs/example/use_CN.md)
- [稳定性契约](docs/reference/stability_CN.md)
- [问题排查](docs/troubleshooting/troubleshooting_CN.md)
- [从 0.x 升级](docs/troubleshooting/upgrade-from-0x_CN.md)
- [工作原理](docs/reference/how-it-works_CN.md)

## Release 完整性

每个 GitHub Release 都会额外发布一个 `checksums.txt`，其中包含所有平台二进制的 SHA-256 摘要。
如果你是直接使用 Release 资产安装，请先对照该文件校验后再放到 `PATH` 中。
完整校验步骤请看 [快速开始](docs/getting-started/quick-start_CN.md)。

## 社区贡献者

感谢所有参与 Typo 构建的贡献者。

<p align="center">
  <img src=".github/CONTRIBUTORS.svg" alt="Typo 贡献者">
</p>

## License

MIT
