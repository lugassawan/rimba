---
title: Removing
parent: Troubleshooting
nav_order: 5
---

# Removing

### Uncommitted changes block removal

```
<git error about dirty state>
To fix: commit or stash changes, or use --force to discard
```

**Why:** `rimba remove <task>` found uncommitted changes in the worktree. The default behavior is
safe — it refuses to discard work.

**Fix:**

```sh
cd /path/to/worktree
git stash                       # save changes
rimba remove auth               # then remove cleanly

# OR, to discard changes permanently:
rimba remove auth --force
```
