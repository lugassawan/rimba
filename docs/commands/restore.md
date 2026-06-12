---
title: rimba restore
parent: Command Reference
nav_order: 7
---

# rimba restore

Restore an archived worktree by recreating its directory from a branch that was previously archived with [`rimba archive`](archive). Copies dotfiles, installs dependencies, and runs post-create hooks — just like `rimba add`.

## Synopsis

```sh
rimba restore <task> [flags]
```

## Examples

```sh
rimba restore my-feature
rimba restore my-feature --skip-deps
rimba restore my-feature --skip-hooks
```

## Common workflows

**Resume paused work**
```sh
rimba list --archived          # Check archived branches
rimba restore big-refactor     # Recreate the worktree
cd $(rimba open big-refactor)  # Navigate into it
```

**Fast restore (skip slow steps)**
```sh
rimba restore my-feature --skip-deps --skip-hooks
```

{: .note }
> Restoring copies dotfiles, installs dependencies, and runs post-create hooks — just like `rimba add`. Use [`rimba archive`](archive) to archive a worktree.

## Flags

| Flag | Description |
|------|-------------|
| `--skip-deps` | Skip dependency detection and installation |
| `--skip-hooks` | Skip post-create hooks |

## Related commands

- [rimba archive](archive) · archive a worktree (remove directory, keep branch)
- [rimba list](list) · use `--archived` to see restorable branches
- [rimba add](add) · create a brand-new worktree
- [rimba trust](trust) · approve post-create shell commands that run on restore
