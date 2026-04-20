package agentfile

// Content functions return the template text for each agent instruction file.
// Each function embeds the content directly — no external files.

// agentsBlock returns the rimba block for AGENTS.md (shared file, block-based).
func agentsBlock() string {
	return `<!-- BEGIN RIMBA -->
<!-- Managed by rimba — do not edit this block manually -->

# rimba — Git Worktree Manager

rimba manages parallel git worktrees so you can work on multiple tasks simultaneously.
It is optional and detected via ` + "`" + `.rimba/settings.toml` + "`" + ` in the repo root.

## Prerequisites

Run ` + "`" + `rimba version` + "`" + ` to check if rimba is installed.
If not found, **ask the user** before installing. Never install automatically.

Install command:
` + "```" + `sh
curl -sSfL https://raw.githubusercontent.com/lugassawan/rimba/main/scripts/install.sh | bash
` + "```" + `

## Command Reference

| Concern | Commands |
|---------|----------|
| Create & navigate | ` + "`" + `rimba add <task>` + "`" + ` (or ` + "`" + `rimba add service/task` + "`" + ` for monorepos), ` + "`" + `rimba open <task>` + "`" + ` |
| Inspect | ` + "`" + `rimba list` + "`" + `, ` + "`" + `rimba status` + "`" + `, ` + "`" + `rimba log` + "`" + ` |
| Sync & merge | ` + "`" + `rimba sync [task]` + "`" + `, ` + "`" + `rimba merge <task>` + "`" + ` |
| Clean up | ` + "`" + `rimba clean --merged` + "`" + `, ` + "`" + `rimba archive <task>` + "`" + `, ` + "`" + `rimba remove <task>` + "`" + ` |
| Cross-cutting | ` + "`" + `rimba exec <cmd>` + "`" + `, ` + "`" + `rimba conflict-check` + "`" + `, ` + "`" + `rimba deps status` + "`" + ` |
| AI integration | ` + "`" + `rimba mcp` + "`" + ` (MCP server for AI coding agents) |

## Workflow Recipes

**Create a worktree and start working:**
` + "```" + `sh
rimba add my-feature        # creates worktree + branch
rimba open my-feature       # prints worktree path (use: cd $(rimba open my-feature))
` + "```" + `

**Check health and clean up stale worktrees:**
` + "```" + `sh
rimba status                # overview of all worktrees
rimba clean --merged        # remove worktrees whose branches are merged
` + "```" + `

**Merge and clean up:**
` + "```" + `sh
rimba merge my-feature      # merge into main and auto-clean up
` + "```" + `

## Monorepo (Service-Scoped Worktrees)

In monorepos, prefix the task with a service directory name to create service-scoped branches:

` + "```" + `sh
rimba add auth-api/my-feature           # branch: auth-api/feature/my-feature
rimba add auth-api/my-feature --bugfix  # branch: auth-api/bugfix/my-feature
rimba list --service auth-api           # filter worktrees by service
` + "```" + `

**How it works:** rimba auto-detects services — if the first segment before ` + "`" + `/` + "`" + ` matches a directory in the repo root, it becomes the service scope. No configuration needed.

**Branch pattern:** ` + "`" + `<service>/<prefix>/<task>` + "`" + ` (e.g. ` + "`" + `auth-api/feature/my-feature` + "`" + `)

MCP tools also accept ` + "`" + `service/task` + "`" + ` format in the ` + "`" + `task` + "`" + ` parameter.

## JSON Output

Commands that support ` + "`" + `--json` + "`" + `: list, status, exec, conflict-check, deps status.

Envelope: ` + "`" + `{"version": "...", "command": "...", "data": ...}` + "`" + `
Error: ` + "`" + `{"version": "...", "command": "...", "error": "...", "code": "..."}` + "`" + `

## Best Practices

- Prefer ` + "`" + `rimba archive` + "`" + ` over ` + "`" + `rimba remove` + "`" + ` to preserve branches for later reference
- Use ` + "`" + `--force` + "`" + ` only when you understand the implications (skips dirty checks)
- Never modify ` + "`" + `.rimba/settings.toml` + "`" + ` programmatically without asking the user
- Use ` + "`" + `RIMBA_DEBUG=1 rimba <cmd>` + "`" + ` to log git command timing to stderr when troubleshooting

<!-- END RIMBA -->`
}

// copilotBlock returns the rimba block for .github/copilot-instructions.md (shared file, block-based).
func copilotBlock() string {
	return `<!-- BEGIN RIMBA -->
<!-- Managed by rimba — do not edit this block manually -->

## rimba (Git Worktree Manager)

See AGENTS.md at the repo root for full rimba documentation.

### Key Commands

- ` + "`" + `rimba add <task>` + "`" + ` — create worktree (` + "`" + `rimba add service/task` + "`" + ` for monorepos)
- ` + "`" + `rimba list` + "`" + ` / ` + "`" + `rimba status` + "`" + ` — inspect worktrees (` + "`" + `--service <svc>` + "`" + ` to filter)
- ` + "`" + `rimba merge <task>` + "`" + ` — merge into main and auto-clean up
- ` + "`" + `rimba clean --merged` + "`" + ` — remove merged worktrees
- ` + "`" + `rimba exec <cmd>` + "`" + ` — run command across all worktrees
- ` + "`" + `rimba mcp` + "`" + ` — start MCP server for AI tool integration

### Config Shape (` + "`" + `.rimba/settings.toml` + "`" + `)

` + "```" + `toml
copy_files = [".env", ".env.local", ".envrc", ".tool-versions"]
post_create = []
` + "```" + `

### Coding Conventions

- Commit format: ` + "`" + `[type] Description` + "`" + ` (e.g. ` + "`" + `[feat] Add login page` + "`" + `)
- Run ` + "`" + `make test` + "`" + ` before committing
- Run ` + "`" + `make lint` + "`" + ` to check for issues

<!-- END RIMBA -->`
}

// cursorContent returns the full content for .cursor/rules/rimba.mdc (whole-file, rimba-owned).
func cursorContent() string {
	return `---
description: rimba git worktree manager commands and workflows
globs:
  - "*.go"
  - ".rimba/settings.toml"
alwaysApply: false
---

# rimba — Git Worktree Manager

See AGENTS.md at the repo root for full documentation.

## Top Commands

1. ` + "`" + `rimba add <task>` + "`" + ` — create worktree + branch
2. ` + "`" + `rimba list` + "`" + ` — list all worktrees
3. ` + "`" + `rimba status` + "`" + ` — health overview (dirty, stale, behind)
4. ` + "`" + `rimba open <task>` + "`" + ` — print path or run shortcut (--ide, --agent)
5. ` + "`" + `rimba sync [task]` + "`" + ` — rebase worktree(s) onto main
6. ` + "`" + `rimba merge <task>` + "`" + ` — merge into main and auto-clean up
7. ` + "`" + `rimba remove <task>` + "`" + ` — delete worktree + branch
8. ` + "`" + `rimba archive <task>` + "`" + ` — remove worktree, keep branch
9. ` + "`" + `rimba exec <cmd>` + "`" + ` — run across all worktrees
10. ` + "`" + `rimba clean --merged` + "`" + ` — remove merged worktrees
11. ` + "`" + `rimba mcp` + "`" + ` — start MCP server for AI tool integration

## Workflow Recipes

**New feature:** ` + "`" + `rimba add <task>` + "`" + ` then work in the worktree directory.
**Monorepo feature:** ` + "`" + `rimba add service/task` + "`" + ` — auto-detects service from repo directories.
**Finish feature:** ` + "`" + `rimba merge <task>` + "`" + ` (auto-removes worktree).
**Housekeeping:** ` + "`" + `rimba status` + "`" + ` then ` + "`" + `rimba clean --merged` + "`" + `.

## JSON Output

Use ` + "`" + `--json` + "`" + ` with: list, status, exec, conflict-check, deps status.
Envelope: ` + "`" + `{"version", "command", "data"}` + "`" + ` or ` + "`" + `{"version", "command", "error", "code"}` + "`" + `.

## Best Practices

- Prefer ` + "`" + `archive` + "`" + ` over ` + "`" + `remove` + "`" + ` to keep branches for reference.
- Use ` + "`" + `--force` + "`" + ` only when you understand the implications.
- Never modify ` + "`" + `.rimba/settings.toml` + "`" + ` without asking the user.
`
}

// geminiBlock returns the rimba block for GEMINI.md (shared file, block-based).
func geminiBlock() string {
	return `<!-- BEGIN RIMBA -->
<!-- Managed by rimba — do not edit this block manually -->

# rimba — Git Worktree Manager

rimba manages parallel git worktrees so you can work on multiple tasks simultaneously.
It is optional and detected via ` + "`" + `.rimba/settings.toml` + "`" + ` in the repo root.

## Prerequisites

Run ` + "`" + `rimba version` + "`" + ` to check if rimba is installed.
If not found, **ask the user** before installing. Never install automatically.

## Command Reference

| Concern | Commands |
|---------|----------|
| Create & navigate | ` + "`" + `rimba add <task>` + "`" + `, ` + "`" + `rimba open <task>` + "`" + ` |
| Inspect | ` + "`" + `rimba list` + "`" + `, ` + "`" + `rimba status` + "`" + ` |
| Sync & merge | ` + "`" + `rimba sync [task]` + "`" + `, ` + "`" + `rimba merge <task>` + "`" + ` |
| Clean up | ` + "`" + `rimba clean --merged` + "`" + `, ` + "`" + `rimba archive <task>` + "`" + `, ` + "`" + `rimba remove <task>` + "`" + ` |
| Cross-cutting | ` + "`" + `rimba exec <cmd>` + "`" + `, ` + "`" + `rimba conflict-check` + "`" + ` |
| AI integration | ` + "`" + `rimba mcp` + "`" + ` (MCP server for AI coding agents) |

<!-- END RIMBA -->`
}

// windsurfContent returns the full content for .windsurf/rules/rimba.md (whole-file, project-level).
func windsurfContent() string {
	return `# rimba — Git Worktree Manager

rimba manages parallel git worktrees so you can work on multiple tasks simultaneously.
It is optional and detected via ` + "`" + `.rimba/settings.toml` + "`" + ` in the repo root.

## Prerequisites

Run ` + "`" + `rimba version` + "`" + ` to check if rimba is installed.
If not found, **ask the user** before installing. Never install automatically.

## Top Commands

1. ` + "`" + `rimba add <task>` + "`" + ` — create worktree + branch
2. ` + "`" + `rimba list` + "`" + ` — list all worktrees
3. ` + "`" + `rimba status` + "`" + ` — health overview
4. ` + "`" + `rimba merge <task>` + "`" + ` — merge into main and auto-clean up
5. ` + "`" + `rimba clean --merged` + "`" + ` — remove merged worktrees
6. ` + "`" + `rimba sync [task]` + "`" + ` — rebase onto main
7. ` + "`" + `rimba mcp` + "`" + ` — start MCP server for AI tool integration
`
}

// rooContent returns the full content for .clinerules/rimba.md (whole-file, project-level).
func rooContent() string {
	return `# rimba — Git Worktree Manager

rimba manages parallel git worktrees so you can work on multiple tasks simultaneously.
It is optional and detected via ` + "`" + `.rimba/settings.toml` + "`" + ` in the repo root.

## Prerequisites

Run ` + "`" + `rimba version` + "`" + ` to check if rimba is installed.
If not found, **ask the user** before installing. Never install automatically.

## Top Commands

1. ` + "`" + `rimba add <task>` + "`" + ` — create worktree + branch
2. ` + "`" + `rimba list` + "`" + ` — list all worktrees
3. ` + "`" + `rimba status` + "`" + ` — health overview
4. ` + "`" + `rimba merge <task>` + "`" + ` — merge into main and auto-clean up
5. ` + "`" + `rimba clean --merged` + "`" + ` — remove merged worktrees
6. ` + "`" + `rimba sync [task]` + "`" + ` — rebase onto main
7. ` + "`" + `rimba mcp` + "`" + ` — start MCP server for AI tool integration
`
}

// globalClaudeSkillContent returns the user-level content for ~/.claude/skills/rimba/SKILL.md.
func globalClaudeSkillContent() string {
	return `---
name: rimba
description: Use when user wants to manage git worktrees — creating, listing, syncing, merging, or cleaning up parallel working directories
---

# rimba — Git Worktree Manager

## Prerequisite

Run ` + "`" + `rimba version` + "`" + ` to check if rimba is installed.
If not found, **ask the user** if they want to install it. Never install automatically.

` + "```" + `sh
curl -sSfL https://raw.githubusercontent.com/lugassawan/rimba/main/scripts/install.sh | bash
` + "```" + `

Check for ` + "`" + `.rimba/settings.toml` + "`" + ` in the current repo to confirm rimba is configured for this project.

## Decision Logic

| User wants to... | Run |
|-------------------|-----|
| Start a new task | ` + "`" + `rimba add <task>` + "`" + ` |
| Start a task in a monorepo service | ` + "`" + `rimba add service/task` + "`" + ` |
| See all worktrees | ` + "`" + `rimba list` + "`" + ` |
| Check worktree health | ` + "`" + `rimba status` + "`" + ` |
| Navigate to a worktree | ` + "`" + `cd $(rimba open <task>)` + "`" + ` |
| Update from source branch | ` + "`" + `rimba sync <task>` + "`" + ` |
| Finish a feature | ` + "`" + `rimba merge <task>` + "`" + ` |
| Clean up merged work | ` + "`" + `rimba clean --merged` + "`" + ` |
| Use MCP server | ` + "`" + `rimba mcp` + "`" + ` |
`
}

// globalCursorContent returns the user-level content for ~/.cursor/rules/rimba.mdc.
func globalCursorContent() string {
	return `---
description: rimba git worktree manager commands and workflows
alwaysApply: true
---

# rimba — Git Worktree Manager

Check for ` + "`" + `.rimba/settings.toml` + "`" + ` in the current repo to confirm rimba is configured.

## Top Commands

1. ` + "`" + `rimba add <task>` + "`" + ` — create worktree + branch
2. ` + "`" + `rimba list` + "`" + ` — list all worktrees
3. ` + "`" + `rimba status` + "`" + ` — health overview
4. ` + "`" + `rimba merge <task>` + "`" + ` — merge into main and auto-clean up
5. ` + "`" + `rimba clean --merged` + "`" + ` — remove merged worktrees
6. ` + "`" + `rimba sync [task]` + "`" + ` — rebase onto main
7. ` + "`" + `rimba mcp` + "`" + ` — start MCP server for AI tool integration
`
}

// globalCopilotBlock returns the user-level rimba block for ~/.github/copilot-instructions.md.
func globalCopilotBlock() string {
	return `<!-- BEGIN RIMBA -->
<!-- Managed by rimba — do not edit this block manually -->

## rimba (Git Worktree Manager)

rimba manages parallel git worktrees. Check for ` + "`" + `.rimba/settings.toml` + "`" + ` to confirm it is configured in the current project.

### Key Commands

- ` + "`" + `rimba add <task>` + "`" + ` — create worktree + branch
- ` + "`" + `rimba list` + "`" + ` / ` + "`" + `rimba status` + "`" + ` — inspect worktrees
- ` + "`" + `rimba merge <task>` + "`" + ` — merge into main and auto-clean up
- ` + "`" + `rimba clean --merged` + "`" + ` — remove merged worktrees
- ` + "`" + `rimba mcp` + "`" + ` — start MCP server for AI tool integration

<!-- END RIMBA -->`
}

// globalCodexBlock returns the user-level rimba block for ~/.codex/AGENTS.md.
func globalCodexBlock() string {
	return `<!-- BEGIN RIMBA -->
<!-- Managed by rimba — do not edit this block manually -->

# rimba — Git Worktree Manager

rimba manages parallel git worktrees so you can work on multiple tasks simultaneously.
Check for ` + "`" + `.rimba/settings.toml` + "`" + ` in the current repo to confirm it is configured.

Run ` + "`" + `rimba version` + "`" + ` to check if rimba is installed.
If not found, **ask the user** before installing. Never install automatically.

## Command Reference

| Concern | Commands |
|---------|----------|
| Create & navigate | ` + "`" + `rimba add <task>` + "`" + `, ` + "`" + `rimba open <task>` + "`" + ` |
| Inspect | ` + "`" + `rimba list` + "`" + `, ` + "`" + `rimba status` + "`" + ` |
| Sync & merge | ` + "`" + `rimba sync [task]` + "`" + `, ` + "`" + `rimba merge <task>` + "`" + ` |
| Clean up | ` + "`" + `rimba clean --merged` + "`" + `, ` + "`" + `rimba archive <task>` + "`" + ` |
| AI integration | ` + "`" + `rimba mcp` + "`" + ` (MCP server for AI coding agents) |

<!-- END RIMBA -->`
}

// globalGeminiBlock returns the user-level rimba block for ~/.gemini/GEMINI.md.
func globalGeminiBlock() string {
	return `<!-- BEGIN RIMBA -->
<!-- Managed by rimba — do not edit this block manually -->

# rimba — Git Worktree Manager

rimba manages parallel git worktrees so you can work on multiple tasks simultaneously.
Check for ` + "`" + `.rimba/settings.toml` + "`" + ` in the current repo to confirm it is configured.

Run ` + "`" + `rimba version` + "`" + ` to check if rimba is installed.
If not found, **ask the user** before installing. Never install automatically.

## Command Reference

| Concern | Commands |
|---------|----------|
| Create & navigate | ` + "`" + `rimba add <task>` + "`" + `, ` + "`" + `rimba open <task>` + "`" + ` |
| Inspect | ` + "`" + `rimba list` + "`" + `, ` + "`" + `rimba status` + "`" + ` |
| Sync & merge | ` + "`" + `rimba sync [task]` + "`" + `, ` + "`" + `rimba merge <task>` + "`" + ` |
| Clean up | ` + "`" + `rimba clean --merged` + "`" + `, ` + "`" + `rimba archive <task>` + "`" + ` |
| AI integration | ` + "`" + `rimba mcp` + "`" + ` (MCP server for AI coding agents) |

<!-- END RIMBA -->`
}

// globalWindsurfBlock returns the user-level rimba block for ~/.codeium/windsurf/memories/global_rules.md.
func globalWindsurfBlock() string {
	return `<!-- BEGIN RIMBA -->
<!-- Managed by rimba — do not edit this block manually -->

## rimba (Git Worktree Manager)

rimba manages parallel git worktrees. Check for ` + "`" + `.rimba/settings.toml` + "`" + ` to confirm it is configured in the current project.

### Key Commands

- ` + "`" + `rimba add <task>` + "`" + ` — create worktree + branch
- ` + "`" + `rimba list` + "`" + ` / ` + "`" + `rimba status` + "`" + ` — inspect worktrees
- ` + "`" + `rimba merge <task>` + "`" + ` — merge into main and auto-clean up
- ` + "`" + `rimba clean --merged` + "`" + ` — remove merged worktrees
- ` + "`" + `rimba mcp` + "`" + ` — start MCP server for AI tool integration

<!-- END RIMBA -->`
}

// globalRooContent returns the user-level content for ~/.roo/rules/rimba.md.
func globalRooContent() string {
	return `# rimba — Git Worktree Manager

rimba manages parallel git worktrees so you can work on multiple tasks simultaneously.
Check for ` + "`" + `.rimba/settings.toml` + "`" + ` in the current repo to confirm it is configured.

Run ` + "`" + `rimba version` + "`" + ` to check if rimba is installed.
If not found, ask the user before installing. Never install automatically.

## Top Commands

1. ` + "`" + `rimba add <task>` + "`" + ` — create worktree + branch
2. ` + "`" + `rimba list` + "`" + ` — list all worktrees
3. ` + "`" + `rimba status` + "`" + ` — health overview
4. ` + "`" + `rimba merge <task>` + "`" + ` — merge into main and auto-clean up
5. ` + "`" + `rimba clean --merged` + "`" + ` — remove merged worktrees
6. ` + "`" + `rimba sync [task]` + "`" + ` — rebase onto main
7. ` + "`" + `rimba mcp` + "`" + ` — start MCP server for AI tool integration
`
}

// claudeSkillContent returns the full content for .claude/skills/rimba/SKILL.md (whole-file, rimba-owned).
func claudeSkillContent() string {
	return `---
name: rimba
description: Use when user wants to manage git worktrees — creating, listing, syncing, merging, or cleaning up parallel working directories
---

# rimba — Git Worktree Manager

## Prerequisite

Run ` + "`" + `rimba version` + "`" + ` to check if rimba is installed.
If not found, **ask the user** if they want to install it. Never install automatically.

` + "```" + `sh
curl -sSfL https://raw.githubusercontent.com/lugassawan/rimba/main/scripts/install.sh | bash
` + "```" + `

## Decision Logic

| User wants to... | Run |
|-------------------|-----|
| Start a new task | ` + "`" + `rimba add <task>` + "`" + ` |
| Start a task in a monorepo service | ` + "`" + `rimba add service/task` + "`" + ` (auto-detects service from repo dirs) |
| See all worktrees | ` + "`" + `rimba list` + "`" + ` or ` + "`" + `rimba list --json` + "`" + ` |
| Filter by service (monorepo) | ` + "`" + `rimba list --service <svc>` + "`" + ` |
| Check worktree health | ` + "`" + `rimba status` + "`" + ` |
| Navigate to a worktree | ` + "`" + `cd $(rimba open <task>)` + "`" + ` |
| Update from source branch | ` + "`" + `rimba sync <task>` + "`" + ` or ` + "`" + `rimba sync --all` + "`" + ` |
| Finish a feature | ` + "`" + `rimba merge <task>` + "`" + ` (auto-removes worktree) |
| Clean up merged work | ` + "`" + `rimba clean --merged` + "`" + ` |
| Pause a task | ` + "`" + `rimba archive <task>` + "`" + ` (keeps branch) |
| Run across worktrees | ` + "`" + `rimba exec "<cmd>"` + "`" + ` |
| Check for conflicts | ` + "`" + `rimba conflict-check` + "`" + ` |
| Check dependencies | ` + "`" + `rimba deps status` + "`" + ` |
| Use MCP server | ` + "`" + `rimba mcp` + "`" + ` (stdio transport for AI agents) |

## JSON Output

Commands supporting ` + "`" + `--json` + "`" + `: ` + "`" + `list` + "`" + `, ` + "`" + `status` + "`" + `, ` + "`" + `exec` + "`" + `, ` + "`" + `conflict-check` + "`" + `, ` + "`" + `deps status` + "`" + `.

**Envelope:** ` + "`" + `{"version": "<semver>", "command": "<name>", "data": <payload>}` + "`" + `
**Error:** ` + "`" + `{"version": "<semver>", "command": "<name>", "error": "<msg>", "code": "<CODE>"}` + "`" + `

### Data Shapes

**list:** ` + "`" + `[{task, type, service?, branch, path, is_current, status: {dirty, ahead, behind}}]` + "`" + `
**status:** ` + "`" + `{summary: {total, dirty, stale, behind}, worktrees: [...], stale_days}` + "`" + `
**exec:** ` + "`" + `{command, results: [{task, branch, path, exit_code, stdout, stderr}], success}` + "`" + `
**conflict-check:** ` + "`" + `{overlaps: [{file, branches, severity}], dry_merges?, total_files, total_branches}` + "`" + `
**deps status:** ` + "`" + `[{branch, path, modules: [...], error?}]` + "`" + `

## Error Handling

| Error | Cause | Fix |
|-------|-------|-----|
| "not a git repository" | Not inside a git repo | ` + "`" + `cd` + "`" + ` into a git repo |
| "config not found" | rimba not initialized | Run ` + "`" + `rimba init` + "`" + ` |
| "branch already exists" | Task name in use | Pick a different task name |
| "worktree has uncommitted changes" | Dirty worktree | Commit or stash changes, or use ` + "`" + `--force` + "`" + ` |

> **Tip:** Use ` + "`" + `RIMBA_DEBUG=1 rimba <cmd>` + "`" + ` to log git command timing to stderr when troubleshooting performance issues.

## Best Practices

- Prefer ` + "`" + `rimba archive` + "`" + ` over ` + "`" + `rimba remove` + "`" + ` to preserve branches
- Use ` + "`" + `--force` + "`" + ` only when you understand the implications
- Never modify ` + "`" + `.rimba/settings.toml` + "`" + ` without asking the user
- Always check ` + "`" + `rimba status` + "`" + ` before bulk operations
`
}
