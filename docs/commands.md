# Command Reference

> Back to [README](../README.md) | See also: [Configuration](configuration.md)

---

## Global Flags

These flags are available on every command via the root `rimba` command:

| Flag | Description |
|------|-------------|
| `--json` | Output in JSON format (supported by `list`, `status`, `deps status`, `conflict-check`, `exec`) |
| `--no-color` | Disable colored output (also respects `NO_COLOR` env var) |
| `--debug` | Log git commands and timings to stderr (also respects `RIMBA_DEBUG=1`) |

---

## Setup

### rimba init

Initialize rimba in the current repository. Detects the repo root, creates the `.rimba/` config directory with `settings.toml` (team-shared) and `settings.local.toml` (personal overrides), and sets up the worktree directory.

Agent files (`AGENTS.md`, `.github/copilot-instructions.md`, `.cursor/rules/rimba.mdc`, `.claude/skills/rimba/SKILL.md`) are installed at three tiers:

- `--agents` — project-team level (committed to git)
- `--agents --local` — project-personal level (gitignored)
- `-g` / `--global` — user level (`~/`) — works outside a git repository

When agent files are installed (`--agents` or `-g`), rimba also registers itself as an MCP server (server name: `rimba`, command: `rimba mcp`) in the corresponding client config files (`.mcp.json`, `.cursor/mcp.json`, `~/.claude/settings.json`, `~/.codex/config.toml`, `~/.gemini/settings.json`, `~/.codeium/windsurf/mcp_config.json`, `~/.roo/mcp.json`). Use `--uninstall` with the same flags to remove.

If `.rimba/` already exists, config creation is skipped but agent files are still installed or updated. If a legacy `.rimba.toml` exists, it is migrated into the new directory layout.

```sh
rimba init                        # Initialize config and worktree directory
rimba init --agents               # Also install agent files at project level (committed)
rimba init --agents --local       # Install agent files gitignored (personal)
rimba init -g                     # Install agent files at user level (~/)
rimba init -g --uninstall         # Remove user-level agent files and MCP registration
rimba init --agents --uninstall   # Remove project-team agent files and MCP registration
```

> **Notes:**
> - `--local` is not allowed with `-g`.
> - `--local` without `--agents` errors with: `--local requires --agents`.
> - `--uninstall` requires `-g` or `--agents`.
> - `-g` / `--global` and `--agents` are mutually exclusive.

| Flag | Description |
|------|-------------|
| `--agents` | Install AI agent instruction files at project level |
| `-g`, `--global` | Install agent files at user level (`~/`) — works outside a git repo |
| `--local` | Install agent files gitignored (personal overrides; requires `--agents`) |
| `--uninstall` | Remove agent files and MCP registration (requires `-g` or `--agents`) |
| `--personal` | Gitignore the entire `.rimba/` directory instead of just the local config file |

---

## Worktree Lifecycle

### rimba add

Create a new worktree with a branch named `<prefix><task>` and copy dotfiles from the repo root. In monorepos, prefix the task with a service directory name to create service-scoped branches. Use `pr:<num>` to create a worktree from a GitHub PR's head branch.

```sh
rimba add my-feature
rimba add my-feature --bugfix        # Use bugfix/ prefix instead of feature/
rimba add my-feature -s develop      # Branch from a different source
rimba add auth-api/my-feature        # Monorepo: branch auth-api/feature/my-feature
rimba add auth-api/my-feature --bugfix  # Monorepo: branch auth-api/bugfix/my-feature
rimba add pr:123                     # Create worktree from PR #123's head branch
rimba add pr:123 --task review/auth-tweak  # Override auto-derived task name
```

> **Monorepo:** If the first segment before `/` matches a directory in the repo root, rimba treats it as a service scope. The branch uses a 3-segment pattern: `<service>/<prefix>/<task>`. No configuration needed — detection is automatic.

> **PR mode:** `pr:<num>` requires `gh` to be installed and authenticated. For cross-fork PRs, rimba adds a `gh-fork-<owner>` remote automatically. Without `--task`, the task name is derived as `review/<num>-<slug>`.

| Flag | Description |
|------|-------------|
| `--bugfix` | Use `bugfix/` branch prefix |
| `--hotfix` | Use `hotfix/` branch prefix |
| `--docs` | Use `docs/` branch prefix |
| `--test` | Use `test/` branch prefix |
| `--chore` | Use `chore/` branch prefix |
| `-s`, `--source` | Source branch to create worktree from (default from config) |
| `--task` | Override auto-derived task name (`pr:<num>` mode only) |
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

List all worktrees with task, type, and status. Use `--full` to show all columns including branch, path, and (when `gh` is installed and authenticated) PR number and CI rollup. The current worktree is marked with `*`.

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

Example output (compact, default):

```
TASK            TYPE     STATUS
* auth-flow     feature  [dirty]
  fix-login     bugfix   ↑2 ↓1
  ui-cleanup    chore    ✓
```

Example output with `--full`:

```
TASK            TYPE     BRANCH              PATH               STATUS    PR     CI
* auth-flow     feature  feature/auth-flow   feature-auth-flow  [dirty]   #142   ✓
  fix-login     bugfix   bugfix/fix-login    bugfix-fix-login   ↑2 ↓1     #138   ●
  ui-cleanup    chore    chore/ui-cleanup    chore-ui-cleanup   ✓         –      –
```

> **PR/CI columns:** Require `gh` installed and authenticated; otherwise a yellow warning is printed and those cells render as `–`. CI symbols: ✓ success · ● pending · ✗ failure · – unknown.

> **Note:** `--archived` is mutually exclusive with `--type`, `--dirty`, `--behind`, and `--full`.

| Flag | Description |
|------|-------------|
| `--full` | Show all columns including branch, path, and PR/CI (when `gh` is available) |
| `--type` | Filter by prefix type (e.g. `feature`, `bugfix`, `hotfix`, `docs`, `test`, `chore`) |
| `--dirty` | Show only worktrees with uncommitted changes |
| `--behind` | Show only worktrees behind their upstream branch |
| `--archived` | Show archived branches not in any active worktree (mutually exclusive with other filters) |
| `--service` | Filter by service name (monorepo) |

### rimba status

Show a worktree dashboard with summary stats (total, dirty, stale, behind) and per-worktree age information. Use `--detail` to also show disk size, 7-day commit velocity, and a disk-footprint summary line.

```sh
rimba status
rimba status --detail           # Add SIZE and 7D columns; sort by disk size (largest first)
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

Example output with `--detail`:

```
Worktrees: 4  Dirty: 1  Stale: 1  Behind: 0
Disk: total 1.2 GB  (main: 480 MB, worktrees: 720 MB)

TASK          TYPE     BRANCH              STATUS    AGE   SIZE    7D
  auth-flow   feature  feature/auth-flow   [dirty]   2d    310 MB  4
  payments    feature  feature/payments    ✓         3d    220 MB  1
  fix-login   bugfix   bugfix/fix-login    ✓         5h    140 MB  6
  ui-cleanup  chore    chore/ui-cleanup    ✓         21d   50 MB   0  ⚠ stale
```

> **Columns:** `SIZE` is the on-disk footprint of the worktree directory. `7D` is the number of commits on the worktree's branch in the last 7 days. `--detail` sorts rows largest-first.

| Flag | Description |
|------|-------------|
| `--detail` | Add SIZE and 7D columns; print disk-footprint summary; sort by disk size |
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

> **Note:** `--ide`, `--agent`, and `--with` are mutually exclusive with each other and with inline command arguments. Shortcuts are configured in the `[open]` section of `.rimba/settings.toml` (see [Configuration](configuration.md)).

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

Sync worktree(s) with the main branch by rebasing (default) or merging. Fetches the latest changes from origin first, then automatically pushes after a successful sync. Use `--no-push` to skip the push step.

```sh
rimba sync my-feature                # Rebase a single worktree onto main, then push
rimba sync my-feature --merge        # Use merge instead of rebase
rimba sync my-feature --no-push      # Sync without pushing
rimba sync --all                     # Sync all eligible worktrees
rimba sync --all --include-inherited # Include duplicate worktrees
```

| Flag | Description |
|------|-------------|
| `--all` | Sync all eligible worktrees (skips dirty and inherited by default) |
| `--merge` | Use merge instead of rebase |
| `--include-inherited` | Include inherited/duplicate worktrees when using `--all` |
| `--no-push` | Skip pushing after sync (useful for local-only rebase/merge) |

> **Note:** Dirty worktrees are skipped with a warning. On conflict, the rebase is automatically aborted and a recovery hint is printed. After a successful sync, the branch is pushed to origin by default.

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

## AI Integration

### rimba mcp

Start a Model Context Protocol (MCP) server that exposes rimba's worktree management as tools for AI coding agents. The server runs over stdio and follows the MCP specification.

```sh
rimba mcp    # Start MCP server (stdio transport)
```

#### Setup

**Claude Code** — add to `.mcp.json` or run:

```sh
claude mcp add rimba rimba mcp
```

**Cursor** — add to `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "rimba": {
      "command": "rimba",
      "args": ["mcp"]
    }
  }
}
```

#### MCP Tools

| Tool | Description | Required Parameters |
|------|-------------|---------------------|
| `list` | List all worktrees with branch, path, and status | — |
| `add` | Create a new worktree for a task (supports `service/task` for monorepos) | `task` |
| `remove` | Remove a worktree and optionally delete its branch | `task` |
| `status` | Show worktree dashboard with summary stats and age info | — |
| `sync` | Sync worktree(s) with the main branch via rebase or merge, then push | `task` or `all` |
| `merge` | Merge a worktree branch into main or another worktree | `source` |
| `exec` | Run a shell command across matching worktrees in parallel | `command` |
| `conflict-check` | Detect file overlaps between worktree branches | — |
| `clean` | Clean up stale references, merged branches, or stale worktrees | `mode` (prune, merged, stale) |

> **Note:** All tools return JSON responses. See `rimba mcp` help output for full parameter details.

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

> **Note:** If the binary cannot be replaced due to file permissions, rimba installs to `~/.local/bin` instead.

> **Post-update tips:** After a successful update, rimba prints a one-line tip if agent files are installed at user level (`rimba init -g` to refresh) or in this repo (`rimba init --agents` to refresh). Set `RIMBA_QUIET=1` to suppress.

---

### rimba version

Print version, commit, and build date.

```sh
rimba version
# rimba v0.1.0 (commit: abc1234, built: 2026-01-01T00:00:00Z)
```

---

### rimba completion

Generate shell completion scripts for bash, zsh, fish, or PowerShell.

```sh
rimba completion bash        # Generate bash completions
rimba completion zsh         # Generate zsh completions
rimba completion fish        # Generate fish completions
rimba completion powershell  # Generate PowerShell completions
```

> **Note:** Follow the printed instructions after generating to install the completions for your shell.
