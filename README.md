# Blueprint

Blueprint is a DSL (Domain Specific Language) based rule engine written in Go. It allows you to define and execute complex rules with conditions and actions in a declarative manner.

## Installation

You can install Blueprint using the installation script:

```bash
curl -fsSL https://raw.githubusercontent.com/elpic/blueprint/main/install.sh | sh
```

Or download the latest binary from [releases](https://github.com/elpic/blueprint/releases).

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
│   │   ├── mkdir.go        # Directory creation
│   │   ├── known_hosts.go  # SSH known_hosts management
│   │   ├── gpg_key.go      # GPG key & repository setup
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
└── README.md               # This file
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

3. **Encrypt sensitive files** (optional):
```bash
./blueprint encrypt ~/.ssh/id_rsa --password-id main
```

This creates `~/.ssh/id_rsa.enc` which can be decrypted in your blueprint.

4. **Apply the blueprint** (execute rules):
```bash
./blueprint apply setup.bp
```

This executes all rules and saves the execution history. If decrypt rules are present, you'll be prompted for passwords upfront.

5. **Check current status** (what's installed):
```bash
./blueprint status
```

Shows all packages and cloned repositories currently managed by Blueprint.

6. **Check history** (audit log):
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

## Skip Options

You can selectively skip rules during plan or apply operations using `--skip-group` and `--skip-id` flags:

### Skip by Group

Skip all rules in a specific group:

```bash
# Plan mode - show what would be executed without specific group
./blueprint plan setup.bp --skip-group vim

# Apply mode - execute all rules except those in the group
./blueprint apply setup.bp --skip-group vim
```

**Behavior when skipping:**
- Matching rules are not executed
- Passwords are not prompted for (if decrypt rules)
- State is not modified
- Rules show as "skipped" in output

### Skip by ID

Skip a specific rule by its unique identifier:

```bash
# Skip a single rule
./blueprint plan setup.bp --skip-id decrypt-ssh-key
./blueprint apply setup.bp --skip-id my-custom-install
```

### Combine Skip Options

You can use both flags together:

```bash
# Skip both a group and a specific rule
./blueprint apply setup.bp --skip-group vim --skip-id backup-decrypt
```

### Use Cases

- **Skip optional tools:** Skip a group when certain tools aren't needed
- **Skip sensitive operations:** Skip decrypt rules if you're not ready to set up secrets
- **Skip long-running tasks:** Skip time-consuming clones during quick updates
- **Skip problematic rules:** Temporarily skip rules that are failing

**Example blueprint with groups:**

```blueprint
install vim group: vim on: [mac]
install neovim group: vim on: [mac]

decrypt secrets.enc to: ~/.secrets group: security password-id: main on: [mac]
decrypt ssh-key.enc to: ~/.ssh/id_rsa group: security password-id: main on: [mac]

clone https://github.com/user/dotfiles.git to: ~/.dotfiles id: clone-dotfiles on: [mac]
```

**Usage examples:**

```bash
# Skip all vim tools
./blueprint apply setup.bp --skip-group vim

# Skip security decryption
./blueprint apply setup.bp --skip-group security

# Skip dotfiles clone
./blueprint apply setup.bp --skip-id clone-dotfiles

# Skip both vim tools and security decryption
./blueprint apply setup.bp --skip-group vim --skip-group security
```

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

### Clone Rules

Clone and maintain git repositories at specified paths:

```
clone <repo-url> to: <path> [id: <rule-id>] [branch: <branch>] [after: <dependency>] on: [platform1, platform2, ...]
```

**Options:**
- `to: <path>` - Destination path for cloning (supports `~/` for home directory)
- `branch: <branch>` - Specific branch to clone (optional, defaults to repository default)
- `id: <rule-id>` - Give this rule a unique identifier (optional)
- `after: <dependency>` - Execute after another rule (optional)

**Behavior:**
- If repository doesn't exist: Clones it and records the commit SHA
- If repository exists: Fetches and pulls latest changes, compares SHA
- If SHA changes: Updates and reports the change
- If SHA unchanged: Reports "Already up to date"
- Supports dependencies between clone rules

**Examples:**
```
# Simple clone
clone https://github.com/user/myrepo.git to: ~/projects/myrepo on: [mac]

# Clone specific branch
clone https://github.com/user/myrepo.git to: ~/projects/myrepo branch: develop on: [mac]

# Clone with ID for dependency
clone https://github.com/user/dotfiles.git to: ~/.dotfiles id: setup-dotfiles on: [mac]

# Clone after another rule
clone https://github.com/user/tools.git to: ~/tools after: setup-dotfiles on: [mac]
```

### ASDF Rules

Install and maintain the asdf version manager with plugins and specific versions:

```
asdf [plugin@version ...] [id: <rule-id>] [after: <dependency>] on: [platform1, platform2, ...]
```

**What is asdf?**
asdf is a version manager that can handle multiple runtime versions for different languages and tools. Learn more at https://asdf-vm.com/

**Plugin Syntax:**
- `plugin@version` - Install a specific version of a plugin (e.g., `nodejs@18.19.0`)
- `plugin` - Reference a plugin without installing a specific version
- Multiple plugins can be specified in a single rule
- The first version listed for each plugin is set as the global default

**Options:**
- `id: <rule-id>` - Give this rule a unique identifier (optional, defaults to "asdf")
- `after: <dependency>` - Execute after another rule (optional)
- `on: [platforms]` - Target specific platforms (optional, defaults to all)

**Behavior:**
- Clones asdf from https://github.com/asdf-vm/asdf.git to `~/.asdf` if not already installed
- Automatically adds plugins specified in the rule
- Installs specified versions and sets them as global defaults
- Automatically sources asdf in shell configuration files (`.bashrc`, `.bash_profile`, `.zshrc`, `.zsh_profile`)
- If asdf is already installed, checks for updates and pulls latest changes
- Tracks asdf version via commit SHA
- Auto-uninstalls asdf if removed from blueprint (like packages)

**Examples:**
```
# Simple asdf installation (no plugins)
asdf on: [mac, linux]

# Install asdf with Node.js versions
asdf nodejs@18.19.0 nodejs@21.4.0 on: [mac, linux]

# With ID for dependencies
asdf nodejs@18.19.0 python@3.11.0 id: asdf-tool on: [mac]

# Multiple plugins with specific versions
asdf nodejs@18.19.0 ruby@3.2.0 python@3.11.0 id: languages on: [mac, linux]

# asdf with package that depends on it
asdf nodejs@18.19.0 id: asdf-tool on: [mac]
install nodejs-build-tools on: [mac] after: asdf-tool
```

**Complete example with multiple tools:**
```
# Install asdf version manager with specific language versions
asdf nodejs@18.19.0 nodejs@21.4.0 python@3.11.0 id: version-manager on: [mac, linux]

# Install Node.js build tools after asdf is ready
install nodejs-build-tools on: [mac] after: version-manager

# Install Git
install git on: [mac]
```

**Managing versions after installation:**
Once asdf is set up with Blueprint, you can manage versions manually:
```bash
# Set project-specific version (creates .tool-versions file)
asdf local nodejs 21.4.0

# List all installed versions of a plugin
asdf versions nodejs

# Change global version
asdf global nodejs 21.4.0
```

### Known Hosts Rules

Manage SSH known_hosts file entries for host verification:

```
known_hosts <host> [key-type: <type>] [id: <rule-id>] [after: <dependency>] on: [platform1, platform2, ...]
```

**What is this used for?**
Automatically add SSH hosts to your `~/.ssh/known_hosts` file to prevent "Host key verification failed" prompts during SSH connections.

**Options:**
- `key-type: <type>` - SSH key type to scan for (ed25519, ecdsa, rsa). If not specified, auto-detects in order: ed25519 → ecdsa → rsa (optional)
- `id: <rule-id>` - Give this rule a unique identifier (optional)
- `after: <dependency>` - Execute after another rule (optional)
- `on: [platforms]` - Target specific platforms (optional)

**How it works:**
1. Creates `~/.ssh` directory with permissions 0700 (if not exists)
2. Creates `~/.ssh/known_hosts` file with permissions 0600 (if not exists)
3. Uses `ssh-keyscan` to retrieve host public key
4. Adds host entry to known_hosts file to prevent verification prompts
5. Automatically tries multiple key types with fallback if one fails

**Examples:**

```blueprint
# Simple known_hosts entry (auto-detects key type)
known_hosts github.com on: [mac, linux]

# Explicit key type
known_hosts github.com key-type: ed25519 on: [mac, linux]

# With ID for dependencies
known_hosts gitlab.com key-type: rsa id: gitlab-host on: [mac]

# Multiple hosts with different key types
known_hosts github.com key-type: ed25519 id: github on: [mac, linux]
known_hosts gitlab.com key-type: rsa id: gitlab on: [mac, linux]
known_hosts bitbucket.org key-type: ecdsa id: bitbucket on: [mac, linux]

# With dependencies
clone https://github.com/user/private-repo.git to: ~/projects/repo after: github on: [mac]
known_hosts github.com id: github on: [mac]
clone https://github.com/user/private-repo.git to: ~/projects/repo after: github on: [mac]
```

### Mkdir Rules

Create directories with optional permission settings:

```
mkdir <path> [permissions: <octal>] [id: <rule-id>] [after: <dependency>] on: [platform1, platform2, ...]
```

**What is this used for?**
Create directory structures that your applications need. Useful for setting up project directories, cache directories, data directories, and other folder hierarchies with specific permission requirements.

**Options:**
- `permissions: <octal>` - Set directory permissions in octal (0-777). Examples: 700 (rwx------), 755 (rwxr-xr-x), 750 (rwxr-x---). If not specified, uses system default umask (optional)
- `id: <rule-id>` - Give this rule a unique identifier (optional)
- `after: <dependency>` - Execute after another rule (optional)
- `on: [platforms]` - Target specific platforms (optional)

**How it works:**
1. Creates parent directories automatically using `mkdir -p` (like Unix mkdir with -p flag)
2. Applies permissions with `chmod` if specified
3. Idempotent - safe to run multiple times, won't fail if directory exists
4. Supports `~` for home directory expansion

**Common Permission Values:**
- `700` - Owner only (drwx------)
- `755` - Owner full, others read+execute (drwxr-xr-x)
- `750` - Owner full, group read+execute (drwxr-x---)
- `777` - Everyone full access (drwxrwxrwx)

**Examples:**

```blueprint
# Simple directory creation
mkdir ~/.config on: [mac, linux]

# With specific permissions
mkdir ~/.config permissions: 700 on: [mac, linux]

# Nested directories (parent directories created automatically)
mkdir ~/.config/myapp/data permissions: 750 on: [mac, linux]

# With ID for dependencies
mkdir ~/projects id: create-projects on: [mac, linux]

# Multiple directories with dependencies
mkdir ~/projects id: create-projects on: [mac, linux]
mkdir ~/projects/myapp after: create-projects on: [mac, linux]
mkdir ~/projects/myapp/data permissions: 700 after: create-projects on: [mac, linux]

# Project structure setup
mkdir ~/workspace/projects permissions: 755 id: setup-workspace on: [mac, linux]
mkdir ~/workspace/projects/active permissions: 750 after: setup-workspace on: [mac, linux]
mkdir ~/workspace/archive permissions: 700 after: setup-workspace on: [mac, linux]
```

**Security Note:**
When creating directories that will hold sensitive data, use restricted permissions:
- `700` for private directories (owner only)
- `750` for team access (owner read/write/execute, group read/execute, others nothing)
- Avoid `777` unless absolutely necessary

### GPG Key Rules

Add GPG keys and configure Debian repositories with signature verification:

```
gpg-key <url> keyring: <name> deb-url: <url> [id: <rule-id>] [after: <dependency>] on: [platform1, platform2, ...]
```

**What is this used for?**
Add GPG keys for Debian package repositories to enable secure package verification. This is commonly used when adding third-party package repositories.

**Options:**
- `keyring: <name>` - Name for the keyring file (stored as `/usr/share/keyrings/<name>.gpg`)
- `deb-url: <url>` - Debian repository URL for the sources.list entry
- `id: <rule-id>` - Give this rule a unique identifier (optional)
- `after: <dependency>` - Execute after another rule (optional)
- `on: [platforms]` - Target specific platforms (Linux only) (optional)

**How it works:**
1. Downloads GPG key from URL using `curl`
2. Converts key from ASCII-armored format to binary using `gpg --dearmor`
3. Stores key in `/usr/share/keyrings/<name>.gpg`
4. Adds repository source to `/etc/apt/sources.list.d/<name>.list` with signature verification
5. Sets keyring file permissions to 0644
6. Runs `sudo apt update` to refresh package cache
7. Auto-uninstalls when rule is removed from blueprint

**Examples:**

```blueprint
# Simple GPG key and repository
gpg-key https://apt.fury.io/wez/gpg.key keyring: wezterm-fury deb-url: https://apt.fury.io/wez/ on: [linux]

# With ID for dependencies
gpg-key https://example.com/repo.key keyring: example-repo deb-url: https://example.com/apt id: example-setup on: [linux]

# Multiple repositories
gpg-key https://apt.fury.io/wez/gpg.key keyring: wezterm-fury deb-url: https://apt.fury.io/wez/ on: [linux]
gpg-key https://keyserver.ubuntu.com/export/key.asc keyring: ubuntu-ppa deb-url: https://ppa.launchpad.net/example/ppa/ubuntu on: [linux]

# With dependencies
install curl id: curl-setup on: [linux]
gpg-key https://apt.fury.io/wez/gpg.key keyring: wezterm-fury deb-url: https://apt.fury.io/wez/ after: curl-setup on: [linux]
install wezterm after: wezterm-fury on: [linux]
```

**Command Executed:**
The following commands are executed for a gpg-key rule:
```bash
curl -fsSL <url> | sudo gpg --yes --dearmor -o /usr/share/keyrings/<name>.gpg
echo 'deb [signed-by=/usr/share/keyrings/<name>.gpg] <deb-url> * *' | sudo tee /etc/apt/sources.list.d/<name>.list
sudo chmod 644 /usr/share/keyrings/<name>.gpg
sudo apt update
```

**Security notes:**
- Keys are verified using GPG's standard verification
- Repository entries use signed-by flag for secure verification
- Keyring files are world-readable but only root can modify
- When removed from blueprint, both the key and repository source are deleted
- Works only on Linux systems with apt package manager

### Decrypt Rules

Decrypt encrypted files to specified locations with optional password protection:

```
decrypt <encrypted-file> to: <destination> [group: <group>] [password-id: <id>] [id: <rule-id>] [after: <dependency>] on: [platform1, platform2, ...]
```

**What is this used for?**
Safely store sensitive files (SSH keys, certificates, config files) in encrypted form in your repository, then decrypt them during blueprint execution.

**Options:**
- `to: <destination>` - Where to decrypt the file (supports `~/` for home directory)
- `group: <group>` - Group name for grouping related decrypt rules (optional)
- `password-id: <id>` - Unique identifier for password grouping (optional, defaults to "default")
- `id: <rule-id>` - Give this rule a unique identifier (optional)
- `after: <dependency>` - Execute after another rule (optional)
- `on: [platforms]` - Target specific platforms (optional)

**How it works:**
1. Blueprint prompts for all unique `password-id` values at the start of execution
2. Encrypted files are decrypted using the provided password
3. Decrypted files are written with restricted permissions (0600)
4. Multiple decrypt rules can share the same `password-id` (prompted only once)
5. Works with both local files and git repository blueprints

**Examples:**

```blueprint
# Simple decrypt
decrypt id_rsa.enc to: ~/.ssh/id_rsa password-id: main on: [mac, linux]

# Multiple decrypts with same password
decrypt id_rsa.enc to: ~/.ssh/id_rsa password-id: main on: [mac, linux]
decrypt config.enc to: ~/.config/app.conf password-id: main on: [mac, linux]

# With group for organization
decrypt ssh-key.enc to: ~/.ssh/id_rsa group: security password-id: main on: [mac]
decrypt ssl-cert.enc to: ~/.certs/cert.pem group: security password-id: main on: [mac]

# With dependencies
asdf nodejs@18.19.0 id: setup-node on: [mac]
decrypt npm-token.enc to: ~/.npmrc password-id: npm-creds after: setup-node on: [mac]
```

**Creating encrypted files:**

Use the `encrypt` command to create encrypted files for your blueprint:

```bash
# Encrypt a file with default password-id
./blueprint encrypt ~/.ssh/id_rsa

# Encrypt with specific password-id
./blueprint encrypt ~/.ssh/id_rsa --password-id main

# Creates: ~/.ssh/id_rsa.enc
```

This creates `~/.ssh/id_rsa.enc` which can be added to version control safely.

**Security notes:**
- Encrypted files use AES-256-GCM encryption
- Each encryption uses a random nonce
- Passwords are derived using SHA-256
- Decrypted files are written with 0600 permissions (user-only)
- Passwords are cached during execution but never saved
- Works with repository-based blueprints (clones to temp directory)

### Automatic Cleanup

Blueprint automatically removes packages and tools that were previously installed but are no longer in your blueprint configuration. It uses execution history to track what was installed and compares it with the current blueprint:

- When you remove a package from the blueprint and run `apply`, Blueprint will automatically uninstall it
- When you remove an asdf rule and run `apply`, Blueprint will automatically uninstall asdf
- Only packages that were successfully installed on the current OS will be removed
- History is tracked per blueprint file and OS to ensure accuracy

**Example:** If you have asdf installed and then remove the `asdf` rule from your blueprint:
```
# Before (old setup.bp)
asdf on: [mac]
install git on: [mac]

# After (new setup.bp)
install git on: [mac]

# When you run: ./blueprint apply setup.bp
# Result: asdf is automatically uninstalled, ~/.asdf directory removed,
#         and shell configuration files cleaned up
```

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

# Install asdf version manager
asdf id: version-manager on: [mac, linux]

# Install tools that depend on asdf
install nodejs on: [mac] after: version-manager
install python3 on: [mac, linux] after: version-manager

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

## Status Tracking

Blueprint maintains a current status file at `~/.blueprint/status.json` that tracks what is currently installed and cloned on your system. This is updated after each `apply` operation.

### Check Current Status

View what is currently installed and cloned:

```bash
./blueprint status
```

**Output example:**
```
=== Blueprint Status ===

Installed Packages:
  ● git (2025-12-06 01:00:00) [mac, setup.bp]
  ● curl (2025-12-06 01:05:00) [mac, setup.bp]
  ● nodejs (2025-12-06 01:10:00) [mac, setup.bp]

Cloned Repositories:
  ● ~/.dotfiles (2025-12-06 01:15:00) [mac, setup.bp]
     URL: https://github.com/user/dotfiles.git
     SHA: abc123de
  ● ~/.asdf (2025-12-06 01:20:00) [mac, setup.bp]
     URL: https://github.com/asdf-vm/asdf.git
     SHA: def456ab
```

**Status file location:**
```
~/.blueprint/status.json
```

### Status Information

The status file tracks:
- **Packages:** Name, installation timestamp, blueprint source, OS
- **Cloned repositories:** URL, path, commit SHA, clone timestamp, blueprint source, OS

This information is useful for:
- Understanding what your current system setup includes
- Tracking which blueprint file installed what
- Seeing commit SHAs for cloned repositories
- Auditing your system configuration

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
Each handler (InstallHandler, CloneHandler, DecryptHandler, etc.) implements these interfaces to:
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
