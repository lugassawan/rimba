---
title: Command
nav_order: 2
has_children: true
---

# Command

> See also: [Configuration](configuration.md)

Browse the full list of rimba commands below. Each command has its own page with synopsis, examples, common workflows, flags, and related commands.
{: .fs-6 .fw-300 }

---

## Global Flags

These flags are available on every command:
{: .note }

| Flag | Description |
|------|-------------|
| `--json` | Output in JSON format (supported by `list`, `status`, `deps status`, `conflict-check`, `exec`, `log`) |
| `--no-color` | Disable colored output (also respects `NO_COLOR` env var) |
| `--debug` | Log git commands and timings to stderr (also respects `RIMBA_DEBUG=1`) |
| `--yes` | Approve committed shell commands without prompting (see `rimba trust`; also respects `RIMBA_TRUST_YES=1`) |

---

## Setup

<div class="rimba-features">
  <a class="rimba-feature" href="{{ '/commands/init' | relative_url }}">
    <h2>rimba init</h2>
    <p>Initialize rimba config, worktree directory, and optional AI agent files</p>
  </a>
</div>

---

## Worktree Lifecycle

<div class="rimba-features">
  <a class="rimba-feature" href="{{ '/commands/add' | relative_url }}">
    <h2>rimba add</h2>
    <p>Create a new worktree (supports monorepo scoping, PR checkout, branch promotion)</p>
  </a>
  <a class="rimba-feature" href="{{ '/commands/remove' | relative_url }}">
    <h2>rimba remove</h2>
    <p>Remove a worktree and delete the local branch</p>
  </a>
  <a class="rimba-feature" href="{{ '/commands/rename' | relative_url }}">
    <h2>rimba rename</h2>
    <p>Rename a worktree's task, branch, and directory</p>
  </a>
  <a class="rimba-feature" href="{{ '/commands/duplicate' | relative_url }}">
    <h2>rimba duplicate</h2>
    <p>Create a new worktree from an existing one, inheriting its branch prefix</p>
  </a>
  <a class="rimba-feature" href="{{ '/commands/archive' | relative_url }}">
    <h2>rimba archive</h2>
    <p>Archive a worktree (remove directory, keep branch)</p>
  </a>
  <a class="rimba-feature" href="{{ '/commands/restore' | relative_url }}">
    <h2>rimba restore</h2>
    <p>Restore an archived worktree from its preserved branch</p>
  </a>
</div>

---

## Inspection

<div class="rimba-features">
  <a class="rimba-feature" href="{{ '/commands/list' | relative_url }}">
    <h2>rimba list</h2>
    <p>List all worktrees with task, type, and status</p>
  </a>
  <a class="rimba-feature" href="{{ '/commands/status' | relative_url }}">
    <h2>rimba status</h2>
    <p>Show a worktree dashboard with summary stats and age information</p>
  </a>
  <a class="rimba-feature" href="{{ '/commands/log' | relative_url }}">
    <h2>rimba log</h2>
    <p>Show the most recent commit from each worktree</p>
  </a>
  <a class="rimba-feature" href="{{ '/commands/open' | relative_url }}">
    <h2>rimba open</h2>
    <p>Open a worktree path or run a command inside it</p>
  </a>
</div>

---

## Merging & Syncing

<div class="rimba-features">
  <a class="rimba-feature" href="{{ '/commands/merge' | relative_url }}">
    <h2>rimba merge</h2>
    <p>Merge a worktree's branch into main or another worktree</p>
  </a>
  <a class="rimba-feature" href="{{ '/commands/sync' | relative_url }}">
    <h2>rimba sync</h2>
    <p>Sync worktree(s) with the main branch via rebase or merge</p>
  </a>
  <a class="rimba-feature" href="{{ '/commands/merge-plan' | relative_url }}">
    <h2>rimba merge-plan</h2>
    <p>Analyze file overlaps and recommend an optimal merge order</p>
  </a>
  <a class="rimba-feature" href="{{ '/commands/conflict-check' | relative_url }}">
    <h2>rimba conflict-check</h2>
    <p>Scan branches for files modified in multiple worktrees</p>
  </a>
</div>

---

## Bulk Operations

<div class="rimba-features">
  <a class="rimba-feature" href="{{ '/commands/exec' | relative_url }}">
    <h2>rimba exec</h2>
    <p>Run a shell command in parallel across matching worktrees</p>
  </a>
</div>

---

## Hooks

<div class="rimba-features">
  <a class="rimba-feature" href="{{ '/commands/hook' | relative_url }}">
    <h2>rimba hook</h2>
    <p>Manage Git hooks for worktree workflow automation</p>
  </a>
</div>

---

## Dependencies

<div class="rimba-features">
  <a class="rimba-feature" href="{{ '/commands/deps' | relative_url }}">
    <h2>rimba deps</h2>
    <p>Detect, clone, and install worktree dependencies</p>
  </a>
</div>

---

## AI Integration

<div class="rimba-features">
  <a class="rimba-feature" href="{{ '/commands/mcp' | relative_url }}">
    <h2>rimba mcp</h2>
    <p>Start an MCP server exposing rimba tools to AI coding agents</p>
  </a>
</div>

---

## Maintenance

<div class="rimba-features">
  <a class="rimba-feature" href="{{ '/commands/clean' | relative_url }}">
    <h2>rimba clean</h2>
    <p>Prune stale references or remove merged/stale worktrees</p>
  </a>
  <a class="rimba-feature" href="{{ '/commands/trust' | relative_url }}">
    <h2>rimba trust</h2>
    <p>Review and approve committed shell commands for this repo</p>
  </a>
  <a class="rimba-feature" href="{{ '/commands/update' | relative_url }}">
    <h2>rimba update</h2>
    <p>Check for and install the latest rimba release</p>
  </a>
  <a class="rimba-feature" href="{{ '/commands/version' | relative_url }}">
    <h2>rimba version</h2>
    <p>Print version, commit, build date, and platform info</p>
  </a>
  <a class="rimba-feature" href="{{ '/commands/completion' | relative_url }}">
    <h2>rimba completion</h2>
    <p>Generate shell completion scripts</p>
  </a>
</div>
