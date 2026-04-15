# How Typo Works

English | [简体中文](how-it-works_CN.md)

## Correction priority

Typo corrects commands in this order:

1. Error parsing
2. User rules
3. History
4. Built-in rules
5. Subcommand repair
6. Edit-distance matching

In practice, this means real command output and explicit user overrides win before fuzzy matching runs.

## Error parsing

Typo can extract suggestions from real `stderr` output when available.

Currently documented parser coverage:

- `git`: `did you mean...`, missing upstream, and related suggestions
- `docker`: unknown command suggestions
- `npm`: command-not-found suggestions

zsh, bash, and PowerShell integration can pass the `stderr` cache automatically through `typo fix -s <file>`.
In PowerShell, the first supported release guarantees this flow most reliably for native commands.
Fish integration supports current-buffer correction and empty-buffer last-command correction with exit-code context, but it does not create or pass a `typo-stderr-*` real-time `stderr` cache in its first supported release.

Examples:

```bash
typo fix -s git.stderr "git remove -v"
typo fix -s git.stderr "git pull"
typo fix -s docker.stderr "docker psa"
typo fix -s npm.stderr "npm isntall react"
```

## Subcommand correction

Typo can correct both top-level tools and tool subcommands.

Common supported tools include:

- `git`, `docker`, `npm`, `yarn`, `kubectl`, `cargo`, `go`
- `pip`, `brew`, `terraform`, `helm`
- Cloud CLIs such as `aws`, `sam`, `cdk`, `eksctl`, `gcloud`, `gsutil`, `az`, `func`, `azd`, `doctl`, `oci`, and `linode-cli`

Notes:

- Built-in command candidates cover common cloud tools even before PATH discovery runs.
- Built-in tree-shaped subcommands cover common `git`, `docker`, and `kubectl` nested commands even before dynamic discovery runs.
- Typo caches discovered subcommands in `~/.typo/subcommands.json` using `schema_version: 2`.
- Hierarchical subcommand discovery is supported for `git`, `docker`, `aws`, `gcloud`, and `az`; `kubectl` resource correction uses a conservative built-in resource tree.
- Older subcommand cache files without `schema_version: 2` are moved aside automatically and regenerated.

## Local files

Typo stores local state in `~/.typo/`:

```text
~/.typo/
├── config.json
├── rules.json
├── usage_history.json
└── subcommands.json
```

File roles:

- `config.json`: runtime settings managed by `typo config`
- `rules.json`: learned and user-defined rules
- `usage_history.json`: accepted correction history
- `subcommands.json`: cached discovered subcommand trees

## Configuration model

The default configuration currently exposes:

- `similarity-threshold`
- `max-edit-distance`
- `max-fix-passes`
- `keyboard`
- `history.enabled`
- `rules.<scope>.enabled`

Supported keyboard layouts:

- `qwerty`
- `dvorak`
- `colemak`

Builtin rule scopes:

- `git`, `docker`, `npm`, `yarn`, `kubectl`, `cargo`, `brew`
- `helm`, `terraform`, `python`, `pip`, `go`, `java`, `system`

## Build and verification

Use the repository Makefile targets for local development:

```bash
make build
make build-all
make install
make test
make coverage
make lint
```

Related checks:

- `make markdown-lint` for Markdown docs
- `make codespell-check` for spelling

## Related docs

- For CLI usage, see [Command Reference](commands.md).
- For what stays stable across v1.x releases, see [Stability Contract](stability.md).
- For installation, see [Quick Start](../getting-started/quick-start.md).
- For user-facing scenarios, see [Usage Examples](../example/use.md).
