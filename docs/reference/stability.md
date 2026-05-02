# Stability and Compatibility Contract

English | [简体中文](stability_CN.md)

This document defines what users and packagers can rely on across v1.x releases.
Typo follows [Semantic Versioning](https://semver.org/). Within the v1.x line,
breaking changes to **stable** interfaces will not occur without a major version
bump.

## Stability tiers

Every public interface falls into one of two tiers:

| Tier | Meaning |
|------|---------|
| **Stable** | Will not change in a backward-incompatible way during v1.x. Deprecations will be announced at least one minor release before removal in a future major version. |
| **Experimental** | May change or be removed in any minor release. Experimental features are documented as such wherever they appear. |

## Config directory (`~/.typo`)

**Tier: Stable**

- Typo stores all user data under `~/.typo/`.
- The directory and its contents will remain at this path throughout v1.x.
- Files are created with mode `0600` and the directory with mode `0755`.
- `typo uninstall` removes the entire `~/.typo/` directory and will continue to do so.

### Stable files

| File | Purpose |
|------|---------|
| `config.json` | Runtime settings managed by `typo config` |
| `rules.json` | Learned and user-defined rules |
| `usage_history.json` | Accepted correction history |
| `subcommands.json` | Cached discovered subcommand trees |

New files may be added to `~/.typo/` in minor releases, but existing files will
not be renamed, moved, or have their purpose changed.

### Migration

If a minor release changes the internal schema of a config file, Typo will
migrate the file automatically on first load and preserve a backup
(`<file>.backup-<timestamp>`). Manual migration will never be required within
v1.x.

Cache files may use a different path. If `subcommands.json` changes format,
typo may quarantine the old cache as `subcommands.json.corrupt-<timestamp>` and
regenerate it instead of migrating it in place. This is allowed because
`subcommands.json` contains only discovered command metadata.

## Config file format

**Tier: Stable**

The following `config.json` field names are stable and will not be renamed or
removed during v1.x:

- `similarity_threshold`
- `max_edit_distance`
- `max_fix_passes`
- `auto_learn_threshold`
- `keyboard`
- `history.enabled`
- `rules.<scope>.enabled`

The `typo config` CLI uses hyphenated key names for the top-level numeric
settings; see [Command Reference](commands.md) for the CLI form.

New keys may be added in minor releases. Unrecognized keys are ignored, so
config files written by a newer v1.x release will still load in an older v1.x
binary (unknown keys are silently skipped).

Experimental config keys may also be added in minor releases. They are opt-in
and are not covered by the stable field-name guarantee above.

`usage_history.json` entries may include `rule_applied: true` to indicate that
the exact `(from, to)` pair has already been promoted into a user rule and
should no longer grow its history count.

### Supported keyboard layouts

The following values for `keyboard` are stable:

- `qwerty`
- `dvorak`
- `colemak`

Additional layouts may be added in minor releases.

### Builtin rule scopes

The following scopes for `rules.<scope>.enabled` are stable:

`git`, `docker`, `npm`, `yarn`, `kubectl`, `cargo`, `brew`, `helm`,
`terraform`, `python`, `pip`, `go`, `java`, `system`

Additional scopes may be added in minor releases. Existing scopes will not be
renamed or removed.

## CLI flags and subcommands

### Stable subcommands

The following top-level subcommands and their documented flags are stable:

| Command | Stable flags |
|---------|-------------|
| `typo fix <cmd>` | `-s <file>`, `--exit-code <n>`, `--no-history`, `--alias-context <file>`, `--debug`, `--debug=json`, `--trace-file <file>` |
| `typo explain <cmd>` | `-s <file>`, `--exit-code <n>`, `--alias-context <file>` |
| `typo learn <from> <to>` | *(none)* |
| `typo config list` | *(none)* |
| `typo config get <key>` | *(none)* |
| `typo config set <key> <value>` | *(none)* |
| `typo config reset` | *(none)* |
| `typo config gen` | `--force` |
| `typo rules list` | *(none)* |
| `typo rules add <from> <to>` | *(none)* |
| `typo rules remove <from>` | *(none)* |
| `typo rules enable <scope>` | *(none)* |
| `typo rules disable <scope>` | *(none)* |
| `typo history list` | *(none)* |
| `typo history clear` | *(none)* |
| `typo stats` | `--since <days>`, `--top <n>` |
| `typo init <shell>` | *(none)* |
| `typo doctor` | *(none)* |
| `typo version` | *(none)* |
| `typo uninstall` | *(none)* |

Stable subcommands and flags will not be removed or have their behavior changed
in an incompatible way during v1.x. New flags may be added to existing commands
in minor releases.

### Experimental features

The following experimental config key is currently available:

- CLI key: `experimental.long-option-correction.enabled`
- `config.json` key: `experimental.long_option_correction.enabled`
- Default: `false`
- Behavior: enables experimental typo correction for `--long-option` tokens only

Short options such as `-A` and `-v` are intentionally excluded from fuzzy
correction in this experimental feature.

## Shell integration API

**Tier: Stable**

Shell scripts generated by `typo init <shell>` may depend on the following
contract:

- **Supported shells**: `zsh`, `bash`, `fish`, `powershell` (including the `pwsh`
  alias). These will remain supported throughout v1.x.
- **Trigger binding**: `Esc` `Esc` is the default keybinding and will remain the
  default. Users may rebind to an alternative key as documented in the
  troubleshooting guide.
- **Environment variable**: `TYPO_SHELL_INTEGRATION=1` is set when shell
  integration is active. Scripts may check this variable to detect whether Typo
  integration is loaded.
- **Stderr cache**: zsh, bash, and PowerShell integration create temporary
  `typo-stderr-*` files for passing real error output to `typo fix -s`. This
  mechanism will remain available throughout v1.x, though internal file naming
  may change.
- **Shell correction context**: supported shell integrations may pass a
  temporary `typo-alias-*` context file to `typo fix --alias-context`. That file
  may contain alias-like command shorthands and environment variable names. The
  flag is stable; the internal environment variable name and temporary file
  naming are not part of the public API.

### What may change

- The internal implementation of the shell init scripts (function names,
  internal variables) is not part of the public API and may change in any
  release.
- The exact format of `typo doctor` output is informational and may change in
  minor releases. Do not parse it programmatically.

## Exit codes

**Tier: Stable**

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | General error |

Additional exit codes may be introduced in minor releases for more specific error
conditions. Existing codes will not change meaning.

## What this contract does not cover

- **Build toolchain version**: the minimum Go version may be updated in minor
  releases.
- **Internal package API**: the `internal/` Go packages are not part of the
  public API.
- **CI workflows and Makefile targets**: these are development tools and may
  change at any time.

## Related docs

- For CLI usage, see [Command Reference](commands.md).
- For configuration and local files, see [How Typo Works](how-it-works.md).
- For installation, see [Quick Start](../getting-started/quick-start.md).
