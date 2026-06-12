---
title: Home
layout: home
nav_order: 1
---

<div class="rimba-hero">
  <img class="rimba-hero-logo" src="{{ '/assets/rimba-512.svg' | relative_url }}" width="128" height="128" alt="rimba logo">
  <h1 class="fs-9">rimba</h1>
  <p class="fs-6 fw-300">Automates the full git worktree lifecycle — branch naming, file copying, dependency sharing, post-create hooks, and cleanup — so you can develop across multiple branches simultaneously with zero friction.</p>
  <div class="rimba-hero-actions">
    <a href="{{ '/commands' | relative_url }}" class="btn btn-primary">Get started</a>
    <a href="https://github.com/lugassawan/rimba" class="btn">View on GitHub</a>
  </div>
</div>

<div class="rimba-features">
  <div class="rimba-feature">
    <h3>Worktree lifecycle</h3>
    <p>Create, list, and remove git worktrees with a single command. Each task gets its own isolated branch and directory.</p>
  </div>
  <div class="rimba-feature">
    <h3>Smart branch naming</h3>
    <p>Typed prefixes (<code>feature/</code>, <code>bugfix/</code>, <code>docs/</code>, …) are applied automatically based on flags — no manual branch naming.</p>
  </div>
  <div class="rimba-feature">
    <h3>File &amp; dependency sharing</h3>
    <p>Declare files and vendor directories to copy or clone into every new worktree, so dependencies are ready from the first second.</p>
  </div>
  <div class="rimba-feature">
    <h3>Post-create hooks</h3>
    <p>Run arbitrary shell commands after each worktree is created — install tools, set env vars, open your editor, anything.</p>
  </div>
  <div class="rimba-feature">
    <h3>Sync, merge &amp; cleanup</h3>
    <p>Rebase from the default branch, merge finished work, and delete the worktree and branch in one step with <code>rimba merge</code>.</p>
  </div>
  <div class="rimba-feature">
    <h3>MCP integration</h3>
    <p>Expose rimba operations as MCP tools so AI coding agents can create and manage worktrees on your behalf.</p>
  </div>
</div>

## Quick Start

```sh
# Install (Linux/macOS)
curl -sSfL https://raw.githubusercontent.com/lugassawan/rimba/main/scripts/install.sh | bash

# Initialize a repo
rimba init

# Create a worktree for a task
rimba add my-feature --feature

# List all worktrees
rimba status

# Merge and clean up when done
rimba merge
```

## Reference

- [Command Reference](commands.md) — all commands, flags, and examples
- [Configuration](configuration.md) — `.rimba/settings.toml` options
- [Troubleshooting](troubleshooting.md) — error messages, hints, and fixes
