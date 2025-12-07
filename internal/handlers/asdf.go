package handlers

import (
	"fmt"
	"os/exec"
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

// Up initializes asdf and installs specified packages
func (h *AsdfHandler) Up() (string, error) {
	// Check if asdf is installed
	checkCmd := "which asdf"
	if err := exec.Command("sh", "-c", checkCmd).Run(); err != nil {
		return "", fmt.Errorf("asdf is not installed")
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
		if output, err := exec.Command("sh", "-c", installCmd).CombinedOutput(); err != nil {
			return "", fmt.Errorf("failed to install %s@%s: %w", plugin, version, err)
		} else {
			// Extract SHA if available
			sha := extractSHAFromOutput(string(output))
			if sha != "" {
				return fmt.Sprintf("Installed (SHA: %s)", sha), nil
			}
		}

		// Set local version
		setCmd := fmt.Sprintf("asdf local %s %s", plugin, version)
		if err := exec.Command("sh", "-c", setCmd).Run(); err != nil {
			return "", fmt.Errorf("failed to set %s version to %s: %w", plugin, version, err)
		}
	}

	return "Installed", nil
}

// Down uninstalls asdf and removes versions
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

	// Remove plugins if no other versions installed
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

	return "Removed asdf packages", nil
}
