<!-- BEGIN RIMBA -->
<!-- Managed by rimba — do not edit this block manually -->

## rimba (Git Worktree Manager)

See AGENTS.md at the repo root for full rimba documentation.

### Key Commands

- `rimba add <task>` — create worktree (`rimba add service/task` for monorepos)
- `rimba add pr:<num>` — create worktree from a GitHub PR
- `rimba list` (`--full` adds PR/CI columns) / `rimba status` (`--detail` adds disk/velocity) — inspect worktrees (`--service <svc>` to filter)
- `rimba merge <task>` — merge into main and auto-clean up
- `rimba clean --merged` — remove merged worktrees
- `rimba exec <cmd>` — run command across all worktrees
- `rimba mcp` — start MCP server for AI tool integration

### Config Shape (`.rimba/settings.toml`)

```toml
copy_files = [".env", ".env.local", ".envrc", ".tool-versions"]
post_create = []
```

### Coding Conventions

- Commit format: `[type] Description` (e.g. `[feat] Add login page`)
- Run `make test` before committing
- Run `make lint` to check for issues

### MCP Tools

When running inside an MCP-connected agent, prefer the native `mcp__rimba__*` tools over
shelling out to the `rimba` CLI — they skip a subprocess round-trip. Fall back to the CLI
when no MCP connection is available.

| MCP tool | CLI equivalent |
|----------|----------------|
| `mcp__rimba__add`            | `rimba add <task>`     |
| `mcp__rimba__list`           | `rimba list`           |
| `mcp__rimba__status`         | `rimba status`         |
| `mcp__rimba__sync`           | `rimba sync [task]`    |
| `mcp__rimba__merge`          | `rimba merge <task>`   |
| `mcp__rimba__remove`         | `rimba remove <task>`  |
| `mcp__rimba__clean`          | `rimba clean --merged` |
| `mcp__rimba__exec`           | `rimba exec <cmd>`     |
| `mcp__rimba__conflict-check` | `rimba conflict-check` |

<!-- END RIMBA -->
