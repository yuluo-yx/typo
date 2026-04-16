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

参考：https://ghostty.org/docs/help/terminfo#ssh

## 3. JetBrains 通过 Gateway 启动的 Remote IDE 终端里绑定 `Esc` 键无效？

确认是 JetBrains IDE 的问题。将 shell 集成脚本（`typo init zsh`、`typo init bash`、`typo init fish` 或 `typo init powershell`）中的按键改为其他键即可生效。

```shell
# zsh
bindkey '\e\e' _typo_fix_command
bindkey '^T' _typo_fix_command

# bash
bind -x '"\e\e":_typo_fix_command'
bind -x '"\C-t":_typo_fix_command'

# fish
bind escape,escape _typo_fix_command
bind ctrl-t _typo_fix_command

# PowerShell
Set-PSReadLineKeyHandler -Chord Escape,Escape -ScriptBlock { __typo_FixCommand }
Set-PSReadLineKeyHandler -Chord Ctrl+t -ScriptBlock { __typo_FixCommand }
```

## 4. 为什么 `Invoke-Expression (& typo init powershell)` 在 PowerShell 里会失败？

常见原因：

- 你当前运行的是 Windows PowerShell 5.1，而不是 PowerShell 7+
- `PSReadLine` 不可用或加载失败
- 当前 host 对应的 profile 文件还不存在
- 执行策略阻止了 profile 脚本加载

推荐处理方式：

1. 先确认当前 PowerShell 版本：

   ```powershell
   $PSVersionTable.PSVersion
   ```

2. 确保 `PSReadLine` 可用：

   ```powershell
   Install-Module PSReadLine -Scope CurrentUser
   Import-Module PSReadLine
   ```

3. 如有需要，先创建当前 host 的 profile，再写入 typo 集成：

   ```powershell
   if (!(Test-Path -Path $PROFILE.CurrentUserCurrentHost)) {
     New-Item -ItemType File -Path $PROFILE.CurrentUserCurrentHost -Force
   }
   Add-Content -Path $PROFILE.CurrentUserCurrentHost -Value 'Invoke-Expression (& typo init powershell)'
   ```

4. 如果是执行策略阻止 profile 加载，可为当前用户放开本地脚本：

   ```powershell
   Set-ExecutionPolicy -Scope CurrentUser RemoteSigned
   ```

补充说明：

- 当前首版主要聚焦 native command 的 stderr 缓存链路。
- cmdlet 的 error stream 捕获会受宿主环境影响，暂时不保证都能写入 typo 的 stderr 缓存。

## 5. 为什么 `/tmp` 或 `$TMPDIR` 里会出现 `typo-stderr-*` 文件？为什么有时退出后还会残留？

这是 zsh、bash 和 PowerShell 集成的正常工作机制。

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

- 正常退出时，zsh 会在 `zshexit` 钩子里删除缓存，bash 会在 `EXIT` trap 中删除缓存，PowerShell 会在 `PowerShell.Exiting` 事件里删除缓存。
- 如果终端崩溃、shell 被强制杀死，或者系统异常中断，退出钩子可能来不及执行，这种场景下残留旧文件是可以接受的。
- typo 会在后续 shell 启动时清理一天前遗留的旧缓存，避免 `/tmp` 长期堆积。

## 6. Shell 集成没有加载

常见现象：

- `typo doctor` 显示 `shell integration: ✗ not loaded`
- 按 `Esc` `Esc` 没有任何反应
- `echo $TYPO_SHELL_INTEGRATION` 输出为空或未设置

常见原因：

- shell 配置文件中缺少 init 行（`eval "$(typo init <shell>)"`）。
- init 行写在了错误的文件里。例如写在 `~/.bash_profile` 而不是 `~/.bashrc`，导致非登录 shell 不会加载。
- shell 启动时 `typo` 不在 `PATH` 中，`typo init` 静默失败。
- init 行被注释掉或存在语法错误。

推荐处理方式：

1. 运行 `typo doctor` 查看检测到的 shell 以及期望的配置文件：

   ```bash
   typo doctor
   ```

2. 将正确的 init 行添加到对应的配置文件中：

   ```bash
   # zsh  → ~/.zshrc
   eval "$(typo init zsh)"

   # bash → ~/.bashrc
   eval "$(typo init bash)"

   # fish → ~/.config/fish/config.fish
   typo init fish | source

   # PowerShell → $PROFILE.CurrentUserCurrentHost
   Invoke-Expression (& typo init powershell | Out-String)
   ```

3. 确认 `typo` 在 init 行执行前可用：

   ```bash
   which typo    # 或: command -v typo
   ```

4. 重启终端或重新 source 配置文件：

   ```bash
   source ~/.zshrc   # 或 ~/.bashrc
   ```

## 7. `Esc` `Esc` 无法触发纠正

常见现象：

- Shell 集成已加载（`TYPO_SHELL_INTEGRATION=1`），但按 `Esc` `Esc` 没有反应。
- 快捷键时灵时不灵，或只在某些终端窗口生效。

常见原因：

- 终端模拟器或 IDE 在 shell 接收到 `Esc` 之前就拦截了它。
- Shell 运行在 **vi 模式** 下，`Esc` 会切换模式而不是触发 typo。
- 在 bash 中，readline 的 `keyseq-timeout` 设置过短，导致无法识别连续按两次 `Esc`。
- 其他插件或自定义绑定覆盖了 `Esc` `Esc` 序列。

推荐处理方式：

1. 检查终端是否拦截了 `Esc`。JetBrains IDE 请参考 [第 1 节](#1-在-idea-终端中按-esc-会退出终端) 和 [第 3 节](#3-jetbrains-通过-gateway-启动的-remote-ide-终端里绑定-esc-键无效)。

2. 如果 shell 使用 **vi 模式**，`Esc` 会被模式切换消费。改为绑定其他键：

   ```bash
   # zsh
   bindkey '^T' _typo_fix_command

   # bash
   bind -x '"\C-t":"_typo_fix_command"'

   # fish
   bind ctrl-t _typo_fix_command

   # PowerShell
   Set-PSReadLineKeyHandler -Chord Ctrl+t -ScriptBlock { __typo_FixCommand }
   ```

3. 在 **bash 4.x** 中，如果双 `Esc` 不能稳定识别，尝试增大 keyseq 超时：

   ```bash
   bind 'set keyseq-timeout 200'
   ```

4. 检查是否存在冲突的按键绑定：

   ```bash
   # zsh — 列出 Esc 相关绑定
   bindkey -L | grep '\\e\\e'

   # bash — 列出 Esc 相关绑定
   bind -p | grep '\\e\\e'

   # fish — 列出 escape 相关绑定
   bind | grep escape
   ```

   如果其他插件占用了 `\e\e`，可以将 `typo init` 放在该插件之后加载以确保 typo 的绑定优先生效，或改用其他键（见上方示例）。

## 8. `~/.typo` 权限问题

常见现象：

- `typo config set` 或 `typo rules add` 报权限错误。
- `typo fix` 输出 `typo: invalid config file` 或无法写入历史记录。
- `typo config gen` 无法创建 `~/.typo/config.json`。

常见原因：

- `~/.typo` 由其他用户创建（例如误用 `sudo typo config gen`）。
- 目录或文件权限过于严格。
- 文件系统只读或磁盘空间已满。

推荐处理方式：

1. 检查所有权和权限：

   ```bash
   ls -la ~/.typo
   ```

2. 如有需要，修复所有权：

   ```bash
   sudo chown -R "$(whoami)" ~/.typo
   chmod 755 ~/.typo
   chmod 600 ~/.typo/*.json
   ```

3. 如果配置文件已损坏，typo 在下次加载时会自动将其隔离（重命名为 `config.json.corrupt-<时间戳>`）。可以重新生成默认配置：

   ```bash
   typo config gen --force
   ```

4. 如果目录完全无法创建，检查磁盘空间和文件系统状态：

   ```bash
   df -h ~
   ```

补充说明：

- typo 的所有用户数据存储在 `~/.typo/` 下：`config.json`、`rules.json`、`usage_history.json` 以及子命令缓存。
- 所有文件创建时使用 `0600` 权限，目录使用 `0755` 权限。
- `typo uninstall` 会删除整个 `~/.typo/` 目录。

## 9. 与其他 shell 插件冲突

常见现象：

- 安装其他 shell 插件后 typo 停止工作。
- 加载 typo 集成后，其他插件出现异常。
- `Esc` `Esc` 触发了错误的插件或完全无响应。
- Bash 提示符行为异常或输出延迟。

常见原因：

- 其他插件覆盖了 `Esc` `Esc` 按键绑定。
- 在 bash 中，其他插件直接替换了 `PROMPT_COMMAND` 而不是追加，导致 typo 的 `_typo_precmd` 钩子被移除。
- 在 bash 中，其他插件安装了自己的 `DEBUG` trap，覆盖了 typo 的 `_typo_debug_trap`。
- 其他插件的 `preexec`/`precmd` 钩子干扰了 typo 的 stderr 重定向。

推荐处理方式：

1. **加载顺序很重要。** 将 `eval "$(typo init <shell>)"` 放在配置文件中所有 shell 集成行的 **最后**。这样可以确保 typo 的按键绑定和钩子优先生效：

   ```bash
   # ~/.zshrc
   source ~/.oh-my-zsh/oh-my-zsh.sh    # 其他插件在前
   eval "$(typo init zsh)"              # typo 在最后
   ```

2. **Bash `PROMPT_COMMAND` 冲突。** typo 会在 `PROMPT_COMMAND` 前追加 `_typo_precmd` 并保留已有值。如果其他插件在 typo 之后完全覆写了 `PROMPT_COMMAND`，typo 的 precmd 钩子就会丢失。调整插件加载顺序让 typo 最后加载，或手动重新添加钩子：

   ```bash
   PROMPT_COMMAND="_typo_precmd; $PROMPT_COMMAND"
   ```

3. **Bash `DEBUG` trap 冲突。** typo 在安装自己的 `DEBUG` trap 时会链接之前已有的 trap。如果其他插件在 typo 之后安装了 `DEBUG` trap，可能会打断链接。让 typo 最后加载，或检查 trap 链：

   ```bash
   trap -p DEBUG
   ```

4. **按键绑定冲突。** 如果其他插件也使用 `Esc` `Esc`，请按 [第 7 节](#7-esc-esc-无法触发纠正) 的说明将 typo 绑定到其他键。

5. **stderr 重定向冲突。** typo 通过 `tee` 重定向 stderr 来捕获错误输出。如果其他插件也重定向 fd 2（stderr），可能会产生干扰。如果出现错误输出重复或丢失，请将 typo 放在冲突插件之后加载。

已知兼容的插件：

- oh-my-zsh、Prezto、zsh-autosuggestions、zsh-syntax-highlighting
- bash-preexec（typo 会链接已有的 DEBUG trap）
- fisher、oh-my-fish

如果你遇到特定的兼容性问题，请附上插件名称和 shell 版本提交 issue。
