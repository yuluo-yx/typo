# Typo 可以修复的场景示例

> Tips：
> - Typo 在修复时，不用回车在执行之后在修正，在打完之后就能修正了。
> - Typo 如果出现修正不准确的情况？type learn 一下，将修正规则添加到 rules 集合里面，或者 Issue & PR。
> - Typo 同时支持 `gti statuus` 修正，支持主命令同时也支持字命令。

## 普通命令

> 包含 git，docker，brew，apt 等在内的常用 linux & mac 命令

```shell
gti  <e, e>

gti

brewd <e, e>

brew
```

## 子命令

> git status，git commit，docker images 等在内的 subcommands

```shell
gti stauts <e, e>

git status

docker imagess <e, e>

docker images
```

## && 连字命令

支持同时修正 && 左右两侧的命令。

```shell
echo ok && gti status <e, e>

echo ok && git status

ehco ok && gti status <e, e>

echo ok && git status
```

## 支持 Shell 自建命令

例如 source, echo time 等

```shell
sourec ~/.zshrc <e, e>

source ~/.zshrc
```

## 支持管道连接命令

```shell
$ cat ~/.zshrc | grpe "zsh"     
zsh: command not found: grpe <enter, e, e>

cat ~/.zshrc | grep "zsh"
```

## 支持 git pull --set-upstream

你是否遇到过下面这类问题，当 git pull 的时候，突然出现：

```shell
$ git pull
There is no tracking information for the current branch.
Please specify which branch you want to rebase against.
See git-pull(1) for details.

    git pull <remote> <branch>

If you wish to set tracking information for this branch you can do so with:

    git branch --set-upstream-to=origin/<branch> 0322-yuluo/inprove-add-check
```

不要着急，两次 esc 自动 fix。

## sudo 没有权限？

> 打完一个命令之后，发现没有权限！！！

```shell
$ mkcd test    <enter, e, e>
mkdir: test: Permission denied 

# fix it.
$ sudo mkdir test
```
