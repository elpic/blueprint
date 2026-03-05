# Run-sh Rules

Download a shell script from a URL and execute it:

```
run-sh <url> [unless: <check>] [sudo: true|false] [undo: <command>] [id: <rule-id>] [after: <dependency>] on: [platform1, platform2, ...]
```

**What is this used for?**
Install tools whose official method is a piped curl+sh pattern (e.g. Calibre, Homebrew, Rust). Using `run-sh` is cleaner than embedding a long pipe chain in a `run` command — Blueprint downloads the script to a temp file and runs it with `sh`, so piping directly into a shell is not needed.

**Options:**
- `unless: <check>` - Skip if this check exits 0 (idempotency) (optional)
- `sudo: true` - Run the script with `sudo sh` instead of `sh` (optional, default false)
- `undo: <command>` - Command to run when this rule is removed from the blueprint (optional)
- `id: <rule-id>` - Give this rule a unique identifier (optional)
- `after: <dependency>` - Execute after another rule (optional)
- `on: [platforms]` - Target specific platforms (optional)

**How it works:**
1. If `unless:` is set, runs the check. If it exits 0, skips
2. Downloads the script from the URL to a secure temp file
3. Executes it with `sh` (or `sudo sh` if `sudo: true`)
4. Removes the temp file after execution
5. Tracks the URL in status for undo when removed from blueprint

**Examples:**

```blueprint
# Install Calibre (Linux official installer)
run-sh https://download.calibre-ebook.com/linux-installer.sh unless: calibre --version sudo: true on: [linux]

# Install a tool with undo
run-sh https://example.com/install.sh unless: some-tool --version undo: some-tool uninstall id: install-tool on: [linux]
```

**Security notes:**
- Always use HTTPS URLs to avoid man-in-the-middle attacks
- Prefer `unless:` checks so the script only runs once
- Review scripts before adding them to your blueprint
