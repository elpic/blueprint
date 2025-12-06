# Blueprint

Blueprint is a DSL (Domain Specific Language) based rule engine written in Go. It allows you to define and execute complex rules with conditions and actions in a declarative manner.

## Project Structure

```
.
├── cmd/
│   └── blueprint/          # Main CLI
│       └── main.go
├── internal/
│   ├── parser/             # DSL parser
│   │   └── parser.go
│   ├── engine/             # Rule executor / history
│   │   └── engine.go
│   ├── git/                # Git operations (clone, auth)
│   │   └── git.go
│   └── models/             # Shared structs
│       └── types.go
├── config/                 # Example includes
│   ├── base.bp
│   ├── dev-tools.bp
│   └── runtimes.bp
├── .gitignore              # Git ignore rules
├── justfile                # Build recipes
├── history.json            # Generated at runtime
├── setup.bp                # Example configuration
├── setup-modular.bp        # Modular example with includes
└── README.md
```

## Usage

### Build

#### For Current OS

```bash
go build -o blueprint ./cmd/blueprint
```

#### Cross-Platform Builds

Use `just` (a modern Make alternative) to build for all platforms:

```bash
# Build for all platforms (default)
just build

# Build for specific platform
just build-linux      # Linux (amd64 and arm64)
just build-windows    # Windows (amd64)
just build-macos      # macOS (amd64 and arm64)

# Show all available recipes
just help
```

**Install `just`:** https://github.com/casey/just#installation

This creates:
- `blueprint-linux-amd64` - Linux (x86_64)
- `blueprint-linux-arm64` - Linux (ARM64, e.g., Raspberry Pi, Apple Silicon)
- `blueprint-windows-amd64.exe` - Windows (x86_64)
- `blueprint-macos-amd64` - macOS Intel
- `blueprint-macos-arm64` - macOS Apple Silicon
- `blueprint` - Current OS

### Quick Start

The typical workflow is:

1. **Create a blueprint file** (`setup.bp`):
```bash
install git curl on: [mac]
install python3 ruby on: [linux, mac]
```

2. **Preview the plan** (dry-run, no changes):
```bash
./blueprint plan setup.bp
```

This shows all rules that will be executed without making any changes.

3. **Apply the blueprint** (execute rules):
```bash
./blueprint apply setup.bp
```

This executes all rules and saves the execution history.

4. **Check history** (audit log):
```bash
cat ~/.blueprint/history.json | jq '.'
```

### Run

#### Linux

```bash
# Intel/AMD x86_64
./blueprint-linux-amd64 plan setup.bp
./blueprint-linux-amd64 apply setup.bp

# ARM64 (Raspberry Pi, etc.)
./blueprint-linux-arm64 plan setup.bp
./blueprint-linux-arm64 apply setup.bp
```

#### macOS

```bash
# Intel Macs
./blueprint-macos-amd64 plan setup.bp

# Apple Silicon (M1/M2/M3)
./blueprint-macos-arm64 plan setup.bp
```

#### Windows

```cmd
blueprint-windows-amd64.exe plan setup.bp
blueprint-windows-amd64.exe apply setup.bp
```

#### Current OS

```bash
./blueprint plan setup.bp     # Dry-run mode
./blueprint apply setup.bp    # Execute rules
```

### Workflow & Examples

#### Step-by-Step Example

**1. Create your blueprint:**
```bash
cat > setup.bp << 'EOF'
install git curl on: [mac]
install python3 on: [mac, linux]
include config/dev-tools.bp
EOF
```

**2. Preview what will happen:**
```bash
./blueprint plan setup.bp
```

Output:
```
═══ [PLAN MODE - DRY RUN] ═══

Blueprint: setup.bp
Current OS: mac
Applicable Rules: 3

Rule #1:
  Action: install
  Packages: git, curl
  Command: brew install git curl

Rule #2:
  Action: install
  Packages: python3
  Command: brew install python3

[No changes will be applied]
```

**3. Execute the blueprint:**
```bash
./blueprint apply setup.bp
```

Output:
```
═══ [APPLY MODE] ═══

OS: mac
Executing 3 rules from setup.bp

[1/3] install git, curl ✓ Done
[2/3] install python3 ✓ Done
[3/3] install make ✓ Done
```

**4. Later, remove `curl` from blueprint:**
```bash
cat > setup.bp << 'EOF'
install git on: [mac]
install python3 on: [mac, linux]
EOF
```

**5. Apply again - `curl` is automatically uninstalled:**
```bash
./blueprint apply setup.bp
```

Output:
```
═══ [APPLY MODE] ═══

OS: mac
Executing 2 rules + 1 auto-uninstall from setup.bp

[1/2] install git ✓ Done
[2/2] install python3 ✓ Done
[1/1] uninstall curl ✓ Done
```

#### Dependency Management Example

You can control the execution order by specifying dependencies:

**Blueprint file:**
```bash
cat > setup.bp << 'EOF'
install git id: base-git on: [mac]
install curl after: base-git on: [mac]
install wget after: git on: [mac]
EOF
```

**Plan mode shows the dependencies:**
```bash
./blueprint plan setup.bp
```

Output:
```
═══ [PLAN MODE - DRY RUN] ═══

Blueprint: setup.bp
Current OS: mac
Applicable Rules: 3

Rule #1:
  Action: install
  ID: base-git
  Packages: git
  Command: brew install git

Rule #2:
  Action: install
  Packages: curl
  After: base-git
  Command: brew install curl

Rule #3:
  Action: install
  Packages: wget
  After: git
  Command: brew install wget

[No changes will be applied]
```

**Apply mode executes in dependency order:**
```bash
./blueprint apply setup.bp
```

Output:
```
═══ [APPLY MODE] ═══

OS: mac
Executing 3 rules from setup.bp

[1/3] install git ✓ Done
[2/3] install curl ✓ Done
[3/3] install wget ✓ Done
```

**Dependency Features:**
- Use `id: <name>` to give a rule an identifier
- Use `after: <dependency>` to specify dependencies (by ID or package name)
- Multiple dependencies: `after: dep1, dep2`
- Circular dependencies are detected and reported as errors
- Dependencies work across all rules (including auto-uninstall rules)

### Run From Git Repository

Blueprint can clone and execute blueprints from remote git repositories without requiring the git CLI installed. It automatically handles authentication and cleans up temporary directories.

#### Basic Usage

```bash
# With HTTPS URL
./blueprint plan https://github.com/username/blueprint-repo.git

# With git SSH URL (with automatic HTTPS fallback for public repos)
./blueprint plan git@github.com:username/blueprint-repo.git
```

#### Specify Branch

Clone from a specific branch:

```bash
# Using @ syntax
./blueprint plan "https://github.com/username/repo.git@develop"
./blueprint plan "git@github.com:username/repo.git@main"
```

#### Specify Custom Setup File Path

If your blueprint file is not in the root directory:

```bash
# Using : syntax
./blueprint plan "https://github.com/username/repo.git:config/setup.bp"
./blueprint plan "https://github.com/username/repo.git:blueprints/dev.bp"
```

#### Combine Branch and Path

```bash
# Using @branch:path syntax
./blueprint plan "https://github.com/username/repo.git@develop:config/setup.bp"
./blueprint plan "git@github.com:username/repo.git@staging:blueprints/setup.bp"
```

#### Authentication

For **private repositories**, set environment variables:

**HTTPS with GitHub Personal Access Token:**
```bash
export GITHUB_USER="your_username"
export GITHUB_TOKEN="your_personal_access_token"
./blueprint plan https://github.com/username/private-repo.git
```

**SSH with SSH Agent:**
```bash
# Ensure your SSH key is loaded in SSH agent
ssh-add ~/.ssh/id_rsa
./blueprint plan git@github.com:username/private-repo.git
```

#### How It Works

1. Clones repository to temporary directory
2. Finds and reads the specified setup file (defaults to `setup.bp`)
3. Parses and executes rules for your OS
4. Automatically cleans up temporary directory

**Note:** SSH URLs automatically fall back to HTTPS for public repositories if SSH authentication fails.

## DSL Format

The DSL format uses lines with keywords to define rules and include other files:

### Install Rules

Install packages on specified platforms:

```
install <package> [package2] ... [id: <rule-id>] [after: <dependency>] on: [platform1, platform2, ...]
```

**Options:**
- `id: <rule-id>` - Give this rule a unique identifier (optional)
- `after: <dependency>` - Execute after another rule (by ID or package name) (optional)

**Examples:**
```
# Simple install
install git on: [mac]

# Install with ID
install git id: setup-git on: [mac]

# Install after another package (by name)
install curl after: git on: [mac]

# Install after another rule (by ID)
install curl after: setup-git on: [mac]

# Multiple dependencies
install curl wget after: git, base-tools on: [mac]
```

### Automatic Cleanup

Blueprint automatically removes packages that were previously installed but are no longer in your blueprint configuration. It uses execution history to track what was installed and compares it with the current blueprint:

- When you remove a package from the blueprint and run `apply`, Blueprint will automatically uninstall it
- Only packages that were successfully installed on the current OS will be removed
- History is tracked per blueprint file and OS to ensure accuracy

### Include Statements

Include other blueprint files (supports relative paths and circular include prevention):

```
include path/to/other.bp
include ../config/setup.bp
```

### Examples

**Simple install rule:**
```
install git curl on: [mac]
```

**Multiple packages:**
```
install python3 ruby go on: [mac, linux]
```

**Include with relative paths:**
```
include config/dev-tools.bp
include config/runtimes.bp
```

**Complete example (setup.bp):**
```
# Main blueprint file

# Include configuration files
include config/dev-tools.bp
include config/runtimes.bp

# Install rules
install brew on: [mac]
install xcode-select on: [mac]
install git curl on: [mac]
```

**Included file (config/dev-tools.bp):**
```
# Development tools
install git curl on: [mac]
install make on: [mac, linux]
```

## Automatic Command Generation

Blueprint automatically generates the correct package manager commands for both explicit rules and automatic cleanup:

### macOS
- Install rules → `brew install <packages>`
- Auto-cleanup (removed packages) → `brew uninstall -y <packages>`

### Linux
- Install rules → `apt-get install -y <packages>`
- Auto-cleanup (removed packages) → `apt-get remove -y <packages>`

The system also automatically adds `sudo` when needed on Linux (if not running as root).

## History Tracking

Blueprint automatically saves execution history to `~/.blueprint/history.json` after each `apply` operation. This allows you to:

- Track what was executed and when
- Review command output and errors
- Maintain an audit log of all system changes

### History File Location

```
~/.blueprint/history.json
```

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

### View History

```bash
# View all execution history
cat ~/.blueprint/history.json | jq .

# View latest execution
cat ~/.blueprint/history.json | jq '.[-1]'

# Filter by blueprint
cat ~/.blueprint/history.json | jq '.[] | select(.blueprint == "/path/to/setup.bp")'
```

## Modular Blueprints

Blueprint supports modular configurations through the `include` statement. This allows you to:

- **Organize rules** into logical groups (dev-tools, databases, runtimes, etc.)
- **Reuse configurations** across multiple blueprints
- **Maintain DRY principles** by avoiding duplication
- **Prevent circular includes** automatically

### Example Structure

```
├── setup.bp                 # Main blueprint
├── config/
│   ├── dev-tools.bp        # Development tools
│   ├── runtimes.bp         # Language runtimes
│   └── databases.bp        # Database tools
└── scripts/
    └── post-install.bp     # Post-installation hooks
```

### Execution Order

Includes are processed in order, so rules from included files are executed in the sequence they appear:

```
setup.bp:
  include config/dev-tools.bp    -> Rules from dev-tools.bp
  include config/runtimes.bp     -> Rules from runtimes.bp
  install brew on: [mac]         -> Local rules
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

### Git Module (`internal/git/git.go`)
- Clones repositories using pure Go (no git CLI required)
- Supports HTTPS and SSH authentication
- Handles temporary directory management
- Auto-cleanup of cloned repositories

### Models (`internal/models/types.go`)
- Defines shared structures: `Rule` and `ExecutionHistory`
- JSON serializable

## Development

### Requirements
- Go 1.20+
- `just` (optional, for convenient build commands) - https://github.com/casey/just#installation

### Build Commands

```bash
# Build for all platforms (default)
just build

# Build for specific platform
just build-linux
just build-windows
just build-macos

# Clean build artifacts
just clean

# Show all available recipes
just help
```

Or use `go build` directly:

```bash
go build -o blueprint ./cmd/blueprint
```

### Install Dependencies

```bash
go mod download
```

### Run Tests

```bash
go test ./...
```
