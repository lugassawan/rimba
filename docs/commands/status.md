---
title: rimba status
parent: Command Reference
nav_order: 9
---

# rimba status

Show a worktree dashboard with summary stats (total, dirty, stale, behind) and per-worktree age information. Use `--detail` to also show disk size, 7-day commit velocity, and a disk-footprint summary line.

## Synopsis

```sh
rimba status [flags]
```

## Examples

```sh
rimba status
rimba status --detail           # Add SIZE and 7D columns; sort by disk size (largest first)
rimba status --stale-days 7     # Consider worktrees stale after 7 days
rimba status --json             # Output as JSON
```

## Common workflows

**Daily health check**
```sh
rimba status
```
```
Worktrees: 4  Dirty: 1  Stale: 1  Behind: 0

TASK          TYPE     BRANCH              STATUS    AGE
  auth-flow   feature  feature/auth-flow   [dirty]   2d
  fix-login   bugfix   bugfix/fix-login    ✓         5h
  ui-cleanup  chore    chore/ui-cleanup    ✓         21d ⚠ stale
  payments    feature  feature/payments    ✓         3d
```

**Disk usage audit before a machine wipe**
```sh
rimba status --detail
```
```
Worktrees: 4  Dirty: 1  Stale: 1  Behind: 0
Disk: total 1.2 GB  (main: 480 MB, worktrees: 720 MB)

TASK          TYPE     BRANCH              STATUS    AGE   SIZE    7D
  auth-flow   feature  feature/auth-flow   [dirty]   2d    310 MB  4
  payments    feature  feature/payments    ✓         3d    220 MB  1
  fix-login   bugfix   bugfix/fix-login    ✓         5h    140 MB  6
  ui-cleanup  chore    chore/ui-cleanup    ✓         21d   50 MB   0  ⚠ stale
```

**Tighten the stale threshold**
```sh
rimba status --stale-days 7   # Flag anything untouched for a week
```

{: .note }
> **Columns:** `SIZE` is the on-disk footprint of the worktree directory. `7D` is the number of commits on the worktree's branch in the last 7 days. `--detail` sorts rows largest-first.

## Flags

| Flag | Description |
|------|-------------|
| `--detail` | Add SIZE and 7D columns; print disk-footprint summary; sort by disk size |
| `--stale-days` | Number of days after which a worktree is considered stale (default: 14) |

## Related commands

- [rimba list](list) · compact listing with filter options
- [rimba log](log) · most recent commit per worktree
- [rimba clean](clean) · remove stale worktrees
