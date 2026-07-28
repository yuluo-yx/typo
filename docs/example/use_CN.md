# Typo 可以修复的场景示例

[English](use.md) | 简体中文

> 提示：
> - Typo 可以在你按下回车前直接修正命令，不需要先执行失败再回头修。
> - 如果修正不准确，可以用 `typo learn` 添加自己的规则，或者提交 Issue / PR。
> - Typo 同时支持主命令和子命令修正，例如 `gti statuus`。

## 普通命令

> 包含 git、docker、brew、apt 等常见 Linux 与 macOS 命令。

```shell
gti <Esc, Esc>

git

brewd <Esc, Esc>

brew
```

## 子命令

> 覆盖 `git status`、`git commit`、`docker images` 等子命令场景。

```shell
gti stauts <Esc, Esc>

git status

docker imagess <Esc, Esc>

docker images
```

## `&&` 连接的命令

支持同时修正 `&&` 左右两侧的命令。

```shell
echo ok && gti status <Esc, Esc>

echo ok && git status

ehco ok && gti status <Esc, Esc>

echo ok && git status
```

## Shell 内建命令

例如 `source`、`echo`、`time` 等。

```shell
sourec ~/.zshrc <Esc, Esc>

source ~/.zshrc
```

## 管道连接命令

```shell
$ cat ~/.zshrc | grpe "zsh"
zsh: command not found: grpe <Enter, Esc, Esc>

cat ~/.zshrc | grep "zsh"
```

## `git push --set-upstream`

首次推送本地分支时，Git 会给出设置 upstream 的准确命令：

```shell
$ git push
fatal: The current branch feature/topic has no upstream branch.
To push the current branch and set the remote as upstream, use

    git push --set-upstream origin feature/topic
```

按两次 `Esc`，当远端名和分支名可在所有支持的 Shell 中安全表示时，Typo 会采用 Git 给出的具体建议；否则保持原命令不变。

## `git pull --set-upstream`

当前分支没有上游分支（upstream）时，Git 可能在建议中保留远端分支占位符：

```shell
$ git pull
There is no tracking information for the current branch.

    git branch --set-upstream-to=origin/<branch> test/dev
```

按两次 `Esc`。Typo 会根据明确的本地分支解析占位符，并确认当前仓库同时存在 `test/dev` 和 `origin/test/dev`，然后给出以下命令：

```shell
git pull --set-upstream origin test/dev
```

如果远端名也是占位符、分支名存在歧义或任一引用不存在，Typo 会保持原命令不变。

## `git pull --rebase`

当本地分支和远端分支已经分叉，Git 会拒绝继续 pull：

```shell
$ git pull
hint: You have divergent branches and need to specify how to reconcile them.
hint: You can pass --rebase, --no-rebase, or --ff-only on the command line.
fatal: Need to specify how to reconcile divergent branches.
```

这时按两次 `Esc`，Typo 会用命令级 rebase 策略重试同一次 pull。

```shell
git pull --rebase
```

## 没有权限？自动补 `sudo`

> 命令本身没问题，只是执行时缺少权限。

```shell
$ mkdir test <Enter, Esc, Esc>
mkdir: test: Permission denied

# fix it.
$ sudo mkdir test
```

## 多级子命令修复

像下面这样：

```shell
gti stash scave <Esc, Esc>
```

太痛苦了！！！Typo 可以一次性修正这条命令路径：

```shell
gti stash save
```
