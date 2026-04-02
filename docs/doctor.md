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
Resources recorded in status whose rule no longer exists in the blueprint file they came from. This happens when you remove a rule from your blueprint but the entry lingers in status because the `apply` was never re-run.

Doctor fetches the blueprint at the exact git SHA that was applied, so the comparison is against the version you actually ran — not the current HEAD.

> **Note:** `asdf`, `mise`, and `sudoers` are excluded from orphan detection. Their status keys encode version or composite information that makes key-based comparison unreliable — orphan cleanup for these is handled automatically by `blueprint apply`.

### Stale symlinks
Dotfile symlinks in `~` whose target no longer exists (broken symlinks). This can happen if the dotfiles repo was deleted or moved.

### Missing clone directories
Clone entries in status whose local directory no longer exists on disk. This can happen if you manually deleted the cloned repo.

### Missing download files
Download entries in status whose destination file no longer exists on disk. This can happen if you manually deleted the file or a dependency removed it.

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
Repairs auto-fixable issues in place and rewrites `~/.blueprint/status.json`. For issues that cannot be auto-fixed, prints a hint explaining what to do next.

| Check | `--fix` behavior |
|-------|-----------------|
| Stale blueprint URLs | Auto-fixed — normalizes URLs in status |
| Duplicate entries | Auto-fixed — removes duplicates |
| Orphaned entries | Auto-fixed — removes orphaned entries from status |
| Stale symlinks | Auto-fixed — recreates the symlink if the source file still exists in the clone dir; removes the broken link if the source is also gone |
| Missing clone directories | **Not auto-fixed** — run `blueprint apply <file>` to restore |
| Missing download files | **Not auto-fixed** — run `blueprint apply <file>` to restore |

Missing clone directories and download files are not auto-fixed because removing the status entry would just hide the problem — the resource is still absent from disk. Re-applying the blueprint is the correct fix.

## Example output

**Check mode with issues:**
```
=== Blueprint Doctor ===

Checking status file...

Checking for orphaned entries...
  Fetching blueprint https://github.com/user/setup @ a1b2c3d4...

Checking for stale symlinks...

Checking for missing clone directories...

Checking for missing downloaded files...

  ✗ 1 orphaned entries (resource no longer exists in blueprint)
    rm -rf ~/.local/bin/vim && ln -s $(which nvim) ~/.local/bin/vim (blueprint: https://github.com/user/setup)
  ✗ 1 downloaded file(s) missing from disk
    ~/.local/bin/antigen.zsh
    Hint: Run 'blueprint apply <file>' to restore missing downloaded files.

2 issues found. Run 'blueprint doctor --fix' to repair.
```

**Fix mode:**
```
=== Blueprint Doctor (fix mode) ===

Checking status file...

Checking for orphaned entries...
  Fetching blueprint https://github.com/user/setup @ a1b2c3d4...

Checking for stale symlinks...

Checking for missing clone directories...

Checking for missing downloaded files...

  ✓ Fixed: 1 orphaned entries (resource no longer exists in blueprint)
  ℹ Cannot auto-fix: 1 downloaded file(s) missing from disk
    Hint: Run 'blueprint apply <file>' to restore missing downloaded files.

All auto-fixable issues repaired.
```

## Notes

- Doctor does not run or undo any commands — it only reads and optionally rewrites the status file.
- Orphan detection requires network access to fetch the blueprint at the recorded SHA. Blueprints that cannot be fetched are skipped (no false positives).
- For stale symlinks, the fix recreates the symlink pointing at the corresponding file inside the dotfiles clone directory. The source file path is derived from the symlink name and the clone path stored in status.
