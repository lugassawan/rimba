---
title: rimba exec
parent: Command
nav_order: 16
---

# rimba exec

Run a shell command in parallel across matching worktrees. Requires `--all` or `--type` to select targets. Output from each worktree is labeled so you can tell results apart.

## Synopsis

```sh
rimba exec "<command>" --all [flags]
rimba exec "<command>" --type <prefix> [flags]
```

## Examples

```sh
rimba exec "npm test" --all                  # Run in all worktrees
rimba exec "git status" --type bugfix        # Run in bugfix worktrees only
rimba exec "npm test" --all --dirty          # Run only in dirty worktrees
rimba exec "npm test" --all --fail-fast      # Stop after first failure
rimba exec "npm test" --all --concurrency 4  # Limit to 4 parallel runs
rimba exec "npm test" --all --json           # Output as JSON
```

## Common workflows

**Run the test suite across every branch**
```sh
rimba exec "npm test" --all
```

**Check git status across all open work**
```sh
rimba exec "git status --short" --all
```

**Lint only feature branches in parallel**
```sh
rimba exec "npm run lint" --type feature
```

**CI-style: fail fast, limited parallelism**
```sh
rimba exec "make test" --all --fail-fast --concurrency 2
```

{: .warning }
> Either `--all` or `--type` is required to select worktrees.

## Flags

| Flag | Description |
|------|-------------|
| `--all` | Run in all eligible worktrees |
| `--type` | Filter by prefix type (e.g. `feature`, `bugfix`, `hotfix`, `docs`, `test`, `chore`) |
| `--dirty` | Run only in worktrees with uncommitted changes |
| `--fail-fast` | Stop execution after the first failure |
| `--concurrency` | Max parallel executions (default: 0 = unlimited) |

## Related commands

- [rimba open](open) · run a command in a single worktree
- [rimba list](list) · inspect which worktrees would be targeted
