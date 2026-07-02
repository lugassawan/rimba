---
title: rimba version
parent: Command
nav_order: 23
---

# rimba version

Print version, commit hash, build date, and platform info.

## Synopsis

```sh
rimba version
```

## Examples

```sh
rimba version
```

```
rimba v1.9.5
commit: 18f05da
built:  2026-06-11T02:10:10Z
os:     darwin
arch:   arm64
go:     go1.26.4
```

## Common workflows

**Include in a bug report**
```sh
rimba version
# Copy the full output for accurate reproduction info
```

**Check before running rimba update**
```sh
rimba version   # Note current version
rimba update    # Check for newer release
```

## Related commands

- [rimba update](update) · upgrade to the latest release
