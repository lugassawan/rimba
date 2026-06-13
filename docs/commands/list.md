---
title: rimba list
parent: Command
nav_order: 8
---

# rimba list

List all worktrees with task, type, and status. The current worktree is marked with `*`. Use `--full` to show branch, path, and (when `gh` is installed and authenticated) PR number and CI rollup.

## Synopsis

```sh
rimba list [flags]
```

## Examples

```sh
rimba list
rimba list --full               # Show all columns (branch, path, PR, CI)
rimba list --type bugfix        # Show only bugfix worktrees
rimba list --dirty              # Show only dirty worktrees
rimba list --behind             # Show only worktrees behind upstream
rimba list --archived           # Show archived branches (not in any active worktree)
rimba list --service auth-api   # Show only worktrees for a service (monorepo)
rimba list --json               # Output as JSON
```

## Common workflows

**Quick overview**
```sh
rimba list
```
```
TASK            TYPE     STATUS
* auth-flow     feature  [dirty]
  fix-login     bugfix   ↑2 ↓1
  ui-cleanup    chore    ✓
```

**Full view with PR and CI status**
```sh
rimba list --full
```
```
TASK            TYPE     BRANCH              PATH               STATUS    PR     CI
* auth-flow     feature  feature/auth-flow   feature-auth-flow  [dirty]   #142   ✓
  fix-login     bugfix   bugfix/fix-login    bugfix-fix-login   ↑2 ↓1     #138   ●
  ui-cleanup    chore    chore/ui-cleanup    chore-ui-cleanup   ✓         –      –
```

**Find worktrees that need attention**
```sh
rimba list --dirty     # Uncommitted changes
rimba list --behind    # Behind upstream — need sync
```

**See what can be restored**
```sh
rimba list --archived
```

{: .note }
> **PR/CI columns:** Require `gh` installed and authenticated; otherwise a yellow warning is printed and those cells render as `–`. CI symbols: ✓ success · ● pending · ✗ failure · – unknown.

{: .warning }
> `--archived` is mutually exclusive with `--type`, `--dirty`, `--behind`, and `--full`.

## Flags

| Flag | Description |
|------|-------------|
| `--full` | Show all columns including branch, path, and PR/CI (when `gh` is available) |
| `--type` | Filter by prefix type (e.g. `feature`, `bugfix`, `hotfix`, `docs`, `test`, `chore`) |
| `--dirty` | Show only worktrees with uncommitted changes |
| `--behind` | Show only worktrees behind their upstream branch |
| `--archived` | Show archived branches not in any active worktree (mutually exclusive with other filters) |
| `--service` | Filter by service name (monorepo) |

## Related commands

- [rimba status](status) · dashboard view with summary stats and age
- [rimba log](log) · most recent commit per worktree
- [rimba archive](archive) · archive a worktree (shows in `--archived`)
- [rimba clean](clean) · remove merged or stale worktrees in bulk
