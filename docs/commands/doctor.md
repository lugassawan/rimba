---
title: rimba doctor
parent: Command
nav_order: 21
---

# rimba doctor

Diagnose leftover state from killed worktree operations: stale `index.lock` files and interrupted-cleanup worktrees.

Scans every linked worktree for two kinds of diagnostic issues:

1. **Stale `index.lock` files** — leftover locks from a killed `git worktree remove` on a very large tree. A lock proven to belong to a dead rimba sweep (marker + confirmed-dead owner PID) is recovered automatically; everything else is report-only by default.

2. **Interrupted-cleanup worktrees** — a worktree still registered in git but with all its tracked files partially deleted by an external kill landing mid-`git worktree remove`. The worktree's `git status` shows an all-unstaged-deletion signature.

Both kinds are reported by default. Use `--fix` to remove stale locks (age-based, after age-safety checks) and finish removing interrupted worktrees (the worktree only, not the branch).

## Synopsis

```sh
rimba doctor [--fix] [--force]
```

## Examples

```sh
rimba doctor                 # Report any stale locks or interrupted worktrees
rimba doctor --fix           # Remove all issues (with confirmation)
rimba doctor --fix --force   # Remove all issues without confirmation
```

A plain `rimba doctor` run may report stale locks, interrupted worktrees, or both:

```
No stale index.lock files found.

Interrupted worktree removals:
  /path/to/worktree [task/my-branch] (128 deleted file(s))

Run 'rimba remove <task> --force' to finish removing an affected worktree, or 'rimba doctor --fix' to finish them all.
```

## Common workflows

**Check for stale locks after a killed operation**
```sh
rimba doctor                 # See what's stale before touching anything
```

**Clean up after confirming no git command is in flight**
```sh
rimba doctor --fix           # Prompts before removing each lock
```

**Automated cleanup (e.g. in a script)**
```sh
rimba doctor --fix --force   # Skip the confirmation prompt
```

{: .warning }
> `--fix` deletes files and removes worktrees. For stale locks: a lock can legitimately belong to an in-flight git process — make sure no git command is running before using `--fix`. Locks a still-running rimba sweep owns are always skipped; locks proven dead via a sweep marker are recovered automatically regardless of `--fix`; remaining locks younger than a safety threshold are skipped even with `--fix` to avoid removing a lock an active process still holds. For interrupted worktrees: `--fix` removes only the worktree directory, not the branch — use `rimba remove <task> --force` to remove both.

## Flags

| Flag | Description |
|------|-------------|
| `--fix` | Remove stale `index.lock` files and finish removing interrupted worktrees (report-only without it) |
| `--force` | Skip confirmation prompt when used with `--fix` |

## Related commands

- [rimba clean](clean) · prune stale references or remove merged/stale worktrees
- [rimba remove](remove) · remove a single named worktree
