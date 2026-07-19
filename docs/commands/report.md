---
title: rimba report
parent: Command
nav_order: 26
---

# rimba report

Aggregate this repo's observability metrics into per-command timing stats, designed to be pasted directly into a filed GitHub issue.

rimba can record structured JSONL observability data (a log stream and a compact metrics stream) for every command it runs, written as day-files under `os.UserCacheDir()/rimba/`. `rimba report` finds this repo's day-files, folds every recorded span into per-(command, phase) count/p50/p95/mean duration stats, and prints them alongside an environment header (OS, architecture, CPU count, and rimba version(s) seen in the data).

Works even in a repo without any recorded data, or without `.rimba/` initialized at all — it simply finds zero files and reports "no data found".

## Synopsis

```sh
rimba report [--json]
```

## Examples

```sh
rimba report            # Print the env header and phase timing table
rimba report --json     # Same data as structured JSON
```

A plain `rimba report` run prints an environment header followed by a per-(command, phase) timing table:

```
OS: darwin  Arch: arm64  CPUs: 10  Rimba: 1.9.7
Versions seen in data: 1.9.6, 1.9.7
Unparseable lines: 0

COMMAND     PHASE     COUNT  P50      P95      MEAN
  add       command   12     842.0ms  1210.0ms 903.4ms
  add       deps:api  8      340.0ms  512.0ms  355.1ms
  merge     command   5      612.0ms  780.0ms  655.2ms
```

If no observability data is found for the current repo, it prints a single line instead:

```
No observability data found for this repo.
```

## Flags

| Flag | Description |
|------|-------------|
| `--json` | Output the environment header and phase stats as structured JSON |

## Related commands

- [rimba doctor](doctor) · diagnose leftover state from killed worktree operations
- [rimba version](version) · print version, commit, build date, and platform info
