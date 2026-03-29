# Blueprint Validate

Parse a blueprint file and run semantic checks without applying anything.

```
blueprint validate <file.bp>
blueprint validate <git-url>
```

## What it checks

### Parse errors
Unknown directives, malformed rule syntax, missing required fields. The same errors you'd get from `blueprint apply`, but without touching the system.

### Unresolved `after:` references
Dependencies that don't match any `id:` or primary resource key in the rule set. A dangling `after:` means the dependency ordering you intended won't be applied.

### Unknown `os:` filter values
OS names that Blueprint doesn't recognize (e.g. `darwin` instead of `mac`). Rules with unknown OS filters will never run on any platform.

## Usage

```bash
# Validate a local file
blueprint validate setup.bp

# Validate a remote blueprint
blueprint validate git@github.com:user/setup.git

# Use in CI to catch issues before applying
blueprint validate setup.bp && blueprint apply setup.bp
```

Exits 0 if no issues are found, 1 if any issues are found.

## Example output

**Clean file:**
```
=== Blueprint Validate ===

Parsing setup.bp...
  ✓ parsed 24 rules

✓ No issues found.
```

**With issues:**
```
=== Blueprint Validate ===

Parsing setup.bp...
  ✓ parsed 24 rules

  ✗ rule 8 (clone ~/projects): after: "base" does not match any rule id or resource
  ✗ rule 12 (install git): unknown os filter "darwin" (valid: mac, linux, windows)

2 issues found.
```

## Notes

- `after:` accepts both plain values (`after: base-tools`) and bracket lists (`after: [base-tools, curl]`).
- Sharing the same `id:` across multiple rules is valid — it's a common pattern for grouping related rules under one dependency label.
- For git URLs, the repo is cloned/updated to `~/.blueprint/repos/` (same cache used by `apply`) before validation.
