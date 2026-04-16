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

## 6. Shell integration is not loaded

Symptoms:

- `typo doctor` reports `shell integration: ✗ not loaded`
- Pressing `Esc` `Esc` does nothing
- `echo $TYPO_SHELL_INTEGRATION` prints empty or is unset

Common causes:

- The shell init line (`eval "$(typo init <shell>)"`) is missing from the shell config file.
- The init line is in the wrong file. For example, it is in `~/.bash_profile` instead of `~/.bashrc`, so non-login shells never source it.
- `typo` is not in `PATH` when the shell starts, so `typo init` fails silently.
- The init line is commented out or has a syntax error.

Recommended fixes:

1. Run `typo doctor` to check which shell is detected and what config file it expects:

   ```bash
   typo doctor
   ```

2. Add the correct init line to the matching config file:

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

3. Verify that `typo` is accessible before the init line runs:

   ```bash
   which typo    # or: command -v typo
   ```

4. Restart the terminal or source the config:

   ```bash
   source ~/.zshrc   # or ~/.bashrc
   ```

## 7. `Esc` `Esc` does not trigger correction

Symptoms:

- Shell integration is loaded (`TYPO_SHELL_INTEGRATION=1`), but pressing `Esc` `Esc` does nothing.
- The keybinding works intermittently or only in certain terminal windows.

Common causes:

- The terminal emulator or IDE intercepts `Esc` before the shell receives it.
- The shell is running in **vi mode**, where `Esc` switches to normal mode instead of triggering typo.
- In bash, the readline `keyseq-timeout` is too short for the shell to recognize the double-`Esc` sequence.
- Another plugin or custom binding has overridden the `Esc` `Esc` sequence.

Recommended fixes:

1. Check whether your terminal captures `Esc`. In JetBrains IDEs, see [section 1](#1-why-does-pressing-the-esc-key-exit-the-terminal-in-idea) and [section 3](#3-why-doesnt-the-esc-binding-work-in-a-jetbrains-remote-ide-terminal-launched-via-gateway).

2. If your shell uses **vi mode**, `Esc` is consumed by the mode switch. Bind typo to an alternative key instead:

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

3. In **bash 4.x**, increase the keyseq timeout if the double-`Esc` is not recognized reliably:

   ```bash
   bind 'set keyseq-timeout 200'
   ```

4. Check for conflicting keybindings:

   ```bash
   # zsh — list Esc bindings
   bindkey -L | grep '\\e\\e'

   # bash — list Esc bindings
   bind -p | grep '\\e\\e'

   # fish — list escape bindings
   bind | grep escape
   ```

   If another plugin has claimed `\e\e`, load `typo init` after that plugin so typo's binding takes priority, or switch to an alternative key as shown above.

## 8. Permissions issues with `~/.typo`

Symptoms:

- `typo config set` or `typo rules add` fails with a permission error.
- `typo fix` prints `typo: invalid config file` or cannot write history.
- `typo config gen` cannot create `~/.typo/config.json`.

Common causes:

- `~/.typo` was created by a different user (for example, an accidental `sudo typo config gen`).
- The directory or its files have overly restrictive permissions.
- The filesystem is read-only or full.

Recommended fixes:

1. Check ownership and permissions:

   ```bash
   ls -la ~/.typo
   ```

2. Fix ownership if needed:

   ```bash
   sudo chown -R "$(whoami)" ~/.typo
   chmod 755 ~/.typo
   chmod 600 ~/.typo/*.json
   ```

3. If the config file is corrupt, typo automatically quarantines it on the next load (renamed to `config.json.corrupt-<timestamp>`). You can regenerate defaults:

   ```bash
   typo config gen --force
   ```

4. If the directory cannot be created at all, check disk space and filesystem state:

   ```bash
   df -h ~
   ```

Additional notes:

- typo stores all user data under `~/.typo/`: `config.json`, `rules.json`, `usage_history.json`, and subcommand caches.
- All files are created with mode `0600` and the directory with mode `0755`.
- `typo uninstall` removes the entire `~/.typo/` directory.

## 9. Conflicts with other shell plugins

Symptoms:

- typo stops working after installing another shell plugin.
- Another plugin breaks when typo integration is loaded.
- `Esc` `Esc` triggers the wrong plugin or does nothing.
- Bash prompt behaves erratically or shows delayed output.

Common causes:

- Another plugin overrides the `Esc` `Esc` keybinding.
- In bash, another plugin replaces `PROMPT_COMMAND` instead of appending to it, which removes typo's `_typo_precmd` hook.
- In bash, another plugin installs its own `DEBUG` trap, overriding typo's `_typo_debug_trap`.
- Another plugin's `preexec`/`precmd` hooks interfere with typo's stderr redirection.

Recommended fixes:

1. **Load order matters.** Place `eval "$(typo init <shell>)"` as the **last** shell integration line in your config file. This ensures typo's keybindings and hooks take priority:

   ```bash
   # ~/.zshrc
   source ~/.oh-my-zsh/oh-my-zsh.sh    # other plugins first
   eval "$(typo init zsh)"              # typo last
   ```

2. **Bash `PROMPT_COMMAND` conflicts.** typo prepends `_typo_precmd` to `PROMPT_COMMAND` and preserves existing values. If another plugin overwrites `PROMPT_COMMAND` entirely after typo loads, typo's precmd hook is lost. Reorder the plugins so typo loads last, or manually re-add the hook:

   ```bash
   PROMPT_COMMAND="_typo_precmd; $PROMPT_COMMAND"
   ```

3. **Bash `DEBUG` trap conflicts.** typo chains the previous `DEBUG` trap when it installs its own. If another plugin installs a `DEBUG` trap after typo, it may break the chain. Load typo last to avoid this, or verify the trap chain:

   ```bash
   trap -p DEBUG
   ```

4. **Keybinding conflicts.** If another plugin uses `Esc` `Esc`, bind typo to a different key as described in [section 7](#7-esc-esc-does-not-trigger-correction).

5. **Stderr redirection conflicts.** typo redirects stderr through `tee` to capture error output. Plugins that also redirect fd 2 (stderr) may interfere. If you see duplicated or missing error output, try loading typo after the conflicting plugin.

Known compatible plugins:

- oh-my-zsh, Prezto, zsh-autosuggestions, zsh-syntax-highlighting
- bash-preexec (typo chains the existing DEBUG trap)
- fisher, oh-my-fish

If you encounter a specific incompatibility, please open an issue with the plugin name and shell version.
