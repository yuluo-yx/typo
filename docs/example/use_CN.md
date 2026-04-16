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

## `git pull --set-upstream`

你是否遇到过这类 `git pull` 报错？

```shell
$ git pull
There is no tracking information for the current branch.
Please specify which branch you want to rebase against.
See git-pull(1) for details.

    git pull <remote> <branch>

If you wish to set tracking information for this branch you can do so with:

    git branch --set-upstream-to=origin/<branch> 0322-yuluo/inprove-add-check
```

这时按两次 `Esc`，Typo 会自动补全建议的 upstream 设置。

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

太痛苦了！！！

```shell
gti stash save
```
