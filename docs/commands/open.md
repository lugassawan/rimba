---
title: rimba open
parent: Command
nav_order: 11
---

# rimba open

Open a worktree or run a command inside it. When called with just a task name, prints the worktree path to stdout. When given additional arguments, executes that command inside the worktree directory. Supports configurable shortcuts via the `[open]` config section.

## Synopsis

```sh
rimba open <task> [command...] [flags]
```

## Examples

```sh
rimba open my-task              # Print worktree path
cd $(rimba open my-task)        # Navigate to worktree
rimba open my-task --ide        # Run the 'ide' shortcut
rimba open my-task --agent      # Run the 'agent' shortcut
rimba open my-task -w test      # Run a named shortcut
rimba open my-task npm start    # Run an inline command
```

## Common workflows

**Navigate into a worktree**
```sh
cd $(rimba open my-feature)
```

**Open in your IDE (configured shortcut)**
```sh
# In .rimba/settings.toml:
# [open]
# ide = "code ."
rimba open my-feature --ide
```

**Launch an AI agent in the worktree**
```sh
# In .rimba/settings.toml:
# [open]
# agent = "claude --dangerously-skip-permissions"
rimba open my-feature --agent
```

**Run a one-off command without cd**
```sh
rimba open my-feature git log --oneline -5
rimba open my-feature npm run lint
```

{: .warning }
> `--ide`, `--agent`, and `--with` are mutually exclusive with each other and with inline command arguments. Shortcuts are configured in the `[open]` section of `.rimba/settings.toml` — see [Configuration](../configuration.md).

## Flags

| Flag | Description |
|------|-------------|
| `--ide` | Run the `ide` shortcut from `[open]` config |
| `--agent` | Run the `agent` shortcut from `[open]` config |
| `-w`, `--with` | Run any named shortcut from `[open]` config |

## Related commands

- [rimba list](list) · find task names to open
- [rimba exec](exec) · run a command across multiple worktrees at once
