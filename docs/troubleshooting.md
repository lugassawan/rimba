---
title: Troubleshooting
nav_order: 4
has_children: true
---

# Troubleshooting

> See also: [Command]({{ '/commands' | relative_url }}) · [Configuration]({{ '/configuration' | relative_url }})

rimba prints an actionable `To fix:` hint with most failures. Each section below shows the exact
error, why it happens, and the command to resolve it.
{: .fs-6 .fw-300 }

---

<div class="rimba-features">
  <a class="rimba-feature" href="{{ '/troubleshooting/trust-consent' | relative_url }}">
    <h2>Trust &amp; consent gate</h2>
    <p>Committed shell commands require approval before running. Resolve approval errors and CI non-interactive flows.</p>
  </a>
  <a class="rimba-feature" href="{{ '/troubleshooting/selecting-worktrees' | relative_url }}">
    <h2>Selecting worktrees</h2>
    <p>Errors from <code>rimba exec</code> when no target selector or invalid type is provided.</p>
  </a>
  <a class="rimba-feature" href="{{ '/troubleshooting/syncing' | relative_url }}">
    <h2>Syncing</h2>
    <p>Uncommitted-changes and push-rejection errors from <code>rimba sync</code>.</p>
  </a>
  <a class="rimba-feature" href="{{ '/troubleshooting/adding-promoting' | relative_url }}">
    <h2>Adding / promoting</h2>
    <p>Flag-conflict errors when using <code>rimba add</code> in branch or PR mode.</p>
  </a>
  <a class="rimba-feature" href="{{ '/troubleshooting/removing' | relative_url }}">
    <h2>Removing</h2>
    <p>Dirty-state errors that block <code>rimba remove</code> from discarding work.</p>
  </a>
  <a class="rimba-feature" href="{{ '/troubleshooting/init-setup' | relative_url }}">
    <h2>Init &amp; setup</h2>
    <p>Flag errors, permission failures, and worktree-directory creation problems from <code>rimba init</code>.</p>
  </a>
  <a class="rimba-feature" href="{{ '/troubleshooting/configuration-loading' | relative_url }}">
    <h2>Configuration loading</h2>
    <p>Missing config file and TOML syntax errors surfaced on every command invocation.</p>
  </a>
  <a class="rimba-feature" href="{{ '/troubleshooting/debugging-git' | relative_url }}">
    <h2>Debugging git operations</h2>
    <p>Enable <code>--debug</code> or <code>RIMBA_DEBUG=1</code> to trace git commands and timings to stderr.</p>
  </a>
</div>

---

## Quick reference — error message → page

| Error message | Page |
|---|---|
| `committed shell commands require approval` | [Trust & consent gate]({{ '/troubleshooting/trust-consent' | relative_url }}) |
| `committed shell commands are not trusted for this repo` | [Trust & consent gate]({{ '/troubleshooting/trust-consent' | relative_url }}) |
| `provide --all or --type to select worktrees` | [Selecting worktrees]({{ '/troubleshooting/selecting-worktrees' | relative_url }}) |
| `invalid type "<x>"; valid types: ...` | [Selecting worktrees]({{ '/troubleshooting/selecting-worktrees' | relative_url }}) |
| `--concurrency must be >= 0` | [Selecting worktrees]({{ '/troubleshooting/selecting-worktrees' | relative_url }}) |
| `worktree "<task>" has uncommitted changes` | [Syncing]({{ '/troubleshooting/syncing' | relative_url }}) |
| `push failed for <branch>: ...` | [Syncing]({{ '/troubleshooting/syncing' | relative_url }}) |
| `--source is not valid in branch: mode` | [Adding / promoting]({{ '/troubleshooting/adding-promoting' | relative_url }}) |
| `--task requires a pr:<num> argument` | [Adding / promoting]({{ '/troubleshooting/adding-promoting' | relative_url }}) |
| `<git error about dirty state>` (remove) | [Removing]({{ '/troubleshooting/removing' | relative_url }}) |
| `--local requires --agents` | [Init & setup]({{ '/troubleshooting/init-setup' | relative_url }}) |
| `--uninstall requires -g or --agents` | [Init & setup]({{ '/troubleshooting/init-setup' | relative_url }}) |
| `failed to create worktree directory: ...` | [Init & setup]({{ '/troubleshooting/init-setup' | relative_url }}) |
| `agent files: <error>` | [Init & setup]({{ '/troubleshooting/init-setup' | relative_url }}) |
| `config not found: ...` | [Configuration loading]({{ '/troubleshooting/configuration-loading' | relative_url }}) |
| `invalid config <file>: ...` | [Configuration loading]({{ '/troubleshooting/configuration-loading' | relative_url }}) |
