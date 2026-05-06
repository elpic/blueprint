# Clone Rules

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

**Authentication:**

For private repositories, set `GITHUB_TOKEN` (and optionally `GITHUB_USER`) in your environment. See [github-token.md](github-token.md) for details.

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
