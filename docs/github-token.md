# GITHUB_TOKEN Authentication

Blueprint supports token-based authentication for private GitHub (and GitHub Enterprise) repositories via environment variables.

## Environment Variables

| Variable | Required | Description |
|---|---|---|
| `GITHUB_TOKEN` | Yes | Personal access token or `GITHUB_ACTIONS` token |
| `GITHUB_USER` | No | GitHub username — defaults to `x-access-token` if omitted |

Setting only `GITHUB_TOKEN` is sufficient for most cases, including GitHub Actions CI pipelines.

## Usage

```bash
export GITHUB_TOKEN=ghp_yourtoken

blueprint apply my-private.blueprint
```

In GitHub Actions, `GITHUB_TOKEN` is automatically available:

```yaml
- name: Apply blueprint
  run: blueprint apply my.blueprint
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

## How It Works

When `GITHUB_TOKEN` is set, Blueprint injects HTTP Basic Auth on all HTTPS git operations (clones, fetches, remote ref lookups). The username defaults to `x-access-token`, which is the standard convention for token-based GitHub authentication.

SSH URLs (`git@github.com:...`) are unaffected — they continue to use SSH agent or key files regardless of `GITHUB_TOKEN`.

Public repositories work without any credentials; the token is only applied when present.

## Applies To

- `clone` rules with HTTPS URLs
- `include` directives pointing to private repositories
- Blueprint files fetched from private repos via shorthand (`@github:user/repo`)
