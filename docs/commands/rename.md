---
title: rimba rename
parent: Command
nav_order: 4
---

# rimba rename

Rename a worktree's task, branch, and directory, or change its type (prefix). The branch is renamed to `<prefix>/<new-task>`, where the prefix is inherited from the current branch unless a prefix flag is given. Branches without a recognized prefix (e.g. created directly with `git branch`) are promoted to `feature/` on rename. Use `--push` to publish the renamed branch to origin and delete the old remote branch.

## Synopsis

```sh
rimba rename <old-task> [new-task] [flags]
```

## Examples

```sh
rimba rename old-task new-task
rimba rename old-task new-task -f       # Force rename even if locked
rimba rename old-task new-task --skip-deps   # Skip dep refresh
rimba rename auth --bugfix              # Retype feature/auth → bugfix/auth
rimba rename auth auth-v2 --bugfix      # Rename and retype in one step
rimba rename auth --bugfix --push       # Rename and push, deleting old remote branch
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

**Retype a misclassified branch**
```sh
# Started with the wrong type? Change it without recreating the worktree.
rimba rename auth --bugfix
# Branch: feature/auth → bugfix/auth
# Directory: feature-auth → bugfix-auth
# All uncommitted changes are preserved
```

## Flags

| Flag | Description |
|------|-------------|
| `-f`, `--force` | Force rename even if the worktree is locked |
| `--skip-deps` | Skip dependency refresh after rename |
| `--skip-hooks` | Skip post-rename hooks |
| `--push` | Publish the renamed branch and delete the old remote branch |
| `--bugfix` | Change branch type to `bugfix/` |
| `--fix` | Alias for `--bugfix` |
| `--hotfix` | Change branch type to `hotfix/` |
| `--docs` | Change branch type to `docs/` |
| `--test` | Change branch type to `test/` |
| `--chore` | Change branch type to `chore/` |

> **Note:** There is no `--feature` flag. `feature/` is the default prefix and cannot be selected explicitly. To retype a branch back to `feature/`, use `rimba remove <task>` followed by `rimba add <task>` — note that this discards uncommitted changes, so stash or commit first. Direct `git branch -m` is not recommended because it renames the branch ref but not the worktree directory, leaving them out of sync.

> **Note:** The `--push` flag deletes the old remote branch after publishing the renamed branch. This is a destructive remote operation and cannot be undone. Use with caution, especially on shared or CI-integrated branches.

## Related commands

- [rimba add](add) · create a worktree
- [rimba duplicate](duplicate) · create a copy with a new name
- [rimba trust](trust) · approve post-rename shell commands
