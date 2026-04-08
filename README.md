<p align="center">
  <img src="docs/logo.svg" alt="Typo" width="280">
</p>

<p align="center">Command Auto-Correction Tool</p>

[![Build Status](https://github.com/yuluo-yx/typo/actions/workflows/build-and-test.yml/badge.svg)](https://github.com/yuluo-yx/typo/actions/workflows/build-and-test.yml) [![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://golang.org) [![Version](https://img.shields.io/github/v/tag/yuluo-yx/typo)](https://github.com/yuluo-yx/typo/releases) [![License](https://img.shields.io/github/license/yuluo-yx/typo)](LICENSE) [![Stars](https://img.shields.io/github/stars/yuluo-yx/typo)](https://github.com/yuluo-yx/typo)

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

## Shell Integration

See [Quick Start](docs/getting-started/quick-start.md) for installation and platform-specific setup.

| Shell      | Status |
|------------|--------|
| zsh        | ✅ Supported |
| bash       | ✅ Supported |
| fish       | 🚧 Planned |
| PowerShell | ✅ Supported |

## Run

```bash
# Add to ~/.zshrc
eval "$(typo init zsh)"

# Or add to ~/.bashrc
eval "$(typo init bash)"

# Or add to $PROFILE.CurrentUserCurrentHost
# Tips: The Powershell version must >= 7.x. you can check by `$PSVersionTable.PSVersion`.
Invoke-Expression (& typo init powershell | Out-String)
```

Restart your terminal, run `typo doctor`, then press `Esc` `Esc` after a typo.

## Documentation

- [Quick Start](docs/getting-started/quick-start.md)
- [Command Reference](docs/reference/commands.md)
- [Usage Examples](docs/example/use.md)
- [Troubleshooting](docs/troubleshooting/troubleshooting.md)
- [How Typo Works](docs/reference/how-it-works.md)

## Release Integrity

Each GitHub Release publishes a `checksums.txt` file with SHA-256 hashes for all platform binaries.
If you install from release assets directly, verify the downloaded binary against that file before placing it on your `PATH`.
For step-by-step verification commands, see [Quick Start](docs/getting-started/quick-start.md#verify-a-release-binary).

## Community Love

Thanks to everyone who helped build Typo.

<p align="center">
  <img src=".github/CONTRIBUTORS.svg" alt="Typo Contributors">
</p>

## License

MIT
