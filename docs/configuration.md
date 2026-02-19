# Configuration Reference

> Back to [README](../README.md) | See also: [Command Reference](commands.md)

---

## Overview

`rimba init` creates a `.rimba.toml` file in the repo root. All fields are optional — sensible defaults are applied automatically.

```toml
worktree_dir = '../myrepo-worktrees'
default_source = 'main'
copy_files = ['.env', '.env.local', '.envrc', '.tool-versions', '.vscode']

# Post-create hooks (run in new worktree directory)
post_create = ['./gradlew build']

# Open shortcuts (used by `rimba open --ide`, `--agent`, `-w`)
[open]
ide = 'code .'
agent = 'claude'
test = 'npm test'

# Dependency management (optional — auto-detect is on by default)
[deps]
auto_detect = true

# Manual module overrides (supplements or overrides auto-detected modules)
[[deps.modules]]
dir = 'node_modules'
lockfile = 'package-lock.json'
install = 'npm ci'

[[deps.modules]]
dir = 'api/vendor'
lockfile = 'api/go.sum'
install = 'go mod vendor'
work_dir = 'api'
```

## Field Reference

| Field | Description | Default |
|-------|-------------|---------|
| `worktree_dir` | Directory for worktrees (relative to repo root) | `../<repo-name>-worktrees` |
| `default_source` | Branch to create worktrees from | Default branch (e.g. `main`) |
| `copy_files` | Files or directories to copy from repo root into new worktrees | `.env`, `.env.local`, `.envrc`, `.tool-versions` |
| `post_create` | Shell commands to run in new worktrees after creation | (none) |
| `open.<name>` | Named shortcut command for `rimba open --with <name>` | (none) |
| `deps.auto_detect` | Auto-detect dependency modules from lockfiles | `true` |
| `deps.modules[].dir` | Dependency directory to clone (e.g. `node_modules`) | — |
| `deps.modules[].lockfile` | Lockfile used to match worktrees (e.g. `pnpm-lock.yaml`) | — |
| `deps.modules[].install` | Install command to run if no matching worktree is found | — |
| `deps.modules[].work_dir` | Subdirectory to run the install command in | (repo root) |

## Auto-Detected Ecosystems

When `auto_detect` is enabled (default), rimba recognizes these lockfiles automatically:

| Lockfile | Dep directory | Install command | Behavior |
|----------|---------------|-----------------|----------|
| `pnpm-lock.yaml` | `node_modules` | `pnpm install --frozen-lockfile` | Recursive clone + install fallback |
| `yarn.lock` | `node_modules` | `yarn install` | Recursive clone + `.yarn/cache` |
| `package-lock.json` | `node_modules` | `npm ci` | Recursive clone + install fallback |
| `go.sum` | `vendor` | `go mod vendor` | Clone only (skip if no match) |

> **Note:** Dependencies are shared using copy-on-write clones (`cp -c` on macOS, `cp --reflink=auto` on Linux) for near-instant copies on supported filesystems (APFS, Btrfs). Falls back to regular copy on other systems.

## Environment Variables

| Variable | Description |
|----------|-------------|
| `RIMBA_QUIET` | Suppress pre-execution hints (set to any value, e.g. `RIMBA_QUIET=1`) |
| `NO_COLOR` | Disable colored output globally (per [no-color.org](https://no-color.org)) |
