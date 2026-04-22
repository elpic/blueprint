# Blueprint Export

Generate a standalone shell script from a blueprint file.

```
blueprint export <file.bp> [--format bash|sh] [--output <path>] [--prefer-ssh]
```

## Usage

```bash
# Print script to stdout
blueprint export setup.bp

# Save to a file (created with executable permissions)
blueprint export setup.bp --output setup.sh

# Run directly from a remote repo
blueprint export @github:elpic/setup | bash

# Use POSIX sh instead of bash
blueprint export setup.bp --format sh

# Prefer SSH for git shorthand resolution
blueprint export @github:elpic/setup --prefer-ssh
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `bash` | Shell format: `bash` (uses `set -euo pipefail`) or `sh` (uses `set -eu`) |
| `--output` | stdout | Write to a file instead of stdout |
| `--prefer-ssh` | false | Resolve git shorthand URLs via SSH instead of HTTPS |

## How it works

1. Parses the blueprint and resolves dependencies (same ordering as `apply`)
2. Detects which tools are needed (homebrew, mise, asdf, ollama) and emits install-if-missing blocks once at the top
3. Generates idempotent shell commands for each rule (checks if packages are already installed before re-installing)
4. Command output is redirected to `~/.blueprint/blueprint.log` while colored progress is shown in the terminal

## Skipped actions

Some actions cannot be exported to shell:

- **`decrypt`** -- uses blueprint's built-in AES-256-GCM decryption. The script emits a skip message with guidance to run `blueprint apply <file> --only <id>` instead.
- **`authorized_keys`** with encrypted keys -- same as above.

Skipped steps are shown in yellow in the terminal output.

## Idempotent installs

The exported script checks for existing packages before installing:

- **brew**: checks both `brew list --versions` and `brew list --cask` (handles packages like orbstack that install as casks)
- **apt**: checks `dpkg -s` before `apt-get install`
- **snap**: checks `snap list` before `snap install`
- **clone/dotfiles**: uses `git fetch` + `git reset --hard` instead of `git pull` to handle dirty repos safely
