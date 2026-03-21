# Architecture

This document describes Blueprint's internal structure, components, and design patterns. It is intended for contributors and anyone interested in how the engine works under the hood.

## Project Structure

```
.
├── cmd/
│   └── blueprint/          # Main CLI application
│       └── main.go
├── internal/
│   ├── parser/             # DSL parser
│   │   └── parser.go
│   ├── engine/             # Rule executor, dependency resolution, history
│   │   └── engine.go
│   ├── handlers/           # Rule handlers (install, clone, decrypt, etc.)
│   │   ├── handlers.go     # Handler interfaces & helper functions
│   │   ├── install.go      # Install/uninstall packages
│   │   ├── clone.go        # Clone/pull git repositories
│   │   ├── decrypt.go      # Decrypt encrypted files
│   │   ├── asdf.go         # Version manager setup
│   │   ├── homebrew.go     # Homebrew package manager setup
│   │   ├── mkdir.go        # Directory creation
│   │   ├── download.go     # File download from URLs
│   │   ├── run.go          # Run shell commands and remote scripts
│   │   ├── known_hosts.go  # SSH known_hosts management
│   │   ├── gpg_key.go      # GPG key & repository setup
│   │   ├── dotfiles.go     # Dotfiles repo cloning and symlinking
│   │   └── handlers_test.go# Handler tests
│   ├── git/                # Git operations (clone, auth)
│   │   └── git.go
│   ├── crypto/             # File encryption/decryption
│   │   └── crypto.go
│   ├── ui/                 # Terminal UI formatting
│   │   └── ui.go
│   ├── logging/            # Logging utilities
│   │   └── logging.go
│   └── models/             # Shared data structures
│       └── types.go
├── .gitignore              # Git ignore rules
├── justfile                # Build recipes
├── history.json            # Execution history (generated at runtime)
└── README.md               # Project landing page
```

## Main Components

### Parser (`internal/parser/parser.go`)

- Parses `.bp` files in DSL format with include support
- Supports recursive file includes with relative paths
- Prevents circular includes automatically
- Extracts rules and processes includes

### Engine (`internal/engine/engine.go`)

- Executes rules sequentially
- Maintains execution history
- Saves results to `history.json`
- Supports both local files and git repositories
- Automatically filters rules by operating system
- Handles asdf installation and shell integration
- Manages asdf auto-uninstall when removed from blueprint
- Uses handler interfaces for dynamic rule execution (no hardcoded action types)

### Handlers (`internal/handlers/`)

The handler system uses Go interfaces to support extensible, type-safe rule execution:

**Core Handler Interfaces:**
- `Handler` - Base interface for all handlers
  - `Up()` - Execute the rule (install/add)
  - `Down()` - Undo the rule (uninstall/remove)
  - `GetCommand()` - Return the command that will be executed

**Optional Handler Interfaces:**
- `KeyProvider` - Provides dependency resolution keys
  - `GetDependencyKey()` - Returns a unique key for dependency ordering
  - Uses `getDependencyKey()` helper function to centralize ID check logic

- `DisplayProvider` - Provides display details during execution
  - `GetDisplayDetails(isUninstall bool)` - Returns what to display (e.g., package name, path)
  - Each handler specifies its own display format without engine hardcoding

- `SudoAwareHandler` - Indicates sudo requirements
  - `NeedsSudo()` - Returns true if rule requires elevated privileges

**Handler Implementation Pattern:**

Each handler (InstallHandler, CloneHandler, DecryptHandler, DotfilesHandler, etc.) implements these interfaces to:
1. Define its execution behavior (Up/Down methods)
2. Provide dependency ordering information (KeyProvider)
3. Specify what details should be displayed (DisplayProvider)
4. Indicate sudo requirements (SudoAwareHandler)

**Benefits of this architecture:**
- No hardcoded action type checks in the engine
- Each handler is self-contained and fully responsible for its behavior
- Adding new handlers requires no changes to the engine
- Better separation of concerns: handlers define behavior, not the engine

**Helper Functions:**
- `getDependencyKey(rule, fallback)` - Centralizes rule.ID checking for all handlers
- `DetectRuleType(rule)` - Determines handler type from rule fields

### Git Module (`internal/git/git.go`)

- Clones repositories using pure Go (no git CLI required)
- Supports HTTPS and SSH authentication
- Handles temporary directory management
- Auto-cleanup of cloned repositories

### Models (`internal/models/types.go`)

- Defines shared structures: `Rule` and `ExecutionHistory`
- JSON serializable

## Automatic Command Generation

Blueprint automatically generates the correct package manager commands for both explicit rules and automatic cleanup:

### macOS
- Install rules: `brew install <packages>` (for `install` action)
- Homebrew formula rules: `brew install <formula>` (for `homebrew` action)
- Homebrew cask rules: `brew install --cask <cask>` (for `homebrew cask:` action)
- Auto-cleanup (removed packages): `brew uninstall -y <formula>` or `brew uninstall --cask -y <cask>`

### Linux
- Install rules: `apt-get install -y <packages>` (for `install` action)
- Homebrew formula rules: `brew install <formula>` (for `homebrew` action via Linuxbrew)
- Auto-cleanup (removed packages): `apt-get remove -y <packages>`

The system also automatically adds `sudo` when needed on Linux (if not running as root). Homebrew installation automatically handles dependencies and permissions on both platforms.

## Status Tracking Details

Blueprint maintains a current status file at `~/.blueprint/status.json` that tracks what is currently installed and cloned on your system. This is updated after each `apply` operation.

### Status Information

The status file tracks:
- **Packages:** Name, installation timestamp, blueprint source, OS
- **Homebrew Formulas:** Formula name, version, installation timestamp, blueprint source, OS
- **Cloned repositories:** URL, path, commit SHA, clone timestamp, blueprint source, OS
- **Dotfiles:** URL, clone path, branch, list of created symlinks, clone timestamp, blueprint source, OS
- **Downloaded files:** URL, destination path, download timestamp, blueprint source, OS
- **Run commands:** Command or script URL, undo command, execution timestamp, blueprint source, OS

### Example Output

```
=== Blueprint Status ===

Installed Packages:
  ● git (2025-12-06 01:00:00) [mac, setup.bp]
  ● curl (2025-12-06 01:05:00) [mac, setup.bp]
  ● nodejs (2025-12-06 01:10:00) [mac, setup.bp]

Installed Homebrew Formulas:
  ● git (2025-12-06 01:12:00) [mac, setup.bp]
  ● node@18 (2025-12-06 01:13:00) [mac, setup.bp]
  ● python@3.11 (2025-12-06 01:14:00) [mac, setup.bp]

Cloned Repositories:
  ● ~/.dotfiles (2025-12-06 01:15:00) [mac, setup.bp]
     URL: https://github.com/user/dotfiles.git
     SHA: abc123de
  ● ~/.asdf (2025-12-06 01:20:00) [mac, setup.bp]
     URL: https://github.com/asdf-vm/asdf.git
     SHA: def456ab
```

## History Tracking Details

Blueprint automatically saves execution history to `~/.blueprint/history.json` after each `apply` operation.

### History Record Format

Each execution creates a record with:

```json
{
  "timestamp": "2025-12-05T20:25:01-03:00",
  "blueprint": "/path/to/setup.bp",
  "os": "mac",
  "command": "brew install git curl",
  "status": "success|error",
  "output": "command output here",
  "error": "error message if failed"
}
```

### Querying History

```bash
# View all execution history
cat ~/.blueprint/history.json | jq .

# View latest execution
cat ~/.blueprint/history.json | jq '.[-1]'

# Filter by blueprint
cat ~/.blueprint/history.json | jq '.[] | select(.blueprint == "/path/to/setup.bp")'
```
