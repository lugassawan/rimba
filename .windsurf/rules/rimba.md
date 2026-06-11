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
