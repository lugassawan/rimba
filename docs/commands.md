---
title: Command
nav_order: 2
has_children: true
---

# Command

> See also: [Configuration]({{ '/configuration' | relative_url }})

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
    <span class="rimba-feature-title">rimba init</span>
    <p>Initialize rimba config, worktree directory, and optional AI agent files</p>
  </a>
</div>

---

## Worktree Lifecycle

<div class="rimba-features">
  <a class="rimba-feature" href="{{ '/commands/add' | relative_url }}">
    <span class="rimba-feature-title">rimba add</span>
    <p>Create a new worktree (supports monorepo scoping, PR checkout, branch promotion)</p>
  </a>
  <a class="rimba-feature" href="{{ '/commands/remove' | relative_url }}">
    <span class="rimba-feature-title">rimba remove</span>
    <p>Remove a worktree and delete the local branch</p>
  </a>
  <a class="rimba-feature" href="{{ '/commands/rename' | relative_url }}">
    <span class="rimba-feature-title">rimba rename</span>
    <p>Rename a worktree's task, branch, and directory</p>
  </a>
  <a class="rimba-feature" href="{{ '/commands/duplicate' | relative_url }}">
    <span class="rimba-feature-title">rimba duplicate</span>
    <p>Create a new worktree from an existing one, inheriting its branch prefix</p>
  </a>
  <a class="rimba-feature" href="{{ '/commands/archive' | relative_url }}">
    <span class="rimba-feature-title">rimba archive</span>
    <p>Archive a worktree (remove directory, keep branch)</p>
  </a>
  <a class="rimba-feature" href="{{ '/commands/restore' | relative_url }}">
    <span class="rimba-feature-title">rimba restore</span>
    <p>Restore an archived worktree from its preserved branch</p>
  </a>
</div>

---

## Inspection

<div class="rimba-features">
  <a class="rimba-feature" href="{{ '/commands/list' | relative_url }}">
    <span class="rimba-feature-title">rimba list</span>
    <p>List all worktrees with task, type, and status</p>
  </a>
  <a class="rimba-feature" href="{{ '/commands/status' | relative_url }}">
    <span class="rimba-feature-title">rimba status</span>
    <p>Show a worktree dashboard with summary stats and age information</p>
  </a>
  <a class="rimba-feature" href="{{ '/commands/log' | relative_url }}">
    <span class="rimba-feature-title">rimba log</span>
    <p>Show the most recent commit from each worktree</p>
  </a>
  <a class="rimba-feature" href="{{ '/commands/open' | relative_url }}">
    <span class="rimba-feature-title">rimba open</span>
    <p>Open a worktree path or run a command inside it</p>
  </a>
</div>

---

## Merging & Syncing

<div class="rimba-features">
  <a class="rimba-feature" href="{{ '/commands/merge' | relative_url }}">
    <span class="rimba-feature-title">rimba merge</span>
    <p>Merge a worktree's branch into main or another worktree</p>
  </a>
  <a class="rimba-feature" href="{{ '/commands/sync' | relative_url }}">
    <span class="rimba-feature-title">rimba sync</span>
    <p>Sync worktree(s) with the main branch via rebase or merge</p>
  </a>
  <a class="rimba-feature" href="{{ '/commands/merge-plan' | relative_url }}">
    <span class="rimba-feature-title">rimba merge-plan</span>
    <p>Analyze file overlaps and recommend an optimal merge order</p>
  </a>
  <a class="rimba-feature" href="{{ '/commands/conflict-check' | relative_url }}">
    <span class="rimba-feature-title">rimba conflict-check</span>
    <p>Scan branches for files modified in multiple worktrees</p>
  </a>
</div>

---

## Bulk Operations

<div class="rimba-features">
  <a class="rimba-feature" href="{{ '/commands/exec' | relative_url }}">
    <span class="rimba-feature-title">rimba exec</span>
    <p>Run a shell command in parallel across matching worktrees</p>
  </a>
</div>

---

## Hooks

<div class="rimba-features">
  <a class="rimba-feature" href="{{ '/commands/hook' | relative_url }}">
    <span class="rimba-feature-title">rimba hook</span>
    <p>Manage Git hooks for worktree workflow automation</p>
  </a>
</div>

---

## Dependencies

<div class="rimba-features">
  <a class="rimba-feature" href="{{ '/commands/deps' | relative_url }}">
    <span class="rimba-feature-title">rimba deps</span>
    <p>Detect, clone, and install worktree dependencies</p>
  </a>
</div>

---

## AI Integration

<div class="rimba-features">
  <a class="rimba-feature" href="{{ '/commands/mcp' | relative_url }}">
    <span class="rimba-feature-title">rimba mcp</span>
    <p>Start an MCP server exposing rimba tools to AI coding agents</p>
  </a>
</div>

---

## Maintenance

<div class="rimba-features">
  <a class="rimba-feature" href="{{ '/commands/clean' | relative_url }}">
    <span class="rimba-feature-title">rimba clean</span>
    <p>Prune stale references or remove merged/stale worktrees</p>
  </a>
  <a class="rimba-feature" href="{{ '/commands/doctor' | relative_url }}">
    <span class="rimba-feature-title">rimba doctor</span>
    <p>Diagnose and remove stale git index.lock files left by killed worktree operations</p>
  </a>
  <a class="rimba-feature" href="{{ '/commands/trust' | relative_url }}">
    <span class="rimba-feature-title">rimba trust</span>
    <p>Review and approve committed shell commands for this repo</p>
  </a>
  <a class="rimba-feature" href="{{ '/commands/update' | relative_url }}">
    <span class="rimba-feature-title">rimba update</span>
    <p>Check for and install the latest rimba release</p>
  </a>
  <a class="rimba-feature" href="{{ '/commands/version' | relative_url }}">
    <span class="rimba-feature-title">rimba version</span>
    <p>Print version, commit, build date, and platform info</p>
  </a>
  <a class="rimba-feature" href="{{ '/commands/completion' | relative_url }}">
    <span class="rimba-feature-title">rimba completion</span>
    <p>Generate shell completion scripts</p>
  </a>
</div>
