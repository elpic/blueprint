# Clone Rules

Clone and maintain git repositories at specified paths.

```
clone <url> to: <path> [branch: <branch>] [id: <rule-id>] [after: <dependency>] [bare: true] [workdir: true] on: [platform1, platform2, ...]
```

## Options

| Option | Required | Description |
|--------|----------|-------------|
| `to:` | ✅ | Destination path. Supports `~/` for home directory, and `${VAR}` for variable interpolation. |
| `branch:` | ❌ | Specific branch or tag to clone. Defaults to the repository's default branch (usually `main` or `master`). |
| `id:` | ❌ | Give this rule a unique identifier. If omitted, auto-generated as `clone-<URL>`. Used for `after:` dependencies. |
| `after:` | ❌ | Execute only after the named rule (by `id:`) completes successfully. |
| `on:` | ❌ | Platform filter. Clone only runs on matching operating systems. Example: `on: [mac, linux]`. |
| `bare:` | ❌ | When set to `true`, clones bare into `<path>/.git` with the branch as a worktree at `<path>/<branch>` — the layout [worktrunk](https://worktrunk.dev) expects. Cannot be combined with `workdir:`. |
| `workdir:` | ❌ | When set to `true`, clones directly to the target path with the `.git` directory intact (full working copy). Cannot be combined with `bare:`. Default behavior (without either option) uses a two-stage cache-and-copy strategy. |

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

### Bare clone (`bare: true`)

When `bare: true` is set, blueprint clones bare into `<path>/.git` and checks out a branch as a
worktree — the layout git worktree managers such as [worktrunk](https://worktrunk.dev) expect:

```
myproject/
├── .git/       # bare repository
├── main/       # default branch worktree
└── feature/    # feature branch worktree (created by `wt switch --create feature`)
```

Because the bare repository has no working tree of its own, no branch gets special treatment —
every branch is a linked worktree at an equal path.

- The worktree is created for `branch:` when set, otherwise for the repository's default branch
- On subsequent runs blueprint **fetches only**: new branches and commits become visible to
  `wt switch`, but existing worktrees are never reset — they may hold uncommitted work
- Because worktrees are left alone, a worktree checked out on the default branch does not
  advance on its own; pull inside it (or `wt merge`) to pick up new commits
- Removing the rule removes the whole directory, bare repository and worktrees included

Worktrunk prompts to configure `worktree-path` the first time you switch in a bare repository;
accepting it places worktrees in subdirectories as shown above. Blueprint does not write
worktrunk's own configuration.

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

For `bare: true` clones the tracked SHA is the tip of the tracked remote branch, not the bare
repository's HEAD: a fetch only advances `refs/remotes/origin/*`, since moving
`refs/heads/<branch>` would yank the branch out from under whichever worktree has it checked
out. `Updated` therefore means "new commits were fetched", not "your worktree moved".

## Authentication

For **private repositories**, set `GITHUB_TOKEN` (and optionally `GITHUB_USER`) in your environment. See [github-token.md](github-token.md) for details.

- HTTPS URLs use token-based auth (`GITHUB_TOKEN`)
- SSH URLs (`git@github.com:...`) use your SSH agent or key files — unaffected by `GITHUB_TOKEN`
- Public repositories work without any credentials

## Uninstall

Clone rules support automatic uninstall. When a clone rule is removed from your blueprint, the next `blueprint apply` will detect the removed rule and delete the target directory.

Simply remove the `clone` line from your `.bp` file and re-apply — blueprint handles the rest.

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

# Bare clone for git worktrees (worktrunk): ~/projects/app/.git + ~/projects/app/main
clone git@github.com:user/app.git to: ~/projects/app bare: true on: [mac]

# Bare clone pinned to a specific branch's worktree
clone git@github.com:user/app.git to: ~/projects/app branch: develop bare: true on: [mac, linux]

# With ID for dependency resolution
clone https://github.com/user/dotfiles.git to: ~/.dotfiles id: setup-dotfiles on: [mac]

# Clone after another rule completes
clone https://github.com/user/tools.git to: ~/tools after: setup-dotfiles on: [mac]

# Variable interpolation in path
clone @github:${ORG}/${REPO} to: ~/projects/${REPO_NAME} workdir: true on: [mac, linux]
```
