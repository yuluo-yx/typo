# 快速开始

[English](quick-start.md) | 简体中文

## 安装

### Homebrew

Homebrew 会安装 macOS 或 Linux 对应的预编译 Release 二进制。
由于 tap 维护在当前仓库内，请先使用显式仓库 URL 添加 tap。

```bash
brew tap yuluo-yx/typo https://github.com/yuluo-yx/typo
brew install typo
```

升级已有的 Homebrew 安装：

```bash
brew update
brew upgrade typo
```

卸载 typo 并移除 tap：

```bash
brew uninstall typo
brew untap yuluo-yx/typo
```

安装完成后，请继续参考 README 中的 `Shell 集成` 部分配置 shell 集成。

### macOS / Linux 脚本安装

```bash
curl -fsSL https://raw.githubusercontent.com/yuluo-yx/typo/main/tools/scripts/install.sh | bash
```

脚本默认下载预编译的 Release 二进制。只有在从 `main` 分支源码构建时才需要 `Go`。

可选参数：

```bash
# 显式安装最新 Release
curl -fsSL https://raw.githubusercontent.com/yuluo-yx/typo/main/tools/scripts/install.sh | bash -s -- -s latest

# 安装指定 Release 版本（语义化版本号）
curl -fsSL https://raw.githubusercontent.com/yuluo-yx/typo/main/tools/scripts/install.sh | bash -s -- -s 0.2.0

# 从 main 分支源码构建（需要 Go）
curl -fsSL https://raw.githubusercontent.com/yuluo-yx/typo/main/tools/scripts/install.sh | bash -s -- -b
```

补充说明：

- 安装脚本当前支持 macOS 和 Linux。
- WSL 请在 Linux 环境内走同一套安装流程。
- 当对应 Release 提供 `checksums.txt` 时，脚本会自动校验下载的二进制。如果历史 Release 缺少 `checksums.txt`，脚本会输出警告并在未执行 checksum 校验的情况下继续安装。

### Windows PowerShell 7+ 快速安装

```powershell
iwr -useb https://raw.githubusercontent.com/yuluo-yx/typo/main/tools/scripts/quick-install.ps1 | iex
```

Windows 快速安装脚本会下载最新 Release 的二进制，在该 Release 提供 `checksums.txt` 时做校验，将 `typo.exe` 安装到 `%LOCALAPPDATA%\Programs\typo\bin`，然后输出后续 PowerShell 集成步骤。
如果历史 Release 缺少 `checksums.txt`，脚本会输出警告并在未执行 checksum 校验的情况下继续安装。二进制下载失败仍会终止安装。

## 平台支持

Typo 当前支持：

- macOS
- Linux
- WSL
- 搭配 `PSReadLine` 的原生 Windows PowerShell 7+

### 原生 Windows

推荐步骤：

1. 使用上面的 quick-install 命令安装。
2. 运行 `Invoke-Expression (& typo init powershell | Out-String)`。
3. 运行 `typo doctor`。

`typo doctor` 的预期表现：

- 显示 `shell: powershell`
- shell integration 提示中指向 `$PROFILE.CurrentUserCurrentHost`

当前 Windows 限制：

- PowerShell 集成依赖 PowerShell 7+ 和 `PSReadLine`。
- 当前 PowerShell 集成对 native command 的 `stderr` 辅助纠错最稳定。
- cmdlet 的 error stream 捕获仍可能受 PowerShell host 影响。

### WSL

推荐步骤：

1. 在 WSL 内使用 Linux 安装脚本安装 typo。
2. 在 WSL 内运行 `eval "$(typo init zsh)"` 或 `eval "$(typo init bash)"`。
3. 在 WSL 内运行 `typo doctor`。

`typo doctor` 的预期表现：

- 显示当前 Linux shell，通常是 `bash` 或 `zsh`
- 给出对应的 `~/.bashrc` 或 `~/.zshrc` shell integration 提示

## 校验 Release 二进制

如果你是从 GitHub Release 页面手动下载二进制，请同时下载同一版本里的 `checksums.txt`，并在安装前先校验文件完整性。
部分历史 Release 可能缺少 `checksums.txt`；如果你需要自动或手动 checksum 校验，请优先使用较新的 Release。
请在“二进制文件和 `checksums.txt` 位于同一目录”时执行下面的命令。

Linux 示例：

```bash
grep ' typo-linux-amd64$' checksums.txt > typo-linux-amd64.checksums
sha256sum -c typo-linux-amd64.checksums
```

macOS 示例：

```bash
grep ' typo-darwin-arm64$' checksums.txt > typo-darwin-arm64.checksums
shasum -a 256 -c typo-darwin-arm64.checksums
```

请将命令里的文件名替换成你实际下载的产物名。校验成功时会输出 `OK`。

## 下一步

- shell 接入方式请看 [README](../../README_CN.md) 中的 `Shell 集成`。
- 命令说明请看 [命令参考](../reference/commands_CN.md)。
- 典型修正场景请看 [使用示例](../example/use_CN.md)。
- v1.x 稳定性承诺请看 [稳定性契约](../reference/stability_CN.md)。
- 环境问题排查请看 [问题排查](../troubleshooting/troubleshooting_CN.md)。
