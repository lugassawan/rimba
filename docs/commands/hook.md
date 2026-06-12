---
title: rimba hook
parent: Command Reference
nav_order: 17
---

# rimba hook

Manage Git hooks for worktree workflow automation. Installs two hooks: a `post-merge` hook that automatically cleans up merged worktrees after `git pull`, and a `pre-commit` hook that prevents direct commits to main/master.

The `hook` command has three subcommands: `install`, `uninstall`, and `status`.

{: .note }
> `rimba hook` works with or without `rimba init`. The hooks coexist with existing user-defined hooks in the same hook files.

---

## rimba hook install

Install the rimba Git hooks:
- **`post-merge`** — runs `rimba clean --merged --force` automatically after `git pull` on the default branch.
- **`pre-commit`** — prevents direct commits to main/master.

### Synopsis

```sh
rimba hook install
```

### Examples

```sh
rimba hook install           # Install both hooks
```

---

## rimba hook uninstall

Remove the rimba `post-merge` and `pre-commit` hooks. Preserves any other content in the hook files.

### Synopsis

```sh
rimba hook uninstall
```

### Examples

```sh
rimba hook uninstall         # Remove both rimba hooks
```

---

## rimba hook status

Show whether the rimba `post-merge` and `pre-commit` hooks are currently installed.

### Synopsis

```sh
rimba hook status
```

### Examples

```sh
rimba hook status            # Check installation status
```

---

## Common workflows

**Set up hooks once after cloning**
```sh
rimba hook install
# Now: git pull on main auto-cleans merged worktrees
# Now: git commit on main/master is blocked
```

**Verify hooks are installed**
```sh
rimba hook status
```

**Remove hooks before uninstalling rimba**
```sh
rimba hook uninstall
```

## Related commands

- [rimba init](init) · initialize rimba config (hook install is separate)
- [rimba clean](clean) · the command the post-merge hook calls
