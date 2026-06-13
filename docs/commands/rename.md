---
title: rimba rename
parent: Command
nav_order: 4
---

# rimba rename

Rename a worktree's task, branch, and directory. The branch is renamed to `<prefix>/<new-task>`, where the prefix is inherited from the current branch.

## Synopsis

```sh
rimba rename <old-task> <new-task> [flags]
```

## Examples

```sh
rimba rename old-task new-task
rimba rename old-task new-task -f       # Force rename even if locked
rimba rename old-task new-task --skip-deps   # Skip dep refresh
```

## Common workflows

**Rename after refining scope**
```sh
rimba rename auth-changes auth-jwt-migration
# Branch: feature/auth-changes → feature/auth-jwt-migration
# Directory renamed to match
```

**Rename without re-running hooks**
```sh
rimba rename my-task my-task-v2 --skip-hooks
```

## Flags

| Flag | Description |
|------|-------------|
| `-f`, `--force` | Force rename even if the worktree is locked |
| `--skip-deps` | Skip dependency refresh after rename |
| `--skip-hooks` | Skip post-rename hooks |

## Related commands

- [rimba add](add) · create a worktree
- [rimba duplicate](duplicate) · create a copy with a new name
- [rimba trust](trust) · approve post-rename shell commands
