# Dotfiles Rules

Clone a dotfiles git repository and symlink every top-level item into your home directory:

```
dotfiles <url> [branch: <branch>] [id: <rule-id>] [after: <dependency>] on: [platform1, platform2, ...]
```

**What is this used for?**
Manage your shell config, editor settings, and other dotfiles via a git repo. Blueprint clones the repo and creates symlinks in `~` for every top-level entry, so your dotfiles are always version-controlled and portable.

**Options:**
- `branch: <branch>` - Checkout a specific branch (optional, defaults to repo default)
- `id: <rule-id>` - Give this rule a unique identifier (optional, auto-generated as `dotfiles-<reponame>`)
- `after: <dependency>` - Execute after another rule (optional)
- `on: [platforms]` - Target specific platforms (optional)

**How it works:**
1. Clones the repository to `~/.blueprint/dotfiles/<reponame>` (or updates it if already cloned)
2. Reads every top-level entry in the cloned repo
3. Skips entries whose name starts with `readme` (case-insensitive) and the `.github` folder
4. For each remaining entry, creates a symlink `~/<name>` → `~/.blueprint/dotfiles/<reponame>/<name>`
5. If a symlink already points to the correct source — skips it (idempotent)
6. If a real file exists at the target — warns and skips (never overwrites user files)

**Examples:**

```blueprint
# Link all dotfiles from a GitHub repo (mac and linux)
dotfiles https://github.com/user/dotfiles on: [mac, linux]

# Use a specific branch with a custom ID
dotfiles https://github.com/user/dotfiles branch: main id: my-dotfiles on: [mac]
```

**Auto-uninstall:**
When you remove a `dotfiles` rule from your blueprint and run `apply`, Blueprint removes the symlinks it created and deletes the local clone.

**Clone path:**
The repository is always cloned to `~/.blueprint/dotfiles/<reponame>`. For `https://github.com/user/dotfiles.git` this becomes `~/.blueprint/dotfiles/dotfiles`.

**Skip rules applied to top-level entries:**
- Names with case-insensitive prefix `readme` (e.g. `README.md`, `readme.rst`, `Readme`)
- Exact name `.github`
