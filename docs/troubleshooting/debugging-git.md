---
title: Debugging git operations
parent: Troubleshooting
nav_order: 8
---

# Debugging git operations

Pass `--debug` to any command (or set `RIMBA_DEBUG=1`) to log every git command and its timing to
stderr. This is useful for diagnosing slow operations or unexpected git output.

```sh
rimba list --debug
rimba sync auth --debug
RIMBA_DEBUG=1 rimba exec --all "git status"
```

The debug output shows the command, arguments, working directory, and elapsed time. Compare against
what you'd expect from running the git commands directly to narrow down the problem.
