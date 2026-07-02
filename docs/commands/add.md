---
title: rimba add
parent: Command
nav_order: 2
---

# rimba add

Create a new worktree with a branch named `<prefix>/<task>` and copy dotfiles from the repo root. The default prefix is `feature/`; use a prefix flag to override.

In monorepos, prefix the task with a service directory name (`service/task`) to create service-scoped branches. Use `pr:<num>` to create a worktree from a GitHub PR's head branch. Use `branch:<branch>` to promote the currently checked-out branch into its own worktree.

## Synopsis

```sh
rimba add <task> [flags]
rimba add pr:<num> [--task <name>] [flags]
rimba add branch:<branch> [flags]
```

## Examples

```sh
rimba add my-feature
rimba add my-feature --bugfix          # Use bugfix/ prefix instead of feature/
rimba add my-feature -s develop        # Branch from a different source
rimba add auth-api/my-feature          # Monorepo: branch auth-api/feature/my-feature
rimba add auth-api/my-feature --bugfix # Monorepo: branch auth-api/bugfix/my-feature
rimba add pr:123                       # Create worktree from PR #123's head branch
rimba add pr:123 --task review/auth-tweak  # Override auto-derived task name
rimba add branch:feature/my-feature   # Promote current branch to its own worktree
```

## Common workflows

**Start a new feature**
```sh
rimba add payments-refactor
# Branch: feature/payments-refactor
# Dotfiles copied, deps installed, post_create hooks run
```

**Fix a bug on a specific base branch**
```sh
rimba add auth-null-check --bugfix -s release/2.x
# Branch: bugfix/auth-null-check, branched from release/2.x
```

**Review a pull request**
```sh
rimba add pr:456
# Fetches PR #456's head branch, creates worktree for review
cd $(rimba open review/456-<slug>)
```

**Promote a branch you're already on**
```sh
# On branch feature/refactor in the main repo:
rimba add branch:feature/refactor
# Moves uncommitted changes via git stash into the new worktree
```

{: .note }
> **Monorepo:** If the first segment before `/` matches a directory in the repo root, rimba treats it as a service scope. The branch uses a 3-segment pattern: `<service>/<prefix>/<task>`. No configuration needed — detection is automatic.

{: .note }
> **PR mode:** `pr:<num>` requires `gh` to be installed and authenticated. For cross-fork PRs, rimba adds a `gh-fork-<owner>` remote automatically. Without `--task`, the task name is derived as `review/<num>-<slug>`.

{: .note }
> **Branch mode:** `branch:<branch>` requires that `<branch>` is the currently checked-out branch in the main repo and is not the default branch. Any uncommitted changes are transferred to the new worktree via `git stash`. `--source` is not valid in `branch:` mode. `--skip-deps` and `--skip-hooks` are accepted but have no effect — branch promotion does not run dependency installation or post-create hooks.

## Flags

| Flag | Description |
|------|-------------|
| `--bugfix` | Use `bugfix/` branch prefix |
| `--hotfix` | Use `hotfix/` branch prefix |
| `--docs` | Use `docs/` branch prefix |
| `--test` | Use `test/` branch prefix |
| `--chore` | Use `chore/` branch prefix |
| `-s`, `--source` | Source branch to create worktree from (default from config) |
| `--task` | Override auto-derived task name (`pr:<num>` mode only) |
| `--skip-deps` | Skip dependency detection and installation |
| `--skip-hooks` | Skip post-create hooks |

## Related commands

- [rimba remove](remove) · remove a worktree
- [rimba rename](rename) · rename a worktree
- [rimba duplicate](duplicate) · create another worktree from an existing one
- [rimba archive](archive) · archive a worktree for later
- [rimba trust](trust) · approve post-create shell commands
