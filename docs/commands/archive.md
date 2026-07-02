---
title: rimba archive
parent: Command
nav_order: 6
---

# rimba archive

Archive a worktree by removing its directory but preserving the local branch. The branch can later be restored with [`rimba restore`](restore).

Archiving is useful when you need to pause a task and reclaim disk space without losing work.

## Synopsis

```sh
rimba archive <task> [flags]
```

## Examples

```sh
rimba archive my-feature
rimba archive my-feature -f      # Force archival even if dirty
rimba archive my-feature --dry-run
```

## Common workflows

**Pause a task to free disk space**
```sh
rimba archive big-refactor
# Worktree directory removed; branch preserved locally
# Use 'rimba list --archived' to see archived branches
```

**Force-archive a dirty worktree**
```sh
rimba archive wip-spike --force
# Commits are preserved in the branch; uncommitted changes are lost
```

{: .note }
> The branch is preserved locally. Use [`rimba restore`](restore) to recreate the worktree from the archived branch.

## Flags

| Flag | Description |
|------|-------------|
| `-f`, `--force` | Force archival even if the worktree has uncommitted changes |
| `--dry-run` | Preview what would be archived without making changes |

## Related commands

- [rimba restore](restore) · recreate a worktree from an archived branch
- [rimba remove](remove) · remove both worktree and branch permanently
- [rimba list](list) · use `--archived` to see archived branches
