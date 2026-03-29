# Blueprint Doctor

Inspect `~/.blueprint/status.json` for issues and optionally repair them in place.

```
blueprint doctor [--fix]
```

## What it checks

### Stale blueprint URLs
Status entries written before URL normalization may store blueprint sources in non-canonical form (e.g. `https:/github.com/user/repo` with a single slash, or with a trailing `.git`). These entries are functionally correct but cause false positives in duplicate detection.

### Duplicate entries
Two or more entries tracking the same resource on the same OS from the same blueprint (after URL normalization). This can happen if `apply` was interrupted or if status was manually edited.

### Orphaned entries
Resources recorded in status whose rule no longer exists in the blueprint file they came from. This happens when you remove a rule from your blueprint but the entry lingers in status because there was no `undo:` command or the `apply` was never re-run.

Doctor fetches the blueprint at the exact git SHA that was applied, so the comparison is against the version you actually ran — not the current HEAD.

## Usage

**Check only (default):**
```bash
blueprint doctor
```
Prints all issues found and exits with code 1 if any exist. Nothing is written.

**Fix mode:**
```bash
blueprint doctor --fix
```
Repairs all issues in place and rewrites `~/.blueprint/status.json`:
- Normalizes stale blueprint URLs
- Removes duplicate entries
- Removes orphaned entries

## Example output

```
=== Blueprint Doctor ===

Checking status file...

Checking for orphaned entries...
  Fetching blueprint https://github.com/user/setup @ a1b2c3d4...

  ✗ 1 orphaned entries (resource no longer exists in blueprint)
    rm -rf ~/.local/bin/vim && ln -s $(which nvim) ~/.local/bin/vim (blueprint: https://github.com/user/setup)

1 issue found. Run 'blueprint doctor --fix' to repair.
```

```
=== Blueprint Doctor (fix mode) ===

Checking status file...

Checking for orphaned entries...
  Fetching blueprint https://github.com/user/setup @ a1b2c3d4...

  ✓ Fixed: 1 orphaned entries (resource no longer exists in blueprint)

All issues fixed.
```

## Notes

- Doctor does not run or undo any commands — it only reads and optionally rewrites the status file.
- Orphan detection requires network access to fetch the blueprint at the recorded SHA. Blueprints that cannot be fetched are skipped (no false positives).
- Actions like `asdf`, `mise`, and `sudoers` are excluded from key-based orphan detection because their status entries store composite keys. Orphan cleanup for these is handled automatically by `blueprint apply`.
