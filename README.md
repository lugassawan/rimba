<div align="center">
  <img src=".github/rimba-128.png" alt="rimba" width="128" height="128" />
  <h1>rimba</h1>
  <p>Git worktree lifecycle manager</p>
</div>

<p align="center">
  <a href="https://github.com/lugassawan/rimba/releases/latest"><img src="https://img.shields.io/github/v/release/lugassawan/rimba" alt="Release" /></a>
  <a href="https://github.com/lugassawan/rimba/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/lugassawan/rimba/ci.yml?branch=main&label=CI" alt="CI" /></a>
  <a href="go.mod"><img src="https://img.shields.io/badge/go-%3E%3D1.25-00ADD8?logo=go" alt="Go" /></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-green" alt="License" /></a>
</p>

---

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Commands](#commands)
- [Configuration](#configuration)
- [Contributing](CONTRIBUTING.md)
- [License](#license)

## Features

🌿 **Core Workflow**

- **Automatic branch naming** — configurable prefix (e.g. `feat/`, `fix/`) applied to task names
- **File & directory copying** — auto-copies `.env`, `.envrc`, `.tool-versions`, `.vscode/` and other files or directories into new worktrees
- **Duplicate worktrees** — copy an existing worktree with auto-suffixed or custom name
- **Local merge** — merge worktree branches into main or other worktrees with auto-cleanup
- **Sync worktrees** — rebase or merge onto the latest main branch, with bulk sync support

🔧 **Automation**

- **Shared dependencies** — auto-detect lockfiles and clone dependency directories using copy-on-write
- **Post-create hooks** — run shell commands after worktree creation (e.g. `./gradlew build`)
- **Auto-cleanup hook** — post-merge Git hook that cleans merged worktrees after `git pull`
- **Stale cleanup** — prune stale references or auto-detect and remove merged worktrees

🖥️ **Developer Experience**

- **Status dashboard** — colored tabular view with dirty state, ahead/behind counts, and filtering
- **Pre-execution hints** — shows available flags before long-running commands, auto-filtered and suppressible
- **Worktree navigation** — open worktrees or run commands inside them via `open`
- **Shell completions** — bash, zsh, fish, and PowerShell
- **Cross-platform** — Linux, macOS, and Windows (amd64/arm64)

## Installation

### Quick install (Linux/macOS)

```sh
curl -sSfL https://raw.githubusercontent.com/lugassawan/rimba/main/scripts/install.sh | bash
```

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

# Remove a worktree and its branch when done
rimba remove my-feature
```

## Commands

| Command | Description |
|---------|-------------|
| `rimba init` | Initialize rimba in the current repository |
| `rimba add <task>` | Create a new worktree with auto-prefixed branch |
| `rimba remove <task>` | Remove a worktree and delete its branch |
| `rimba rename <old> <new>` | Rename a worktree's task, branch, and directory |
| `rimba duplicate <task>` | Create a copy of an existing worktree |
| `rimba archive <task>` | Archive a worktree (remove directory, keep branch) |
| `rimba restore <task>` | Restore an archived worktree from its preserved branch |
| `rimba list` | List all worktrees with status dashboard |
| `rimba status` | Show worktree dashboard with summary stats and age info |
| `rimba log` | Show last commit from each worktree, sorted by recency |
| `rimba open <task>` | Open a worktree or run a command inside it |
| `rimba merge <task>` | Merge a worktree branch into main or another worktree |
| `rimba sync <task>` | Rebase or merge a worktree onto the latest main |
| `rimba merge-plan` | Recommend optimal merge order to minimize conflicts |
| `rimba conflict-check` | Detect file overlaps between worktree branches |
| `rimba exec <command>` | Run a shell command across worktrees |
| `rimba hook install` | Install post-merge and pre-commit hooks |
| `rimba hook uninstall` | Remove the rimba hooks |
| `rimba hook status` | Show whether the rimba hooks are installed |
| `rimba deps status` | Show detected dependency modules for all worktrees |
| `rimba deps install <task>` | Detect and install dependencies for a worktree |
| `rimba clean` | Prune stale references or remove merged/stale worktrees |
| `rimba update` | Check for updates and replace the binary in place |
| `rimba version` | Print version, commit, and build date |

> See [docs/commands.md](docs/commands.md) for the full reference with all flags, examples, and notes.

## Configuration

`rimba init` creates a `.rimba.toml` file in the repo root:

```toml
worktree_dir = '../myrepo-worktrees'
default_source = 'main'
copy_files = ['.env', '.env.local', '.envrc', '.tool-versions', '.vscode']
post_create = ['./gradlew build']

[open]
ide = 'code .'
agent = 'claude'
```

> See [docs/configuration.md](docs/configuration.md) for the full field reference, dependency management, and environment variables.

## License

[MIT](LICENSE)
