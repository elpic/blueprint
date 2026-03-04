# Homebrew Rules

Install and maintain packages using Homebrew package manager:

```
homebrew <formula[@version]> [id: <rule-id>] [after: <dependency>] on: [platform1, platform2, ...]
homebrew cask: <cask-name> [id: <rule-id>] [after: <dependency>] on: [platform1, platform2, ...]
```

**What is Homebrew?**
Homebrew is a package manager for macOS and Linux that simplifies software installation and management. Learn more at https://brew.sh/

**One package per rule** — each `homebrew` line installs exactly one formula or one cask. This keeps IDs, dependency tracking, and status unambiguous.

**Formula Syntax:**
- `homebrew <formula>` - Install a formula (e.g., `git`, `wget`)
- `homebrew <formula@version>` - Install a specific version (e.g., `node@18`, `python@3.11`)

**Cask Syntax:**
- `homebrew cask: <name>` - Install a Homebrew Cask (macOS GUI apps, fonts, drivers) using `brew install --cask`

**Options:**
- `id: <rule-id>` - Give this rule a unique identifier (optional)
- `after: <dependency>` - Execute after another rule (optional)
- `on: [platforms]` - Target specific platforms (macOS: "mac", Linux: "linux") (optional)

**Behavior:**
- Automatically installs Homebrew if not already present
  - On macOS: Uses official Homebrew installation script
  - On Linux: Installs dependencies (git, curl, build-essential) then runs official script
- Thread-safe installation prevents concurrent conflicts
- Tracks installed packages and casks with version information
- Auto-uninstalls packages and casks if removed from blueprint
- Supports installation on both macOS and Linux (casks are macOS-only)

**Examples:**

```blueprint
# Formulas
homebrew git on: [mac]
homebrew node@18 on: [mac]
homebrew python@3.11 on: [mac, linux]

# Casks (macOS GUI apps)
homebrew cask: visual-studio-code on: [mac]
homebrew cask: 1password on: [mac]
homebrew cask: font-jetbrains-mono on: [mac]

# With ID for dependencies
homebrew git id: brew-git on: [mac]
homebrew node@18 after: brew-git on: [mac]
```

**Auto-Uninstall Example:**
When you remove a homebrew rule from your blueprint, the package or cask is automatically uninstalled:

```blueprint
# Before (old setup.bp)
homebrew curl on: [mac]
homebrew cask: visual-studio-code on: [mac]

# After (new setup.bp) — both rules removed
homebrew git on: [mac]

# When you run: blueprint apply setup.bp
# Result: brew uninstall -y curl
#         brew uninstall --cask -y visual-studio-code
```

**Platform Support:**
- **macOS**: Full support via Homebrew — formulas and casks
- **Linux**: Formulas only via Linuxbrew (casks are not supported on Linux)
- **Windows**: Not supported

**Security Notes:**
- Homebrew installation downloads official scripts from GitHub
- All installations use HTTPS
- Packages are verified by Homebrew's official repositories
- When removed from blueprint, packages are cleanly uninstalled
