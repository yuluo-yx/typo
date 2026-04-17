# Security Policy

Typo is a command-line tool for correcting mistyped shell commands.

## Reporting a Vulnerability

If you suspect a security issue, please report it privately. Do not open a public GitHub issue for an unpatched vulnerability.

- Email: `yuluo08290126@gmail.com`
- Suggested subject: `Typo security report`

If GitHub Security Advisories private reporting is enabled for the repository, you may use that channel instead.

Please include as much of the following as possible:

- Typo version, install method, and whether shell integration is enabled
- macOS / Linux version
- The exact command, workflow, or interaction that triggered the issue
- Minimal reproduction steps or a proof of concept
- Whether the issue involves the install script / release integrity, `PATH` hijacking, `sudo` suggestions, `stderr` temporary files, `~/.typo` local data files, or path / symlink handling

## Response Expectations

- We aim to acknowledge reports within 7 calendar days
- If no fix or mitigation is available within 30 days, we aim to provide a status update
- We coordinate disclosure after a fix, mitigation, or clear user guidance is ready

This is a maintainer-led open source project. Response times are best-effort, but security issues are prioritized over normal bug reports.

## Supported Versions

Security fixes are only guaranteed for:

- The latest published release
- The current `main` branch

Older versions may not receive security fixes. If you run high-risk commands or use shared environments, stay on a supported version.

## Current Security Boundaries

Typo is a local command-correction CLI. The current implementation has the following boundaries:

- Typo does not automatically execute corrected commands
- Typo does not automatically escalate privileges
- Permission-related logic only suggests a command prefixed with `sudo`; the user decides whether to run it
- Subcommand discovery looks up tools from the current `PATH` and may call local tools with `--help` or `help` to build cache data
- The current GitHub Release workflow publishes a `checksums.txt` file with SHA-256 hashes for all platform binaries; some historical Releases may not include that file
- The macOS/Linux install script currently downloads release binaries from GitHub over HTTPS, or downloads the `main` branch source and builds locally; it does not currently verify checksums or signatures automatically
- The Windows quick-install script verifies release checksums when `checksums.txt` is available; if that file is missing for a historical Release, it warns and continues without checksum verification

As a result, the following may be security-relevant:

- Unsafe help-command execution caused by a malicious executable earlier in `PATH`
- Incorrect `sudo` suggestions
- Correction logic that can materially increase destructive impact
- Integrity issues in the install, release, or download chain

## Local Data and Temporary Files

Typo may currently write the following files locally:

- `~/.typo/config.json`: runtime configuration
- `~/.typo/rules.json`: user-learned rules
- `~/.typo/usage_history.json`: correction history, including original and corrected commands
- `~/.typo/subcommands.json`: subcommand cache
- `typo-stderr-*` under `/tmp` or `$TMPDIR`: temporary files used by zsh, bash, and PowerShell shell integration to read the previous command's `stderr`

Current implementation notes:

- The JSON files above are currently written with `0600` permissions
- The `~/.typo` directory is currently created with `0755` permissions
- `typo-stderr-*` files are usually removed when the shell exits normally, but may remain after terminal crashes, forced shell termination, or abnormal system shutdown
- Fish integration does not create `typo-stderr-*` files in its first supported release
- There is currently no content-level redaction, secret scanning, or automatic exclusion of sensitive fields from command content or `stderr`

If your command arguments, command history, or error output may contain sensitive data:

- Be cautious when enabling shell integration on shared machines, shared accounts, or high-sensitivity environments
- You can disable persisted history with `typo config set history.enabled false`
- Evaluate whether temporarily storing the previous command's `stderr` in the system temp directory is acceptable for your environment
- If you install from GitHub Release assets directly, download the matching `checksums.txt` file and verify the SHA-256 hash before placing the binary on your `PATH` when that file is available

## What We Usually Treat as a Security Issue

The following issues are usually considered security issues:

- Integrity flaws in the install script, release artifacts, or download workflow
- Unauthorized read, write, destruction, or unintended exposure of `~/.typo` local data files
- Data leakage, ownership mix-ups, or cleanup-boundary flaws involving `typo-stderr-*` temporary files
- Flaws related to paths, symlinks, file overwrite behavior, or temporary file handling
- Privilege suggestions that should not happen, or correction results that significantly increase destructive impact

The following are usually treated as normal bugs rather than security vulnerabilities:

- Low-confidence correction mistakes that are not executed automatically
- UX issues, wording problems, or compatibility issues that do not cross a security boundary

If you are unsure whether something is security-relevant, please report it privately first.
