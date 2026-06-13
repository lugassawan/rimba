---
title: Init & setup
parent: Troubleshooting
nav_order: 6
---

# Init & setup

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

See [Configuration]({{ '../configuration' | relative_url }}) for all `[worktree]` options.

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
