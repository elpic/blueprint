# ASDF Rules

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
