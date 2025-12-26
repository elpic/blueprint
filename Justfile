# Blueprint build recipes

# Default recipe - build for all platforms
default: build

# Build for all platforms (Linux, Windows, macOS)
build: build-linux build-windows build-macos
  @echo "✓ Built binaries for all platforms"

# Build for Linux (amd64 and arm64)
build-linux:
  @echo "Building for Linux (amd64)..."
  GOOS=linux GOARCH=amd64 go build -o blueprint-linux-amd64 ./cmd/blueprint
  @echo "Building for Linux (arm64)..."
  GOOS=linux GOARCH=arm64 go build -o blueprint-linux-arm64 ./cmd/blueprint
  @echo "✓ Linux binaries built"

# Build for Windows (amd64)
build-windows:
  @echo "Building for Windows (amd64)..."
  GOOS=windows GOARCH=amd64 go build -o blueprint-windows-amd64.exe ./cmd/blueprint
  @echo "✓ Windows binary built"

# Build for macOS (amd64 and arm64)
build-macos:
  @echo "Building for macOS (amd64)..."
  GOOS=darwin GOARCH=amd64 go build -o blueprint-macos-amd64 ./cmd/blueprint
  @echo "Building for macOS (arm64)..."
  GOOS=darwin GOARCH=arm64 go build -o blueprint-macos-arm64 ./cmd/blueprint
  @echo "✓ macOS binaries built"

# Run tests with optional flags
# Usage: just test [flags]
# Examples: just test "-v", just test "--coverage -v"
# Flags: -v, --verbose, --coverage
test FLAGS="":
  #!/bin/bash
  set -e

  # Parse flags
  VERBOSE=""
  COVERAGE=""

  if [[ "{{FLAGS}}" == *"-v"* ]] || [[ "{{FLAGS}}" == *"--verbose"* ]]; then
    VERBOSE="-v"
  fi
  if [[ "{{FLAGS}}" == *"--coverage"* ]]; then
    COVERAGE="-coverprofile=coverage.out"
  fi

  if [[ -n "$COVERAGE" ]]; then
    go test $VERBOSE $COVERAGE ./...
  else
    go test $VERBOSE ./...
  fi

# Format code with gofmt
format:
  @echo "Formatting code..."
  go fmt ./...
  @echo "✓ Code formatted"

# Run linter (golangci-lint)
lint:
  @echo "Running linter..."
  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin
  $(go env GOPATH)/bin/golangci-lint run ./... --timeout=5m
  @echo "✓ Lint checks passed"

# Run security scanner (gosec)
security:
  @echo "Running security scan..."
  curl -sfL https://raw.githubusercontent.com/securego/gosec/master/install.sh | sh -s -- -b $(go env GOPATH)/bin
  $(go env GOPATH)/bin/gosec -exclude=G204,G304,G115 ./...
  @echo "✓ Security scan completed"

# Run quick checks (test and security, no lint)
verify: test security
  @echo "✓ All checks passed"

# Run all checks (test, lint, security)
check: test lint security
  @echo "✓ All checks passed"

# Clean build artifacts
clean:
  @echo "Cleaning build artifacts..."
  rm -f blueprint blueprint-*
  go clean -testcache
  @echo "✓ Cleaned"

# Show list of recipes
help:
  @echo "Blueprint Build Recipes:"
  @echo ""
  @echo "BUILD:"
  @echo "  just build          - Build for all platforms (default)"
  @echo "  just build-linux    - Build for Linux (amd64 and arm64)"
  @echo "  just build-windows  - Build for Windows (amd64)"
  @echo "  just build-macos    - Build for macOS (amd64 and arm64)"
  @echo ""
  @echo "TEST:"
  @echo "  just test                      - Run all tests"
  @echo "  just test \"-v\"                 - Run all tests with verbose output"
  @echo "  just test \"--coverage\"         - Run all tests with coverage report"
  @echo "  just test \"--coverage -v\"      - Run all tests with coverage and verbose"
  @echo ""
  @echo "QUALITY CHECKS:"
  @echo "  just format         - Format code with gofmt"
  @echo "  just lint           - Run golangci-lint"
  @echo "  just security       - Run security scan (gosec)"
  @echo "  just verify         - Run quick checks (test and security, no lint)"
  @echo "  just check          - Run all checks (test, lint, security)"
  @echo ""
  @echo "MAINTENANCE:"
  @echo "  just clean          - Remove all build artifacts"
  @echo "  just help           - Show this help message"
