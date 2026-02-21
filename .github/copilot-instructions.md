<!-- BEGIN RIMBA -->
<!-- Managed by rimba — do not edit this block manually -->

## rimba (Git Worktree Manager)

See AGENTS.md at the repo root for full rimba documentation.

### Key Commands

- `rimba add <task>` — create worktree
- `rimba list` / `rimba status` — inspect worktrees
- `rimba merge <task>` — merge back to source branch
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

<!-- END RIMBA -->
