---
title: Home
layout: home
nav_order: 1
---

# rimba

**rimba** automates the full git worktree lifecycle — branch naming, file copying, dependency
sharing, post-create hooks, and cleanup — so you can develop across multiple branches
simultaneously with zero friction.

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

---

[rimba on GitHub](https://github.com/lugassawan/rimba){: .btn .btn-primary }
