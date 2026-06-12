---
title: rimba duplicate
parent: Command Reference
nav_order: 5
---

# rimba duplicate

Create a new worktree from an existing worktree, inheriting its branch prefix. Auto-generates a `-1`, `-2`, etc. suffix unless `--as` is provided. Useful for running a parallel experiment on the same base work.

## Synopsis

```sh
rimba duplicate <task> [flags]
```

## Examples

```sh
rimba duplicate auth              # Creates feature/auth-1 from feature/auth
rimba duplicate auth --as auth-v2 # Creates feature/auth-v2 from feature/auth
rimba duplicate auth --dry-run    # Preview without making changes
```

## Common workflows

**Parallel exploration from the same base**
```sh
rimba duplicate payments
# Creates feature/payments-1 alongside feature/payments
# Both share the same history; diverge independently
```

**Named duplicate for an A/B approach**
```sh
rimba duplicate login-flow --as login-flow-approach-b
```

## Flags

| Flag | Description |
|------|-------------|
| `--as` | Custom name for the duplicate worktree (instead of auto-suffix) |
| `--skip-deps` | Skip dependency detection and installation |
| `--skip-hooks` | Skip post-create hooks |
| `--dry-run` | Preview what would be duplicated without making changes |

## Related commands

- [rimba add](add) · create a worktree from scratch
- [rimba rename](rename) · rename an existing worktree
- [rimba remove](remove) · remove a worktree when done
- [rimba trust](trust) · approve post-create shell commands that run after duplication
