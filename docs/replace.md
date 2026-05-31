# Replace Rules

Perform exact string find-and-replace in managed files (first occurrence only):

```
replace <path> match: <text> with: <text> [id: <rule-id>] [after: <dependency>] on: [platform1, platform2, ...]
```

**What is this used for?**
Modify configuration files, dotfiles, or any text file during machine setup. Useful for setting tokens, enabling/disabling features, changing defaults, or patching values in files you don't want to fully manage as templates.

**Options:**
- `match: <text>` - The exact text to search for (may contain spaces, reads until the next keyword)
- `with: <text>` - The replacement text (may contain spaces, supports `${VAR}` interpolation)
- `id: <rule-id>` - Give this rule a unique identifier (optional)
- `after: <dependency>` - Execute after another rule (optional)
- `on: [platforms]` - Target specific platforms (optional)

**How it works:**
1. Reads the entire file at `<path>`
2. Searches for the first occurrence of the `match:` text using an exact string search (not regex)
3. Replaces that occurrence with the `with:` text
4. Writes the modified content back to the file
5. Errors immediately if the match text is not found (no silent failures)
6. Supports `~` for home directory expansion
7. Supports `${VAR_NAME}` variable interpolation in all fields

**Examples:**

```blueprint
# Simple text replacement
replace ~/.config/app.conf match: enabled=false with: enabled=true

# Using variables for dynamic values
var SECRET_TOKEN
replace ~/.config/app.conf match: api_key= with: api_key=${SECRET_TOKEN}

# Multi-word match and replacement
replace ~/.config/shell/config match: default editor vim with: default editor nvim

# With ID for dependencies
mkdir ~/bin id: create-bin on: [mac, linux]
download https://example.com/install.sh to: ~/bin/install.sh permissions: 0755 after: create-bin on: [mac, linux]
replace ~/bin/install.sh match: /usr/local with: ~/.local after: create-bin on: [mac, linux]
```

**Auto-Uninstall Example:**
When you remove a replace rule from your blueprint, the operation is automatically reversed — the `with:` text is found and replaced back with the `match:` text:

```blueprint
# Before (old setup.bp)
replace ~/.config/app.conf match: enabled=false with: enabled=true

# After (new setup.bp) - rule removed

# When you run: ./blueprint apply setup.bp
# Result: "enabled=true" is found and replaced back to "enabled=false"
```

**Error Handling:**
- If the `match:` text is not found in the file, the rule fails immediately with a clear error message
- This prevents silent drift where a replacement doesn't actually happen because the file changed under you
- If the file does not exist, the rule fails (no implicit file creation)

**Variable Interpolation:**
The `with:` field supports `${VAR}` variable interpolation, resolved at execution time from `var` rules or `--var` CLI flags:

```blueprint
var USER_NAME

# The ${USER_NAME} will be replaced when the rule runs
replace ~/.config/git/config match: name = with: name = ${USER_NAME}
```

**Drift Detection:**
The `replace` action detects external changes (drift) by checking the actual file content, not just the status database. During `blueprint apply`, `IsInstalled()`:

1. Reads the file and checks if the `with:` text is present
2. If `with:` IS present and `match:` is absent → the replacement is still in effect → skip
3. If `match:` IS present → the file may have been reverted → re-run the replacement

This is analogous to how `elpic/actions/github/drift-check` works for templates: re-render and compare. For `replace`, the rule definition (`match:` → `with:`) is the source of truth, and the file content is checked against it.

```blueprint
# After initial apply, the file contains "enabled=true"
replace ~/.config/app.conf match: enabled=false with: enabled=true

# If someone manually reverts to "enabled=false":
# Next apply → IsInstalled finds "enabled=false" (match) → returns false
# Engine calls Up() → re-applies the replacement → "enabled=true" restored
```

The `Up()` method is also idempotent — if the `with:` text is already in place and `match:` is absent, it skips without error. This makes repeated applies safe.

**Known limitation:** The content check is best-effort. If `match:` text appears elsewhere in the file (e.g., a second occurrence that was never the first), the check may falsely detect drift. For critical files, pair `replace` with a `line-match` rule for an independent assertion.

**Security Note:**
Since `replace` modifies files in-place, be careful with the `match:` text:
- Use specific enough `match:` text to avoid false positives
- The rule only replaces the first occurrence — if the text appears multiple times, only the first is changed
- Make sure the parent file already exists before running a replace (use a `mkdir` or other rule with `after:` if needed)
