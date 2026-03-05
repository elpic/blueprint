# Mise Rules

Install and maintain the mise version manager with tool versions, either globally or scoped to a project directory:

```
mise [tool@version ...] [path: <dir>] [id: <rule-id>] [after: <dependency>] on: [platform1, platform2, ...]
```

**What is mise?**
mise (mise-en-place) is a modern polyglot tool version manager. It is faster than asdf (no shims), has native Windows support, and uses a single command to install tools. Learn more at https://mise.jdx.dev/

**Tool Syntax:**
- `tool@version` - Install a specific version (e.g., `node@20`, `python@3.11`)
- `tool` - Install the latest version of a tool
- Multiple tools can be specified in a single rule

**Options:**
- `path: <dir>` - Project directory for a local install (optional, see below)
- `id: <rule-id>` - Give this rule a unique identifier (optional)
- `after: <dependency>` - Execute after another rule (optional)
- `on: [platforms]` - Target specific platforms (optional, defaults to all)

**Global vs. project-scoped installs:**

Without `path:`, tools are installed globally (`mise use -g`), writing to `~/.config/mise/config.toml` and making them available system-wide.

With `path:`, tools are installed locally (`mise use`) in the given directory. mise writes a `mise.toml` file inside that folder, pinning the tools to that project only. The directory is created if it does not exist.

**Shell activation:**
Blueprint does not modify your shell rc files. To make globally installed tools available in interactive shells, add one of the following to your `.zshrc` / `.bashrc`:
```bash
eval "$(mise activate zsh)"   # dynamic PATH mode (recommended)
# or
eval "$(mise activate --shims zsh)"  # shims mode (similar to asdf)
```

**Behavior:**
- Installs mise via Homebrew on macOS or `curl https://mise.run | sh` on Linux if not already present
- Runs `mise use -g tool@version` for global installs, `mise use tool@version` (in the project directory) for local installs
- Auto-uninstalls tools if removed from the blueprint; removes mise itself if no global tools remain

**Examples:**
```
# Install Node.js and Python globally
mise node@20 python@3.11 on: [mac, linux]

# Install a specific version globally with a rule ID
mise node@20 id: my-node on: [mac]

# Install tools scoped to a project directory
mise node@20 python@3.11 path: ~/projects/myapp on: [mac, linux]

# Project-scoped with ID and dependency
mise node@20 path: ~/projects/myapp id: myapp-tools after: clone-myapp on: [mac]
```

**Managing tools after installation:**
```bash
# List installed tools
mise ls

# Set a version globally
mise use -g node@22

# Set a version for the current project
mise use node@22

# Show active tool versions in the current directory
mise current
```
