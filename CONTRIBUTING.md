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

- Use the imperative mood: "Fix typo correction for docker commands"
- Keep the subject line under 72 characters
- Separate subject from body with a blank line
- Use the body to explain *what* and *why*, not *how*

## Pull Request Expectations

- All CI checks must pass. The pipeline runs tests, linting, builds, and end-to-end tests on every pull request.
- Keep pull requests focused on a single logical change.
- Update documentation (README, doc comments) if the change affects user-facing behavior.
- Link related issues using `Closes #123` or `Fixes #123`.
- Be responsive to review feedback.

## License

By contributing to Typo, you agree that your contributions will be licensed under the [MIT License](LICENSE).
