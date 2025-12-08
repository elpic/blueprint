package handlers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/elpic/blueprint/internal/parser"
)

// AsdfHandler handles asdf version manager operations
type AsdfHandler struct {
	BaseHandler
}

// NewAsdfHandler creates a new asdf handler
func NewAsdfHandler(rule parser.Rule, basePath string) *AsdfHandler {
	return &AsdfHandler{
		BaseHandler: BaseHandler{
			Rule:     rule,
			BasePath: basePath,
		},
	}
}

// Up installs asdf (if not present) and then installs specified packages
func (h *AsdfHandler) Up() (string, error) {
	// Check if asdf is installed
	isInstalled := h.isAsdfInstalled()

	// If asdf is not installed, install it
	if !isInstalled {
		if err := h.installAsdf(); err != nil {
			return "", fmt.Errorf("failed to install asdf: %w", err)
		}
	}

	// Install plugins and versions
	for _, pkg := range h.Rule.AsdfPackages {
		parts := strings.Split(pkg, "@")
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid asdf package format: %s, expected format: plugin@version", pkg)
		}

		plugin := parts[0]
		version := parts[1]

		// Add plugin
		addPluginCmd := fmt.Sprintf("asdf plugin add %s 2>/dev/null || true", plugin)
		if err := exec.Command("sh", "-c", addPluginCmd).Run(); err != nil {
			// Continue even if plugin add fails (might already exist)
		}

		// Install version
		installCmd := fmt.Sprintf("asdf install %s %s", plugin, version)
		if err := exec.Command("sh", "-c", installCmd).Run(); err != nil {
			return "", fmt.Errorf("failed to install %s@%s: %w", plugin, version, err)
		}

		// Set local version
		setCmd := fmt.Sprintf("asdf local %s %s", plugin, version)
		if err := exec.Command("sh", "-c", setCmd).Run(); err != nil {
			return "", fmt.Errorf("failed to set %s version to %s: %w", plugin, version, err)
		}
	}

	if isInstalled {
		return "Installed plugins and versions", nil
	}
	return "Installed asdf and plugins", nil
}

// Down uninstalls asdf and all versions completely
func (h *AsdfHandler) Down() (string, error) {
	// Uninstall each version
	for _, pkg := range h.Rule.AsdfPackages {
		parts := strings.Split(pkg, "@")
		if len(parts) != 2 {
			continue
		}

		plugin := parts[0]
		version := parts[1]

		// Uninstall version
		uninstallCmd := fmt.Sprintf("asdf uninstall %s %s", plugin, version)
		if err := exec.Command("sh", "-c", uninstallCmd).Run(); err != nil {
			// Continue even if uninstall fails
		}
	}

	// Remove plugins
	for _, pkg := range h.Rule.AsdfPackages {
		parts := strings.Split(pkg, "@")
		if len(parts) != 2 {
			continue
		}

		plugin := parts[0]

		// Remove plugin
		removeCmd := fmt.Sprintf("asdf plugin remove %s 2>/dev/null || true", plugin)
		if err := exec.Command("sh", "-c", removeCmd).Run(); err != nil {
			// Continue even if remove fails
		}
	}

	// Uninstall asdf completely
	if err := h.uninstallAsdf(); err != nil {
		return "", fmt.Errorf("failed to uninstall asdf: %w", err)
	}

	return "Uninstalled asdf and all plugins", nil
}

// isAsdfInstalled checks if asdf is installed on the system
func (h *AsdfHandler) isAsdfInstalled() bool {
	checkCmd := "which asdf"
	return exec.Command("sh", "-c", checkCmd).Run() == nil
}

// installAsdf installs asdf using the best available method
func (h *AsdfHandler) installAsdf() error {
	switch runtime.GOOS {
	case "darwin":
		return h.installAsdfMacOS()

	case "linux":
		return h.installAsdfLinux()

	default:
		return fmt.Errorf("asdf installation not supported on %s", runtime.GOOS)
	}
}

// installAsdfMacOS installs asdf on macOS using Homebrew
func (h *AsdfHandler) installAsdfMacOS() error {
	// First install coreutils as dependency (continue if it fails, might already be installed)
	depCmd := "brew install coreutils 2>/dev/null || true"
	if _, err := executeCommandWithCache(depCmd); err != nil {
		// Continue even if coreutils fails
	}

	// Install asdf via Homebrew
	installCmd := "brew install asdf"
	if _, err := executeCommandWithCache(installCmd); err != nil {
		return fmt.Errorf("failed to install asdf: %w", err)
	}

	return nil
}

// installAsdfLinux installs asdf on Linux using pre-compiled binary
// Pre-compiled binary method is used to avoid Go dependency
// Downloads from: https://github.com/asdf-vm/asdf/releases
func (h *AsdfHandler) installAsdfLinux() error {
	// Install dependencies: git and bash
	dependencyCmds := []string{
		"DEBIAN_FRONTEND=noninteractive apt-get update -qq 2>/dev/null || true",
		"DEBIAN_FRONTEND=noninteractive apt-get install -y -qq git bash 2>/dev/null || true",
	}

	for _, depCmd := range dependencyCmds {
		executeCommandWithCache(depCmd)
	}

	// Detect architecture
	arch := getLinuxArchitecture()
	if arch == "" {
		return fmt.Errorf("unable to detect system architecture")
	}

	// Download latest asdf release from GitHub
	// We use the latest release which is typically stable
	downloadCmd := fmt.Sprintf(`
curl -fsSL https://api.github.com/repos/asdf-vm/asdf/releases/latest \
  | grep 'browser_download_url.*linux-%s' | grep -v md5 \
  | head -1 | grep -o 'https://[^"]*' | xargs -I {} wget -q {} -O /tmp/asdf.tar.gz
`, arch)

	if _, err := executeCommandWithCache(downloadCmd); err != nil {
		return fmt.Errorf("failed to download asdf: %w", err)
	}

	// Extract and install to /usr/local/bin
	extractCmd := `
cd /tmp && \
tar -xzf asdf.tar.gz && \
mv asdf /usr/local/bin/ && \
rm -f asdf.tar.gz
`

	if _, err := executeCommandWithCache(extractCmd); err != nil {
		return fmt.Errorf("failed to extract and install asdf: %w", err)
	}

	return nil
}

// getLinuxArchitecture detects the Linux system architecture
// Returns the architecture name as used in asdf GitHub releases
func getLinuxArchitecture() string {
	// Use uname -m to get architecture
	output, err := exec.Command("uname", "-m").Output()
	if err != nil {
		return ""
	}

	arch := strings.TrimSpace(string(output))
	switch arch {
	case "x86_64":
		return "amd64"
	case "aarch64", "arm64":
		return "arm64"
	case "armv7l", "armv6l":
		return "armv7"
	case "386", "i386", "i686":
		return "386"
	default:
		// Return as-is if not recognized - might match GitHub release name
		return arch
	}
}

// uninstallAsdf completely removes asdf from the system
func (h *AsdfHandler) uninstallAsdf() error {
	switch runtime.GOOS {
	case "darwin":
		return h.uninstallAsdfMacOS()

	case "linux":
		return h.uninstallAsdfLinux()

	default:
		return fmt.Errorf("asdf uninstallation not supported on %s", runtime.GOOS)
	}
}

// uninstallAsdfMacOS removes asdf from macOS using Homebrew
func (h *AsdfHandler) uninstallAsdfMacOS() error {
	uninstallCmd := "brew uninstall asdf"

	if _, err := executeCommandWithCache(uninstallCmd); err != nil {
		return fmt.Errorf("failed to uninstall asdf: %w", err)
	}

	// Remove asdf data directory
	homeDir, err := os.UserHomeDir()
	if err == nil {
		asdfDir := filepath.Join(homeDir, ".asdf")
		if err := os.RemoveAll(asdfDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to remove %s: %v\n", asdfDir, err)
		}
	}

	return nil
}

// uninstallAsdfLinux removes asdf from Linux (installed via pre-compiled binary)
func (h *AsdfHandler) uninstallAsdfLinux() error {
	homeDir, err := os.UserHomeDir()
	if err == nil {
		// Remove asdf data directory
		asdfDir := filepath.Join(homeDir, ".asdf")
		if err := os.RemoveAll(asdfDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to remove %s: %v\n", asdfDir, err)
		}
	}

	// Remove asdf binary from /usr/local/bin (requires sudo)
	removeCmd := "rm -f /usr/local/bin/asdf"

	if _, err := executeCommandWithCache(removeCmd); err != nil {
		// Log but don't fail - file might not exist or insufficient permissions
		fmt.Fprintf(os.Stderr, "Note: asdf binary at /usr/local/bin/asdf could not be removed\n")
	}

	return nil
}
