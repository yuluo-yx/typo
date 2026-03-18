# Typo - 命令快速修正工具

![Go Coverage](https://img.shields.io/badge/coverage-97.7%25-brightgreen) ![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)

一个类似 thefuck 的命令自动修正工具，按两次 Esc 键自动修正输错的命令。

## 安装

### 从 Release 下载

从 [Releases](https://github.com/yuluo-yx/typo/releases) 页面下载对应平台的二进制文件：

| 平台 | 文件 |
|------|------|
| Linux AMD64 | `typo-linux-amd64` |
| Linux ARM64 | `typo-linux-arm64` |
| macOS AMD64 | `typo-darwin-amd64` |
| macOS ARM64 | `typo-darwin-arm64` |
| Windows AMD64 | `typo-windows-amd64.exe` |

下载后添加执行权限并移动到 PATH：

```bash
chmod +x typo-linux-amd64
sudo mv typo-linux-amd64 /usr/local/bin/typo
```

### 从源码安装

```bash
go install github.com/yuluo-yx/typo/cmd/typo@latest
```

或者从源码编译：

```bash
git clone https://github.com/yuluo-yx/typo.git
cd typo
make install
```

## Zsh 集成

将以下内容添加到 `~/.zshrc`：

```bash
eval "$(typo init zsh)"
```

重启终端后，按 `Esc` `Esc` 即可修正当前命令。

## 子命令

### typo fix

修正命令。

```bash
typo fix <command>              # 修正命令
typo fix -s <file> <command>    # 使用 stderr 文件进行错误解析修正
```

示例：
```bash
$ typo fix "gut stauts"
git status

$ typo fix "dcoker ps"
docker ps
```

### typo learn

手动学习一条修正规则，保存到历史记录中。

```bash
typo learn <from> <to>
```

示例：
```bash
$ typo learn "gut" "git"
Learned: gut -> git
```

### typo rules

管理修正规则。

```bash
typo rules list              # 列出所有规则
typo rules add <from> <to>   # 添加用户规则
typo rules remove <from>     # 删除用户规则
```

示例：
```bash
$ typo rules add "mytypo" "mycommand"
Added rule: mytypo -> mycommand

$ typo rules list
gut -> git [global] (enabled)
dcoker -> docker [global] (enabled)
mytypo -> mycommand [user] (enabled)
```

### typo history

管理修正历史。

```bash
typo history list    # 列出修正历史
typo history clear   # 清除修正历史
```

示例：
```bash
$ typo history list
gut -> git (used 5 times)
dcoker -> docker (used 3 times)

$ typo history clear
History cleared
```

### typo init

打印 shell 集成脚本。

```bash
typo init zsh    # 打印 zsh 集成脚本
```

### typo doctor

检查配置状态，诊断常见问题。

```bash
$ typo doctor
Checking typo configuration...

[1/4] typo command: ✓ available (/usr/local/bin/typo)
[2/4] config directory: ✓ /home/user/.typo
[3/4] shell integration: ✓ loaded
[4/4] Go bin PATH: ✓ configured

All checks passed!
```

如果发现问题，doctor 会给出具体的修复建议：

- **typo command 未找到**：检查是否已安装，或 Go bin 目录是否在 PATH 中
- **shell integration 未加载**：在 `~/.zshrc` 中添加 `eval "$(typo init zsh)"`
- **Go bin PATH 未配置**：如果你使用 `go install` 安装，需要添加 `export PATH="$PATH:$(go env GOPATH)/bin"`

### typo version

打印版本信息。

```bash
$ typo version
typo dev (commit: 68572a5, built: unknown)
```

### typo uninstall

彻底卸载 typo，包括配置目录和清理指引。

```bash
$ typo uninstall
Uninstalling typo...

[1/3] Removing config directory: ✓ removed /home/user/.typo
[2/3] Zsh integration: please remove the following line from ~/.zshrc:

    eval "$(typo init zsh)"

[3/3] Binary: please remove the binary manually:

    rm /usr/local/bin/typo

Uninstallation complete.
```

**注意**：出于安全考虑，程序不会自动修改 `~/.zshrc` 或删除二进制文件，请根据提示手动完成。

## 修正策略

Typo 按以下优先级尝试修正命令：

1. **错误解析** - 解析 stderr 中的 "did you mean" 等建议
2. **历史记录** - 使用之前学习过的修正
3. **规则匹配** - 内置和用户自定义规则
4. **编辑距离** - 基于键盘布局的模糊匹配

## 支持的错误解析

- **git**: `did you mean...`、无 upstream 分支等
- **docker**: 未知命令建议
- **npm**: 命令未找到建议

## 子命令智能修正

Typo 会自动解析工具的子命令，用于智能修正：

```bash
$ typo fix "git stattus"
git status
typo: did you mean: status?

$ typo fix "docker biuld"
docker build
typo: did you mean: build?
```

**工作原理**：
1. 当你首次修正某个工具的命令时（如 `typo fix "git stattus"`），typo 会自动运行 `git help -a` 解析子命令
2. 解析结果缓存到 `~/.typo/subcommands.json`，有效期 7 天
3. 修正命令时，会同时检查子命令是否正确并给出建议

**支持的工具**：git, docker, npm, yarn, kubectl, cargo, go, pip, brew, terraform, helm 等

## 配置文件

配置文件存储在 `~/.typo/` 目录：

```
~/.typo/
├── history.json       # 修正历史
├── rules.json         # 用户自定义规则
└── subcommands.json   # 子命令缓存
```

## 编译

```bash
make build              # 编译当前平台
make build-all          # 编译所有平台
make test               # 运行测试
make lint               # 运行代码检查
make ci                 # 运行 CI 检查 (fmt, lint, test)
```

## License

MIT
