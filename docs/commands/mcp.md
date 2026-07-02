---
title: rimba mcp
parent: Command
nav_order: 19
---

# rimba mcp

Start a Model Context Protocol (MCP) server that exposes rimba's worktree management as tools for AI coding agents. The server runs over stdio and follows the MCP specification.

{: .note }
> All MCP tools return JSON responses. See `rimba mcp` help output for full parameter details. The easiest way to register this server is `rimba init --agents`, which writes the MCP config alongside the agent instruction files.

## Synopsis

```sh
rimba mcp
```

## Examples

```sh
rimba mcp    # Start MCP server (stdio transport)
```

## Setup

**Claude Code** — add to `.mcp.json` or run:

```sh
claude mcp add rimba rimba mcp
```

**Cursor** — add to `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "rimba": {
      "command": "rimba",
      "args": ["mcp"]
    }
  }
}
```

**Auto-register with agent files:**

```sh
rimba init --agents    # Writes agent files + MCP config simultaneously
```

## MCP Tools

| Tool | Description | Required Parameters |
|------|-------------|---------------------|
| `list` | List all worktrees with branch, path, and status | — |
| `add` | Create a new worktree for a task (supports `service/task` for monorepos) | `task` |
| `remove` | Remove a worktree and optionally delete its branch | `task` |
| `status` | Show worktree dashboard with summary stats and age info | — |
| `sync` | Sync worktree(s) with the main branch via rebase or merge, then push | `task` or `all` |
| `merge` | Merge a worktree branch into main or another worktree | `source` |
| `exec` | Run a shell command across matching worktrees in parallel | `command` |
| `conflict-check` | Detect file overlaps between worktree branches | — |
| `clean` | Clean up stale references, merged branches, or stale worktrees | `mode` (prune, merged, stale) |

## Common workflows

**Register once, use everywhere**
```sh
rimba init --agents
# MCP config written to .mcp.json (Claude Code) and .cursor/mcp.json (Cursor)
# Agent instruction files written alongside
```

**Ask your AI agent to create a worktree**
```
"Create a worktree for the login-refactor task"
# Agent calls rimba MCP tool: add(task="login-refactor")
```

## Related commands

- [rimba init](init) · `--agents` flag registers the MCP server automatically
- [rimba list](list) · equivalent of the MCP `list` tool
