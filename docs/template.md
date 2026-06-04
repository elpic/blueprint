# blueprint template

Scaffold a project from a template directory — interactively prompts for all variables and renders every file in one step.

Unlike `blueprint render` (which requires you to pass `--var KEY=VALUE` for every variable), `blueprint template` discovers the variables automatically and asks you for each one. It is designed for **scaffolding** — creating a new project from a shared template repository.

```
blueprint template <template-path> --output <output-dir> [--var KEY=VALUE] [--prefer-ssh]
```

## Arguments

| Argument | Description |
|----------|-------------|
| `<template-path>` | Path to a template directory — local path, `@github:` shorthand, or git URL |
| `--output <dir>` | Output directory where rendered files are written (**required**) |
| `--var KEY=VALUE` | Pre-set a template variable (repeatable) — skips the prompt for that variable |
| `--prefer-ssh` | Prefer SSH over HTTPS for git operations |

## How It Works

### 1. Resolve template source

The template path supports the same formats as `blueprint render`:

| Format | Example |
|--------|---------|
| Local directory | `./my-template` |
| `@github:` shorthand | `@github:org/templates@main:python-service` |
| SSH URL | `git@github.com:org/templates.git` |
| HTTPS URL | `https://github.com/org/templates.git` |

Remote templates are cloned to `~/.blueprint/repos/` and cached for subsequent runs.

### 2. Load defaults from `setup.bp`

If the template directory contains a `setup.bp` file, its `var` rules provide default values:

```bp
var PORT 8000           # optional — shown as default, can be overridden
var APP_NAME            # required — must provide a value
```

### 3. Discover variables from templates

All `.tmpl` files are scanned for variable references:

| Template syntax | Behavior |
|----------------|----------|
| `{{ var "NAME" }}` | Required — prompts with no default |
| `{{ toValue "NAME" }}` | Required — prompts with no default |
| `{{ default "NAME" "val" }}` | Optional — uses `"val"` as the default in the prompt |

### 4. Interactive prompt

Each variable is presented one at a time:

- **Required variables** are shown in **yellow**: `APP_NAME (required):`
- **Optional variables** are shown in **blue**: `PORT (default: 8000):`

Press **Enter** to accept the default for optional variables. Required variables loop until you provide a value.

Variables passed via `--var KEY=VALUE` on the command line skip the prompt entirely. Use this to provide complex values (JSON arrays, long strings) or to automate from scripts.

```bash
blueprint template @github:org/templates@main:drift-check \
  --output ./my-api \
  --var CHECKS='[{"file":"setup.bp","template":"@github:org/templates@main:go","against":"."}]'
```

### 5. Render

Once all variables are collected, every `.tmpl` file in the template directory is rendered into `--output`. The `.tmpl` extension is stripped from output filenames, and the directory structure is preserved.

## Examples

### Scaffold from a local template

```bash
blueprint template ./my-template --output ./my-project
```

### Scaffold from a remote template library

```bash
blueprint template @github:org/templates@main:python-service \
  --output ./my-new-api
```

### Scaffold with pre-set variables

```bash
blueprint template @github:org/templates@main:drift-check \
  --output ./my-api \
  --var CHECKS='[{"file":"setup.bp","template":"@github:org/templates@main:go","against":"."}]' \
  --var TIMEOUT_MINUTES=15
```

Variables passed with `--var` are not prompted — useful for complex values or automation.

### Scaffold using SSH for git operations

```bash
blueprint template @github:org/templates@main:web-app \
  --output ./frontend \
  --prefer-ssh
```

## Creating a Template Directory

A blueprint template is a directory with:

1. **`.tmpl` files** — Go templates that use `{{ var "NAME" }}`, `{{ toValue "NAME" }}`, and `{{ default "NAME" "fallback" }}` to declare variables
2. **`setup.bp`** (optional) — declares `var` rules with default values

Example structure:

```
python-service/
  setup.bp              # var PORT 8000  /  var APP_NAME
  Dockerfile.tmpl        # FROM python:3.12\nEXPOSE {{ var "PORT" }}
  entrypoint.sh.tmpl     # exec {{ var "APP_NAME" }}
  config/
    settings.py.tmpl     # PORT = {{ default "PORT" "8000" }}
```

When a user runs:

```bash
blueprint template @github:org/templates@main:python-service \
  --output ./my-api
```

They are prompted for `PORT` (default: 8000) and `APP_NAME` (required), then the full tree is rendered into `./my-api/`.

## Variable Precedence

The value used at render time follows this order (highest wins):

1. Value entered at the prompt
2. `var KEY value` in the template's `setup.bp`
3. `{{ default "KEY" "fallback" }}` in the template file

This means templates own sensible defaults, template authors can pin custom values in `setup.bp`, and users always have the final say at the prompt.

## Comparison with `blueprint render`

| | `blueprint render` | `blueprint template` |
|--|-------------------|---------------------|
| Requires a `.bp` file | Yes | No (optional for defaults) |
| Variables | Pass via `--var` | Interactive prompts |
| Use case | Updating existing projects | Scaffolding new projects |
| `--var` support | Yes | Yes (skips prompt) |
