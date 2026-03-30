# Contributing to Typo

Thank you for your interest in contributing to Typo! This document outlines the standards and expectations for all contributions.

## Prerequisites

- [Go 1.25+](https://golang.org/dl/)
- [golangci-lint v2](https://golangci-lint.run/welcome/install/)
- [Docker](https://docs.docker.com/get-docker/) (optional, for end-to-end tests)
- [GNU Make](https://www.gnu.org/software/make/)

## Development Commands

```bash
make build          # Build for current platform
make build-all      # Cross-compile for all supported platforms
make install        # Install to $GOPATH/bin
make test           # Run unit tests with race detection
make coverage       # Run tests and report coverage
make benchmark      # Run benchmarks
make test-e2e       # Run end-to-end tests locally
make test-e2e-docker # Run end-to-end tests in Docker
make fmt            # Format code with gofmt and goimports
make lint           # Run golangci-lint (v2 required)
make ci             # Run formatting, linting, and tests in sequence
```

Run `make ci` before pushing any changes.

## Code Style

- Follow standard [Go conventions](https://go.dev/doc/effective_go) and the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments) guide.
- Run `make fmt` before committing. The project uses `gofmt` with simplification and `goimports` with local prefix `github.com/yuluo-yx/typo`.
- All lint checks defined in `.golangci.yml` must pass. Run `make lint` to verify.
- Keep functions focused and cyclomatic complexity under 15.
- Export only what needs to be exported.

## Testing Standards

- Write table-driven tests where appropriate.
- Tests must be deterministic and must not depend on external services.
- Race detection is enabled by default (`make test` uses `-race`).
- New features should maintain or improve code coverage.

## Commit Messages

All commit messages must follow the format:

```
<type>(<scope>): <message>
```

Use the imperative mood in `<message>` (e.g., "add support for..." not "added support for..."). Keep the subject line under 72 characters. Separate subject from body with a blank line, and use the body to explain *what* and *why*, not *how*.

### Types

| Type | Purpose |
|------|---------|
| `feat` | New feature or user-facing functionality |
| `fix` | Bug fix |
| `refactor` | Code change that neither fixes a bug nor adds a feature |
| `test` | Adding or updating tests |
| `chore` | Maintenance, tooling, CI, dependencies, or documentation |
| `perf` | Performance improvement |
| `style` | Code formatting (no logic changes) |
| `ci` | CI/CD configuration changes |

### Scopes

The `<scope>` should identify the part of the codebase affected. Examples:

- `cmd` -- CLI entry point and command handling
- `engine` -- Correction engine and matching logic
- `rules` -- Built-in or user-defined rule management
- `history` -- Correction history
- `install` -- Shell integration and install scripts
- `e2e` -- End-to-end tests
- `docs` -- README, CONTRIBUTING, or other documentation
- `deps` -- Dependency changes
- `release` -- Release configuration
- `build` -- Makefile and build configuration

### Breaking Changes

Append `!` after the type (or scope) to indicate a breaking change:

```
<type>(<scope>)!: <message>
```

Alternatively, include a `BREAKING CHANGE:` footer in the commit body. Both forms trigger a SemVer major version bump (or minor bump while the project is pre-`v1`).

### Version Bumps

Releases are automated via [release-please](https://github.com/googleapis/release-please). The PR title (set at squash-merge time) determines the version bump:

| Commit pattern | Version bump |
|----------------|-------------|
| `fix(scope): ...` | Patch |
| `feat(scope): ...` | Minor |
| `feat(scope)!: ...` or `BREAKING CHANGE:` footer | Major |

### Examples

```
feat(engine): add keyboard-aware cost for Dvorak layout
fix(cmd): handle quoted arguments in compound commands
refactor(rules): extract subcommand cache into separate module
test(engine): add table-driven tests for edit distance
chore(docs): update installation instructions for Homebrew
chore(deps): bump mvdan.cc/sh to v3.14.0
ci(workflows): add Go 1.26 to test matrix
perf(engine): reduce allocations in fuzzy matching
feat(cmd)!: rename --verbose flag to --debug
fix(engine)!: change scoring return type from int to float64
```

## Pull Request Expectations

- All CI checks must pass. The pipeline runs tests, linting, builds, and end-to-end tests on every pull request.
- Keep pull requests focused on a single logical change.
- Update documentation (README, doc comments) if the change affects user-facing behavior.
- Link related issues using `Closes #123` or `Fixes #123`.
- Be responsive to review feedback.

## License

By contributing to Typo, you agree that your contributions will be licensed under the [MIT License](LICENSE).
