---
title: rimba trust
parent: Command
nav_order: 22
---

# rimba trust

Review and approve the shell commands configured in `.rimba/settings.toml`.

rimba will not automatically run committed `post_create`, `post_rename`, or `deps.modules[].install` shell commands until you explicitly approve them. This prevents a malicious or accidental settings change from running arbitrary code on your machine without your knowledge.

Approval is stored locally in `.rimba/trust.local.toml` (gitignored) and is keyed by a **hash of the current command set**. Changing any shell command in `settings.toml` automatically re-arms the consent gate — you will be prompted to approve again.

## Synopsis

```sh
rimba trust [flags]
```

## Examples

```sh
rimba trust           # Review commands and approve interactively
rimba trust --show    # Inspect commands and approval status without prompting
rimba trust --yes     # Approve without prompting (e.g. in CI)
```

## How it works

When rimba encounters unapproved shell commands during `add`, `rename`, `duplicate`, `restore`, or dependency installation, it:

1. Displays the configured commands.
2. Prompts: `Run these commands? [y/N]`
3. On approval, records the hash in `.rimba/trust.local.toml`.
4. On decline (or non-interactive stdin), the invoking command exits with an error and a remediation hint. (`rimba trust` itself exits 0 on decline.)

Once approved, rimba runs the commands without prompting — until the command set changes.

## Common workflows

**Approve commands after cloning a repo**
```sh
rimba trust
# Shows commands from settings.toml; prompts for approval
```

**Inspect without approving**
```sh
rimba trust --show
```
```
Configured shell commands:
  npm install
  npx prisma generate

Hash: a1b2c3d4...
Status: not trusted — run 'rimba trust' to approve
```

**Approve in CI without a prompt**
```sh
rimba trust --yes
# Or set RIMBA_TRUST_YES=1 in your CI environment
```

**Re-approve after a settings change**
```sh
# A teammate updated post_create in settings.toml
git pull
rimba trust   # Hash changed; re-prompts for approval
```

{: .note }
> `.rimba/trust.local.toml` is gitignored — approval is per-machine, not shared. Every developer on the team must approve independently. This is intentional: trust is a local consent decision.

## Flags

| Flag | Description |
|------|-------------|
| `--show` | Display configured commands and approval status without prompting |
| `--yes` | Approve committed shell commands without prompting (also: `RIMBA_TRUST_YES=1`) |

## Related commands

- [rimba init](init) · set up `.rimba/settings.toml` where shell commands are configured
- [rimba add](add) · triggers the trust gate when `post_create` hooks are configured
- [rimba rename](rename) · triggers the trust gate when `post_rename` hooks are configured
- [rimba duplicate](duplicate) · triggers the trust gate when `post_create` hooks are configured
- [rimba restore](restore) · triggers the trust gate when `post_create` hooks are configured
- [rimba deps](deps) · triggers the trust gate when `deps.modules[].install` is configured
