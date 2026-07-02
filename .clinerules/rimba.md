# rimba — Git Worktree Manager

rimba manages parallel git worktrees so you can work on multiple tasks simultaneously.
It is optional and detected via `.rimba/settings.toml` in the repo root.

## Prerequisites

Run `rimba version` to check if rimba is installed.
If not found, **ask the user** before installing. Never install automatically.

## Top Commands

1. `rimba add <task>` — create worktree + branch
2. `rimba list` — list all worktrees
3. `rimba status` — health overview
4. `rimba merge <task>` — merge into main and auto-clean up
5. `rimba clean --merged` — remove merged worktrees
6. `rimba sync [task]` — rebase onto main
7. `rimba mcp` — start MCP server for AI tool integration

## MCP Tools

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
