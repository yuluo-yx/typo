# Command Reference

English | [简体中文](commands_CN.md)

## `typo fix`

Fix a command and print the corrected result to stdout.

```bash
typo fix "gut stauts"
typo fix "gut status && dcoker ps"
typo fix "gut status | grep main"
typo fix "typ doctro"
typo fix "typo hsitory lsit"
```

Useful flags:

- `-s <file>`: read `stderr` from a file captured by shell integration.
- `--exit-code <n>`: reuse the previous exit code as additional correction context.
- `--no-history`: do not persist the accepted correction into history.
- `--alias-context <file>`: read the shell alias context captured by shell integration.

`--alias-context` is mainly used by `typo init <shell>` scripts. The context is
temporary and session-local; it lets Typo expand aliases such as `k=kubectl`,
correct the expanded command, and print the result back with the original alias
when that is safe.

Repeated accepted corrections can be promoted into silent user rules automatically once they reach the configured threshold. Set `typo config set auto-learn-threshold 0` to disable this behavior.

## `typo learn`

Teach Typo a personal correction pair.

```bash
typo learn "gst" "git status"
```

Use `learn` for day-to-day teaching. `typo learn` and `typo rules add` both add the same user rule, persist it to `~/.typo/rules.json`, and clear conflicting history; `learn` is the simpler user-facing command.

It is especially useful as a last-resort override for outrageous typos that the shortest-path matcher may not infer, such as teaching `gitsss` -> `git`. For shell aliases, prefer the shell integration first: zsh, bash, fish, and PowerShell can pass the active alias context automatically. Use `learn` for aliases only when you want a persistent manual rule outside that live shell context.

## `typo config`

Manage persisted runtime settings in `~/.typo/config.json`.

```bash
typo config list
typo config get keyboard
typo config set keyboard dvorak
typo config reset
typo config gen
typo config gen --force
```

The current configurable keys are:

- `similarity-threshold`
- `max-edit-distance`
- `max-fix-passes`
- `auto-learn-threshold`
- `keyboard`
- `history.enabled`
- `rules.<scope>.enabled`

## `typo rules`

Manage user rules and builtin rule scopes.

```bash
typo rules list
typo rules add "gst" "git status"
typo rules remove "gst"
typo rules disable git
typo rules enable docker
```

Persistence details:

- `typo rules add` and `typo rules remove` update user rules in `~/.typo/rules.json`.
- `typo rules enable` and `typo rules disable` update builtin scope switches in `~/.typo/config.json` through `rules.<scope>.enabled`.

Builtin scopes currently available through `rules.<scope>.enabled`:

- `git`, `docker`, `npm`, `yarn`, `kubectl`, `cargo`, `brew`, `helm`
- `terraform`, `python`, `pip`, `go`, `java`, `system`

## `typo history`

Inspect or clear accepted correction history.

```bash
typo history list
typo history clear
```

## `typo init`

Print the shell integration script for a supported shell.

```bash
typo init zsh
typo init bash
typo init fish
typo init powershell
```

Supported shell names:

- `zsh`
- `bash`
- `fish`
- `powershell`
- `pwsh` is accepted as an alias and normalizes to `powershell`

## `typo doctor`

Check the current environment, effective config, and shell integration hints.

```bash
typo doctor
```

The output includes:

- shell detection
- binary discovery
- config directory state
- shell integration guidance
- install method detection for Homebrew, the curl install script, manual Release binaries, Windows quick install, and `go install`
- common shell setup misconfiguration warnings, such as fish using the wrong init command style
- Go bin `PATH` guidance when installed through `go install`

## `typo version`

Print the current version, commit, and build date when available.

```bash
typo version
```

## `typo uninstall`

Remove local Typo config files and print any remaining manual cleanup steps.

```bash
typo uninstall
```

## Related docs

- For real correction scenarios, see [Usage Examples](../example/use.md).
- For correction strategy, config files, and build commands, see [How Typo Works](how-it-works.md).
- For what stays stable across v1.x releases, see [Stability Contract](stability.md).
- For installation and platform setup, see [Quick Start](../getting-started/quick-start.md).
