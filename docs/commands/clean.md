---
title: rimba clean
parent: Command
nav_order: 20
---

# rimba clean

Prune stale worktree references, or detect and remove worktrees whose branches have been merged into main or are stale (no recent commits).

Running `rimba clean` without any mode flags prunes stale remote-tracking refs across all configured remotes.

## Synopsis

```sh
rimba clean [--merged | --stale] [flags]
```

## Examples

```sh
rimba clean                              # Prune stale worktree references
rimba clean --dry-run                    # Preview what would be pruned
rimba clean --merged                     # Detect and remove merged worktrees (with confirmation)
rimba clean --merged --force             # Remove merged worktrees without confirmation
rimba clean --merged --dry-run           # Show merged worktrees without removing
rimba clean --stale                      # Detect and remove stale worktrees (with confirmation)
rimba clean --stale --stale-days 7       # Use a 7-day threshold instead of 14
rimba clean --stale --force              # Remove stale worktrees without confirmation
rimba clean --stale --dry-run            # Show stale worktrees without removing
```

## Common workflows

**After a busy sprint: remove all merged branches**
```sh
rimba clean --merged --dry-run   # Preview first
rimba clean --merged --force     # Then remove
```

**Remove dormant worktrees**
```sh
rimba clean --stale --stale-days 7   # More aggressive: 7-day threshold
```

**Automated cleanup via Git hook**
```sh
rimba hook install
# Installs a post-merge hook that runs 'rimba clean --merged --force' on git pull
```

{: .warning }
> `--merged` and `--stale` are mutually exclusive. `--merged` works with or without `rimba init`. Without a config file, it falls back to auto-detecting the default branch. By default, `rimba clean` prunes stale remote-tracking refs across all configured remotes (not just `origin`).

## Flags

| Flag | Description |
|------|-------------|
| `--dry-run` | Show what would be pruned/removed without making changes |
| `--merged` | Detect and remove worktrees whose branches are merged into main |
| `--stale` | Remove worktrees with no recent commits |
| `--stale-days` | Number of days to consider a worktree stale (default: 14, used with `--stale`) |
| `--force` | Skip confirmation prompt when used with `--merged` or `--stale` |

## Related commands

- [rimba hook](hook) · automate `clean --merged --force` via a post-merge hook
- [rimba remove](remove) · remove a single named worktree
- [rimba status](status) · use `--stale-days` to spot stale worktrees
