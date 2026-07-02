---
title: Trust & consent gate
parent: Troubleshooting
nav_order: 1
---

# Trust & consent gate

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
