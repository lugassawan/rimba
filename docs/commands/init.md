---
title: rimba init
parent: Command
nav_order: 1
---

# rimba init

Initialize rimba in the current repository. Detects the repo root, creates the `.rimba/` config directory with `settings.toml` (team-shared) and `settings.local.toml` (personal overrides), and sets up the worktree directory.

Agent files (`AGENTS.md`, `.github/copilot-instructions.md`, `.cursor/rules/rimba.mdc`, `.claude/skills/rimba/SKILL.md`) can be installed at three tiers:

- `--agents` — project-team level (committed to git)
- `--agents --local` — project-personal level (gitignored)
- `-g` / `--global` — user level (`~/`) — works outside a git repository

When agent files are installed, rimba also registers itself as an MCP server (`rimba mcp`) in client config files: `.mcp.json`, `.cursor/mcp.json`, `~/.claude.json`, `~/.codex/config.toml`, `~/.gemini/settings.json`, `~/.codeium/windsurf/mcp_config.json`, `~/.roo/mcp.json`. Use `--uninstall` with the same flags to remove.

If `.rimba/` already exists, config creation is skipped but agent files are still installed or updated. If a legacy `.rimba.toml` exists, it is migrated into the new directory layout.

## Synopsis

```sh
rimba init [flags]
```

## Examples

```sh
rimba init                        # Initialize config and worktree directory
rimba init --agents               # Also install agent files at project level (committed)
rimba init --agents --local       # Install agent files gitignored (personal)
rimba init -g                     # Install agent files at user level (~/)
rimba init -g --uninstall         # Remove user-level agent files and MCP registration
rimba init --agents --uninstall   # Remove project-team agent files and MCP registration
```

## Common workflows

**First-time setup for a team repo**
```sh
rimba init
rimba init --agents   # Commit agent files so teammates get AI context
git add .rimba/ .github/copilot-instructions.md AGENTS.md
git commit -m "chore: add rimba config and agent files"
```

**Personal setup without affecting teammates**
```sh
rimba init --agents --local   # Agent files are gitignored
```

**Global user-level install (works in any repo)**
```sh
rimba init -g   # Installs to ~/ and registers MCP globally
```

**Migrate from legacy .rimba.toml**
```sh
rimba init   # Detects .rimba.toml and migrates to .rimba/ layout automatically
```

{: .warning }
> - `--local` is not allowed with `-g`.
> - `--local` without `--agents` errors with: `--local requires --agents`.
> - `--uninstall` requires `-g` or `--agents`.
> - `-g` / `--global` and `--agents` are mutually exclusive.

## Flags

| Flag | Description |
|------|-------------|
| `--agents` | Install AI agent instruction files at project level |
| `-g`, `--global` | Install agent files at user level (`~/`) — works outside a git repo |
| `--local` | Install agent files gitignored (personal overrides; requires `--agents`) |
| `--uninstall` | Remove agent files and MCP registration (requires `-g` or `--agents`) |
| `--personal` | Gitignore the entire `.rimba/` directory instead of the `.rimba/*.local.toml` glob |

## Related commands

- [rimba trust](trust) · approve committed shell commands after editing `settings.toml`
- [rimba hook](hook) · install Git hooks for automated cleanup
- [rimba add](add) · create your first worktree
