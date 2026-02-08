# rimba

Git worktree manager CLI — branch naming conventions, dotfile copying, and worktree status dashboards.

## Features

- **Automatic branch naming** — configurable prefix (e.g. `feat/`, `fix/`) applied to task names
- **Dotfile copying** — auto-copies files like `.env`, `.envrc`, `.tool-versions` into new worktrees
- **Status dashboard** — colored tabular view of all worktrees with dirty state, ahead/behind counts, current worktree indicator, and filtering
- **Duplicate worktrees** — create a copy of an existing worktree with auto-suffixed or custom name
- **Local merge** — merge worktree branches into main or other worktrees with auto-cleanup
- **Stale cleanup** — prune stale worktree references with dry-run support
- **Shell completions** — built-in completion for bash, zsh, fish, and PowerShell
- **Cross-platform** — builds for Linux, macOS, and Windows (amd64/arm64)

## Installation

### Go install

```sh
go install github.com/lugassawan/rimba@latest
```

### Build from source

```sh
git clone https://github.com/lugassawan/rimba.git
cd rimba
make build
# Binary is at ./bin/rimba
```

## Quick Start

```sh
# Initialize rimba in your repo
rimba init

# Create a worktree for a task
rimba add my-feature

# List all worktrees with status (colored output)
rimba list

# Remove a worktree when done
rimba remove my-feature --branch
```

## Commands

### `rimba init`

Initialize rimba in the current repository. Detects the repo root, creates `.rimba.toml`, and sets up the worktree directory.

```sh
rimba init
```

### `rimba add <task>`

Create a new worktree with a branch named `<prefix><task>` and copy dotfiles from the repo root.

```sh
rimba add my-feature
rimba add my-feature -p fix/         # Override branch prefix
rimba add my-feature -s develop      # Branch from a different source
```

| Flag | Description |
|------|-------------|
| `-p`, `--prefix` | Branch prefix (default from config) |
| `-s`, `--source` | Source branch to create worktree from (default from config) |

### `rimba list`

List all worktrees with their task name, type, branch, path, and status. The current worktree is marked with `*`. Output is colored by default.

```sh
rimba list
rimba list --type bugfix        # Show only bugfix worktrees
rimba list --dirty              # Show only dirty worktrees
rimba list --behind             # Show only worktrees behind upstream
rimba list --no-color           # Disable colored output
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
| `--no-color` | Disable colored output (also respects `NO_COLOR` env var) |

### `rimba remove <task>`

Remove the worktree for the given task. Optionally delete the local branch.

```sh
rimba remove my-feature
rimba remove my-feature --branch     # Also delete the local branch
rimba remove my-feature -f           # Force removal even if dirty
```

| Flag | Description |
|------|-------------|
| `--branch` | Also delete the local branch |
| `-f`, `--force` | Force removal even if the worktree is dirty |

### `rimba rename <task> <new-task>`

Rename a worktree's task, branch, and directory.

```sh
rimba rename old-task new-task
rimba rename old-task new-task -f  # Force rename even if locked
```

| Flag | Description |
|------|-------------|
| `-f`, `--force` | Force rename even if the worktree is locked |

### `rimba duplicate <task>`

Create a new worktree from an existing worktree, inheriting its branch prefix. Auto-generates a `-1`, `-2`, etc. suffix unless `--as` is provided.

```sh
rimba duplicate auth              # Creates feature/auth-1 from feature/auth
rimba duplicate auth --as auth-v2 # Creates feature/auth-v2 from feature/auth
```

| Flag | Description |
|------|-------------|
| `--as` | Custom name for the duplicate worktree (instead of auto-suffix) |

### `rimba merge <source-task>`

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

### `rimba clean`

Prune stale worktree references.

```sh
rimba clean
rimba clean --dry-run                # Preview what would be pruned
```

| Flag | Description |
|------|-------------|
| `--dry-run` | Show what would be pruned without pruning |

### `rimba version`

Print version, commit, and build date.

```sh
rimba version
# rimba v0.1.0 (commit: abc1234, built: 2026-01-01T00:00:00Z)
```

## Configuration

`rimba init` creates a `.rimba.toml` file in the repo root:

```toml
worktree_dir = '../myrepo-worktrees'
default_prefix = 'feat/'
default_source = 'main'
copy_files = ['.env', '.env.local', '.envrc', '.tool-versions']
```

| Field | Description | Default |
|-------|-------------|---------|
| `worktree_dir` | Directory for worktrees (relative to repo root) | `../<repo-name>-worktrees` |
| `default_prefix` | Branch name prefix | `feat/` |
| `default_source` | Branch to create worktrees from | Default branch (e.g. `main`) |
| `copy_files` | Files to copy from repo root into new worktrees | `.env`, `.env.local`, `.envrc`, `.tool-versions` |

> **Tip:** Add `.rimba.toml` to your `.gitignore`.

## License

[MIT](LICENSE)
