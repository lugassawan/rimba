<!-- BEGIN RIMBA -->
<!-- Managed by rimba — do not edit this block manually -->

## rimba (Git Worktree Manager)

See AGENTS.md at the repo root for full rimba documentation.

### Key Commands

- `rimba add <task>` — create worktree (`rimba add service/task` for monorepos)
- `rimba add pr:<num>` — create worktree from a GitHub PR
- `rimba rename <task> [new-task]` — rename a worktree's task, branch, and directory
- `rimba duplicate <task>` — create a new worktree from an existing one
- `rimba list` (`--full` adds PR/CI columns) / `rimba status` (`--detail` adds disk/velocity) — inspect worktrees (`--service <svc>` to filter)
- `rimba doctor` — diagnose stale git index.lock files
- `rimba merge <task>` — merge into main and auto-clean up
- `rimba clean --merged` — remove merged worktrees
- `rimba archive <task>` / `rimba restore <task>` — remove worktree keeping branch / recreate from an archived branch
- `rimba exec <cmd>` — run command across all worktrees
- `rimba trust` — approve committed shell commands
- `rimba mcp` — start MCP server for AI tool integration

### Config Shape (`.rimba/settings.toml`)

```toml
copy_files = [".env", ".claude"]  # auto-detected on init from gitignored local files
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
| `mcp__rimba__add` | `rimba add <task>` |
| `mcp__rimba__list` | `rimba list` |
| `mcp__rimba__status` | `rimba status` |
| `mcp__rimba__sync` | `rimba sync [task]` |
| `mcp__rimba__merge` | `rimba merge <task>` |
| `mcp__rimba__remove` | `rimba remove <task>` |
| `mcp__rimba__clean` | `rimba clean --merged` |
| `mcp__rimba__exec` | `rimba exec <cmd>` |
| `mcp__rimba__conflict-check` | `rimba conflict-check` |
| `mcp__rimba__rename` | `rimba rename <task> [new-task]` |
| `mcp__rimba__merge-plan` | `rimba merge-plan` |
| `mcp__rimba__log` | `rimba log` |
| `mcp__rimba__archive` | `rimba archive <task>` |
| `mcp__rimba__restore` | `rimba restore <task>` |

<!-- END RIMBA -->
