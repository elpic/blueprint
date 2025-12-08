package handlers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	gitpkg "github.com/elpic/blueprint/internal/git"
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

// installAsdfLinux installs asdf on Linux by cloning the repository
// Clones asdf to ~/.asdf and configures shell initialization
func (h *AsdfHandler) installAsdfLinux() error {
	// Install dependencies: bash
	depCmd := "DEBIAN_FRONTEND=noninteractive apt-get update -qq 2>/dev/null && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq bash 2>/dev/null || true"
	if _, err := executeCommandWithCache(depCmd); err != nil {
		// Don't fail - bash might already be installed
	}

	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Clone asdf repository to ~/.asdf using go-git library
	asdfPath := filepath.Join(homeDir, ".asdf")
	_, _, _, err = gitpkg.CloneOrUpdateRepository(
		"https://github.com/asdf-vm/asdf.git",
		asdfPath,
		"master",
	)
	if err != nil {
		return fmt.Errorf("failed to clone asdf repository: %w", err)
	}

	// Add asdf.sh to bashrc if not already present
	bashrcPath := filepath.Join(homeDir, ".bashrc")
	content, err := os.ReadFile(bashrcPath)
	if err == nil {
		contentStr := string(content)
		if !strings.Contains(contentStr, ". $HOME/.asdf/asdf.sh") {
			// Append asdf initialization
			if !strings.HasSuffix(contentStr, "\n") {
				contentStr += "\n"
			}
			contentStr += ". $HOME/.asdf/asdf.sh\n"
			if err := os.WriteFile(bashrcPath, []byte(contentStr), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not update %s: %v\n", bashrcPath, err)
			}
		}
	} else {
		// Create bashrc if it doesn't exist
		asdfInit := ". $HOME/.asdf/asdf.sh\n"
		if err := os.WriteFile(bashrcPath, []byte(asdfInit), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not create %s: %v\n", bashrcPath, err)
		}
	}

	// Source bashrc to load asdf in current shell
	sourceCmd := fmt.Sprintf("bash -c 'source %s'", bashrcPath)
	if _, err := executeCommandWithCache(sourceCmd); err != nil {
		// Don't fail if sourcing fails - asdf will be available in new shell
		fmt.Fprintf(os.Stderr, "Warning: could not source %s\n", bashrcPath)
	}

	return nil
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

// uninstallAsdfLinux removes asdf from Linux (cloned from GitHub repository)
func (h *AsdfHandler) uninstallAsdfLinux() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Remove asdf directory that was cloned from GitHub
	asdfDir := filepath.Join(homeDir, ".asdf")
	if err := os.RemoveAll(asdfDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to remove %s: %v\n", asdfDir, err)
	}

	// Remove asdf initialization from bashrc
	bashrcPath := filepath.Join(homeDir, ".bashrc")
	content, err := os.ReadFile(bashrcPath)
	if err == nil {
		// Remove the line that sources asdf.sh
		lines := strings.Split(string(content), "\n")
		var newLines []string
		for _, line := range lines {
			if !strings.Contains(line, ". $HOME/.asdf/asdf.sh") {
				newLines = append(newLines, line)
			}
		}
		updatedContent := strings.Join(newLines, "\n")
		if err := os.WriteFile(bashrcPath, []byte(updatedContent), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to update %s: %v\n", bashrcPath, err)
		}
	}

	return nil
}
