# Quick Start

English | [简体中文](quick-start_CN.md)

## Install

### Homebrew

Homebrew installs the prebuilt Release binary for macOS or Linux.
Because the tap is maintained in this repository, add it with the explicit repository URL first.

```bash
brew tap yuluo-yx/typo https://github.com/yuluo-yx/typo
brew install typo
```

Upgrade an existing Homebrew installation:

```bash
brew update
brew upgrade typo
```

Uninstall typo and remove the tap:

```bash
brew uninstall typo
brew untap yuluo-yx/typo
```

After installing, continue with the shell integration steps in the `Shell Integration` section of the README.

### macOS / Linux via script

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

Notes:

- The install script currently supports macOS and Linux.
- WSL uses the same Linux install flow inside the WSL environment.
- The script downloads the selected Release binary over HTTPS, but it does not verify checksums automatically.

### Windows PowerShell 7+ via quick install

```powershell
iwr -useb https://raw.githubusercontent.com/yuluo-yx/typo/main/tools/scripts/quick-install.ps1 | iex
```

The Windows quick-install script downloads the latest Release binary, verifies it against `checksums.txt`, installs `typo.exe` into `%LOCALAPPDATA%\Programs\typo\bin`, and prints the next PowerShell integration steps.

## Platform support

Typo currently supports:

- macOS
- Linux
- WSL
- Native Windows PowerShell 7+ with `PSReadLine`

### Native Windows

Recommended setup:

1. Install typo with the quick-install command above.
2. Run `Invoke-Expression (& typo init powershell | Out-String)`.
3. Run `typo doctor`.

What `typo doctor` should report:

- `shell: powershell`
- A shell-integration hint that points to `$PROFILE.CurrentUserCurrentHost`

Current Windows limitations:

- PowerShell integration requires PowerShell 7+ and `PSReadLine`.
- The current PowerShell integration reliably uses `stderr` assistance for native commands.
- Cmdlet error-stream capture can still vary by PowerShell host.

### WSL

Recommended setup:

1. Install typo inside WSL with the Linux script.
2. Run `eval "$(typo init zsh)"` or `eval "$(typo init bash)"` inside WSL.
3. Run `typo doctor` inside WSL.

What `typo doctor` should report:

- Your Linux shell, usually `bash` or `zsh`
- The matching shell-integration hint for `~/.bashrc` or `~/.zshrc`

## Verify a Release binary

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

## Next steps

- For shell setup, see the `Shell Integration` section in the [README](../../README.md).
- For command details, see [Command Reference](../reference/commands.md).
- For practical correction scenarios, see [Usage Examples](../example/use.md).
- For what stays stable across v1.x, see [Stability Contract](../reference/stability.md).
- For common environment issues, see [Troubleshooting](../troubleshooting/troubleshooting.md).
