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

## Testing

rimba uses a two-tier test layout:

**Unit tests** live next to source under `internal/**/*_test.go` and `cmd/*_test.go`. They run in-process and use mock runners injected through the `git.Runner` / `gh.Runner` interfaces (`cmd/mock_runner_test.go`, `internal/operations/mock_runner_test.go`, `internal/gh/mock_runner_test.go`).

**End-to-end tests** live in `tests/e2e/`. `TestMain` in `tests/e2e/e2e_test.go` builds the binary once with coverage instrumentation (`go build -cover -o … .`) and executes it as a subprocess against a real git repo. Use the `rimba` / `rimbaSuccess` / `rimbaFail` helpers defined in `e2e_test.go:146-203`. Repo-setup helpers come from `testutil/githelper.go` (`NewTestRepo`).

| Target | Runs |
|--------|------|
| `make test-short` | Unit tests only |
| `make test-e2e` | End-to-end tier only |
| `make test-coverage` | Both tiers, aggregated coverage |

## Adding a Subcommand

Use `cmd/add.go` as the canonical recent reference (added in #138 with the `pr:<num>` variant).

1. Create `cmd/<name>.go`. Declare per-file `flag*` / `hint*` consts at the top.
2. Declare `var <name>Cmd = &cobra.Command{Use, Short, Long, Args, RunE}`. In `RunE`, read config via `config.FromContext(cmd.Context())`, build a `git.Runner` via `newRunner()`, and (if needed) a `gh.Runner` via the package-level overridable `var newGHRunner = gh.Default` so tests can swap it.
3. Keep business logic out of `cmd/`. Delegate to `internal/operations/<name>.go` so the same pipeline can back an MCP tool (see `internal/operations.ListWorktrees` / `internal/mcp/tool_list.go`).
4. Wrap user-facing errors with `errhint.WithFix(err, "<actionable fix>")`.
5. Bind flags and register the command in `init()` via `rootCmd.AddCommand(<name>Cmd)`.
6. Add a unit test in `cmd/<name>_test.go` using a mock runner and an e2e test in `tests/e2e/<name>_test.go` using the subprocess helpers.

## Project Conventions

- **`internal/gh`** — thin wrapper around the `gh` CLI. `runner.go` defines the `Runner` interface; package-level helpers: `IsAvailable` (`detect.go`), `CheckAuth` (`auth.go`), `FetchPRMeta` (`pr_meta.go`), `QueryPRStatus` (`pr_status.go`). Use `gh.Default()` in production; inject a fake in tests.
- **`internal/operations.ListWorktrees`** (`internal/operations/list_worktrees.go`) — the shared pipeline behind both `cmd/list.go` and the MCP `list` tool (`internal/mcp/tool_list.go`). Mirror this pattern when adding a subcommand that should also be reachable from MCP.
- **`internal/errhint.WithFix`** (`internal/errhint/errhint.go`) — wrap user-facing errors with an actionable fix line. Preserves sentinel via `%w`. See `cmd/add.go`, `cmd/exec.go`, `cmd/sync.go` for examples.

## License

All contributions are licensed under the [MIT](LICENSE) license.
