# 问题排查

[English](troubleshooting.md) | 简体中文

## 1. 在 IDEA 终端中按 `Esc` 会退出终端？

按照下图取消 `Esc` 快捷键绑定即可。

![IDEA Config](imgs/idea.png)

## 2. 使用 Ghostty 通过 SSH 连接时，为什么 typo 运行失败或终端行为异常？

> 严格来说，这不是 typo 的问题，而是 Ghostty 终端本身的兼容性问题。

常见现象：

- 输入内容重复
- 按 `Delete` 会变成插入空格等异常行为
- 出现 `missing or unsuitable terminal: xterm-ghostty`
- 出现 `Error opening terminal: xterm-ghostty`
- 出现 `WARNING: terminal is not fully functional`

根因说明：

- Ghostty 默认会使用 `TERM=xterm-ghostty` 来声明终端能力。
- 如果 SSH 连接到的远端机器没有安装对应的 `xterm-ghostty` `terminfo` 条目，一些终端程序就无法正确识别能力，进而出现异常。

推荐处理方式：

1. 优先把 Ghostty 的 `terminfo` 安装到远端机器：

   ```bash
   infocmp -x xterm-ghostty | ssh YOUR-SERVER -- tic -x -
   ```

2. 如果暂时不方便安装 `terminfo`，可在 `~/.ssh/config` 中为目标主机降级为通用终端类型：

   ```sshconfig
   Host example.com
     SetEnv TERM=xterm-256color
   ```

补充说明：

- 第 2 种方式依赖 OpenSSH 8.7 或更新版本。
- `xterm-256color` 只是兼容性回退方案，无法覆盖 Ghostty 的全部高级终端特性。
- 如果开启 Ghostty 的 shell integration，也可以使用 `shell-integration-features = ssh-terminfo` 自动安装远端 `terminfo`，或使用 `shell-integration-features = ssh-env` 自动回退到 `xterm-256color`。
- 如果同时启用 `ssh-terminfo,ssh-env`，Ghostty 会先尝试安装 `terminfo`，失败后再自动回退。
- 在 macOS Sonoma 之前的系统上，系统自带 `infocmp` 版本过旧，可能无法生成可被新版 `tic` 正确读取的条目。此时需要先通过 Homebrew 安装 `ncurses`，再改用 `/opt/homebrew/opt/ncurses/bin/infocmp` 或 `/usr/local/opt/ncurses/bin/infocmp`。

See: https://ghostty.org/docs/help/terminfo#ssh

## 3. Jetbraines 通过 Gateway 启动的 Remote IDE 终端里绑定 esc 键无效？

确认是 Jetbrains IDE 的问题。将 shell 集成脚本（`typo init zsh` 或 `typo init bash`）中的按键改为其他键即可生效。

```shell
# zsh
bindkey '\e\e' _typo_fix_command
bindkey '^T' _typo_fix_command

# bash
bind -x '"\e\e":_typo_fix_command'
bind -x '"\C-t":_typo_fix_command'
```

## 4. 为什么 `/tmp` 或 `$TMPDIR` 里会出现 `typo-stderr-*` 文件？为什么有时退出后还会残留？

这是 zsh 集成脚本的正常工作机制。

根因说明：

- typo 会把上一条命令的 `stderr` 暂存到 `typo-stderr-*` 文件里。
- 当你按下 `<Esc><Esc>` 时，`typo fix -s <file>` 会读取这份缓存，用真实报错信息辅助纠错。
- 所以在 shell 运行过程中看到 `typo-stderr-*` 文件是正常现象，不代表异常。

什么时候属于正常：

- 当前 shell 还在运行，缓存文件仍然存在。
- 看到类似 `typo-stderr-AbCdEf` 这类随机后缀文件名，这是优先使用 `mktemp` 创建临时文件的正常路径。
- 某些环境里如果 `mktemp` 不可用或创建失败，会回退成 `typo-stderr-20357` 这种带当前 shell PID 的文件名。

什么时候才算问题：

- shell 已经正常退出，但对应的 `typo-stderr-*` 文件还一直留在 `/tmp` 或 `$TMPDIR` 中。
- 打开嵌套 shell 后，父 shell 和子 shell 互相覆盖或误删对方的缓存文件。

补充说明：

- 正常退出时，typo 会在 `zshexit` 钩子里删除当前 shell 自己的缓存文件。
- 如果终端崩溃、shell 被强制杀死，或者系统异常中断，退出钩子可能来不及执行，这种场景下残留旧文件是可以接受的。
- typo 会在后续 shell 启动时清理一天前遗留的旧缓存，避免 `/tmp` 长期堆积。
