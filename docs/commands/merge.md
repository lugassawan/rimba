---
title: rimba merge
parent: Command Reference
nav_order: 12
---

# rimba merge

Merge a worktree's branch into main or another worktree. When merging into main, the source worktree and branch are auto-deleted unless `--keep` is set. When merging between worktrees, the source is kept unless `--delete` is set.

## Synopsis

```sh
rimba merge <task> [flags]
```

## Examples

```sh
rimba merge auth                           # Merge into main, delete source
rimba merge auth --keep                    # Merge into main, keep source
rimba merge auth --into dashboard          # Merge into worktree, keep source
rimba merge auth --into dashboard --delete # Merge into worktree, delete source
rimba merge auth --no-ff                   # Force merge commit
rimba merge auth --dry-run                 # Preview without making changes
```

## Common workflows

**Standard merge to main and clean up**
```sh
rimba merge my-feature
# Merges feature/my-feature → main, removes worktree and branch
```

**Merge to main but keep the branch for tagging or reference**
```sh
rimba merge my-feature --keep
```

**Cross-worktree merge (e.g. integrate a shared utility)**
```sh
rimba merge shared-utils --into my-feature
# Merges shared-utils' branch into my-feature's worktree; both are kept
```

**Preview before merging**
```sh
rimba merge my-feature --dry-run
```

{: .warning }
> `--keep` and `--delete` are mutually exclusive. Merging to main deletes the source by default; merging to another worktree keeps it by default.

## Flags

| Flag | Description |
|------|-------------|
| `--into` | Target worktree task to merge into (default: main/repo root) |
| `--no-ff` | Force a merge commit (no fast-forward) |
| `--keep` | Keep source worktree after merging into main |
| `--delete` | Delete source worktree after merging into another worktree |
| `--dry-run` | Preview what would be merged/cleaned up without making changes |

## Related commands

- [rimba sync](sync) · rebase/merge with the main branch (not into main)
- [rimba merge-plan](merge-plan) · plan the optimal merge order
- [rimba conflict-check](conflict-check) · detect file overlaps before merging
- [rimba remove](remove) · manually remove a worktree after merging via GitHub
