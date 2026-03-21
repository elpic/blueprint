# Contributing to Blueprint

Thank you for your interest in contributing to Blueprint. This guide covers everything you need to get started.

## Requirements

- Go 1.20+
- `just` (optional, for convenient build commands) -- https://github.com/casey/just#installation

## Getting Started

Clone the repository and install dependencies:

```bash
git clone https://github.com/elpic/blueprint.git
cd blueprint
go mod download
```

## Building

### Current OS

```bash
go build -o blueprint ./cmd/blueprint
```

### Cross-Platform Builds

Use `just` to build for all platforms:

```bash
# Build for all platforms (default)
just build

# Build for specific platform
just build-linux      # Linux (amd64 and arm64)
just build-windows    # Windows (amd64)
just build-macos      # macOS (amd64 and arm64)

# Clean build artifacts
just clean

# Show all available recipes
just help
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
go test ./...
```

## Project Structure

See [`docs/architecture.md`](docs/architecture.md) for a full project structure tree, component descriptions, and handler interface documentation.

## Code Style

- Run `gofmt -l .` before committing to ensure code is formatted
- Follow standard Go conventions and idioms
- Each handler lives in its own file under `internal/handlers/`
- Action documentation lives in `docs/` (one `.md` per action)
