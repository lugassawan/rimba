---
title: Adding / promoting
parent: Troubleshooting
nav_order: 4
---

# Adding / promoting

### `--source is not valid in branch: mode`

```
--source is not valid in branch: mode
To fix: remove the --source flag: branch: promotes an existing branch, not a new one
```

**Why:** `rimba add branch:<branch> --source <ref>` was used. `branch:` mode promotes the currently
checked-out branch as-is; it does not create a new branch from a source ref.

**Fix:**

```sh
rimba add branch:feature/my-feature   # no --source
```

### `--task requires a pr:<num> argument`

```
--task requires a pr:<num> argument
To fix: pass a PR argument: rimba add pr:<num> --task <name>
```

**Why:** `--task` was passed without a `pr:<num>` positional argument. The flag is only valid in PR
mode to override the auto-derived task name.

**Fix:**

```sh
rimba add pr:123 --task review/auth
```
