---
name: rimba
description: Use when user wants to manage git worktrees — creating, listing, syncing, merging, or cleaning up parallel working directories
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
| See all worktrees | `rimba list` or `rimba list --json` |
| Check worktree health | `rimba status` |
| Navigate to a worktree | `cd $(rimba open <task>)` |
| Update from source branch | `rimba sync <task>` or `rimba sync` (all) |
| Finish a feature | `rimba merge <task>` then `rimba remove <task>` |
| Clean up merged work | `rimba clean --merged` |
| Pause a task | `rimba archive <task>` (keeps branch) |
| Run across worktrees | `rimba exec "<cmd>"` |
| Check for conflicts | `rimba conflict-check` |
| Check dependencies | `rimba deps status` |

## JSON Output

Commands supporting `--json`: `list`, `status`, `exec`, `conflict-check`, `deps status`.

**Envelope:** `{"version": "<semver>", "command": "<name>", "data": <payload>}`
**Error:** `{"version": "<semver>", "command": "<name>", "error": "<msg>", "code": "<CODE>"}`

### Data Shapes

**list:** `[{task, type, branch, path, is_current, status: {dirty, ahead, behind}}]`
**status:** `{summary: {total, dirty, stale, behind}, worktrees: [...], stale_days}`
**exec:** `{command, results: [{task, branch, path, exit_code, stdout, stderr}], success}`
**conflict-check:** `{overlaps: [{file, branches, severity}], dry_merges?, total_files, total_branches}`
**deps status:** `[{branch, path, modules: [...], error?}]`

## Error Handling

| Error | Cause | Fix |
|-------|-------|-----|
| "not a git repository" | Not inside a git repo | `cd` into a git repo |
| ".rimba.toml not found" | rimba not initialized | Run `rimba init` |
| "worktree_dir must not be empty" | Bad config | Check `.rimba.toml` |
| "branch already exists" | Task name in use | Pick a different task name |
| "worktree has uncommitted changes" | Dirty worktree | Commit or stash changes, or use `--force` |

## Best Practices

- Prefer `rimba archive` over `rimba remove` to preserve branches
- Use `--force` only when you understand the implications
- Never modify `.rimba.toml` without asking the user
- Always check `rimba status` before bulk operations
