# Contributing to rimba

Thanks for your interest in contributing! This guide covers the development workflow.

## Development Setup

```sh
git clone https://github.com/lugassawan/rimba.git
cd rimba
make hooks  # Activate git hooks
make build  # Build the binary
```

## Git Hooks

Running `make hooks` configures Git to use the hooks in `.githooks/`:

- **pre-commit** — prevents direct commits to main/master, formats staged Go files with `make fmt`, and runs `make lint`
- **commit-msg** — enforces the `[type] Description` commit message format

## Commit Convention

All commit messages must follow the format `[type] Description`, where type is one of:

| Type | Purpose |
|------|---------|
| `feat` | New feature |
| `fix` | Bug fix |
| `refactor` | Code restructuring (no behavior change) |
| `test` | Adding or updating tests |
| `ci` | CI/CD changes |
| `docs` | Documentation |
| `perf` | Performance improvement |
| `chore` | Maintenance tasks |
| `polish` | Minor improvements and cleanups |
| `breaking` | Breaking changes |

This convention is enforced locally by git hooks and in CI by PR title validation.

## Useful Make Targets

| Target | Description |
|--------|-------------|
| `make build` | Build the binary to `./bin/rimba` |
| `make test` | Run all tests (verbose) |
| `make test-short` | Run tests in short mode (skip slow tests) |
| `make test-e2e` | Run end-to-end tests |
| `make test-coverage` | Run tests with coverage report |
| `make clean` | Remove build artifacts |
| `make fmt` | Format all Go source files |
| `make lint` | Run linters via golangci-lint |
| `make bench` | Run benchmarks |
| `make hooks` | Activate git hooks from `.githooks/` |

## License

All contributions are licensed under the [MIT](LICENSE) license.
