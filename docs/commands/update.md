---
title: rimba update
parent: Command Reference
nav_order: 22
---

# rimba update

Check for the latest release on GitHub and update the binary in place. If the binary cannot be replaced due to file permissions, rimba installs to `~/.local/bin` instead.

## Synopsis

```sh
rimba update [flags]
```

## Examples

```sh
rimba update             # Check and update to latest
rimba update --force     # Also works on dev builds
```

## Common workflows

**Check and apply update**
```sh
rimba update
# Compares current version against latest GitHub release
# Downloads and replaces binary if newer version found
```

**Update a dev build**
```sh
rimba update --force
# Dev builds are otherwise skipped (version string differs from release tags)
```

{: .tip }
> After a successful update, rimba prints a one-line tip if agent files are installed at user level (`rimba init -g` to refresh) or in this repo (`rimba init --agents` to refresh). Set `RIMBA_QUIET=1` to suppress.

{: .note }
> If the binary cannot be replaced due to file permissions, rimba installs to `~/.local/bin` instead.

## Flags

| Flag | Description |
|------|-------------|
| `--force` | Update even if running a development build |

## Related commands

- [rimba version](version) · print the current version
- [rimba init](init) · refresh agent files after updating (`-g` or `--agents`)
