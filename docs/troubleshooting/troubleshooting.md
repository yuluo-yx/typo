# Troubleshooting

English | [简体中文](troubleshooting_CN.md)

## 1. Why does pressing the `Esc` key exit the terminal in IDEA?

Remove the `Esc` key binding as shown below.

![IDEA Config](imgs/idea.png)

## 2. Why does typo fail or the terminal behave abnormally when using Ghostty over SSH?

> Strictly speaking, this is a Ghostty terminal issue rather than a typo issue.

Common symptoms:

- Repeated input characters
- Pressing `Delete` inserts spaces or behaves unexpectedly
- `missing or unsuitable terminal: xterm-ghostty`
- `Error opening terminal: xterm-ghostty`
- `WARNING: terminal is not fully functional`

Root cause:

- Ghostty prefers `TERM=xterm-ghostty` to advertise its terminal capabilities.
- If the remote machine does not have the `xterm-ghostty` `terminfo` entry installed, terminal applications over SSH may fail or behave incorrectly.

Recommended fixes:

1. Prefer installing Ghostty's `terminfo` entry on the remote host:

   ```bash
   infocmp -x xterm-ghostty | ssh YOUR-SERVER -- tic -x -
   ```

2. If installing `terminfo` is not practical, configure SSH to fall back to a widely supported terminal type in `~/.ssh/config`:

   ```sshconfig
   Host example.com
     SetEnv TERM=xterm-256color
   ```

Additional notes:

- The fallback approach requires OpenSSH 8.7 or newer.
- `xterm-256color` is only a compatibility fallback and does not expose all Ghostty-specific terminal features.
- If you use Ghostty shell integration, `shell-integration-features = ssh-terminfo` can install the remote `terminfo` automatically, and `shell-integration-features = ssh-env` can configure the SSH fallback automatically.
- If both `ssh-terminfo,ssh-env` are enabled, Ghostty tries to install the `terminfo` entry first and falls back only if installation fails.
- On macOS versions earlier than Sonoma, the bundled `infocmp` is too old for this workflow. Install a newer `ncurses` via Homebrew and use `/opt/homebrew/opt/ncurses/bin/infocmp` or `/usr/local/opt/ncurses/bin/infocmp` instead.

See: https://ghostty.org/docs/help/terminfo#ssh

## 3. Why doesn't the `Esc` binding work in a JetBrains Remote IDE terminal launched via Gateway?

This appears to be a JetBrains IDE limitation. Change the binding in your shell integration script (`typo init zsh`, `typo init bash`, `typo init fish`, or `typo init powershell`) to another key, and it should work again.

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

## 4. Why does `Invoke-Expression (& typo init powershell)` fail in PowerShell?

Common causes:

- You are running Windows PowerShell 5.1 instead of PowerShell 7+
- `PSReadLine` is unavailable or failed to load
- Your profile doesn't exist yet
- Your execution policy blocks loading profile scripts

Recommended fixes:

1. Verify that you are using PowerShell 7+:

   ```powershell
   $PSVersionTable.PSVersion
   ```

2. Ensure `PSReadLine` is available:

   ```powershell
   Install-Module PSReadLine -Scope CurrentUser
   Import-Module PSReadLine
   ```

3. Create the current-host profile if needed and add typo to it:

   ```powershell
   if (!(Test-Path -Path $PROFILE.CurrentUserCurrentHost)) {
     New-Item -ItemType File -Path $PROFILE.CurrentUserCurrentHost -Force
   }
   Add-Content -Path $PROFILE.CurrentUserCurrentHost -Value 'Invoke-Expression (& typo init powershell)'
   ```

4. If your profile is blocked by execution policy, allow local profile scripts for the current user:

   ```powershell
   Set-ExecutionPolicy -Scope CurrentUser RemoteSigned
   ```

Additional notes:

- The first PowerShell release focuses on native-command stderr capture.
- Cmdlet error-stream capture can vary by host and may not populate typo's stderr cache yet.

## 5. Why are there `typo-stderr-*` files in `/tmp` or `$TMPDIR`, and why do some remain after exit?

This is normal for zsh, bash, and PowerShell integration.

Root cause:

- typo caches the previous command's `stderr` in a `typo-stderr-*` file.
- When you press `<Esc><Esc>`, `typo fix -s <file>` can use the real error output to improve the correction.

When this is normal:

- The current shell is still running and the cache file is still present.
- The file name looks random, such as `typo-stderr-AbCdEf`, because the shell created it from a temp-file API.

When this is actually a problem:

- The shell exited normally but the matching cache file remains indefinitely.
- Different nested shells overwrite or delete each other's cache files.

Additional notes:

- zsh removes its cache in the `zshexit` hook.
- bash removes its cache in the `EXIT` trap.
- PowerShell removes its cache through the `PowerShell.Exiting` engine event.
- If the terminal crashes or the shell is killed forcibly, stale files can remain temporarily. typo cleans up caches older than one day on the next shell start.
