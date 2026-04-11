<!-- BEGIN RIMBA -->
<!-- Managed by rimba — do not edit this block manually -->

# rimba — Git Worktree Manager

rimba manages parallel git worktrees so you can work on multiple tasks simultaneously.
It is optional and detected via `.rimba/settings.toml` in the repo root.

## Prerequisites

Run `rimba version` to check if rimba is installed.
If not found, **ask the user** before installing. Never install automatically.

Install command:
```sh
curl -sSfL https://raw.githubusercontent.com/lugassawan/rimba/main/scripts/install.sh | bash
```

## Command Reference

| Concern | Commands |
|---------|----------|
| Create & navigate | `rimba add <task>` (or `rimba add service/task` for monorepos), `rimba open <task>` |
| Inspect | `rimba list`, `rimba status`, `rimba log` |
| Sync & merge | `rimba sync [task]`, `rimba merge <task>` |
| Clean up | `rimba clean --merged`, `rimba archive <task>`, `rimba remove <task>` |
| Cross-cutting | `rimba exec <cmd>`, `rimba conflict-check`, `rimba deps status` |
| AI integration | `rimba mcp` (MCP server for AI coding agents) |

## Workflow Recipes

**Create a worktree and start working:**
```sh
rimba add my-feature        # creates worktree + branch
rimba open my-feature       # prints worktree path (use: cd $(rimba open my-feature))
```

**Check health and clean up stale worktrees:**
```sh
rimba status                # overview of all worktrees
rimba clean --merged        # remove worktrees whose branches are merged
```

**Merge and clean up:**
```sh
rimba merge my-feature      # merge into main and auto-clean up
```

## Monorepo (Service-Scoped Worktrees)

In monorepos, prefix the task with a service directory name to create service-scoped branches:

```sh
rimba add auth-api/my-feature           # branch: auth-api/feature/my-feature
rimba add auth-api/my-feature --bugfix  # branch: auth-api/bugfix/my-feature
rimba list --service auth-api           # filter worktrees by service
```

**How it works:** rimba auto-detects services — if the first segment before `/` matches a directory in the repo root, it becomes the service scope. No configuration needed.

**Branch pattern:** `<service>/<prefix>/<task>` (e.g. `auth-api/feature/my-feature`)

MCP tools also accept `service/task` format in the `task` parameter.

## JSON Output

Commands that support `--json`: list, status, exec, conflict-check, deps status.

Envelope: `{"version": "...", "command": "...", "data": ...}`
Error: `{"version": "...", "command": "...", "error": "...", "code": "..."}`

## Best Practices

- Prefer `rimba archive` over `rimba remove` to preserve branches for later reference
- Use `--force` only when you understand the implications (skips dirty checks)
- Never modify `.rimba/settings.toml` programmatically without asking the user
- Use `RIMBA_DEBUG=1 rimba <cmd>` to log git command timing to stderr when troubleshooting

<!-- END RIMBA -->
