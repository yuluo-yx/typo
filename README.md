<p align="center">
  <img src="docs/logo.svg" alt="Typo" width="280">
</p>

<p align="center">Command Auto-Correction Tool</p>

[![Build Status](https://github.com/yuluo-yx/typo/actions/workflows/build-and-test.yml/badge.svg)](https://github.com/yuluo-yx/typo/actions/workflows/ci.yml) [![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://golang.org) [![Version](https://img.shields.io/github/v/tag/yuluo-yx/typo)](https://github.com/yuluo-yx/typo/releases) [![License](https://img.shields.io/github/license/yuluo-yx/typo)](LICENSE) [![Stars](https://img.shields.io/github/stars/yuluo-yx/typo)](https://github.com/yuluo-yx/typo)

English | **[简体中文](README_CN.md)**

Press `Esc` `Esc` to fix typos automatically.

<p align="center">
  <img src="docs/typo-demo.gif" alt="Typo Demo">
</p>

## Why Typo?

There were a few reasons:

1. TheFuck is no longer actively maintained, and issues and PRs are not being handled. This was the biggest reason.
2. TheFuck is tied to Python versions, so installation took extra effort.
3. TheFuck does not handle commands containing `""` very well.

For these reasons, I wrote Typo in Go. It is not a translation of TheFuck. It is built from scratch.

## Quick Start

### Install via Homebrew

Coming soon.

### Or via script

```bash
curl -fsSL https://raw.githubusercontent.com/yuluo-yx/typo/main/tools/scripts/install.sh | bash
```

The script downloads a prebuilt Release binary by default. `Go` is only required when building from the `main` branch source.

Optional arguments:

```bash
# Install the latest release explicitly
curl -fsSL https://raw.githubusercontent.com/yuluo-yx/typo/main/tools/scripts/install.sh | bash -s -- -s latest

# Install a specific release (semver)
curl -fsSL https://raw.githubusercontent.com/yuluo-yx/typo/main/tools/scripts/install.sh | bash -s -- -s 0.2.0

# Build from the main branch source (requires Go)
curl -fsSL https://raw.githubusercontent.com/yuluo-yx/typo/main/tools/scripts/install.sh | bash -s -- -b
```

Note: The install script currently supports macOS and Linux.
It downloads the selected release binary over HTTPS, but it does not verify checksums automatically.

### Verify a Release Binary

If you download a binary manually from a GitHub Release, download `checksums.txt` from the same release and verify the file before installing it.
Run the following commands from the directory that contains both the downloaded binary and `checksums.txt`.

Linux example:

```bash
grep ' typo-linux-amd64$' checksums.txt > typo-linux-amd64.checksums
sha256sum -c typo-linux-amd64.checksums
```

macOS example:

```bash
grep ' typo-darwin-arm64$' checksums.txt > typo-darwin-arm64.checksums
shasum -a 256 -c typo-darwin-arm64.checksums
```

Replace the filename in the command with the asset you downloaded. A successful verification prints `OK`.

### Shell Integration

| Shell     | Status    |
|-----------|-----------|
| zsh       | ✅ Supported |
| bash      | ✅ Supported |
| fish      | 🚧 Planned   |
| PowerShell| ✅ Supported |

### Run

```bash
# Add to ~/.zshrc
eval "$(typo init zsh)"

# Or add to ~/.bashrc
eval "$(typo init bash)"

# Or add to $PROFILE.CurrentUserCurrentHost
# Tips: The Powershell version must >= 7.x. you can check by `$PSVersionTable.PSVersion`.
Invoke-Expression (& typo init powershell | Out-String)
```

Restart your terminal, then press `Esc` `Esc` after a typo. (No Enter can!)

PowerShell notes:

- Shell integration requires PowerShell 7+ and `PSReadLine`.
- `typo init pwsh` is supported as an alias, but `typo init powershell` is the primary command.
- The first PowerShell release guarantees stderr-assisted correction for native commands. Cmdlet error-stream capture can vary by host.

## Commands

> More usage examples: [use](./docs/example/use.md)

### `typo fix` - Fix a command

```bash
typo fix "gut stauts"                 # → git status
typo fix "gut status && dcoker ps"    # → git status && docker ps
typo fix "gut status | grep main"     # → git status | grep main
```

### `typo learn` - Learn a correction

```bash
typo learn "gst" "git status"         # Recommended for recurring personal fixes
```

Use `learn` for normal day-to-day teaching. `typo learn` and `typo rules add` both add the same user rule and clear conflicting history; `learn` is the simpler user-facing command, while `rules add` fits explicit rule management alongside `rules list` and `rules remove`.

### `typo config` - Manage global settings

```bash
typo config list                         # List all effective config values
typo config get keyboard                 # Show one config value
typo config set keyboard dvorak          # Persist a config override
typo config reset                        # Reset config.json to defaults
typo config gen                          # Generate the default config file
typo config gen --force                  # Overwrite an existing config file
```

### `typo rules` - Manage rules

```bash
typo rules list                    # List all rules
typo rules add "gst" "git status"  # Same effect as `learn`, but in rule-management flow
typo rules remove "gst"            # Remove rule
typo rules disable git             # Disable builtin git rule scope
typo rules enable docker           # Re-enable builtin docker rule scope
```

### `typo history` - View correction history

```bash
typo history list      # Show past corrections
typo history clear     # Clear history
```

### `typo doctor` - Diagnose issues

```bash
typo doctor            # Check environment status, effective config, and shell-specific setup hints
```

### Other commands

```bash
typo init zsh          # Print shell integration script
typo init bash         # Print shell integration script
typo init powershell   # Print PowerShell integration script
typo version           # Show version
typo uninstall         # Remove local config and print remaining cleanup steps
```

## How It Works

Typo corrects commands in this priority:

1. **Error Parsing** - Extracts command-specific suggestions from stderr when available
2. **User Rules** - Applies learned and user-defined overrides first
3. **History** - Reuses previously accepted corrections
4. **Built-in Rules** - Applies bundled typo mappings
5. **Subcommand Repair** - Tries known tool subcommands before falling back further
6. **Edit Distance** - Uses keyboard-aware fuzzy matching with lower cost for adjacent-key substitutions

### Supported Error Parsing

- **git**: `did you mean...`, missing upstream, etc.
- **docker**: Unknown command suggestions
- **npm**: Command not found suggestions

`-s <file>` tells `typo fix` to read stderr from a file. This is mainly for parser-based fixes and is usually passed automatically by the shell integration after a command fails. In PowerShell, the shell integration currently guarantees this stderr cache for native commands.

Examples:

```bash
typo fix -s git.stderr "git remove -v"      # → git remote -v
typo fix -s git.stderr "git pull"           # → git pull --set-upstream origin main
typo fix -s docker.stderr "docker psa"      # → docker ps
typo fix -s npm.stderr "npm isntall react"  # → npm install react
```

### Smart Subcommand Correction

Automatically parses tool subcommands for intelligent suggestions:

```bash
typo fix "git stattus"   # → git status
typo fix "docker biuld"  # → docker build
```

Supported: git, docker, npm, yarn, kubectl, cargo, go, pip, brew, terraform, helm

## Configuration

Files stored in `~/.typo/`:

```
~/.typo/
├── config.json                 # Global runtime settings managed by `typo config`
├── rules.json                  # Learned and user-defined rules
├── usage_history.json          # Correction history persisted from accepted/direct fixes
└── subcommands.json            # Subcommand cache
```

`config.json` controls thresholds, keyboard layout, history recording, and rule-set toggles. Use `typo config gen` to create a full editable file with defaults.

## Build

```bash
make build      # Build for current platform
make build-all  # Build for all platforms
make install    # Install typo to Go BIN
make test       # Run tests
make coverage   # Run tests with coverage
make lint       # Run linter
```

## Release Integrity

Each GitHub Release publishes a `checksums.txt` file with SHA-256 hashes for all platform binaries. If you install from release assets directly, verify the downloaded binary against that file before placing it on your `PATH`.

## Community Love

Thanks to everyone who helped build Typo.

<p align="center">
  <img src=".github/CONTRIBUTORS.svg" alt="Typo Contributors">
</p>

## License

MIT
