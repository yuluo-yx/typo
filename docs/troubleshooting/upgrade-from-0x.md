# Upgrading from 0.x to v1

English | [简体中文](upgrade-from-0x_CN.md)

This guide covers breaking changes and migration steps for users upgrading from Typo 0.x releases to v1.

## Before you upgrade

1. Check your current version:

   ```bash
   typo version
   ```

2. Back up your local data directory:

   ```bash
   cp -r ~/.typo ~/.typo.backup
   ```

3. Note which shell integration line you use. Run `typo doctor` to confirm.

## Breaking changes

### Config file format

Typo 0.1.x did not have a config file. If you are upgrading from 0.1.x, no migration is needed — v1 creates `~/.typo/config.json` on first use with sensible defaults.

If you are upgrading from 0.2.x, the config file format (`~/.typo/config.json`) is forward-compatible with v1. The following keys are unchanged:

- `similarity_threshold`
- `max_edit_distance`
- `max_fix_passes`
- `auto_learn_threshold`
- `keyboard`
- `history.enabled`
- `rules.<scope>.enabled`

v1 may introduce additional config keys. Unknown keys in your existing file are preserved but ignored.

If your config file is invalid JSON, v1 quarantines it automatically (renamed to `config.json.corrupt-<timestamp>`) and falls back to defaults. You can regenerate a fresh config with:

```bash
typo config gen --force
```

### CLI changes

| 0.x command | v1 equivalent | Notes |
|---|---|---|
| `typo fix <command>` | `typo fix <command>` | Unchanged |
| `typo learn <from> <to>` | `typo learn <from> <to>` | Unchanged |
| N/A | `typo config list\|get\|set\|reset\|gen` | New in 0.2.0, stable in v1 |
| N/A | `typo rules list\|add\|remove\|enable\|disable` | New in 0.2.0, stable in v1 |
| N/A | `typo history list\|clear` | New in 0.2.0, stable in v1 |
| N/A | `typo doctor` | New in v1 |
| N/A | `typo uninstall` | New in v1 |

If you have scripts that call `typo fix`, they continue to work as before. The `-s <file>` and `--exit-code <n>` flags are unchanged.

### Shell integration

The shell init commands are unchanged from 0.2.x:

```bash
# zsh
eval "$(typo init zsh)"

# bash
eval "$(typo init bash)"

# fish
typo init fish | source

# PowerShell
Invoke-Expression (& typo init powershell | Out-String)
```

Changes to be aware of:

- **fish support** is new in v1. If you were using zsh or bash on 0.x and want to switch to fish, add the fish init line above.
- **`TYPO_SHELL_INTEGRATION` environment variable** is set to `1` by all shell integrations in v1. `typo doctor` checks for this variable. If you have custom scripts that check for typo's presence, use this variable.
- **`TYPO_ACTIVE_SHELL` environment variable** is set by the fish integration to help typo detect the shell. Other shells use `$SHELL` detection.
- Shell integration scripts now set owner-tracking variables (`TYPO_STDERR_CACHE_OWNER`, `TYPO_ORIG_STDERR_FD_OWNER`) to safely handle nested shell sessions. These are internal and should not be set manually.

### Directory structure

The `~/.typo/` directory layout has expanded:

```text
~/.typo/
├── config.json          (new in 0.2.0)
├── rules.json           (user rules, previously learned corrections)
├── usage_history.json   (correction history)
└── subcommands.json     (cached subcommand discovery)
```

- If upgrading from 0.1.x, `rules.json` is the only file that may already exist. All other files are created automatically.
- `subcommands.json` is a cache file. Deleting it is safe — it will be regenerated on next use.
- The directory permissions should be `0755` and file permissions should be `0600`.

### Subcommand cache format

Current v1 builds write `subcommands.json` with `schema_version: 3`. This format
stores a tree of subcommands plus long-option metadata so typo can correct nested
command paths such as `aws cloudformation wait stack-create-complete` and
`gcloud container clusters list`, and can reuse cached option candidates for
experimental `--long-option` correction.

Older cache files from previous builds may not have a `schema_version` field,
may use a flat list format, or may use the previous tree-only version 2 format.
On first load, typo moves those files aside as
`subcommands.json.corrupt-<timestamp>` and creates a fresh cache when
subcommand discovery runs again.

No manual migration is required. The cache does not store user rules, history,
or configuration. If you want to force a refresh, delete `~/.typo/subcommands.json`
and run a command that uses subcommand correction.

### Rule scopes

v1 introduces additional builtin rule scopes beyond what was available in 0.2.0:

| Scope | Since |
|---|---|
| `git`, `docker`, `npm` | 0.1.0 |
| `yarn`, `kubectl`, `cargo`, `brew` | 0.2.0 |
| `helm`, `terraform`, `python`, `pip`, `go`, `java`, `system` | 0.2.0 |

All scopes are enabled by default. If you previously disabled a scope, your setting is preserved.

### Install methods

v1 supports these install methods (all verified by `typo doctor`):

- `curl` install script (macOS / Linux)
- Windows PowerShell quick-install script
- Homebrew
- Manual GitHub Release binary

If you installed with Homebrew, upgrade through Homebrew:

```bash
typo update
```

If you used the install script, use `typo update`. By default it builds the
`main` branch locally and requires Go:

```bash
typo update
```

`typo update --version main` and `typo update --version latest` are accepted
aliases for the same main-branch source build. Do not use `--version @latest`;
`@latest` is Go module syntax.

To install a specific Release through the script-managed path:

```bash
typo update --version 1.1.0
```

`typo update` intentionally rejects `go install`, manual Release, and Windows
quick-install binaries. Use the command printed by `typo doctor` for those
install methods.

## Post-upgrade checklist

After upgrading:

1. Restart your terminal (or source your shell config).
2. Run `typo doctor` to verify the new version is loaded and all checks pass.
3. Run `typo version` to confirm the version number.
4. Run `typo config list` to verify your settings carried over.
5. Run `typo rules list` to verify your custom rules are intact.

## Rolling back

If you need to revert to 0.x:

1. Restore your backup:

   ```bash
   rm -rf ~/.typo
   mv ~/.typo.backup ~/.typo
   ```

2. Install the specific 0.x version:

   ```bash
   curl -fsSL https://raw.githubusercontent.com/yuluo-yx/typo/main/tools/scripts/install.sh | bash -s -- -s 0.2.0
   ```

3. Restart your terminal.

## Related docs

- [Troubleshooting](troubleshooting.md)
- [Command Reference](../reference/commands.md)
- [How Typo Works](../reference/how-it-works.md)
- [Quick Start](../getting-started/quick-start.md)
