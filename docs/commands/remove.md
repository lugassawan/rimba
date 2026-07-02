---
title: rimba remove
parent: Command
nav_order: 3
---

# rimba remove

Remove the worktree for the given task and delete its local branch. By default the branch is deleted and the worktree must be clean; use flags to override.

## Synopsis

```sh
rimba remove <task> [flags]
```

## Examples

```sh
rimba remove my-feature
rimba remove my-feature -k           # Keep the branch after removal
rimba remove my-feature -f           # Force removal even if dirty
rimba remove my-feature --dry-run    # Preview what would be removed
```

## Common workflows

**Clean up after merging**
```sh
rimba merge my-feature    # merge deletes automatically by default
# If you merged externally (e.g. via GitHub), clean up manually:
rimba remove my-feature
```

**Keep the branch for future reference**
```sh
rimba remove my-feature --keep-branch
# Worktree removed; branch preserved for later checkout or restore
```

**Discard a dirty worktree you no longer need**
```sh
rimba remove my-feature --force
```

## Flags

| Flag | Description |
|------|-------------|
| `-k`, `--keep-branch` | Keep the local branch after removing the worktree |
| `-f`, `--force` | Force removal even if the worktree is dirty |
| `--dry-run` | Preview what would be removed without making changes |

## Related commands

- [rimba add](add) · create a worktree
- [rimba archive](archive) · keep the branch but remove the worktree directory
- [rimba merge](merge) · merge into main (auto-removes by default)
- [rimba clean](clean) · batch-remove merged or stale worktrees
