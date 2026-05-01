# blueprint render / check / get

Use your blueprint as a single source of truth for generated files — Dockerfiles, CI configs, Makefiles, shell scripts — and detect drift before it causes problems.

The core idea: define your versions, packages, and variables **once** in `setup.bp`. Every generated file — no matter how many — stays in sync by rendering from that single source.

---

## Commands

### `blueprint render`

Renders one or more templates against a blueprint and writes the output to stdout, a file, or a directory.

```
blueprint render <file.bp> --template <file.tmpl|dir> [--output <path>] [--var KEY=VALUE] [--prefer-ssh]
```

**Single file → stdout:**
```bash
blueprint render setup.bp --template Dockerfile.tmpl
```

**Single file → file:**
```bash
blueprint render setup.bp --template Dockerfile.tmpl --output Dockerfile
```

**Entire directory → current directory:**
```bash
blueprint render setup.bp --template ./templates --output .
```

**Remote template directory → current directory:**
```bash
blueprint render @github:org/templates@main:containers/python \
  --template @github:org/templates@main:containers/python \
  --output . \
  --var APP_NAME=myapp
```

**With variables:**
```bash
blueprint render setup.bp --template Dockerfile.tmpl \
  --var APP_NAME=myapp \
  --var PORT=9000 \
  --output Dockerfile
```

---

### `blueprint check`

Renders a template and compares the result against existing files. Exits `0` if all are identical, `1` if any have drifted. Designed for CI gates.

```
blueprint check <file.bp> --template <file.tmpl|dir> --against <file|dir> [--var KEY=VALUE] [--prefer-ssh]
```

**Single file:**
```bash
blueprint check setup.bp --template Dockerfile.tmpl --against Dockerfile
```

**Entire directory:**
```bash
blueprint check setup.bp --template ./templates --against .
```

**Remote templates:**
```bash
blueprint check @github:org/templates@main:containers/python/setup.bp \
  --template @github:org/templates@main:containers/python \
  --against . \
  --var APP_NAME=myapp
```

On drift, prints a unified diff and the exact command to fix it:

```
error: Dockerfile is out of date.
--- Dockerfile (existing)
+++ Dockerfile (rendered)
-FROM python:3.12-slim
+FROM python:3.13-slim

Run to fix:
  blueprint render setup.bp --template Dockerfile.tmpl --output Dockerfile
```

**CI example (GitHub Actions):**
```yaml
- name: Check Dockerfiles are up to date
  run: |
    blueprint check setup.bp \
      --template @github:org/templates@main:containers/python \
      --against . \
      --var APP_NAME=myapp
```

---

### `blueprint get`

Returns the value of a single field from the blueprint. Useful in Makefiles and shell scripts without needing a template.

```
blueprint get <file.bp> <action> <key>
```

```bash
blueprint get setup.bp mise python      # → 3.13
blueprint get setup.bp mise ruby        # → 3.3.0
blueprint get setup.bp asdf nodejs      # → 21.4.0
blueprint get setup.bp homebrew formula # → wget jq
blueprint get setup.bp homebrew cask    # → docker rectangle
blueprint get setup.bp packages         # → git curl vim
blueprint get setup.bp var APP_NAME     # → myapp
```

**Makefile example:**
```makefile
PYTHON_VERSION := $(shell blueprint get setup.bp mise python)

build:
	docker build --build-arg PYTHON_VERSION=$(PYTHON_VERSION) .
```

---

## Directory Mode

When `--template` is a directory, blueprint collects all `*.tmpl` files recursively and renders them all in one pass.

- Files whose basename starts with `_` are skipped (use this for partials or shared snippets).
- All templates are validated before any file is written — if one fails, nothing is touched.
- `--output` becomes the root for the rendered files, preserving the directory structure relative to `--template`.

```
templates/
  containers/
    python/
      Dockerfile.tmpl          →  ./Dockerfile
      Dockerfile.local.tmpl    →  ./Dockerfile.local
      entrypoint.sh.tmpl       →  ./entrypoint.sh
      local-entrypoint.sh.tmpl →  ./local-entrypoint.sh
      .dockerignore.tmpl       →  ./.dockerignore
```

```bash
blueprint render setup.bp --template ./templates/containers/python --output .
# rendered  Dockerfile
# rendered  Dockerfile.local
# rendered  entrypoint.sh
# rendered  local-entrypoint.sh
# rendered  .dockerignore
```

The `check` command mirrors this exactly:

```bash
blueprint check setup.bp --template ./templates/containers/python --against .
# ok        Dockerfile
# ok        Dockerfile.local
# error: entrypoint.sh is out of date.
```

---

## Remote Templates

`--template` accepts the same `@provider:` shorthands and git URLs as blueprint files. The repository is cloned to the local blueprint cache (`~/.blueprint/repos/`) and reused on subsequent runs.

```bash
# Using @github: shorthand (branch is required for the path syntax)
blueprint render @github:org/templates@main:containers/python/setup.bp \
  --template @github:org/templates@main:containers/python \
  --output .

# Using a full HTTPS URL
blueprint render https://github.com/org/templates@main:containers/python/setup.bp \
  --template https://github.com/org/templates@main:containers/python \
  --output .
```

This means you can maintain a shared template library in a central repository and use it across every service — one place to update, every project benefits.

**Benefits:**
- No copy-paste of Dockerfiles between services
- Template improvements propagate to all consumers on next render
- The blueprint file (`setup.bp`) stays in the project; only the templates are remote
- The cache means subsequent renders are instant (no re-clone)

---

## Local Overrides

When rendering from a remote template directory, any `.tmpl` file found **in the output directory** with the same relative path as a remote template takes precedence over the remote version.

This lets you override specific templates for a project without forking the entire template repository.

**How it works:**

```
my-service/
  setup.bp
  entrypoint.sh.tmpl       ← local override (shadows remote entrypoint.sh.tmpl)
  local-entrypoint.sh.tmpl ← local override
```

```bash
blueprint render setup.bp \
  --template @github:org/templates@main:containers/python \
  --output . \
  --var APP_NAME=myapp

# rendered  Dockerfile
# rendered  Dockerfile.local
# rendered  entrypoint.sh (local override)
# rendered  local-entrypoint.sh (local override)
# rendered  .dockerignore
```

**Benefits:**
- Adopt shared templates for 90% of files, customise the rest
- The shared template handles Dockerfile best practices; your entrypoint handles your app's startup logic
- No forking, no drift between your customisation and upstream template updates
- Override any file — not just entrypoints — by dropping a `.tmpl` alongside your `setup.bp`

**Example: service-specific entrypoint**

The shared template ships a generic `entrypoint.sh.tmpl` with commented-out hooks. A Django service that needs migrations and collectstatic drops its own:

```sh
# my-service/entrypoint.sh.tmpl
#!/bin/sh
set -e

echo 'Running migrations'
python manage.py migrate

echo 'Running collectstatic'
python manage.py collectstatic --clear --no-input -v 0

exec "$@"
```

Everything else (`Dockerfile`, `Dockerfile.local`, `.dockerignore`) comes from the shared template unchanged.

---

## Variables

Variables let you pass project-specific values into templates without hardcoding them.

**Define in `setup.bp`:**
```
var APP_NAME myapp           # optional with default
var PORT 8000                # optional with default
var APP_NAME                 # required — must be passed via --var or defined
```

**Pass at render time:**
```bash
blueprint render setup.bp --template Dockerfile.tmpl \
  --var APP_NAME=myapp \
  --var PORT=9000
```

**Use in templates:**
```dockerfile
# Required variable — fails loudly if not set
LABEL app="{{ var "APP_NAME" }}"

# Optional with fallback — never errors
EXPOSE {{ default "PORT" "8000" }}
```

**Precedence (highest → lowest):**
1. `--var KEY=VALUE` on the command line
2. `var KEY value` in `setup.bp`
3. `{{ default "KEY" "fallback" }}` in the template

This means templates own their sensible defaults, projects can pin values in `setup.bp`, and operators can override anything at render time.

---

## Template Functions

Templates use Go's [`text/template`](https://pkg.go.dev/text/template) syntax. Blueprint provides the following functions:

| Function | Description | Example output |
|----------|-------------|----------------|
| `{{ mise "python" }}` | Version pinned in a `mise:` rule | `3.13` |
| `{{ asdf "nodejs" }}` | Version pinned in an `asdf:` rule | `21.4.0` |
| `{{ packages }}` | All `install:` packages, space-separated | `git curl libpq5` |
| `{{ packages "apt" }}` | Packages filtered by package manager | `git curl` |
| `{{ packages "" "build" }}` | Packages filtered by stage | `libpq-dev build-essential` |
| `{{ packages "" "runtime" }}` | Runtime-stage packages only | `libpq5 ca-certificates` |
| `{{ homebrewFormulas }}` | All homebrew formulas, space-separated | `wget jq` |
| `{{ homebrewCasks }}` | All homebrew casks, space-separated | `docker rectangle` |
| `{{ cloneURL "myapp" }}` | Clone URL of a matching `clone:` rule | `https://github.com/user/myapp` |
| `{{ var "KEY" }}` | Required variable — errors if not set | `myapp` |
| `{{ default "KEY" "fallback" }}` | Optional variable with fallback | `8000` |
| `{{ toArgs "cmd arg1 arg2" }}` | Converts a string to a JSON exec-form array | `["cmd","arg1","arg2"]` |

**Missing keys fail loudly.** If you reference `{{ mise "python" }}` but there is no `mise python@...` rule in the blueprint, the render fails with a clear error. A `Dockerfile` with `FROM python:-slim` is worse than a render error.

---

## Stage Filtering for Dockerfiles

The `install` action supports a `stage:` clause that lets you separate build-time and runtime system packages:

```
# setup.bp
install libpq-dev build-essential on: [linux] stage: build
install libpq5 ca-certificates on: [linux] stage: runtime
```

In a multi-stage Dockerfile template:

```dockerfile
# deps stage — build tools only, not in the final image
RUN apt-get install -y --no-install-recommends \
    {{ packages "" "build" }}

# runtime stage — minimal final image
RUN apt-get install -y --no-install-recommends \
    {{ packages "" "runtime" }}
```

This keeps the final image small while still having access to compilers and headers during dependency installation.

---

## Full Example: Python Service

**`setup.bp`:**
```
var APP_NAME myapp
var PORT 8000
var CMD uvicorn myapp.asgi:application --host 0.0.0.0 --port 8000

mise python@3.13

install libpq-dev on: [linux] stage: build
install libpq5 ca-certificates on: [linux] stage: runtime
```

**Render everything:**
```bash
blueprint render setup.bp \
  --template @github:org/templates@main:containers/python \
  --output . \
  --var APP_NAME=myapp
```

**Check drift in CI:**
```yaml
- name: Check Dockerfiles are up to date
  run: |
    blueprint check setup.bp \
      --template @github:org/templates@main:containers/python \
      --against . \
      --var APP_NAME=myapp
```

**Override the entrypoint for this service:**
```sh
# entrypoint.sh.tmpl  (local — shadows the remote template)
#!/bin/sh
set -e
python manage.py migrate
exec "$@"
```

Next render picks up your local entrypoint automatically, leaves everything else from the shared template untouched.

---

## Workflow: Keep Generated Files in Sync

1. Pin versions and variables in `setup.bp` — the single source of truth
2. Render all templates: `blueprint render setup.bp --template . --output .`
3. Commit `setup.bp` and all rendered files together
4. CI enforces they stay in sync: `blueprint check setup.bp --template . --against .`
5. When a version changes, re-render and commit — all generated files update in one step
