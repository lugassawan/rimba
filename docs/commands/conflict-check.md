---
title: rimba conflict-check
parent: Command Reference
nav_order: 15
---

# rimba conflict-check

Scan all active worktrees and report files modified in multiple branches, indicating potential merge conflicts. Optionally simulate merges with `git merge-tree` to confirm whether conflicts are real.

## Synopsis

```sh
rimba conflict-check [flags]
```

## Examples

```sh
rimba conflict-check
rimba conflict-check --dry-merge    # Simulate merges with git merge-tree
rimba conflict-check --json         # Output as JSON
```

## Common workflows

**Pre-merge scan**
```sh
rimba conflict-check
```
```
FILE              BRANCHES              SEVERITY
src/auth.go       auth-flow, fix-login  high
src/config.go     auth-flow, payments   medium

2 file overlap(s) found across 3 branches.
```

**Confirm actual conflicts (not just overlaps)**
```sh
rimba conflict-check --dry-merge
# Runs git merge-tree per branch pair; requires git 2.38+
```

**Feed into a script**
```sh
rimba conflict-check --json | jq '.overlaps[] | select(.severity == "high")'
```

## Flags

| Flag | Description |
|------|-------------|
| `--dry-merge` | Simulate merges with `git merge-tree` (requires git 2.38+) |

## Related commands

- [rimba merge-plan](merge-plan) · optimal merge order based on overlap analysis
- [rimba sync](sync) · sync branches with main before merging
- [rimba merge](merge) · merge a branch into main
