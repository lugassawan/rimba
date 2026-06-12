---
title: Troubleshooting
nav_order: 4
---

# Troubleshooting

> See also: [Commands](commands.md) · [Configuration](configuration.md)

---

rimba prints an actionable `To fix:` hint with most failures. Each section below shows the exact
error, why it happens, and the command to resolve it.

---

## Trust & consent gate

rimba will not run committed shell commands until they are approved. See
[README § Consent gate](https://github.com/lugassawan/rimba#consent-gate) for the full background on the approval flow.

### `committed shell commands require approval`

```
committed shell commands require approval
To fix: review the commands above, then run 'rimba trust' (or pass --yes / set RIMBA_TRUST_YES=1 in CI)
```

**Why:** The interactive prompt was declined (answered "N") or stdin was non-interactive (EOF).

**Fix:**

```sh
rimba trust            # review commands and approve interactively
rimba trust --yes      # approve without prompt
RIMBA_TRUST_YES=1 rimba add my-task   # CI / non-interactive
```

### `committed shell commands are not trusted for this repo`

```
committed shell commands are not trusted for this repo
To fix: run 'rimba trust' to approve them, or set RIMBA_TRUST_YES=1 for CI
```

**Why:** Emitted by the MCP non-interactive path (`rimba mcp`) when committed commands exist but have
not been approved and `RIMBA_TRUST_YES` is unset.

**Fix:**

```sh
rimba trust            # approve from the CLI, then retry via MCP
RIMBA_TRUST_YES=1 ...  # or pre-approve in the environment
```

---

## Selecting worktrees (`exec`)

### `provide --all or --type to select worktrees`

```
provide --all or --type to select worktrees
To fix: run: rimba exec --all <cmd>  OR  rimba exec --type <prefix> <cmd>
```

**Why:** `rimba exec` was called without specifying which worktrees to target.

**Fix:**

```sh
rimba exec --all "git status"
rimba exec --type feature "npm test"
```

### `invalid type "<x>"; valid types: ...`

```
invalid type "foo"; valid types: feature, bugfix, hotfix, docs, test, chore
To fix: choose one of the built-in prefix types: feature, bugfix, hotfix, docs, test, chore
```

**Why:** The value passed to `--type` is not a recognized prefix type.

**Fix:** Choose one of the types printed in the error message (`feature`, `bugfix`, `hotfix`, `docs`,
`test`, `chore`).

```sh
rimba exec --type feature "npm test"
```

### `--concurrency must be >= 0`

```
--concurrency must be >= 0
To fix: run: rimba exec --concurrency <n>  (n >= 0; 0 = unlimited)
```

**Why:** A negative value was passed to `--concurrency`.

**Fix:**

```sh
rimba exec --all --concurrency 4 "npm test"   # limit to 4 parallel
rimba exec --all --concurrency 0 "npm test"   # unlimited (default)
```

---

## Syncing

### `worktree "<task>" has uncommitted changes`

```
worktree "auth" has uncommitted changes
To fix: Commit or stash changes before syncing: cd /path/to/worktree
```

**Why:** `rimba sync <task>` was called on a worktree with a dirty working tree. rimba refuses to
rebase or merge over uncommitted work to avoid data loss.

**Fix:**

```sh
cd /path/to/worktree
git stash          # or: git commit -am "WIP"
rimba sync auth
```

> **Note:** `rimba sync --all` does *not* error on dirty worktrees — it skips them and prints
> `Skipping <branch> (dirty)` so the rest continue. Run `rimba sync <task>` individually after
> cleaning up.

### `push failed for <branch>: ...`

```
push failed for feature/auth: exit status 1
To fix: cd /path/to/worktree && git push --force-with-lease
```

**Why:** The remote rejected the push after a rebase (non-fast-forward). With `--merge`, the push
is a fast-forward so the hint changes to a plain `git push`.

**Fix (after rebase):**

```sh
cd /path/to/worktree && git push --force-with-lease
```

**Fix (after `--merge`):**

```sh
cd /path/to/worktree && git push
```

---

## Adding / promoting

### `--source is not valid in branch: mode`

```
--source is not valid in branch: mode
To fix: remove the --source flag: branch: promotes an existing branch, not a new one
```

**Why:** `rimba add branch:<branch> --source <ref>` was used. `branch:` mode promotes the currently
checked-out branch as-is; it does not create a new branch from a source ref.

**Fix:**

```sh
rimba add branch:feature/my-feature   # no --source
```

### `--task requires a pr:<num> argument`

```
--task requires a pr:<num> argument
To fix: pass a PR argument: rimba add pr:<num> --task <name>
```

**Why:** `--task` was passed without a `pr:<num>` positional argument. The flag is only valid in PR
mode to override the auto-derived task name.

**Fix:**

```sh
rimba add pr:123 --task review/auth
```

---

## Removing

### Uncommitted changes block removal

```
<git error about dirty state>
To fix: commit or stash changes, or use --force to discard
```

**Why:** `rimba remove <task>` found uncommitted changes in the worktree. The default behavior is
safe — it refuses to discard work.

**Fix:**

```sh
cd /path/to/worktree
git stash                       # save changes
rimba remove auth               # then remove cleanly

# OR, to discard changes permanently:
rimba remove auth --force
```

---

## Init & setup

### `--local requires --agents`

```
--local requires --agents
To fix: run: rimba init --agents --local
```

**Why:** `rimba init --local` was run without `--agents`. `--local` controls the install tier for
agent files and only makes sense alongside `--agents`.

**Fix:**

```sh
rimba init --agents --local
```

### `--uninstall requires -g or --agents`

```
--uninstall requires -g or --agents
To fix: run: rimba init -g --uninstall  OR  rimba init --agents --uninstall
```

**Why:** `rimba init --uninstall` was run without specifying which tier to uninstall from.

**Fix:**

```sh
rimba init -g --uninstall                    # remove user-level files
rimba init --agents --uninstall              # remove project-team files
rimba init --agents --local --uninstall      # remove project-local files
```

### `failed to create worktree directory: ...`

```
failed to create worktree directory: mkdir /path/...: permission denied
To fix: check directory permissions for the worktree dir, or set worktree.dir in .rimba/settings.toml
```

**Why:** rimba could not create the sibling worktree directory (default: `../<repo>-worktrees`).

**Fix:** Check that the parent directory is writable, or point `worktree.dir` to a different path:

```toml
# .rimba/settings.toml
[worktree]
dir = "../my-worktrees"
```

See [docs/configuration.md](configuration.md) for all `[worktree]` options.

### Agent-file / MCP-server permission errors

```
agent files: <error>
To fix: check write permissions for the install dir
```

```
mcp servers: <error>
To fix: check write permissions for MCP client configs (.mcp.json, .cursor/mcp.json, ~/.claude.json)
```

**Why:** `rimba init --agents` (or `-g`) could not write agent instruction files or register the MCP
server in client config files.

**Fix:** Ensure the target directory is writable. For user-level (`-g`) installs the target is `~/`.
For project-level installs the target is the repo root.

---

## Configuration loading

### `config not found: ...` — run `rimba init`

```
config not found: .rimba/settings.toml does not exist
To fix: run 'rimba init' to create a default .rimba/settings.toml
```

**Why:** rimba could not find `.rimba/settings.toml`. This happens in a fresh clone or when running
outside a rimba-initialized repo.

**Fix:**

```sh
rimba init
```

### `invalid config <file>: ...` — TOML syntax error

```
failed to read team config: invalid config settings.toml: toml: line N: <description>
To fix: fix the TOML syntax in /path/to/.rimba/settings.toml
```

**Why:** `.rimba/settings.toml` (or `settings.local.toml`) contains a TOML syntax error.
`loadRaw()` emits `invalid config <filename>: ...`; `LoadDir()` wraps it with
`failed to read team config: ...` (or `failed to read local config: ...` for `settings.local.toml`).

**Fix:** Open the file and correct the syntax. Common mistakes: missing quotes around strings with
special characters, mismatched brackets, or stray commas.

> Field-level validation errors (wrong types, out-of-range values) also print their own `To fix:`
> hints with the exact field name and expected value.

---

## Debugging git operations

Pass `--debug` to any command (or set `RIMBA_DEBUG=1`) to log every git command and its timing to
stderr. This is useful for diagnosing slow operations or unexpected git output.

```sh
rimba list --debug
rimba sync auth --debug
RIMBA_DEBUG=1 rimba exec --all "git status"
```

The debug output shows the command, arguments, working directory, and elapsed time. Compare against
what you'd expect from running the git commands directly to narrow down the problem.
