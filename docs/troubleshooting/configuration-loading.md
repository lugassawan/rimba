---
title: Configuration loading
parent: Troubleshooting
nav_order: 7
---

# Configuration loading

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
