# How Typo Works

English | [简体中文](how-it-works_CN.md)

## Correction priority

Typo corrects commands in this order:

1. Shell alias context expansion, when provided by shell integration
2. Error parsing
3. User rules
4. History
5. Built-in rules
6. Subcommand repair
7. Edit-distance matching

In practice, this means real command output and explicit user overrides win before fuzzy matching runs.

## Shell alias context

Shell integration can pass the current session's aliases and simple wrappers to
`typo fix --alias-context <file>`. Typo expands that context before the normal
correction chain, then rewrites the corrected result back to the original alias
when it is safe:

```shell
alias k=kubectl
k lgo

# after pressing Esc Esc
k logs
```

The context is temporary and not stored in `~/.typo/`. zsh and bash aliases,
fish abbreviations, PowerShell aliases, and simple one-command function wrappers
are supported. Typo does not execute arbitrary function bodies; complex functions
with pipes, redirects, conditionals, or multiple commands are ignored.
The zsh shell integration only emits entries that are relevant to the current
command, which keeps alias-aware fixes fast even in shells with large plugin
setups.

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

### Subcommand cache version 2

`subcommands.json` uses a versioned tree format:

```json
{
  "schema_version": 2,
  "tools": [
    {
      "tool": "git",
      "tree": {
        "children": {
          "stash": {
            "children": {
              "save": {"terminal": true}
            }
          }
        }
      },
      "updated_at": "2026-04-15T12:00:00Z"
    }
  ]
}
```

Version 2 stores nested subcommands directly instead of keeping a flat root list
plus path-specific child lists. This lets typo correct each command level
independently, for example `gcloud container clusers lisr` to
`gcloud container clusters list`.

Node fields have these meanings:

- `children`: valid child tokens below the current level
- `terminal`: the token can end a command path
- `passthrough`: arguments after this token are treated as user input, not subcommands
- `alias`: canonical token to output when a short alias is used

If typo finds an older cache format, it renames that file to
`subcommands.json.corrupt-<timestamp>` and rebuilds a fresh version 2 cache on
next use. The file only contains discovered command metadata, so no user rules,
history, or configuration are lost.

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
