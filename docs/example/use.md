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

## `git push --set-upstream`

On the first push of a local branch, Git provides the exact command needed to set its upstream:

```shell
$ git push
fatal: The current branch feature/topic has no upstream branch.
To push the current branch and set the remote as upstream, use

    git push --set-upstream origin feature/topic
```

Press `Esc` `Esc`. Typo applies Git's concrete suggestion when the remote and branch names are safe across every supported shell; otherwise it leaves the command unchanged.

## `git pull --rebase`

When Git refuses to pull because the local and remote branches diverged:

```shell
$ git pull
hint: You have divergent branches and need to specify how to reconcile them.
hint: You can pass --rebase, --no-rebase, or --ff-only on the command line.
fatal: Need to specify how to reconcile divergent branches.
```

Press `Esc` `Esc`, and Typo can retry the same pull with a command-level rebase strategy.

```shell
git pull --rebase
```

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
