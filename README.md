<p align="center">
  <img src="docs/logo.svg" alt="Typo" width="280">
</p>

<p align="center"><strong>Fix mistyped shell commands from the command line.</strong></p>

[![Build Status](https://github.com/yuluo-yx/typo/actions/workflows/build-and-test.yml/badge.svg)](https://github.com/yuluo-yx/typo/actions/workflows/build-and-test.yml) [![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://golang.org) [![Version](https://img.shields.io/github/v/tag/yuluo-yx/typo)](https://github.com/yuluo-yx/typo/releases) [![License](https://img.shields.io/github/license/yuluo-yx/typo)](LICENSE) [![Stars](https://img.shields.io/github/stars/yuluo-yx/typo)](https://github.com/yuluo-yx/typo)

English | [简体中文](README_CN.md)

Typo is a command auto-correction tool written in Go. Type a command, press `Esc` `Esc`, and Typo replaces the command line with a likely correction.

<p align="center">
  <img src="docs/typo-demo.gif" alt="Typo Demo">
</p>

## TheFuck?

There is already TheFuck, so why write Typo?

There were a few reasons:

- TheFuck is no longer actively maintained, and issues and PRs are not being handled. This is the main reason.
- TheFuck is tied to Python versions, so installation took extra effort.
- TheFuck does not handle commands containing `""` very well.

For these reasons, I wrote Typo in Go. It is not a translation of TheFuck. It is built from scratch.

## Highlights

- Correct commands in place from zsh, bash, fish, and PowerShell.
- Fix top-level commands, subcommands, compound commands, pipes, and runtime errors.
- Teach personal corrections with `typo learn`; Typo stores user rules and history under `~/.typo`.
- Install native binaries on macOS, Linux, WSL, and Windows PowerShell 7+.
- Written in Go, with binary installs that do not require an external runtime.

## Quick Start

Install with Homebrew:

```bash
brew tap yuluo-yx/typo https://github.com/yuluo-yx/typo
brew install typo
```

Or install on macOS / Linux with the script:

```bash
curl -fsSL https://raw.githubusercontent.com/yuluo-yx/typo/main/tools/scripts/install.sh | bash
```

On Windows PowerShell 7+:

```powershell
iwr -useb https://raw.githubusercontent.com/yuluo-yx/typo/main/tools/scripts/quick-install.ps1 | iex
```

For upgrades, checksum verification, and platform-specific notes, see [Quick Start](docs/getting-started/quick-start.md).

## Shell Integration

Install Typo first, then add the matching init command to your shell config.

| Shell | Config file | Init command |
|-------|-------------|--------------|
| zsh | `~/.zshrc` | `eval "$(typo init zsh)"` |
| bash | `~/.bashrc` | `eval "$(typo init bash)"` |
| fish | `~/.config/fish/config.fish` | `typo init fish \| source` |
| PowerShell 7+ | `$PROFILE.CurrentUserCurrentHost` | `Invoke-Expression (& typo init powershell \| Out-String)` |

Restart your terminal and check the setup:

```bash
typo doctor
```

Now type a command with a mistake and press `Esc` `Esc`:

```shell
gti stauts

# after pressing Esc Esc
git status
```

## CLI Commands

Use the CLI directly when you want explicit output or custom rules:

```bash
typo fix "gut status && dcoker ps"
typo learn "gst" "git status"
typo config list
typo rules list
typo history list
```

See [Command Reference](docs/reference/commands.md) for all subcommands and flags.

## Documentation

- [Quick Start](docs/getting-started/quick-start.md)
- [Command Reference](docs/reference/commands.md)
- [Usage Examples](docs/example/use.md)
- [Stability Contract](docs/reference/stability.md)
- [Troubleshooting](docs/troubleshooting/troubleshooting.md)
- [Upgrading from 0.x](docs/troubleshooting/upgrade-from-0x.md)
- [How Typo Works](docs/reference/how-it-works.md)

## Development

Development requires Go 1.25+ and GNU Make.

```bash
make setup
make test
make ci
```

Before submitting changes, make sure the Git pre-commit hooks pass. See [Contributing](CONTRIBUTING.md) for coding standards, test expectations, and commit message format.

## Release Integrity

Each GitHub Release publishes a `checksums.txt` file with SHA-256 hashes for all platform binaries.
If you install from release assets directly, verify the downloaded binary against that file before placing it on your `PATH`.
For step-by-step verification commands, see [Quick Start](docs/getting-started/quick-start.md#verify-a-release-binary).

## Contributors

Thanks to everyone who helped build Typo.

<p align="center">
  <img src=".github/CONTRIBUTORS.svg" alt="Typo Contributors">
</p>

## License

MIT
