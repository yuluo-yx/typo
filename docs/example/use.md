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

## `git push` rejected, pull first

When the remote has commits that are missing locally, Git rejects the push:

```shell
$ git push origin main
 ! [rejected]        main -> main (fetch first)
error: failed to push some refs to 'github.com:yuluo-yx/typo.git'
hint: Updates were rejected because the remote contains work that you do
hint: not have locally. You may want to first integrate the remote changes
hint: (e.g., 'git pull ...') before pushing again.
```

Press `Esc` `Esc`, and Typo can retry the matching pull command first.

```shell
git pull origin main
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
