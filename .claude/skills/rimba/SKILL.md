---
name: rimba
description: Use when the user wants to create, list, sync, merge, remove, or clean up git worktrees, or before starting new feature/bugfix work that should be isolated in its own branch and directory — covers parallel development across multiple tasks without branch-switching
---

# rimba — Git Worktree Manager

## Prerequisite

Run `rimba version` to check if rimba is installed.
If not found, **ask the user** if they want to install it. Never install automatically.

```sh
curl -sSfL https://raw.githubusercontent.com/lugassawan/rimba/main/scripts/install.sh | bash
```

## Decision Logic

| User wants to... | Run |
|-------------------|-----|
| Start a new task | `rimba add <task>` |
| Start a task in a monorepo service | `rimba add service/task` (auto-detects service from repo dirs) |
| Rename a task | `rimba rename <task> [new-task]` |
| Duplicate a worktree | `rimba duplicate <task>` |
| See all worktrees | `rimba list` or `rimba list --json` |
| Filter by service (monorepo) | `rimba list --service <svc>` |
| Check worktree health | `rimba status` |
| Diagnose stale worktree locks | `rimba doctor` |
| Navigate to a worktree | `cd $(rimba open <task>)` |
| Update from source branch | `rimba sync <task>` or `rimba sync --all` |
| Finish a feature | `rimba merge <task>` (auto-removes worktree) |
| Clean up merged work | `rimba clean --merged` |
| Pause a task | `rimba archive <task>` (keeps branch) |
| Resume a paused task | `rimba restore <task>` |
| Run across worktrees | `rimba exec "<cmd>"` |
| Check for conflicts | `rimba conflict-check` |
| Check dependencies | `rimba deps status` |
| Approve committed shell commands | `rimba trust` |
| Use MCP server | `rimba mcp` (stdio transport for AI agents) |

## JSON Output

Commands supporting `--json`: `list`, `status`, `exec`, `conflict-check`, `deps status`, `add`, `merge`, `remove`, `rename`, `sync`, `clean`, `log`.

**Envelope:** `{"version": "<semver>", "command": "<name>", "data": <payload>}`
**Error:** `{"version": "<semver>", "command": "<name>", "error": "<msg>", "code": "<CODE>"}`

### Data Shapes

**list:** `[{task, type, service?, branch, path, is_current, status: {dirty, ahead, behind}}]`
**status:** `{summary: {total, dirty, stale, behind}, worktrees: [...], stale_days}`
**exec:** `{command, results: [{task, branch, path, exit_code, stdout, stderr}], success}`
**conflict-check:** `{overlaps: [{file, branches, severity}], dry_merges?, total_files, total_branches}`
**deps status:** `[{branch, path, modules: [...], error?}]`

## Error Handling

| Error | Cause | Fix |
|-------|-------|-----|
| "not a git repository" | Not inside a git repo | `cd` into a git repo |
| "config not found" | rimba not initialized | Run `rimba init` |
| "branch already exists" | Task name in use | Pick a different task name |
| "worktree has uncommitted changes" | Dirty worktree | Commit or stash changes, or use `--force` |

> **Tip:** Use `RIMBA_DEBUG=1 rimba <cmd>` to log git command timing to stderr when troubleshooting performance issues.

## Best Practices

- Prefer `rimba archive` over `rimba remove` to preserve branches
- Use `--force` only when you understand the implications
- Never modify `.rimba/settings.toml` without asking the user
- Always check `rimba status` before bulk operations

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
| `mcp__rimba__rename` | `rimba rename <task> [new-task]` |
| `mcp__rimba__merge-plan` | `rimba merge-plan` |
| `mcp__rimba__log` | `rimba log` |
| `mcp__rimba__archive` | `rimba archive <task>` |
| `mcp__rimba__restore` | `rimba restore <task>` |
