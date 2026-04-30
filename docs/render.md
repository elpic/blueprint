# blueprint render / check / get

Use your blueprint as a single source of truth for generated files — Dockerfiles, CI configs, `.tool-versions`, Makefiles — and detect drift before it causes problems.

## Commands

### `blueprint render`

Renders a template file populated with data from the blueprint and writes the result to stdout or a file.

```bash
blueprint render <file.bp> --template <file.tmpl> [--output <path>] [--prefer-ssh]
```

```bash
# Print to stdout
blueprint render setup.bp --template Dockerfile.tmpl

# Write to a file
blueprint render setup.bp --template Dockerfile.tmpl --output Dockerfile

# Two templates, same blueprint
blueprint render setup.bp --template Dockerfile.tmpl --output Dockerfile
blueprint render setup.bp --template Dockerfile.local.tmpl --output Dockerfile.local

# Remote blueprint
blueprint render @github:user/setup --template Dockerfile.tmpl
```

---

### `blueprint check`

Renders a template and compares the result against an existing file. Exits `0` if identical, `1` if drifted. Designed for CI.

```bash
blueprint check <file.bp> --template <file.tmpl> --against <file> [--prefer-ssh]
```

```bash
blueprint check setup.bp --template Dockerfile.tmpl --against Dockerfile
```

On drift, prints a unified diff and the exact command to fix it:

```
error: Dockerfile is out of date.
--- Dockerfile (existing)
+++ Dockerfile (rendered)
-FROM ruby:3.2-slim
+FROM ruby:3.3-slim

Run to fix:
  blueprint render setup.bp --template Dockerfile.tmpl --output Dockerfile
```

**CI example (GitHub Actions):**

```yaml
- name: Check Dockerfile is up to date
  run: blueprint check setup.bp --template Dockerfile.tmpl --against Dockerfile
```

---

### `blueprint get`

Returns the value of a single field from the blueprint. Useful in Makefiles and shell scripts without needing a template.

```bash
blueprint get <file.bp> <action> <key>
```

```bash
blueprint get setup.bp mise ruby        # → 3.3.0
blueprint get setup.bp mise node        # → 20.11.0
blueprint get setup.bp asdf nodejs      # → 21.4.0
blueprint get setup.bp homebrew formula # → wget jq
blueprint get setup.bp homebrew cask    # → docker rectangle
blueprint get setup.bp packages         # → git curl vim
```

**Makefile example:**

```makefile
RUBY_VERSION := $(shell blueprint get setup.bp mise ruby)

build:
	docker build --build-arg RUBY_VERSION=$(RUBY_VERSION) .
```

---

## Template Syntax

Templates use Go's [`text/template`](https://pkg.go.dev/text/template) syntax. Blueprint provides the following functions:

| Function | Description | Example output |
|----------|-------------|----------------|
| `{{ mise "ruby" }}` | Version pinned in a `mise:` rule | `3.3.0` |
| `{{ asdf "nodejs" }}` | Version pinned in an `asdf:` rule | `21.4.0` |
| `{{ packages }}` | All `install:` packages, space-separated | `git curl vim` |
| `{{ packages "snap" }}` | `install:` packages filtered by package manager | `code` |
| `{{ homebrewFormulas }}` | All homebrew formulas, space-separated | `wget jq` |
| `{{ homebrewCasks }}` | All homebrew casks, space-separated | `docker rectangle` |
| `{{ cloneURL "myapp" }}` | Clone URL of a `clone:` rule matching the name | `https://github.com/user/myapp` |

**Missing keys fail loudly.** If you reference `{{ mise "ruby" }}` but there is no `mise ruby@...` rule in the blueprint, the render fails with a clear error rather than silently producing an empty string. A `Dockerfile` with `FROM ruby:-slim` is worse than a render error.

---

## Example: Dockerfile

**`setup.bp`:**
```
mise ruby@3.3.0 node@20.11.0
install git curl libpq-dev on: [linux]
```

**`Dockerfile.tmpl`:**
```dockerfile
FROM ruby:{{ mise "ruby" }}-slim

RUN apt-get update && apt-get install -y \
    {{ packages }} \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY . .

RUN gem install bundler
RUN bundle install
```

**`Dockerfile.local.tmpl`** (dev — no COPY, volume mounted at runtime):
```dockerfile
FROM ruby:{{ mise "ruby" }}-slim

RUN apt-get update && apt-get install -y \
    {{ packages }} \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
```

Both templates share the same blueprint as their data source. When Ruby is bumped in `setup.bp`, re-rendering both templates updates them in one step.

---

## Workflow: Keep Generated Files in Sync

1. Update the version in `setup.bp` (e.g. bump `mise ruby@3.3.0` → `ruby@3.4.0`)
2. Re-render: `blueprint render setup.bp --template Dockerfile.tmpl --output Dockerfile`
3. Commit both `setup.bp` and `Dockerfile`
4. CI enforces it stays in sync: `blueprint check setup.bp --template Dockerfile.tmpl --against Dockerfile`
