---
title: rimba sync
parent: Command Reference
nav_order: 13
---

# rimba sync

Sync worktree(s) with the main branch by rebasing (default) or merging. Fetches the latest changes from origin first, then automatically pushes after a successful sync. Use `--no-push` to skip the push step.

## Synopsis

```sh
rimba sync <task> [flags]
rimba sync --all [flags]
```

## Examples

```sh
rimba sync my-feature                # Rebase a single worktree onto main, then push
rimba sync my-feature --merge        # Use merge instead of rebase
rimba sync my-feature --no-push      # Sync without pushing
rimba sync --all                     # Sync all eligible worktrees
rimba sync --all --include-inherited # Include duplicate worktrees
```

## Common workflows

**Keep a feature branch up to date**
```sh
rimba sync my-feature
# 1. git fetch origin
# 2. git rebase origin/main
# 3. git push (if no conflicts)
```

**Prefer merge over rebase (e.g. for shared branches)**
```sh
rimba sync my-feature --merge
```

**Sync everything at once (CI / morning ritual)**
```sh
rimba sync --all
# Skips dirty worktrees with a warning
# On conflict: rebase is aborted, recovery hint printed
```

**Sync without pushing (local only)**
```sh
rimba sync --all --no-push
```

{: .note }
> Dirty worktrees are skipped with a warning. On conflict, the rebase is automatically aborted and a recovery hint is printed. After a successful sync, the branch is pushed to origin by default.

## Flags

| Flag | Description |
|------|-------------|
| `--all` | Sync all eligible worktrees (skips dirty and inherited by default) |
| `--merge` | Use merge instead of rebase |
| `--include-inherited` | Include inherited/duplicate worktrees when using `--all` |
| `--no-push` | Skip pushing after sync |
| `--dry-run` | Preview what would be synced without making changes |

## Related commands

- [rimba merge](merge) · merge a branch into main (finished work)
- [rimba conflict-check](conflict-check) · detect overlaps before syncing
- [rimba list](list) · use `--behind` to see which worktrees need syncing
