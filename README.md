# Typo - Command Auto-Correction Tool

[![Build Status](https://github.com/yuluo-yx/typo/actions/workflows/ci.yml/badge.svg)](https://github.com/yuluo-yx/typo/actions/workflows/ci.yml) [![codecov](https://codecov.io/gh/yuluo-yx/typo/branch/main/graph/badge.svg)](https://codecov.io/gh/yuluo-yx/typo) [![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://golang.org) [![Version](https://img.shields.io/github/v/release/yuluo-yx/typo)](https://github.com/yuluo-yx/typo/releases) [![License](https://img.shields.io/github/license/yuluo-yx/typo)](LICENSE) [![Stars](https://img.shields.io/github/stars/yuluo-yx/typo)](https://github.com/yuluo-yx/typo)

English | **[简体中文](README_CN.md)**

Press `Esc` `Esc` to fix typos automatically.

![Typo Demo](typo-demo.gif)

## Quick Start

```bash
# Install
go install github.com/yuluo-yx/typo/cmd/typo@latest

# Add to ~/.zshrc
eval "$(typo init zsh)"
```

Restart your terminal, then press `Esc` `Esc` after a typo.

## Installation

<details>
<summary>Download from Release</summary>

Download the binary for your platform from the [Releases](https://github.com/yuluo-yx/typo/releases) page:

| Platform | File |
|----------|------|
| Linux AMD64 | `typo-linux-amd64` |
| Linux ARM64 | `typo-linux-arm64` |
| macOS AMD64 | `typo-darwin-amd64` |
| macOS ARM64 | `typo-darwin-arm64` |
| Windows AMD64 | `typo-windows-amd64.exe` |

```bash
# Example for macOS ARM64
curl -LO https://github.com/yuluo-yx/typo/releases/latest/download/typo-darwin-arm64
chmod +x typo-darwin-arm64
sudo mv typo-darwin-arm64 /usr/local/bin/typo
```

</details>

<details>
<summary>Build from Source</summary>

```bash
git clone https://github.com/yuluo-yx/typo.git
cd typo
make install
```

</details>

## Commands

### `typo fix` - Fix a command

```bash
typo fix "gut stauts"     # → git status
typo fix "dcoker ps"      # → docker ps
```

### `typo learn` - Learn a correction

```bash
typo learn "gut" "git"    # Save rule for future use
```

### `typo rules` - Manage rules

```bash
typo rules list                    # List all rules
typo rules add "gut" "git"         # Add custom rule
typo rules remove "gut"            # Remove rule
```

### `typo history` - View correction history

```bash
typo history list      # Show past corrections
typo history clear     # Clear history
```

### `typo doctor` - Diagnose issues

```bash
typo doctor            # Check configuration status
```

### Other commands

```bash
typo init zsh          # Print shell integration script
typo version           # Show version
typo uninstall         # Uninstall typo
```

## How It Works

Typo corrects commands in this priority:

1. **Error Parsing** - Extracts "did you mean" suggestions from stderr
2. **History** - Uses previously learned corrections
3. **Rules** - Built-in and user-defined patterns
4. **Edit Distance** - Fuzzy matching based on keyboard layout

### Supported Error Parsing

- **git**: `did you mean...`, missing upstream, etc.
- **docker**: Unknown command suggestions
- **npm**: Command not found suggestions

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
├── history.json       # Correction history
├── rules.json         # User rules
└── subcommands.json   # Subcommand cache
```

## Build

```bash
make build      # Build for current platform
make build-all  # Build for all platforms
make test       # Run tests
make lint       # Run linter
```

## License

MIT