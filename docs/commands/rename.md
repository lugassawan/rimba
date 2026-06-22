---
title: rimba rename
parent: Command
nav_order: 4
---

# rimba rename

Rename a worktree's task, branch, and directory, or change its type (prefix). The branch is renamed to `<prefix>/<new-task>`, where the prefix is inherited from the current branch unless a prefix flag is given. Branches without a recognized prefix (e.g. created directly with `git branch`) are promoted to `feature/` on rename.

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
| `--bugfix` | Change branch type to `bugfix/` |
| `--hotfix` | Change branch type to `hotfix/` |
| `--docs` | Change branch type to `docs/` |
| `--test` | Change branch type to `test/` |
| `--chore` | Change branch type to `chore/` |

> **Note:** There is no `--feature` flag. `feature/` is the default prefix and cannot be selected explicitly. To retype a branch back to `feature/`, use `rimba remove <task>` followed by `rimba add <task>`. Direct `git branch -m` is not recommended because it renames the branch ref but not the worktree directory, leaving them out of sync.

## Related commands

- [rimba add]({{ '/docs/commands/add' | relative_url }}) · create a worktree
- [rimba duplicate]({{ '/docs/commands/duplicate' | relative_url }}) · create a copy with a new name
- [rimba trust]({{ '/docs/commands/trust' | relative_url }}) · approve post-rename shell commands
