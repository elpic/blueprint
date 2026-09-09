# Install Rules

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

## Installing a prerelease

The installer uses the latest stable release by default. To install the newest
GitHub prerelease instead:

```bash
curl -fsSL https://install.getbp.dev | sh -s -- --pre-release
```

To install an exact release or prerelease version:

```bash
curl -fsSL https://install.getbp.dev | sh -s -- --version 0.59.0-rc.1
```
