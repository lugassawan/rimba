---
title: rimba doctor
parent: Command
nav_order: 21
---

# rimba doctor

Diagnose stale git `index.lock` files left by killed worktree operations.

Scans every linked worktree's admin directory for a stale `index.lock` file — the kind of leftover a killed `git worktree remove` on a very large tree can leave behind. A lock proven to belong to a dead rimba sweep (marker + confirmed-dead owner PID) is recovered automatically; everything else is report-only by default — use `--fix` to remove it.

## Synopsis

```sh
rimba doctor [--fix] [--force]
```

## Examples

```sh
rimba doctor                 # Report stale index.lock files
rimba doctor --fix           # Remove stale index.lock files (with confirmation)
rimba doctor --fix --force   # Remove stale index.lock files without confirmation
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
> `--fix` deletes files. A lock can legitimately belong to an in-flight git process — make sure no git command is running before using `--fix`. Locks a still-running rimba sweep owns are always skipped; locks proven dead via a sweep marker are recovered automatically regardless of `--fix`; remaining locks younger than a safety threshold are skipped even with `--fix` to avoid removing a lock an active process still holds.

## Flags

| Flag | Description |
|------|-------------|
| `--fix` | Remove stale `index.lock` files (report-only without it) |
| `--force` | Skip confirmation prompt when used with `--fix` |

## Related commands

- [rimba clean](clean) · prune stale references or remove merged/stale worktrees
- [rimba remove](remove) · remove a single named worktree
