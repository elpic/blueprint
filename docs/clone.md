# Clone Rules

Clone and maintain git repositories at specified paths.

```
clone <url> to: <path> [branch: <branch>] [id: <rule-id>] [after: <dependency>] [workdir: true] on: [platform1, platform2, ...]
```

## Options

| Option | Required | Description |
|--------|----------|-------------|
| `to:` | ✅ | Destination path. Supports `~/` for home directory, and `${VAR}` for variable interpolation. |
| `branch:` | ❌ | Specific branch or tag to clone. Defaults to the repository's default branch (usually `main` or `master`). |
| `id:` | ❌ | Give this rule a unique identifier. If omitted, auto-generated as `clone-<URL>`. Used for `after:` dependencies. |
| `after:` | ❌ | Execute only after the named rule (by `id:`) completes successfully. |
| `on:` | ❌ | Platform filter. Clone only runs on matching operating systems. Example: `on: [mac, linux]`. |
| `workdir:` | ❌ | When set to `true`, clones directly to the target path with the `.git` directory intact (full working copy). Default behavior (without this option) uses a two-stage cache-and-copy strategy. |

## URL Formats

Clone accepts any repository URL — HTTPS, SSH, or the shorthand form:

| Format | Example |
|--------|---------|
| HTTPS | `https://github.com/user/repo.git` |
| SSH | `git@github.com:user/repo.git` |
| Shorthand | `@github:user/repo` |

**Shorthand providers** expand automatically to full URLs:

| Prefix | Expands to |
|--------|------------|
| `@github:` | `https://github.com/` / `git@github.com:` |
| `@gitlab:` | `https://gitlab.com/` / `git@gitlab.com:` |
| `@bitbucket:` | `https://bitbucket.org/` / `git@bitbucket.org:` |
| `@codeberg:` | `https://codeberg.org/` / `git@codeberg.org:` |

When `--prefer-ssh` is set (or `BLUEPRINT_PREFER_SSH` env var), shorthand URLs resolve to SSH format instead of HTTPS.

## Behavior

### Two-stage clone (default)

When `workdir:` is **not** set, blueprint uses a two-stage strategy:

1. **Cache in clean storage** — The repository is cloned/fetched to `~/.blueprint/repos/`, a cache directory that acts as the single source of truth.
2. **Rsync to target** — Files are copied (via `rsync --delete`, excluding `.git`) from the cache to the target path.

The target directory **does not contain a `.git` folder** — it is a snapshot of the repository's contents at the cloned SHA. This prevents accidental pushes from consumed/project repos.

### Direct clone (`workdir: true`)

When `workdir: true` is set, blueprint clones directly to the target path with the `.git` directory preserved:

- The target becomes a fully functional working copy
- On subsequent runs, `git pull` (fast-forward only) updates instead of re-cloning
- Use this for repos you intend to develop in

### SHA tracking

Blueprint tracks the commit SHA of every cloned repository in `~/.blueprint/status.json`:

- On each run, the remote HEAD SHA is compared to the stored SHA
- If unchanged, the clone is **skipped** (no network fetch)
- If changed, the repository is updated and the new SHA recorded
- Drift detection (`blueprint check`) uses the stored SHA to detect when a cloned repo has been modified

### Status messages

| Message | Meaning |
|---------|---------|
| `Cloned` | Repository was freshly cloned to the target |
| `Updated` | Remote had new commits; SHA changed since last run |
| `Synced` | Content was re-copied from cache but SHA is the same |
| `Already up to date` | No new commits; target is current |

## Authentication

For **private repositories**, set `GITHUB_TOKEN` (and optionally `GITHUB_USER`) in your environment. See [github-token.md](github-token.md) for details.

- HTTPS URLs use token-based auth (`GITHUB_TOKEN`)
- SSH URLs (`git@github.com:...`) use your SSH agent or key files — unaffected by `GITHUB_TOKEN`
- Public repositories work without any credentials

## Uninstall

Clone rules support automatic uninstall. When a clone rule is removed from a blueprint, running `blueprint apply --prune` removes the target directory.

You can also manually uninstall:

```bash
blueprint apply setup.bp uninstall clone https://github.com/user/repo.git to: ~/projects/repo
```

## Examples

```blueprint
# Simple clone (default branch)
clone https://github.com/user/myrepo.git to: ~/projects/myrepo on: [mac]

# Specific branch
clone https://github.com/user/myrepo.git to: ~/projects/myrepo branch: develop on: [mac]

# Shorthand URL (expands to full HTTPS URL automatically)
clone @github:user/dotfiles to: ~/.dotfiles on: [mac]

# Direct clone with .git (full working copy)
clone git@github.com:user/tools.git to: ~/tools workdir: true on: [mac]

# With ID for dependency resolution
clone https://github.com/user/dotfiles.git to: ~/.dotfiles id: setup-dotfiles on: [mac]

# Clone after another rule completes
clone https://github.com/user/tools.git to: ~/tools after: setup-dotfiles on: [mac]

# Variable interpolation in path
clone @github:${ORG}/${REPO} to: ~/projects/${REPO_NAME} workdir: true on: [mac, linux]
```
