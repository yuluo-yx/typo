# Typo - Command Auto-Correction Tool

[![Build Status](https://github.com/yuluo-yx/typo/actions/workflows/ci.yml/badge.svg)](https://github.com/yuluo-yx/typo/actions/workflows/ci.yml) [![codecov](https://codecov.io/gh/yuluo-yx/typo/branch/main/graph/badge.svg)](https://codecov.io/gh/yuluo-yx/typo) [![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://golang.org)

English | **[简体中文](README_CN.md)**

A command auto-correction tool similar to thefuck. Press `Esc` `Esc` to fix typos automatically.

## Installation

### Download from Release

Download the binary for your platform from the [Releases](https://github.com/yuluo-yx/typo/releases) page:

| Platform | File |
|----------|------|
| Linux AMD64 | `typo-linux-amd64` |
| Linux ARM64 | `typo-linux-arm64` |
| macOS AMD64 | `typo-darwin-amd64` |
| macOS ARM64 | `typo-darwin-arm64` |
| Windows AMD64 | `typo-windows-amd64.exe` |

After downloading, add execute permission and move to PATH:

```bash
chmod +x typo-linux-amd64
sudo mv typo-linux-amd64 /usr/local/bin/typo
```

### Install from Source

```bash
go install github.com/yuluo-yx/typo/cmd/typo@latest
```

Or build from source:

```bash
git clone https://github.com/yuluo-yx/typo.git
cd typo
make install
```

## Zsh Integration

Add the following to your `~/.zshrc`:

```bash
eval "$(typo init zsh)"
```

After restarting your terminal, press `Esc` `Esc` to fix the current command.

## Commands

### typo fix

Fix a command.

```bash
typo fix <command>              # Fix a command
typo fix -s <file> <command>    # Fix with stderr file for error parsing
```

Examples:
```bash
$ typo fix "gut stauts"
git status

$ typo fix "dcoker ps"
docker ps
```

### typo learn

Learn a correction rule and save it to history.

```bash
typo learn <from> <to>
```

Example:
```bash
$ typo learn "gut" "git"
Learned: gut -> git
```

### typo rules

Manage correction rules.

```bash
typo rules list              # List all rules
typo rules add <from> <to>   # Add a user rule
typo rules remove <from>     # Remove a user rule
```

Example:
```bash
$ typo rules add "mytypo" "mycommand"
Added rule: mytypo -> mycommand

$ typo rules list
gut -> git [global] (enabled)
dcoker -> docker [global] (enabled)
mytypo -> mycommand [user] (enabled)
```

### typo history

Manage correction history.

```bash
typo history list    # List correction history
typo history clear   # Clear correction history
```

Example:
```bash
$ typo history list
gut -> git (used 5 times)
dcoker -> docker (used 3 times)

$ typo history clear
History cleared
```

### typo init

Print shell integration script.

```bash
typo init zsh    # Print zsh integration script
```

### typo doctor

Check configuration status and diagnose common issues.

```bash
$ typo doctor
Checking typo configuration...

[1/4] typo command: ✓ available (/usr/local/bin/typo)
[2/4] config directory: ✓ /home/user/.typo
[3/4] shell integration: ✓ loaded
[4/4] Go bin PATH: ✓ configured

All checks passed!
```

If issues are found, doctor provides specific fix suggestions:

- **typo command not found**: Check if installed, or if Go bin directory is in PATH
- **shell integration not loaded**: Add `eval "$(typo init zsh)"` to `~/.zshrc`
- **Go bin PATH not configured**: If installed via `go install`, add `export PATH="$PATH:$(go env GOPATH)/bin"`

### typo version

Print version information.

```bash
$ typo version
typo dev (commit: 68572a5, built: unknown)
```

### typo uninstall

Completely uninstall typo, including config directory and cleanup instructions.

```bash
$ typo uninstall
Uninstalling typo...

[1/3] Removing config directory: ✓ removed /home/user/.typo
[2/3] Zsh integration: please remove the following line from ~/.zshrc:

    eval "$(typo init zsh)"

[3/3] Binary: please remove the binary manually:

    rm /usr/local/bin/typo

Uninstallation complete.
```

**Note**: For safety reasons, the program won't automatically modify `~/.zshrc` or delete the binary. Please follow the instructions to complete manually.

## Correction Strategy

Typo tries to correct commands in the following priority order:

1. **Error Parsing** - Parse suggestions like "did you mean" from stderr
2. **History** - Use previously learned corrections
3. **Rule Matching** - Built-in and user-defined rules
4. **Edit Distance** - Fuzzy matching based on keyboard layout

## Supported Error Parsing

- **git**: `did you mean...`, no upstream branch, etc.
- **docker**: Unknown command suggestions
- **npm**: Command not found suggestions

## Smart Subcommand Correction

Typo automatically parses tool subcommands for smart correction:

```bash
$ typo fix "git stattus"
git status
typo: did you mean: status?

$ typo fix "docker biuld"
docker build
typo: did you mean: build?
```

**How it works**:
1. When you first fix a command for a tool (e.g., `typo fix "git stattus"`), typo automatically runs `git help -a` to parse subcommands
2. Results are cached in `~/.typo/subcommands.json` with a 7-day validity period
3. When fixing commands, it also checks if subcommands are correct and provides suggestions

**Supported tools**: git, docker, npm, yarn, kubectl, cargo, go, pip, brew, terraform, helm, etc.

## Configuration Files

Configuration files are stored in the `~/.typo/` directory:

```
~/.typo/
├── history.json       # Correction history
├── rules.json         # User-defined rules
└── subcommands.json   # Subcommand cache
```

## Build

```bash
make build              # Build for current platform
make build-all          # Build for all platforms
make test               # Run tests
make lint               # Run linter
make ci                 # Run CI checks (fmt, lint, test)
```

## License

MIT