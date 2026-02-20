# Command Reference

> Back to [README](../README.md) | See also: [Configuration](configuration.md)

---

## Global Flags

These flags are available on every command via the root `rimba` command:

| Flag | Description |
|------|-------------|
| `--json` | Output in JSON format (supported by `list`, `status`, `deps status`, `conflict-check`, `exec`) |
| `--no-color` | Disable colored output (also respects `NO_COLOR` env var) |

---

## Setup

### rimba init

Initialize rimba in the current repository. Detects the repo root, creates `.rimba.toml`, and sets up the worktree directory. Use `--agent-files` to also install AI agent instruction files (`AGENTS.md`, `.github/copilot-instructions.md`, `.cursor/rules/rimba.mdc`, `.claude/skills/rimba/SKILL.md`).

If `.rimba.toml` already exists, config creation is skipped but agent files are still installed or updated when `--agent-files` is passed.

```sh
rimba init                  # Initialize config and worktree directory
rimba init --agent-files    # Also install AI agent instruction files
```

| Flag | Description |
|------|-------------|
| `--agent-files` | Install AI agent instruction files (AGENTS.md, copilot, cursor, claude) |

---

## Worktree Lifecycle

### rimba add

Create a new worktree with a branch named `<prefix><task>` and copy dotfiles from the repo root.

```sh
rimba add my-feature
rimba add my-feature --bugfix        # Use bugfix/ prefix instead of feature/
rimba add my-feature -s develop      # Branch from a different source
```

| Flag | Description |
|------|-------------|
| `--bugfix` | Use `bugfix/` branch prefix |
| `--hotfix` | Use `hotfix/` branch prefix |
| `--docs` | Use `docs/` branch prefix |
| `--test` | Use `test/` branch prefix |
| `--chore` | Use `chore/` branch prefix |
| `-s`, `--source` | Source branch to create worktree from (default from config) |
| `--skip-deps` | Skip dependency detection and installation |
| `--skip-hooks` | Skip post-create hooks |

### rimba remove

Remove the worktree for the given task and delete the local branch.

```sh
rimba remove my-feature
rimba remove my-feature -k           # Keep the branch after removal
rimba remove my-feature -f           # Force removal even if dirty
```

| Flag | Description |
|------|-------------|
| `-k`, `--keep-branch` | Keep the local branch after removing the worktree |
| `-f`, `--force` | Force removal even if the worktree is dirty |

### rimba rename

Rename a worktree's task, branch, and directory.

```sh
rimba rename old-task new-task
rimba rename old-task new-task -f  # Force rename even if locked
```

| Flag | Description |
|------|-------------|
| `-f`, `--force` | Force rename even if the worktree is locked |

### rimba duplicate

Create a new worktree from an existing worktree, inheriting its branch prefix. Auto-generates a `-1`, `-2`, etc. suffix unless `--as` is provided.

```sh
rimba duplicate auth              # Creates feature/auth-1 from feature/auth
rimba duplicate auth --as auth-v2 # Creates feature/auth-v2 from feature/auth
```

| Flag | Description |
|------|-------------|
| `--as` | Custom name for the duplicate worktree (instead of auto-suffix) |
| `--skip-deps` | Skip dependency detection and installation |
| `--skip-hooks` | Skip post-create hooks |

### rimba archive

Archive a worktree by removing its directory but preserving the local branch for later restoration with `rimba restore`.

```sh
rimba archive my-feature
rimba archive my-feature -f      # Force archival even if dirty
```

| Flag | Description |
|------|-------------|
| `-f`, `--force` | Force archival even if the worktree has uncommitted changes |

> **Note:** The branch is preserved locally. Use [`rimba restore`](#rimba-restore) to recreate the worktree from the archived branch.

### rimba restore

Restore an archived worktree by recreating it from a branch that was previously archived with `rimba archive`.

```sh
rimba restore my-feature
```

| Flag | Description |
|------|-------------|
| `--skip-deps` | Skip dependency detection and installation |
| `--skip-hooks` | Skip post-create hooks |

> **Note:** Restoring copies dotfiles, installs dependencies, and runs post-create hooks — just like `rimba add`. Use [`rimba archive`](#rimba-archive) to archive a worktree.

---

## Inspection

### rimba list

List all worktrees with their task name, type, branch, path, and status. The current worktree is marked with `*`.

```sh
rimba list
rimba list --type bugfix        # Show only bugfix worktrees
rimba list --dirty              # Show only dirty worktrees
rimba list --behind             # Show only worktrees behind upstream
rimba list --archived           # Show archived branches (not in any active worktree)
rimba list --json               # Output as JSON
```

Example output:

```
TASK            TYPE     BRANCH              PATH              STATUS
* auth-flow     feature  feature/auth-flow   feature-auth-flow [dirty]
  fix-login     bugfix   bugfix/fix-login    bugfix-fix-login  ↑2 ↓1
  ui-cleanup    chore    chore/ui-cleanup    chore-ui-cleanup  ✓
```

| Flag | Description |
|------|-------------|
| `--type` | Filter by prefix type (e.g. `feature`, `bugfix`, `hotfix`, `docs`, `test`, `chore`) |
| `--dirty` | Show only worktrees with uncommitted changes |
| `--behind` | Show only worktrees behind their upstream branch |
| `--archived` | Show archived branches not in any active worktree (mutually exclusive with other filters) |

### rimba status

Show a worktree dashboard with summary stats (total, dirty, stale, behind) and per-worktree age information.

```sh
rimba status
rimba status --stale-days 7     # Consider worktrees stale after 7 days
rimba status --json             # Output as JSON
```

Example output:

```
Worktrees: 4  Dirty: 1  Stale: 1  Behind: 0

TASK          TYPE     BRANCH              STATUS    AGE
  auth-flow   feature  feature/auth-flow   [dirty]   2d
  fix-login   bugfix   bugfix/fix-login    ✓         5h
  ui-cleanup  chore    chore/ui-cleanup    ✓         21d ⚠ stale
  payments    feature  feature/payments    ✓         3d
```

| Flag | Description |
|------|-------------|
| `--stale-days` | Number of days after which a worktree is considered stale (default: 14) |

### rimba log

Show the most recent commit from each worktree, sorted from newest to oldest.

```sh
rimba log
rimba log --limit 5             # Show only the 5 most recent entries
rimba log --since 7d            # Show entries from the last 7 days
```

Example output:

```
Recent commits across 3 worktree(s):

TASK          TYPE     AGE    COMMIT
  auth-flow   feature  5h     Add OAuth2 callback handler
  payments    feature  3d     Implement Stripe webhook endpoint
  ui-cleanup  chore    21d    Remove deprecated CSS classes
```

| Flag | Description |
|------|-------------|
| `--limit` | Maximum number of entries to show (default: 0 = all) |
| `--since` | Show entries since duration (e.g. `7d`, `2w`, `3h`) |

### rimba open

Open a worktree or run a command inside it. When called with just a task name, prints the worktree path. When given additional arguments, executes that command inside the worktree directory. Supports configurable shortcuts via the `[open]` config section.

```sh
rimba open my-task              # Print worktree path
cd $(rimba open my-task)        # Navigate to worktree
rimba open my-task --ide        # Run the 'ide' shortcut
rimba open my-task --agent      # Run the 'agent' shortcut
rimba open my-task -w test      # Run a named shortcut
rimba open my-task npm start    # Run an inline command
```

| Flag | Description |
|------|-------------|
| `--ide` | Run the `ide` shortcut from `[open]` config |
| `--agent` | Run the `agent` shortcut from `[open]` config |
| `-w`, `--with` | Run any named shortcut from `[open]` config |

> **Note:** `--ide`, `--agent`, and `--with` are mutually exclusive with each other and with inline command arguments. Shortcuts are configured in the `[open]` section of `.rimba.toml` (see [Configuration](configuration.md)).

---

## Merging & Syncing

### rimba merge

Merge a worktree's branch into main or another worktree. When merging into main, the source worktree and branch are auto-deleted unless `--keep` is set. When merging between worktrees, the source is kept unless `--delete` is set.

```sh
rimba merge auth                           # Merge into main, delete source
rimba merge auth --keep                    # Merge into main, keep source
rimba merge auth --into dashboard          # Merge into worktree, keep source
rimba merge auth --into dashboard --delete # Merge into worktree, delete source
rimba merge auth --no-ff                   # Force merge commit
```

| Flag | Description |
|------|-------------|
| `--into` | Target worktree task to merge into (default: main/repo root) |
| `--no-ff` | Force a merge commit (no fast-forward) |
| `--keep` | Keep source worktree after merging into main |
| `--delete` | Delete source worktree after merging into another worktree |

> **Note:** `--keep` and `--delete` are mutually exclusive. Merging to main deletes the source by default; merging to another worktree keeps it by default.

### rimba sync

Sync worktree(s) with the main branch by rebasing (default) or merging. Fetches the latest changes from origin first.

```sh
rimba sync my-feature                # Rebase a single worktree onto main
rimba sync my-feature --merge        # Use merge instead of rebase
rimba sync --all                     # Sync all eligible worktrees
rimba sync --all --include-inherited # Include duplicate worktrees
```

| Flag | Description |
|------|-------------|
| `--all` | Sync all eligible worktrees (skips dirty and inherited by default) |
| `--merge` | Use merge instead of rebase |
| `--include-inherited` | Include inherited/duplicate worktrees when using `--all` |

> **Note:** Dirty worktrees are skipped with a warning. On conflict, the rebase is automatically aborted and a recovery hint is printed.

### rimba merge-plan

Analyze file overlaps between worktree branches and recommend an optimal merge order that minimizes conflicts.

```sh
rimba merge-plan
```

Example output:

```
ORDER  BRANCH       CONFLICTS
1      fix-login    0
2      auth-flow    1
3      ui-cleanup   2

Merge in this order to minimize conflicts.
```

### rimba conflict-check

Scan all active worktrees and report files modified in multiple branches, indicating potential merge conflicts. Optionally simulate merges with `git merge-tree`.

```sh
rimba conflict-check
rimba conflict-check --dry-merge    # Simulate merges with git merge-tree
```

Example output:

```
FILE              BRANCHES              SEVERITY
src/auth.go       auth-flow, fix-login  high
src/config.go     auth-flow, payments   medium

2 file overlap(s) found across 3 branches.
```

| Flag | Description |
|------|-------------|
| `--dry-merge` | Simulate merges with `git merge-tree` (requires git 2.38+) |

---

## Bulk Operations

### rimba exec

Run a shell command in parallel across matching worktrees. Requires `--all` or `--type` to select targets.

```sh
rimba exec "npm test" --all                 # Run in all worktrees
rimba exec "git status" --type bugfix       # Run in bugfix worktrees only
rimba exec "npm test" --all --dirty         # Run only in dirty worktrees
rimba exec "npm test" --all --fail-fast     # Stop after first failure
rimba exec "npm test" --all --concurrency 4 # Limit to 4 parallel runs
```

| Flag | Description |
|------|-------------|
| `--all` | Run in all eligible worktrees |
| `--type` | Filter by prefix type (e.g. `feature`, `bugfix`, `hotfix`, `docs`, `test`, `chore`) |
| `--dirty` | Run only in worktrees with uncommitted changes |
| `--fail-fast` | Stop execution after the first failure |
| `--concurrency` | Max parallel executions (default: 0 = unlimited) |

> **Note:** Either `--all` or `--type` is required to select worktrees.

---

## Hooks

### rimba hook

Manage Git hooks for worktree workflow automation. Installs two hooks: a `post-merge` hook for automatic cleanup and a `pre-commit` hook that prevents direct commits to main/master.

#### rimba hook install

Install the rimba Git hooks: a `post-merge` hook that automatically runs `rimba clean --merged --force` after `git pull` (only on the default branch), and a `pre-commit` hook that prevents direct commits to main/master.

```sh
rimba hook install           # Install both hooks
```

#### rimba hook uninstall

Remove the rimba post-merge and pre-commit hooks. Preserves any other content in the hook files.

```sh
rimba hook uninstall         # Remove both rimba hooks
```

#### rimba hook status

Show whether the rimba post-merge and pre-commit hooks are currently installed.

```sh
rimba hook status            # Check installation status
```

> **Note:** `rimba hook` works with or without `rimba init`. The hooks coexist with existing user-defined hooks in the same hook files.

---

## Dependencies

### rimba deps

Manage worktree dependencies — detect lockfiles, clone dependency directories, and install packages.

#### rimba deps status

Show detected dependency modules and lockfile hashes for all worktrees.

```sh
rimba deps status
```

Example output:

```
refs/heads/main (/path/to/repo)
  node_modules [a1b2c3d4e5f6]
  vendor [7g8h9i0j1k2l]
refs/heads/feature/auth (/path/to/worktrees/feature-auth)
  node_modules [a1b2c3d4e5f6]
  vendor [7g8h9i0j1k2l]
```

#### rimba deps install

Detect and install dependencies for a specific worktree. Clones from an existing worktree with a matching lockfile hash, or falls back to the configured install command.

```sh
rimba deps install my-feature
```

---

## Maintenance

### rimba clean

Prune stale worktree references, or detect and remove worktrees whose branches have been merged into main or are stale.

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

| Flag | Description |
|------|-------------|
| `--dry-run` | Show what would be pruned/removed without making changes |
| `--merged` | Detect and remove worktrees whose branches are merged into main |
| `--stale` | Remove worktrees with no recent commits |
| `--stale-days` | Number of days to consider a worktree stale (default: 14, used with `--stale`) |
| `--force` | Skip confirmation prompt when used with `--merged` or `--stale` |

> **Note:** `--merged` and `--stale` are mutually exclusive. `--merged` works with or without `rimba init`. Without a config file, it falls back to auto-detecting the default branch.

---

### rimba update

Check for the latest release on GitHub and update the binary in place.

```sh
rimba update             # Check and update to latest
rimba update --force     # Also works on dev builds
```

| Flag | Description |
|------|-------------|
| `--force` | Update even if running a development build |

> **Note:** If the binary cannot be replaced due to file permissions, rimba automatically retries with `sudo`.

---

### rimba version

Print version, commit, and build date.

```sh
rimba version
# rimba v0.1.0 (commit: abc1234, built: 2026-01-01T00:00:00Z)
```
