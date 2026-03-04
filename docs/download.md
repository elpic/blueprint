# Download Rules

Download files from URLs to specified paths:

```
download <url> to: <path> [overwrite: <true|false>] [permissions: <octal>] [id: <rule-id>] [after: <dependency>] on: [platform1, platform2, ...]
```

**What is this used for?**
Download scripts, binaries, config files, or any other file from the internet as part of your machine setup. Useful for tools not available in a package manager, custom scripts, or files hosted on private servers.

**Options:**
- `to: <path>` - Destination path for the downloaded file (supports `~/` for home directory)
- `overwrite: true|false` - If `false` (default), skips download when the file already exists. If `true`, always re-downloads (optional)
- `permissions: <octal>` - Set file permissions after download. Examples: `0755` (executable), `0600` (private). If not specified, file keeps its default permissions (optional)
- `id: <rule-id>` - Give this rule a unique identifier (optional)
- `after: <dependency>` - Execute after another rule (optional)
- `on: [platforms]` - Target specific platforms (optional)

**How it works:**
1. Expands `~` in the destination path
2. If `overwrite: false` (default) and the file already exists, skips the download
3. Creates parent directories automatically if they don't exist
4. Downloads the file via HTTP GET to a temporary file, then renames it atomically
5. Applies permissions with `chmod` if specified
6. Auto-removes the file if the rule is removed from the blueprint

**Examples:**

```blueprint
# Download a script (skip if already exists)
download https://example.com/setup.sh to: ~/bin/setup.sh

# Always re-download (overwrite existing)
download https://example.com/config.yaml to: ~/.config/app/config.yaml overwrite: true

# Download an executable with permissions
download https://example.com/tool to: ~/bin/tool permissions: 0755

# With ID and dependency
download https://example.com/script.sh to: ~/bin/script.sh permissions: 0755 id: dl-script after: mkdir-bin on: [linux, mac]

# Full example: create directory, then download into it
mkdir ~/bin id: mkdir-bin on: [mac, linux]
download https://example.com/myscript.sh to: ~/bin/myscript.sh permissions: 0755 after: mkdir-bin on: [mac, linux]
```

**Auto-Uninstall Example:**
When you remove a download rule from your blueprint, the file is automatically deleted:

```blueprint
# Before (old setup.bp)
download https://example.com/tool to: ~/bin/tool permissions: 0755

# After (new setup.bp) - rule removed

# When you run: ./blueprint apply setup.bp
# Result: ~/bin/tool is automatically removed
```

**Security notes:**
- Downloads use plain HTTP GET — prefer HTTPS URLs to avoid man-in-the-middle attacks
- Use `permissions: 0755` only for executables you trust
- Use `permissions: 0600` for private files (tokens, credentials)
- Downloaded files are written atomically via a `.tmp` file to avoid partial writes
