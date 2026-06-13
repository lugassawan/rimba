---
title: Selecting worktrees (exec)
parent: Troubleshooting
nav_order: 2
---

# Selecting worktrees (`exec`)

### `provide --all or --type to select worktrees`

```
provide --all or --type to select worktrees
To fix: run: rimba exec --all <cmd>  OR  rimba exec --type <prefix> <cmd>
```

**Why:** `rimba exec` was called without specifying which worktrees to target.

**Fix:**

```sh
rimba exec --all "git status"
rimba exec --type feature "npm test"
```

### `invalid type "<x>"; valid types: ...`

```
invalid type "foo"; valid types: feature, bugfix, hotfix, docs, test, chore
To fix: choose one of the built-in prefix types: feature, bugfix, hotfix, docs, test, chore
```

**Why:** The value passed to `--type` is not a recognized prefix type.

**Fix:** Choose one of the types printed in the error message (`feature`, `bugfix`, `hotfix`, `docs`,
`test`, `chore`).

```sh
rimba exec --type feature "npm test"
```

### `--concurrency must be >= 0`

```
--concurrency must be >= 0
To fix: run: rimba exec --concurrency <n>  (n >= 0; 0 = unlimited)
```

**Why:** A negative value was passed to `--concurrency`.

**Fix:**

```sh
rimba exec --all --concurrency 4 "npm test"   # limit to 4 parallel
rimba exec --all --concurrency 0 "npm test"   # unlimited (default)
```
