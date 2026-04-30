# Blueprint

**Declarative machine setup, one line at a time.**

Blueprint is a DSL-based rule engine that lets you define your entire development environment in a plain-text `.bp` file and apply it with a single command.

```
install git curl on: [mac]
install python3 ruby on: [mac, linux]
clone https://github.com/user/dotfiles.git to: ~/.dotfiles on: [mac]
dotfiles ~/.dotfiles on: [mac]
```

## Installation

```bash
curl -fsSL https://install.getbp.dev | sh
```

Or download the latest binary from [releases](https://github.com/elpic/blueprint/releases).

## Quick Start

**1. Create a blueprint file** (`setup.bp`):
```
install git curl on: [mac]
install python3 on: [mac, linux]
```

**2. Preview the plan** (dry-run, no changes):
```bash
blueprint plan setup.bp
```

**3. Apply the blueprint** (execute rules):
```bash
blueprint apply setup.bp
```

**4. Check current status** (what's installed):
```bash
blueprint status
```

That's it. Blueprint generates the correct package manager commands for your OS, tracks what it installed, and automatically cleans up packages you remove from the file.

## Actions

Each line in a `.bp` file maps to an action. Full documentation for each action lives in [`docs/`](docs/):

| Action | Description | Platforms |
|--------|-------------|-----------|
| [`install`](docs/install.md) | Install packages via the system package manager | mac, linux |
| [`clone`](docs/clone.md) | Clone and keep a git repository up to date | mac, linux |
| [`asdf`](docs/asdf.md) | Install the asdf version manager with plugins and versions | mac, linux |
| [`mise`](docs/mise.md) | Install the mise version manager globally or scoped to a project | mac, linux |
| [`homebrew`](docs/homebrew.md) | Install Homebrew formulas and casks | mac, linux |
| [`known_hosts`](docs/known-hosts.md) | Add SSH hosts to `~/.ssh/known_hosts` | mac, linux |
| [`authorized_keys`](docs/authorized-keys.md) | Add SSH public keys to `~/.ssh/authorized_keys` | mac, linux |
| [`mkdir`](docs/mkdir.md) | Create directories with optional permissions | mac, linux |
| [`download`](docs/download.md) | Download a file from a URL | mac, linux |
| [`run`](docs/run.md) | Execute an arbitrary shell command | mac, linux |
| [`run-sh`](docs/run-sh.md) | Download and execute a shell script from a URL | mac, linux |
| [`dotfiles`](docs/dotfiles.md) | Clone a dotfiles repo and symlink entries into `~` | mac, linux |
| [`gpg_key`](docs/gpg-key.md) | Add a GPG key and configure a Debian repository | linux |
| [`decrypt`](docs/decrypt.md) | Decrypt AES-256-GCM encrypted files | mac, linux |
| [`sudoers`](docs/sudoers.md) | Grant a user passwordless sudo via `/etc/sudoers.d/` | mac, linux |
| [`ollama`](docs/ollama.md) | Pull and manage local LLM models via Ollama | mac, linux |
| [`schedule`](docs/schedule.md) | Install a crontab entry to run blueprint on a schedule | mac, linux |
| [`shell`](docs/shell.md) | Set the default login shell | mac, linux |

All actions share common optional clauses:
- `id: <rule-id>` -- unique identifier for dependency references
- `after: <id>` -- run after the named rule
- `on: [mac, linux]` -- restrict to specific platforms

## Key Features

### Automatic Cleanup

Blueprint tracks what it installs. When you remove a package from the blueprint and re-apply, it is automatically uninstalled:

```
# Before
install git curl on: [mac]

# After (curl removed)
install git on: [mac]

# blueprint apply setup.bp -> curl is auto-uninstalled
```

### Dependency Ordering

Control execution order with `id` and `after`:

```
install git id: base-git on: [mac]
install curl after: base-git on: [mac]
```

Multiple dependencies are supported: `after: dep1, dep2`. Circular dependencies are detected and reported as errors.

### Skip Rules

Selectively skip rules during plan or apply with `--skip-group` and `--skip-id`:

```bash
blueprint apply setup.bp --skip-group vim
blueprint apply setup.bp --skip-id decrypt-ssh-key
blueprint apply setup.bp --skip-group vim --skip-group security
```

### Run From a Git Repository

Apply blueprints directly from a remote repo -- no local clone needed:

```bash
# HTTPS
blueprint plan https://github.com/user/repo.git

# SSH (auto-falls back to HTTPS for public repos)
blueprint plan git@github.com:user/repo.git

# Specific branch
blueprint plan "https://github.com/user/repo.git@develop"

# Custom file path
blueprint plan "https://github.com/user/repo.git:config/setup.bp"

# Branch + path
blueprint plan "https://github.com/user/repo.git@develop:config/setup.bp"
```

For private repos, set `GITHUB_USER` and `GITHUB_TOKEN` (HTTPS) or load your SSH key into the agent.

### Export to Shell Script

Generate a standalone shell script from a blueprint -- useful for machines without blueprint installed, CI pipelines, or Dockerfiles:

```bash
# Preview the generated script
blueprint export setup.bp

# Save to a file
blueprint export setup.bp --output setup.sh

# Run directly from a remote repo
blueprint export @github:elpic/setup | bash

# Choose POSIX sh instead of bash
blueprint export setup.bp --format sh --output setup.sh
```

The exported script is fully standalone: it bootstraps prerequisites (homebrew, mise, asdf, ollama) if missing, checks for already-installed packages before re-installing, and logs command output to `~/.blueprint/blueprint.log` while showing colored progress in the terminal.

Actions that cannot be exported (like `decrypt`) are shown as skipped with guidance on how to run them via blueprint directly.

### Template Rendering & Drift Detection

Use your blueprint as a single source of truth for generated files — Dockerfiles, CI configs, `.tool-versions`, Makefiles — and catch drift before it causes problems.

**Render a template:**

```bash
# Dockerfile.tmpl
blueprint render setup.bp --template Dockerfile.tmpl
blueprint render setup.bp --template Dockerfile.tmpl --output Dockerfile
blueprint render setup.bp --template Dockerfile.local.tmpl --output Dockerfile.local
```

**Example `Dockerfile.tmpl`:**

```dockerfile
FROM ruby:{{ mise "ruby" }}-slim

RUN apt-get update && apt-get install -y \
    {{ packages }} \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY . .
```

**Check for drift in CI** (exits `1` with a diff if the file is out of date):

```yaml
- name: Check Dockerfile is up to date
  run: blueprint check setup.bp --template Dockerfile.tmpl --against Dockerfile
```

**Query a single value** (useful in Makefiles and shell scripts):

```bash
blueprint get setup.bp mise ruby       # → 3.3.0
blueprint get setup.bp asdf nodejs     # → 21.4.0
blueprint get setup.bp homebrew cask   # → docker rectangle

# Use in a Makefile
RUBY_VERSION := $(shell blueprint get setup.bp mise ruby)
docker build --build-arg RUBY_VERSION=$(RUBY_VERSION) .
```

**Available template functions:**

| Function | Returns |
|----------|---------|
| `{{ mise "ruby" }}` | Version pinned in `mise:` rule |
| `{{ asdf "nodejs" }}` | Version pinned in `asdf:` rule |
| `{{ packages }}` | All `install:` packages, space-separated |
| `{{ packages "snap" }}` | Packages filtered by package manager |
| `{{ homebrewFormulas }}` | All homebrew formulas, space-separated |
| `{{ homebrewCasks }}` | All homebrew casks, space-separated |
| `{{ cloneURL "myapp" }}` | Clone URL for a matching `clone:` rule |

Missing keys fail loudly — a `Dockerfile` with `FROM ruby:-slim` is worse than a render error.

### Modular Blueprints

Split configuration across files with `include`:

```
# setup.bp
include config/dev-tools.bp
include config/runtimes.bp
install brew on: [mac]
```

Includes are processed in order. Circular includes are detected and prevented automatically.

### Encrypt and Decrypt

Protect sensitive files with AES-256-GCM encryption:

```bash
blueprint encrypt ~/.ssh/id_rsa --password-id main
```

Then decrypt them in your blueprint:

```
decrypt secrets.enc to: ~/.secrets password-id: main on: [mac]
```

### Status Tracking

Blueprint maintains `~/.blueprint/status.json` to track installed packages, cloned repos, dotfiles symlinks, downloaded files, and executed commands. View it with:

```bash
blueprint status
```

### History

Every `apply` operation is logged to `~/.blueprint/history.json` with timestamps, commands, outputs, and statuses. View it with:

```bash
cat ~/.blueprint/history.json | jq '.'
```

## Cross-Platform Support

Blueprint automatically generates the correct commands for your OS:

| OS | Package manager | Example |
|----|----------------|---------|
| macOS | `brew install` | `install git on: [mac]` |
| Linux | `apt-get install -y` | `install git on: [linux]` |

Homebrew formulas and casks are supported on both platforms via the `homebrew` action. Sudo is added automatically on Linux when needed.

<details>
<summary>Running platform-specific binaries</summary>

```bash
# Linux
./blueprint-linux-amd64 apply setup.bp   # Intel/AMD
./blueprint-linux-arm64 apply setup.bp   # ARM64

# macOS
./blueprint-macos-amd64 apply setup.bp   # Intel
./blueprint-macos-arm64 apply setup.bp   # Apple Silicon

# Windows
blueprint-windows-amd64.exe apply setup.bp
```

</details>

## Further Reading

- [`docs/`](docs/) -- full documentation for every action
- [`docs/export.md`](docs/export.md) -- generate standalone shell scripts from blueprints
- [`docs/render.md`](docs/render.md) -- render templates and detect drift with `render`, `check`, and `get`
- [`docs/doctor.md`](docs/doctor.md) -- inspect and repair `~/.blueprint/status.json`
- [`docs/validate.md`](docs/validate.md) -- parse and semantic-check a blueprint without applying
- [`docs/architecture.md`](docs/architecture.md) -- project structure, engine internals, handler interfaces
- [`CONTRIBUTING.md`](CONTRIBUTING.md) -- development setup, build commands, testing
