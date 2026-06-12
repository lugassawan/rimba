---
title: rimba log
parent: Command Reference
nav_order: 10
---

# rimba log

Show the most recent commit from each worktree, sorted from newest to oldest. Useful for a quick snapshot of where each parallel branch stands.

## Synopsis

```sh
rimba log [flags]
```

## Examples

```sh
rimba log
rimba log --limit 5             # Show only the 5 most recent entries
rimba log --since 7d            # Show entries from the last 7 days
```

## Common workflows

**Morning standup: what did each branch last do?**
```sh
rimba log
```
```
Recent commits across 3 worktree(s):

TASK          TYPE     AGE    COMMIT
  auth-flow   feature  5h     Add OAuth2 callback handler
  payments    feature  3d     Implement Stripe webhook endpoint
  ui-cleanup  chore    21d    Remove deprecated CSS classes
```

**See only recent activity**
```sh
rimba log --since 2d    # Only worktrees with commits in the last 2 days
```

**Cap output for a quick glance**
```sh
rimba log --limit 3
```

## Flags

| Flag | Description |
|------|-------------|
| `--limit` | Maximum number of entries to show (default: 0 = all) |
| `--since` | Show entries since duration (e.g. `7d`, `2w`, `3h`) |
| `--json` | Output as JSON |

## Related commands

- [rimba list](list) · full worktree listing with status
- [rimba status](status) · dashboard with stats and age
