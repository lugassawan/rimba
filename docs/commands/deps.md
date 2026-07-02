---
title: rimba deps
parent: Command
nav_order: 18
---

# rimba deps

Manage worktree dependencies — detect lockfiles, clone dependency directories from existing worktrees with matching lockfile hashes, and install packages.

The `deps` command has two subcommands: `status` and `install`.

---

## rimba deps status

Show detected dependency modules and lockfile hashes for all worktrees.

### Synopsis

```sh
rimba deps status [--json]
```

### Examples

```sh
rimba deps status
```

```
refs/heads/main (/path/to/repo)
  node_modules [a1b2c3d4e5f6]
  vendor [7g8h9i0j1k2l]
refs/heads/feature/auth (/path/to/worktrees/feature-auth)
  node_modules [a1b2c3d4e5f6]
  vendor [7g8h9i0j1k2l]
```

---

## rimba deps install

Detect and install dependencies for a specific worktree. Clones from an existing worktree with a matching lockfile hash, or falls back to the configured install command.

### Synopsis

```sh
rimba deps install <task>
```

### Examples

```sh
rimba deps install my-feature
```

---

## Common workflows

**Check dependency state across worktrees**
```sh
rimba deps status
# Matching hashes means deps were cloned (fast); different hashes mean a fresh install ran
```

**Manually reinstall deps after lockfile change**
```sh
rimba deps install my-feature
```

**Skip auto-install during add, install manually later**
```sh
rimba add my-feature --skip-deps
# ... modify lockfile ...
rimba deps install my-feature
```

## Related commands

- [rimba add](add) · use `--skip-deps` to defer installation
- [rimba restore](restore) · use `--skip-deps` to defer installation
- [rimba trust](trust) · approve `deps.modules[].install` shell commands
