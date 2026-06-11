<!-- BEGIN RIMBA -->
<!-- Managed by rimba — do not edit this block manually -->

# rimba — Git Worktree Manager

rimba manages parallel git worktrees so you can work on multiple tasks simultaneously.
It is optional and detected via `.rimba/settings.toml` in the repo root.

## Prerequisites

Run `rimba version` to check if rimba is installed.
If not found, **ask the user** before installing. Never install automatically.

## Command Reference

| Concern | Commands |
|---------|----------|
| Create & navigate | `rimba add <task>`, `rimba add pr:<num>` (from a GitHub PR), `rimba open <task>` |
| Inspect | `rimba list` (`--full` adds PR/CI columns), `rimba status` (`--detail` adds disk/velocity) |
| Sync & merge | `rimba sync [task]`, `rimba merge <task>` |
| Clean up | `rimba clean --merged`, `rimba archive <task>`, `rimba remove <task>` |
| Cross-cutting | `rimba exec <cmd>`, `rimba conflict-check` |
| AI integration | `rimba mcp` (MCP server for AI coding agents) |

<!-- END RIMBA -->
