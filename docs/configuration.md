# Configuration Reference

> Back to [README](../README.md) | See also: [Command Reference](commands.md)

---

## Overview

`rimba init` creates a `.rimba/` directory in the repo root with two config files:

- **`settings.toml`** — team-shared configuration (commit this to git)
- **`settings.local.toml`** — personal overrides (gitignored, per-developer)

Local settings override team settings. Fields omitted from the local file inherit from the team file. All fields are optional — sensible defaults are applied automatically.

### Team config (`.rimba/settings.toml`)

```toml
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
#   Gitignore: updated (.rimba.toml → .rimba/settings.local.toml)
```

## Field Reference

| Field | Description | Default |
|-------|-------------|---------|
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
| `RIMBA_DEBUG` | Log git command timing to stderr (set to any value, e.g. `RIMBA_DEBUG=1`). The `--debug` flag on any command has the same effect. |
| `RIMBA_QUIET` | Suppress pre-execution hints (set to any value, e.g. `RIMBA_QUIET=1`) |
| `NO_COLOR` | Disable colored output globally (per [no-color.org](https://no-color.org)) |

## MCP server registration

When `rimba init --agents` or `rimba init -g` is run, rimba registers itself as an MCP server (server name: `rimba`, command: `rimba mcp`) in client config files alongside the agent instruction files. The registration is idempotent — running the command again updates the entry without duplicating it. `--agents --local` updates agent files only and does **not** register MCP.

### User-level (`rimba init -g`)

Patches the following files in your home directory:

| File | Format |
|------|--------|
| `~/.claude/settings.json` | JSON (`mcpServers` object) |
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
| `deps.modules[].install` | `deps.modules["<dir>"]: install command is empty` | Set `install = "<command>"` for the module |
| `open.<name>` (empty key) | `open: shortcut name is empty` | Remove the empty-keyed entry under `[open]` |
| `open.<name>` (path separator) | `open["<name>"]: shortcut name must not contain path separators` | Rename the shortcut to a name without `/` |
