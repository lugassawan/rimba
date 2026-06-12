---
title: rimba merge-plan
parent: Command Reference
nav_order: 14
---

# rimba merge-plan

Analyze file overlaps between worktree branches and recommend an optimal merge order that minimizes conflicts. Run this before a multi-branch merge to sequence merges in the safest order.

## Synopsis

```sh
rimba merge-plan
```

## Examples

```sh
rimba merge-plan
```

```
ORDER  BRANCH       CONFLICTS
1      fix-login    0
2      auth-flow    1
3      ui-cleanup   2

Merge in this order to minimize conflicts.
```

## Common workflows

**Plan a sprint landing**
```sh
rimba merge-plan
# Merge branches in the printed order to minimize conflicts:
rimba merge fix-login
rimba merge auth-flow
rimba merge ui-cleanup
```

**Combine with conflict-check for a full picture**
```sh
rimba conflict-check   # Which files overlap?
rimba merge-plan       # In what order should I merge?
```

## Related commands

- [rimba conflict-check](conflict-check) · list files changed in multiple branches
- [rimba merge](merge) · merge a single branch into main
- [rimba sync](sync) · bring branches up to date with main before merging
