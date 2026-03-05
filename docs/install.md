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
