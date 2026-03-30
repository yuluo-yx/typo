# Migrate from calendar versioning to semver with automated release pipeline

## Current behavior

Releases use a calendar-based `YY.MM.DD` format (e.g., `26.03.24`) triggered manually via `workflow_dispatch`.

## Problem

Calendar versioning doesn't communicate the nature of changes between releases. Users who integrate Typo into their shell environment (via `eval "$(typo init zsh)"`) and depend on CLI flags, output format, and config file structure (`~/.typo/rules.json`, `usage_history.json`) have no way to determine from the version alone whether an upgrade is safe or contains breaking changes.

Additionally, the current release workflow requires *manual triggering*, which adds friction and risk of human error.

Go tooling also expects semver tags prefixed with `v` for `go install` to work cleanly.

## Proposal

Adopt [Semantic Versioning](https://semver.org/) (`vMAJOR.MINOR.PATCH`):

- **MAJOR** for breaking changes to CLI commands, output format, config structure, or shell integration
- **MINOR** for new features (e.g., new parsers, additional shell support, new CLI commands)
- **PATCH** for bug fixes and minor corrections

Start at `v0.x.x` to signal that the API surface is still maturing and may change between minor versions.

Version bumps are determined automatically from conventional commit messages on `main` using [release-please](https://github.com/googleapis/release-please):

| Commit type | Version bump |
|-------------|-------------|
| `fix(...)` | Patch |
| `feat(...)` | Minor |
| `feat(...)` with `BREAKING CHANGE:` footer | Major |

## Automated release workflow

### How it works

1. All PRs are **squash merged** into `main`. The PR title becomes the commit message, giving maintainers control over the conventional commit that lands on `main`.
2. On every push to `main`, release-please analyzes new commits and opens (or updates) a **Release PR** containing the calculated next version and an auto-generated changelog.
3. When a maintainer merges the Release PR, release-please creates a git tag and GitHub release. The existing release workflow then builds and publishes artifacts.

### Contributor workflow

Contributors open PRs and write code. They are encouraged to follow the commit message convention in `CONTRIBUTING.md`, but the PR title (set by the maintainer at merge time) is what determines the version bump. Contributors cannot trigger releases.

### Guardrails

- [ ] **Branch protection on `main`**: require PRs, disallow direct pushes
- [ ] **Squash merge only**: disable "merge commit" and "rebase merge" in repo settings so all PRs land as a single conventional commit
- [x] **PR title validation**: add [semantic-pull-request](https://github.com/amannn/action-semantic-pull-request) CI check to block merges when the PR title doesn't match `<type>(<scope>): <message>`
- [ ] **Release PR permissions**: only users with `write` or `maintain` access can merge the release-please Release PR. Configure a `CODEOWNERS` file or GitHub ruleset to require approval from a maintainer for any PR that modifies release-managed files (e.g., `CHANGELOG.md`, version files)
- [ ] **Tag protection**: enable tag protection rules for `v*` tags so only release-please (via its GitHub App or PAT) can create version tags -- contributors and non-maintainers cannot push tags directly

## Other changes required

- [x] Add `release-please-config.json` and `.release-please-manifest.json` to the repository
- [x] Add a release-please GitHub Actions workflow triggered on push to `main`
- [x] Update `release.yml` to trigger on tag pushes matching `v*` instead of `workflow_dispatch`
- [x] Remove the manual `version` input from the release workflow; derive version from the git tag
- [x] Update `install.sh` to resolve semver tags (currently expects `YY.MM.DD` format)
- [x] Verify `Makefile` `LDFLAGS` version injection works with the new tag format
- [x] Update `README.md` install examples that reference the `YY.MM.DD` format

## Release process after migration

```
Contributor merges PR  →  release-please updates Release PR  →  Maintainer merges Release PR  →  Tag + GitHub Release created  →  CI builds and publishes artifacts
```

No manual tagging or workflow triggering needed.

---

If you'd like to adopt this, please comment and let us know.