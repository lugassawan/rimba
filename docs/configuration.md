---
title: Configuration
nav_order: 3
---

# Configuration Reference

> See also: [Command]({{ '/commands' | relative_url }})

Configure rimba via `.rimba/settings.toml` (team-shared) and `.rimba/settings.local.toml` (personal overrides) — all fields are optional with sensible defaults.
{: .fs-6 .fw-300 }

---

## Overview

`rimba init` creates a `.rimba/` directory in the repo root with two config files:

- **`settings.toml`** — team-shared configuration (commit this to git)
- **`settings.local.toml`** — personal overrides (gitignored via `.rimba/*.local.toml`, per-developer)

Local settings override team settings. Fields omitted from the local file inherit from the team file. All fields are optional — sensible defaults are applied automatically.

### Team config (`.rimba/settings.toml`)

`rimba init` auto-detects `copy_files` from gitignored local files present in the repo (falling back to a default set when none are found). Example:

```toml
copy_files = ['.env', '.env.local', '.envrc', '.tool-versions', '.vscode']

# Post-create hooks (run in new worktree directory)
post_create = ['./gradlew build']

# Staged post-create hooks: a nested array groups hooks into stages. Stages
# run in order; every hook within a stage runs concurrently. The nested
# shape alone is the parallelism declaration — no [hooks] section needed.
# post_create = [
#   ['npm run lint', 'npm run format:check'],  # stage 1: independent, run in parallel
#   ['npm run build'],                          # stage 2: runs after stage 1 completes
# ]

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

# Patch an auto-detected module by dir alone — lockfile/install are inherited.
# Only works when auto_detect is true (the default): with auto_detect = false,
# there is no detected module to inherit from, so every [[deps.modules]] entry
# must fully specify lockfile and install itself.
[[deps.modules]]
dir = 'internal-cli/node_modules'
eager = true

# Custom branch prefixes (optional — supplements the built-in feature/bugfix/hotfix/docs/test/chore)
[[resolver.prefix]]
prefix = 'spike/'
aliases = ['experiment']

# Post-create hook execution (optional — serial by default)
[hooks]
parallel = true
```

### Local overrides (`.rimba/settings.local.toml`)

```toml
# Override the IDE shortcut for your editor
[open]
ide = 'vim'
agent = 'cursor'

# Override copy_files for your local setup
copy_files = ['.env', '.env.local']
```

## Migration from `.rimba.toml`

If you have an existing `.rimba.toml` file, run `rimba init` to migrate:

```sh
rimba init
# Migrated rimba config in /path/to/repo
#   Moved:     .rimba.toml → .rimba/settings.toml
#   Created:   .rimba/settings.local.toml
#   Gitignore: updated (.rimba.toml → .rimba/*.local.toml)
```

## Field Reference

| Field | Description | Default |
|-------|-------------|---------|
| `worktree_dir` | Directory (relative to repo root) where worktrees are created | `../<repo-name>-worktrees` |
| `copy_files` | Files or directories to copy from repo root into new worktrees | auto-detected on `rimba init` from gitignored local files; falls back to `.env`, `.env.local`, `.envrc`, `.tool-versions` |
| `post_create` | Shell commands to run in new worktrees after creation. Either a flat array of commands, or a nested array of stages — see the staged example above — where each inner array's commands run concurrently and stages run in order | (none) |
| `post_rename` | Shell commands to run after `rimba rename`. Accepts the same flat-or-staged shape as `post_create` — but a flat `post_rename` list always runs fully serially regardless of `hooks.parallel` (only `post_create`'s flat form honors that flag); only the nested/staged shape opts `post_rename` into parallelism | (none) |
| `command_timeout` | Deadline for internal git/gh subprocess calls, as a Go duration (e.g. `90s`, `2m`) — does not bound `post_create`/`post_rename` hooks or `deps.modules[].install`, which are unbounded | `120s` |
| `open.<name>` | Named shortcut command for `rimba open --with <name>` | (none) |
| `deps.auto_detect` | Auto-detect dependency modules from lockfiles. When `false`, no lockfile scanning happens at all — only modules explicitly listed in `deps.modules` are managed, and each one must fully specify `lockfile`/`install` itself (there's no detected module left to patch/inherit from — see below) | `true` |
| `deps.modules[].dir` | Dependency directory to clone (e.g. `node_modules`) | — |
| `deps.modules[].lockfile` | Lockfile used to match worktrees (e.g. `pnpm-lock.yaml`). May be omitted, together with `install`, when `dir` matches an auto-detected module — both are then inherited from detection. Requires `deps.auto_detect = true`; with detection off, omitting these produces a non-functional module (no lockfile to hash, no install command to run) | — |
| `deps.modules[].install` | Install command to run if no matching worktree is found. Same omission rule and `auto_detect` requirement as `lockfile` | — |
| `deps.modules[].work_dir` | Subdirectory to run the install command in | (repo root) |
| `deps.modules[].eager` | Override the eager/lazy default for this module. Unset: infer from service scope, then default to lazy for modules detected as part of a workspace/monorepo package manager (`Recursive`), eager otherwise. See [rimba deps]({{ '/commands/deps' | relative_url }}#deferred-modules) | (inferred) |
| `deps.concurrency` | Max parallel dependency-module installs | `auto (0)` |
| `resolver.prefix[].prefix` | Custom branch prefix to register, added to the built-ins (e.g. `spike/`) | — |
| `resolver.prefix[].aliases` | Alternative creation tokens for the prefix (e.g. `experiment` → `spike/`) | (none) |
| `hooks.parallel` | Run a flat `post_create` list's hooks concurrently instead of serially (has no effect on a nested/staged `post_create`, or on `post_rename` at all — see above). Opt-in: parallel hooks can break execution-order dependencies between commands (e.g. hook 1 generates a file hook 2 reads), so only enable it if your hooks are independent | `false` |

## Auto-Detected Ecosystems

When `auto_detect` is enabled (default), rimba recognizes these lockfiles automatically:

| Lockfile | Dep directory | Install command | Behavior |
|----------|---------------|-----------------|----------|
| `pnpm-lock.yaml` | `node_modules` | `pnpm install --frozen-lockfile` | Recursive clone + install fallback |
| `yarn.lock` | `node_modules` | `yarn install` | Recursive clone + `.yarn/cache` |
| `package-lock.json` | `node_modules` | `npm ci` | Recursive clone + install fallback |
| `go.sum` | `vendor` | `go mod vendor` | Clone only (skip if no match) |
| `Cargo.lock` | `target` | — | Clone only (skip if no match) |
| `uv.lock` / `poetry.lock` | `.venv` | — | Clone only + path relocation |
| `settings.gradle` / `build.gradle` (+ `.kts`) | `.gradle` (+ `build/`) | — | Clone only (skip if no match) |

{: .note }
> Dependencies are shared using copy-on-write clones (`cp -c` on macOS, `cp --reflink=auto` on Linux) for near-instant copies on supported filesystems (APFS, Btrfs). Falls back to regular copy on other systems.

{: .note }
> **Gradle design note:** rimba clones project-local build state (`.gradle/` and `build/`) from a sibling worktree when lockfile content hashes match. A stale clone is a harmless warm cache — Gradle re-validates via content hashes on next invocation. Global caches (`~/.gradle/caches`) and Maven's `~/.m2` are **not** cloned; rimba's CoW model is scoped to project-local directories only. Maven support (project-local `target/`) is deferred.

## Environment Variables

| Variable | Description |
|----------|-------------|
| `RIMBA_DEBUG` | Log git command timing to stderr (set to any value, e.g. `RIMBA_DEBUG=1`). The `--debug` flag on any command has the same effect. |
| `RIMBA_QUIET` | Suppress informational hints and tips — the pre-execution option hints and the post-update agent-file tip (set to any value, e.g. `RIMBA_QUIET=1`). Does not suppress errors or command output. |
| `NO_COLOR` | Disable colored output globally (per [no-color.org](https://no-color.org)) |

## MCP server registration

When `rimba init --agents` or `rimba init -g` is run, rimba registers itself as an MCP server (server name: `rimba`, command: `rimba mcp`) in client config files alongside the agent instruction files. The registration is idempotent — running the command again updates the entry without duplicating it. `--agents --local` updates agent files only and does **not** register MCP.

### User-level (`rimba init -g`)

Patches the following files in your home directory:

| File | Format |
|------|--------|
| `~/.claude.json` | JSON (`mcpServers` object) |
| `~/.codex/config.toml` | TOML (`[[mcp_servers]]` array) |
| `~/.gemini/settings.json` | JSON (`mcpServers` object) |
| `~/.codeium/windsurf/mcp_config.json` | JSON (`mcpServers` object) |
| `~/.roo/mcp.json` | JSON (`mcpServers` object) |

### Project-level (`rimba init --agents`)

Patches the following files in the repo root:

| File | Format |
|------|--------|
| `.mcp.json` | JSON (`mcpServers` object) |
| `.cursor/mcp.json` | JSON (`mcpServers` object) |

Use `rimba init -g --uninstall` or `rimba init --agents --uninstall` to remove the registration from the corresponding files.

## Validation

Configuration is validated on every command invocation; errors are surfaced together with a `To fix:` hint.

| Field | Error | Fix hint |
|-------|-------|----------|
| `worktree_dir` | `worktree_dir must be relative, got "<dir>"` | Set a path relative to the repo root in `.rimba/settings.toml` |
| `deps.modules[].dir` | `deps.modules[<i>]: dir is empty` | Set `dir = "<path>"` for the module |
| `deps.modules[].dir` (duplicate) | `deps.modules[<i>]: duplicate dir "<dir>"` | Remove the duplicate `[[deps.modules]]` entry |
| `deps.modules[].lockfile`/`install` | `deps.modules["<dir>"]: lockfile and install must be set together` | Set both to define a new module, or remove both to patch an auto-detected module by `dir` |
| `open.<name>` (empty key) | `open: shortcut name is empty` | Remove the empty-keyed entry under `[open]` |
| `open.<name>` (path separator) | `open["<name>"]: shortcut name must not contain path separators` | Rename the shortcut to a name without `/` |
