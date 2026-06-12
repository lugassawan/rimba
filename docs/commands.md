---
title: Command Reference
nav_order: 2
has_children: true
---

# Command Reference

> See also: [Configuration](configuration.md)

Browse the full list of rimba commands below. Each command has its own page with synopsis, examples, common workflows, flags, and related commands.

---

## Global Flags

These flags are available on every command:

| Flag | Description |
|------|-------------|
| `--json` | Output in JSON format (supported by `list`, `status`, `deps status`, `conflict-check`, `exec`) |
| `--no-color` | Disable colored output (also respects `NO_COLOR` env var) |
| `--debug` | Log git commands and timings to stderr (also respects `RIMBA_DEBUG=1`) |
| `--yes` | Approve committed shell commands without prompting (see `rimba trust`; also respects `RIMBA_TRUST_YES=1`) |

---

## Setup

| Command | Description |
|---------|-------------|
| [rimba init](commands/init) | Initialize rimba config, worktree directory, and optional AI agent files |

---

## Worktree Lifecycle

| Command | Description |
|---------|-------------|
| [rimba add](commands/add) | Create a new worktree (supports monorepo scoping, PR checkout, branch promotion) |
| [rimba remove](commands/remove) | Remove a worktree and delete the local branch |
| [rimba rename](commands/rename) | Rename a worktree's task, branch, and directory |
| [rimba duplicate](commands/duplicate) | Create a new worktree from an existing one, inheriting its branch prefix |
| [rimba archive](commands/archive) | Archive a worktree (remove directory, keep branch) |
| [rimba restore](commands/restore) | Restore an archived worktree from its preserved branch |

---

## Inspection

| Command | Description |
|---------|-------------|
| [rimba list](commands/list) | List all worktrees with task, type, and status |
| [rimba status](commands/status) | Show a worktree dashboard with summary stats and age information |
| [rimba log](commands/log) | Show the most recent commit from each worktree |
| [rimba open](commands/open) | Open a worktree path or run a command inside it |

---

## Merging & Syncing

| Command | Description |
|---------|-------------|
| [rimba merge](commands/merge) | Merge a worktree's branch into main or another worktree |
| [rimba sync](commands/sync) | Sync worktree(s) with the main branch via rebase or merge |
| [rimba merge-plan](commands/merge-plan) | Analyze file overlaps and recommend an optimal merge order |
| [rimba conflict-check](commands/conflict-check) | Scan branches for files modified in multiple worktrees |

---

## Bulk Operations

| Command | Description |
|---------|-------------|
| [rimba exec](commands/exec) | Run a shell command in parallel across matching worktrees |

---

## Hooks

| Command | Description |
|---------|-------------|
| [rimba hook](commands/hook) | Manage Git hooks for worktree workflow automation |

---

## Dependencies

| Command | Description |
|---------|-------------|
| [rimba deps](commands/deps) | Detect, clone, and install worktree dependencies |

---

## AI Integration

| Command | Description |
|---------|-------------|
| [rimba mcp](commands/mcp) | Start an MCP server exposing rimba tools to AI coding agents |

---

## Maintenance

| Command | Description |
|---------|-------------|
| [rimba clean](commands/clean) | Prune stale references or remove merged/stale worktrees |
| [rimba trust](commands/trust) | Review and approve committed shell commands for this repo |
| [rimba update](commands/update) | Check for and install the latest rimba release |
| [rimba version](commands/version) | Print version, commit, build date, and platform info |
| [rimba completion](commands/completion) | Generate shell completion scripts |
