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
  node_modules [a1b2c3d4e5f6] installed
  vendor [7g8h9i0j1k2l] installed
refs/heads/feature/auth (/path/to/worktrees/feature-auth)
  node_modules [a1b2c3d4e5f6] deferred
  vendor [7g8h9i0j1k2l] installed
```

Each module's install state — `installed`, `deferred`, or `missing` (expected but absent, e.g. a failed install) — is shown alongside its lockfile hash. See [Deferred modules](#deferred-modules) below.

---

## rimba deps install

Detect and install dependencies for a specific worktree. Clones from an existing worktree with a matching lockfile hash, or falls back to the configured install command.

### Synopsis

```sh
rimba deps install <task> [--path <dir>]
```

### Examples

```sh
rimba deps install my-feature
```

### `--path`

Install only one module instead of everything detected for the worktree:

```sh
rimba deps install my-feature --path standalone-svc-a/node_modules
```

---

## Deferred modules

Modules whose install cost is unbounded — pnpm/yarn/npm `node_modules` in a workspace/monorepo setup — are **deferred by default**: `rimba add`/`restore`/`duplicate` don't install them automatically unless the worktree's service scope specifically implies they're needed (a workspace member with no lockfile of its own, or a service matching the module's own independent lockfile). A deferred module's directory simply doesn't exist until you install it:

```sh
rimba add my-feature
# Dependencies:
#   node_modules: deferred — run `rimba deps install my-feature --path node_modules` if you need it

rimba deps install my-feature --path node_modules
```

Override the default for a specific module in `.rimba/settings.toml`:

```toml
[[deps.modules]]
dir = "internal-cli/node_modules"
eager = true
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
