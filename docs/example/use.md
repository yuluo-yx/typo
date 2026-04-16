# Typo Usage Examples

English | [简体中文](use_CN.md)

> Tips:
> - Typo can fix commands before you press Enter. You do not need to run the command first and correct it afterward.
> - If a correction is inaccurate, use `typo learn` to add your own rule, or open an Issue / PR.
> - Typo supports both top-level command fixes and subcommand fixes, such as `gti statuus`.

## Common commands

> Includes common Linux and macOS commands such as git, docker, brew, apt, and more.

```shell
gti <Esc, Esc>

git

brewd <Esc, Esc>

brew
```

## Subcommands

> Covers subcommands such as `git status`, `git commit`, `docker images`, and more.

```shell
gti stauts <Esc, Esc>

git status

docker imagess <Esc, Esc>

docker images
```

## Commands joined with `&&`

Typo can fix commands on both sides of `&&` in the same line.

```shell
echo ok && gti status <Esc, Esc>

echo ok && git status

ehco ok && gti status <Esc, Esc>

echo ok && git status
```

## Shell built-in commands

For example, `source`, `echo`, `time`, and similar shell commands.

```shell
sourec ~/.zshrc <Esc, Esc>

source ~/.zshrc
```

## Commands connected by pipes

```shell
$ cat ~/.zshrc | grpe "zsh"
zsh: command not found: grpe <Enter, Esc, Esc>

cat ~/.zshrc | grep "zsh"
```

## `git pull --set-upstream`

Have you ever run into this kind of issue when using `git pull`?

```shell
$ git pull
There is no tracking information for the current branch.
Please specify which branch you want to rebase against.
See git-pull(1) for details.

    git pull <remote> <branch>

If you wish to set tracking information for this branch you can do so with:

    git branch --set-upstream-to=origin/<branch> 0322-yuluo/inprove-add-check
```

Press `Esc` `Esc`, and Typo can add the suggested upstream automatically.

## No permission? Use `sudo`

> You finish typing a command and then realize you do not have permission.

```shell
$ mkdir test <Enter, Esc, Esc>
mkdir: test: Permission denied

# fix it.
$ sudo mkdir test
```

## Multi-level subcommand fixes

For example:

```shell
gti stash scave <Esc, Esc>
```

That is painful!!!Typo can fix the command path in one step:

```shell
gti stash save
```
