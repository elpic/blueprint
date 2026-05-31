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
Since `replace` only modifies the first occurrence, detecting external changes (drift) is inherently limited. The current implementation checks the status database only — if there's a record saying the replacement was performed, `IsInstalled` returns `true` regardless of the file's actual content.

If someone manually reverts the file, the replacement won't be re-applied automatically. To detect drift reliably, pair `replace` with a `line-match` rule (coming in a separate PR) that independently asserts a specific line exists:

```blueprint
# Perform the initial replacement
replace ~/.config/app.conf match: enabled=false with: enabled=true

# Independently assert the result — catches manual reverts
# line-match ~/.config/app.conf contains: enabled=true
```

**Security Note:**
Since `replace` modifies files in-place, be careful with the `match:` text:
- Use specific enough `match:` text to avoid false positives
- The rule only replaces the first occurrence — if the text appears multiple times, only the first is changed
- Make sure the parent file already exists before running a replace (use a `mkdir` or other rule with `after:` if needed)
