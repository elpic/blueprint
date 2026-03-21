# Contributing to Blueprint

Thank you for your interest in contributing to Blueprint. This guide covers everything you need to get started.

## Requirements

- [mise](https://mise.jdx.dev) — manages Go version and task runner

## Getting Started

Clone the repository and install dependencies:

```bash
git clone https://github.com/elpic/blueprint.git
cd blueprint
mise install   # installs Go and golangci-lint at the versions in mise.toml
```

## Building

### Current OS

```bash
go build -o blueprint ./cmd/blueprint
```

### Cross-Platform Builds

Use `mise run` to build for all platforms:

```bash
# Build for all platforms (default)
mise run build

# Build for specific platform
mise run build:linux      # Linux (amd64 and arm64)
mise run build:windows    # Windows (amd64)
mise run build:macos      # macOS (amd64 and arm64)

# Clean build artifacts
mise run clean

# Show all available tasks
mise tasks
```

This creates:
- `blueprint-linux-amd64` -- Linux (x86_64)
- `blueprint-linux-arm64` -- Linux (ARM64, e.g., Raspberry Pi)
- `blueprint-windows-amd64.exe` -- Windows (x86_64)
- `blueprint-macos-amd64` -- macOS Intel
- `blueprint-macos-arm64` -- macOS Apple Silicon
- `blueprint` -- Current OS

## Running Tests

```bash
mise run test        # run all tests
mise run lint        # run golangci-lint
mise run check       # run tests + lint + security scan
```

Or directly with Go:

```bash
go test ./...
```

## Project Structure

See [`docs/architecture.md`](docs/architecture.md) for a full project structure tree, component descriptions, and handler interface documentation.

## Code Style

- Run `gofmt -l .` before committing to ensure code is formatted
- Follow standard Go conventions and idioms
- Each handler lives in its own file under `internal/handlers/`
- Action documentation lives in `docs/` (one `.md` per action)
