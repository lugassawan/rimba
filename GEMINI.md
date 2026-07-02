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

<!-- END RIMBA -->
