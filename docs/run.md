# Run Rules

Execute arbitrary shell commands as part of your machine setup:

```
run <command> [unless: <check>] [sudo: true|false] [undo: <command>] [id: <rule-id>] [after: <dependency>] on: [platform1, platform2, ...]
```

**What is this used for?**
Run any shell command that doesn't fit a dedicated action — custom init scripts, symlinking dotfiles, setting defaults, or anything else not expressible via `install`, `clone`, etc.

**Options:**
- `unless: <check>` - Skip the command if this check exits 0 (idempotency). Re-runs are safe (optional)
- `sudo: true` - Prepend `sudo` to the command (optional, default false)
- `undo: <command>` - Command to run when this rule is removed from the blueprint (optional)
- `id: <rule-id>` - Give this rule a unique identifier (optional)
- `after: <dependency>` - Execute after another rule (optional)
- `on: [platforms]` - Target specific platforms (optional)

**How it works:**
1. If `unless:` is set, runs the check command. If it exits 0, skips execution (already done)
2. Runs the command via `sh -c`. If `sudo: true`, prepends `sudo`
3. Tracks the command in status so it can be undone when removed from the blueprint
4. On removal, runs the `undo:` command if one was specified

**Examples:**

```blueprint
# Idempotent setup with undo
run touch ~/.hello-done unless: test -f ~/.hello-done undo: rm -f ~/.hello-done on: [mac, linux]

# Run with sudo
run sysctl -w vm.max_map_count=262144 unless: test "$(sysctl -n vm.max_map_count)" = "262144" sudo: true on: [linux]

# With ID and dependency
run ln -sf ~/dotfiles/.zshrc ~/.zshrc unless: test -L ~/.zshrc undo: rm -f ~/.zshrc id: link-zshrc after: clone-dotfiles on: [mac, linux]
```

**Auto-Uninstall Example:**
When you remove a `run` rule that has an `undo:` command, the undo runs automatically:

```blueprint
# Before (old setup.bp)
run touch ~/.hello-done unless: test -f ~/.hello-done undo: rm -f ~/.hello-done on: [mac]

# After (new setup.bp) — rule removed

# When you run: blueprint apply setup.bp
# Result: rm -f ~/.hello-done is automatically executed
```

> **Note:** Only rules with an `undo:` command trigger auto-cleanup. Rules without `undo:` are silently removed from tracking with no side effects.
