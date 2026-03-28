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
│   │   ├── parser.go       # Rule struct, all ParseXxxRule functions, prefix registry
│   │   └── fields.go       # lineFields / parseFields unified keyword extraction engine
│   ├── engine/             # Rule executor, dependency resolution, history, diff, doctor
│   │   ├── engine.go       # Main apply/run logic
│   │   ├── status.go       # Status display, diff, plan output
│   │   ├── doctor.go       # blueprint doctor check/fix logic
│   │   ├── command.go      # Shell command execution helpers
│   │   ├── crypto.go       # Encryption/decryption helpers for the engine
│   │   ├── utils.go        # Shared engine utilities
│   │   └── ps.go           # Process utilities
│   ├── handlers/           # Action handlers (one file per action)
│   │   ├── handlers.go     # Handler interfaces, StatusEntry interface, status structs
│   │   ├── registry.go     # ActionDef, RegisterAction, GetAction, RuleSummary
│   │   ├── install.go      # install — apt/brew package management
│   │   ├── clone.go        # clone — git repository cloning
│   │   ├── dotfiles.go     # dotfiles — symlink management
│   │   ├── decrypt.go      # decrypt — encrypted file management
│   │   ├── download.go     # download — file downloads
│   │   ├── run.go          # run / run-sh — shell commands and remote scripts
│   │   ├── known_hosts.go  # known-hosts — SSH known_hosts management
│   │   ├── authorized_keys.go # authorized-keys — SSH authorized_keys management
│   │   ├── gpg_key.go      # gpg-key — GPG key and repository setup
│   │   ├── homebrew.go     # homebrew — Homebrew formula/cask management
│   │   ├── asdf.go         # asdf — asdf version manager
│   │   ├── mise.go         # mise — mise version manager
│   │   ├── mkdir.go        # mkdir — directory creation
│   │   ├── sudoers.go      # sudoers — /etc/sudoers.d management
│   │   ├── shell.go        # shell — default shell switching
│   │   ├── schedule.go     # schedule — cron/launchd scheduling
│   │   ├── ollama.go       # ollama — Ollama model management
│   │   └── util.go         # Shared handler utilities
│   ├── git/                # Git operations (clone, auth, remote ref detection)
│   │   └── git.go
│   ├── crypto/             # File encryption/decryption
│   │   └── crypto.go
│   ├── ui/                 # Terminal UI formatting
│   │   └── ui.go
│   ├── logging/            # Logging utilities
│   │   └── logging.go
│   ├── platform/           # Platform abstraction interfaces and mocks
│   │   ├── interfaces.go
│   │   ├── container.go
│   │   └── mocks/
│   └── models/             # Backward-compatibility shims (Rule struct now in parser)
│       └── types.go
├── docs/                   # One .md file per action, plus architecture
├── .gitignore
├── justfile                # Build/test recipes
└── README.md
```

## Main Components

### Parser (`internal/parser/parser.go`)

- Parses `.bp` files in DSL format with include support
- Supports recursive file includes with relative paths and circular include prevention
- All parse functions are exported (`ParseInstallRule`, `ParseCloneRule`, etc.)
- Dispatch uses a prefix registry (`parsers []parseEntry`) — no hardcoded switch
- `ParseXxxRule` functions are also called from the action registry via `FindActionByPrefix`

### Action Registry (`internal/handlers/registry.go`)

Every action is described by an `ActionDef` struct registered at startup via `RegisterAction`:

```go
type ActionDef struct {
    Name        string
    Prefix      string          // parser prefix, e.g. "install ", "clone "
    NewHandler  HandlerFactory  // creates a Handler from a Rule
    RuleKey     RuleKeyFunc     // dedup/dependency key when rule.ID is empty
    Detect      DetectFunc      // detects this action type from an uninstall rule
    Summary     SummaryFunc     // human-readable description for diff/plan output
    OrphanIndex OrphanIndexFunc // indexes resource keys for orphan detection
    OrphanCheckExcluded bool    // skips key-based orphan detection (see field comment)
    AlwaysRunUp         bool    // always calls Up(), bypasses IsInstalled() check
    IsAlias             bool    // marks alias entries (excluded from status checks)
}
```

Key registry functions:
- `RegisterAction(def)` — called in each handler's `init()` function
- `GetAction(name)` — looks up an ActionDef by name
- `AllActions()` — returns all registered defs in registration order
- `FindActionByPrefix(line)` — used by the parser to dispatch lines
- `RuleSummary(rule)` — returns a human-readable summary, resolving uninstall rules via `DetectRuleType`

### Engine (`internal/engine/engine.go`)

- Executes rules sequentially (or skips via `IsInstalled()` idempotency check)
- Resolves dependencies via `KeyProvider` handler interface
- Saves execution history to `~/.blueprint/history.json`
- Supports both local files and git repositories
- Filters rules by operating system
- Calls `FindUninstallRules` to clean up removed entries from status

### Status & Diff (`internal/engine/status.go`)

- Displays current installed state (`blueprint status`)
- Computes and prints diffs between applied state and blueprint rules (`blueprint diff`)
- Uses `RuleSummary(rule)` from the registry for display; no hardcoded action strings

### Doctor (`internal/engine/doctor.go`)

- `blueprint doctor` checks `~/.blueprint/status.json` for:
  - Stale/non-normalized blueprint URLs
  - Duplicate entries
  - Orphaned entries (resource removed from blueprint)
- Orphan detection is registry-driven: each handler's `OrphanIndex` func provides resource keys
- Actions with `OrphanCheckExcluded: true` are skipped (cleanup handled by `FindUninstallRules` on apply instead)
- `blueprint doctor --fix` rewrites the status file with all issues repaired

### Handlers (`internal/handlers/`)

The handler system uses Go interfaces for type-safe, extensible rule execution:

**Core Handler Interface:**
- `Handler`
  - `Up()` — execute the rule (install/add)
  - `Down()` — undo the rule (uninstall/remove)
  - `GetCommand()` — return the command that will be executed

**Optional Handler Interfaces:**
- `KeyProvider` — `GetDependencyKey()` for dependency ordering
- `DisplayProvider` — `GetDisplayDetails(isUninstall bool)` for execution display
- `SudoAwareHandler` — `NeedsSudo()` for sudo detection
- `InstalledChecker` — `IsInstalled()` for idempotency checks

**StatusEntry Interface (`internal/handlers/handlers.go`):**

All status structs implement `StatusEntry`, which exposes:
- `GetResourceKey()`, `GetOS()`, `GetBlueprint()`, `GetAction()`
- Used by doctor, diff, and deduplication logic without type assertions

**Benefits of this architecture:**
- No hardcoded action type checks in the engine or doctor
- Adding a new action requires only: a new handler file + `RegisterAction` call in `init()`
- `OrphanIndex`, `Summary`, `RuleKey`, and `Detect` keep all action-specific logic in the handler

### Git Module (`internal/git/git.go`)

- Clones repositories (pure Go + git CLI fallback)
- Supports HTTPS and SSH authentication
- Remote ref detection with SSH fallback via `git ls-remote`

## Status Tracking

Blueprint maintains `~/.blueprint/status.json` tracking all applied resources. Updated after each `apply`.

Status entry types: packages, homebrew formulas/casks, cloned repos, dotfiles, downloads, run commands, run-sh scripts, known_hosts, authorized_keys, GPG keys, asdf/mise tools, mkdir, sudoers, shell, schedule, ollama models.

## History Tracking

Each `apply` appends a record to `~/.blueprint/history.json`:

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
