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
| Remove a worktree | `rimba remove <task>` |
| Clean up merged work | `rimba clean --merged` |
| Pause a task | `rimba archive <task>` (keeps branch) |
| Resume a paused task | `rimba restore <task>` |
| Run across worktrees | `rimba exec "<cmd>"` |
| Check for conflicts | `rimba conflict-check` |
| Check dependencies | `rimba deps status` |
| Approve committed shell commands | `rimba trust` |

rimba ships no MCP integration for Pi (Pi's own philosophy is CLI-first: "No MCP. Build CLI tools
with READMEs"); use the commands above directly.

## JSON Output

Commands supporting `--json`: `list`, `status`, `exec`, `conflict-check`, `deps status`, `add`, `merge`, `remove`, `rename`, `sync`, `clean`, `log`.

**Envelope:** `{"version": "<semver>", "command": "<name>", "data": <payload>}`
**Error:** `{"version": "<semver>", "command": "<name>", "error": "<msg>", "code": "<CODE>"}`

## Best Practices

- Prefer `rimba archive` over `rimba remove` to preserve branches
- Use `--force` only when you understand the implications
- Never modify `.rimba/settings.toml` without asking the user
- Always check `rimba status` before bulk operations

## Core Commands

| Command |
|---------|
| `rimba add <task>` |
| `rimba list` |
| `rimba status` |
| `rimba sync [task]` |
| `rimba merge <task>` |
| `rimba remove <task>` |
| `rimba clean --merged` |
| `rimba exec <cmd>` |
| `rimba conflict-check` |
| `rimba rename <task> [new-task]` |
| `rimba merge-plan` |
| `rimba log` |
| `rimba archive <task>` |
| `rimba restore <task>` |
