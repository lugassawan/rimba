---
title: Syncing
parent: Troubleshooting
nav_order: 3
---

# Syncing

### `worktree "<task>" has uncommitted changes`

```
worktree "auth" has uncommitted changes
To fix: Commit or stash changes before syncing: cd /path/to/worktree
```

**Why:** `rimba sync <task>` was called on a worktree with a dirty working tree. rimba refuses to
rebase or merge over uncommitted work to avoid data loss.

**Fix:**

```sh
cd /path/to/worktree
git stash          # or: git commit -am "WIP"
rimba sync auth
```

{: .note }
> `rimba sync --all` does *not* error on dirty worktrees — it skips them and prints
> `Skipping <branch> (dirty)` so the rest continue. Run `rimba sync <task>` individually after
> cleaning up.

### `push failed for <branch>: ...`

```
push failed for feature/auth: exit status 1
To fix: cd /path/to/worktree && git push --force-with-lease
```

**Why:** The remote rejected the push after a rebase (non-fast-forward). With `--merge`, the push
is a fast-forward so the hint changes to a plain `git push`.

**Fix (after rebase):**

```sh
cd /path/to/worktree && git push --force-with-lease
```

**Fix (after `--merge`):**

```sh
cd /path/to/worktree && git push
```
