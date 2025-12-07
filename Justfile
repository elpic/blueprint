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

# Clean build artifacts
clean:
    @echo "Cleaning build artifacts..."
    rm -f blueprint blueprint-*
    go clean
    @echo "✓ Cleaned"

# Show list of recipes
help:
    @echo "Blueprint Build Recipes:"
    @echo ""
    @echo "  just build          - Build for all platforms (default)"
    @echo "  just build-linux    - Build for Linux (amd64 and arm64)"
    @echo "  just build-windows  - Build for Windows (amd64)"
    @echo "  just build-macos    - Build for macOS (amd64 and arm64)"
    @echo "  just clean          - Remove all build artifacts"
    @echo "  just help           - Show this help message"
