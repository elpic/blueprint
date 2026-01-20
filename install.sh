#!/bin/sh

# Blueprint installation script
# Detects the operating system and installs the appropriate binary

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Detect OS and architecture
detect_os_arch() {
	OS=$(uname -s)
	ARCH=$(uname -m)

	case "$OS" in
		Linux)
			OS="linux"
			;;
		Darwin)
			OS="macos"
			;;
		*)
			printf "${RED}Error: Unsupported operating system: %s${NC}\n" "$OS"
			exit 1
			;;
	esac

	case "$ARCH" in
		x86_64)
			ARCH="amd64"
			;;
		aarch64)
			ARCH="arm64"
			;;
		arm64)
			ARCH="arm64"
			;;
		*)
			printf "${RED}Error: Unsupported architecture: %s${NC}\n" "$ARCH"
			exit 1
			;;
	esac
}

# Get the latest release version
get_latest_version() {
	# Try to fetch from GitHub releases API
	VERSION=$(curl -s https://api.github.com/repos/elpic/blueprint/releases/latest | grep -o '"tag_name": "[^"]*"' | cut -d'"' -f4)

	if [ -z "$VERSION" ]; then
		printf "${RED}Error: Could not determine latest version${NC}\n"
		exit 1
	fi

	# Remove 'v' prefix if present
	VERSION=$(echo "$VERSION" | sed 's/^v//')
}

# Download and install the binary
install_binary() {
	BINARY_NAME="blueprint-${OS}-${ARCH}"
	DOWNLOAD_URL="https://github.com/elpic/blueprint/releases/download/v${VERSION}/${BINARY_NAME}"
	INSTALL_PATH="${INSTALL_PATH:-/usr/local/bin/blueprint}"

	printf "${YELLOW}Downloading blueprint %s for %s/%s...${NC}\n" "$VERSION" "$OS" "$ARCH"

	# Create temporary file
	TMP_FILE=$(mktemp)
	trap "rm -f $TMP_FILE" EXIT

	# Download the binary
	if ! curl -fsSL -o "$TMP_FILE" "$DOWNLOAD_URL"; then
		printf "${RED}Error: Failed to download blueprint from %s${NC}\n" "$DOWNLOAD_URL"
		exit 1
	fi

	# Check if download was successful
	if [ ! -s "$TMP_FILE" ]; then
		printf "${RED}Error: Downloaded file is empty${NC}\n"
		exit 1
	fi

	# Make it executable
	chmod +x "$TMP_FILE"

	# Move to install location
	printf "${YELLOW}Installing to %s...${NC}\n" "$INSTALL_PATH"

	# Check if we need sudo
	if [ ! -w "$(dirname "$INSTALL_PATH")" ]; then
		if ! sudo mv "$TMP_FILE" "$INSTALL_PATH"; then
			printf "${RED}Error: Failed to install blueprint. You may need to run this script with sudo.${NC}\n"
			exit 1
		fi
		if ! sudo chmod +x "$INSTALL_PATH"; then
			printf "${RED}Error: Failed to make blueprint executable${NC}\n"
			exit 1
		fi
	else
		if ! mv "$TMP_FILE" "$INSTALL_PATH"; then
			printf "${RED}Error: Failed to install blueprint${NC}\n"
			exit 1
		fi
		chmod +x "$INSTALL_PATH"
	fi

	printf "${GREEN}✓ Blueprint installed successfully at %s${NC}\n" "$INSTALL_PATH"
	printf "${GREEN}✓ Version: %s${NC}\n" "$VERSION"
}

# Verify installation
verify_installation() {
	if ! command -v blueprint >/dev/null 2>&1; then
		printf "${YELLOW}Warning: blueprint not found in PATH${NC}\n"
		printf "${YELLOW}You may need to add %s to your PATH${NC}\n" "$(dirname "$INSTALL_PATH")"
		return 1
	fi

	INSTALLED_VERSION=$("$INSTALL_PATH" --version 2>/dev/null | grep -o 'v[0-9.]*' | head -1 || echo "unknown")
	printf "${GREEN}✓ Installed version: %s${NC}\n" "$INSTALLED_VERSION"
}

# Main installation flow
main() {
	printf "${GREEN}=== Blueprint Installer ===${NC}\n\n"

	printf "${YELLOW}Detecting OS and architecture...${NC}\n"
	detect_os_arch
	printf "${GREEN}✓ OS: %s${NC}\n" "$OS"
	printf "${GREEN}✓ Architecture: %s${NC}\n\n" "$ARCH"

	printf "${YELLOW}Fetching latest release...${NC}\n"
	get_latest_version
	printf "${GREEN}✓ Latest version: %s${NC}\n\n" "$VERSION"

	install_binary
	echo ""
	verify_installation

	printf "\n${GREEN}Installation complete!${NC}\n"
	printf "Run ${YELLOW}blueprint --help${NC} to get started.\n"
}

main "$@"
