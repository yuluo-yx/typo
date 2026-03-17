# Typo - 命令快速修正工具

![Go Coverage](https://img.shields.io/badge/coverage-97.7%25-brightgreen) ![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)

一个类似 thefuck 的命令自动修正工具，按两次 ff 键自动修正输错的命令。

## 安装

```bash
go install github.com/shown/typo@latest
```

或者从源码编译：

```bash
git clone https://github.com/shown/typo.git
cd typo
make install
```

## Zsh 集成

将以下内容添加到 `~/.zshrc`：

```bash
eval "$(typo init zsh)"
```

重启终端后，按 `ESC ESC` 即可修正当前命令。

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

### typo version

打印版本信息。

```bash
$ typo version
typo dev (commit: 68572a5, built: unknown)
```

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

## 配置文件

配置文件存储在 `~/.typo/` 目录：

```
~/.typo/
├── history.json    # 修正历史
└── rules.json      # 用户自定义规则
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
