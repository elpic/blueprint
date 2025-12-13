package handlers

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/elpic/blueprint/internal/parser"
)

// InstallHandler handles package installation and uninstallation
type InstallHandler struct {
	BaseHandler
}

// NewInstallHandler creates a new install handler
func NewInstallHandler(rule parser.Rule, basePath string) *InstallHandler {
	return &InstallHandler{
		BaseHandler: BaseHandler{
			Rule:     rule,
			BasePath: basePath,
		},
	}
}

// Up installs the packages
func (h *InstallHandler) Up() (string, error) {
	cmd := h.buildCommand()
	if cmd == "" {
		return "", fmt.Errorf("unable to build install command")
	}

	return executeCommandWithCache(cmd)
}

// Down uninstalls the packages
func (h *InstallHandler) Down() (string, error) {
	// Convert install rule to uninstall command
	uninstallRule := h.Rule
	uninstallRule.Action = "uninstall"

	cmd := h.buildUninstallCommand(uninstallRule)
	if cmd == "" {
		return "", fmt.Errorf("unable to build uninstall command")
	}

	return executeCommandWithCache(cmd)
}

// UpdateStatus updates the status after installing or uninstalling packages
func (h *InstallHandler) UpdateStatus(status *Status, records []ExecutionRecord, blueprint string, osName string) error {
	// Normalize blueprint path for consistent storage and comparison
	blueprint = normalizePath(blueprint)

	if h.Rule.Action == "install" {
		// Check if this rule's command was executed successfully
		cmd := h.buildCommand()
		if needsSudo(cmd) {
			cmd = "sudo " + cmd
		}

		// Look for a successful execution record matching this command
		commandExecuted := false
		for _, record := range records {
			if record.Status == "success" && record.Command == cmd {
				commandExecuted = true
				break
			}
		}

		if commandExecuted {
			// Add or update package status
			for _, pkg := range h.Rule.Packages {
				// Remove existing entry if present
				status.Packages = removePackageStatus(status.Packages, pkg.Name, blueprint, osName)
				// Add new entry
				status.Packages = append(status.Packages, PackageStatus{
					Name:        pkg.Name,
					InstalledAt: time.Now().Format(time.RFC3339),
					Blueprint:   blueprint,
					OS:          osName,
				})
			}
		}
	} else if h.Rule.Action == "uninstall" {
		// Remove uninstalled packages from status
		for _, pkg := range h.Rule.Packages {
			status.Packages = removePackageStatus(status.Packages, pkg.Name, blueprint, osName)
		}
	}

	return nil
}

// needsSudo checks if a command needs sudo
func needsSudo(cmd string) bool {
	return strings.Contains(cmd, "brew") || strings.Contains(cmd, "apt-get")
}

// buildCommand builds the install command based on OS
func (h *InstallHandler) buildCommand() string {
	if len(h.Rule.Packages) == 0 {
		return ""
	}

	pkgNames := ""
	for i, pkg := range h.Rule.Packages {
		if i > 0 {
			pkgNames += " "
		}
		pkgNames += pkg.Name
	}

	// Determine target OS
	targetOS := getOSName()
	if len(h.Rule.OSList) > 0 {
		targetOS = strings.TrimSpace(h.Rule.OSList[0])
	}

	if targetOS == "mac" {
		return fmt.Sprintf("brew install %s", pkgNames)
	}
	return fmt.Sprintf("apt-get install -y %s", pkgNames)
}

// buildUninstallCommand builds the uninstall command based on OS
func (h *InstallHandler) buildUninstallCommand(rule parser.Rule) string {
	if len(rule.Packages) == 0 {
		return ""
	}

	pkgNames := ""
	for i, pkg := range rule.Packages {
		if i > 0 {
			pkgNames += " "
		}
		pkgNames += pkg.Name
	}

	// Determine target OS
	targetOS := getOSName()
	if len(rule.OSList) > 0 {
		targetOS = strings.TrimSpace(rule.OSList[0])
	}

	if targetOS == "mac" {
		return fmt.Sprintf("brew uninstall -y %s", pkgNames)
	}
	return fmt.Sprintf("apt-get remove -y %s", pkgNames)
}

// getOSName returns the current operating system name
func getOSName() string {
	switch runtime.GOOS {
	case "darwin":
		return "mac"
	case "linux":
		return "linux"
	default:
		return runtime.GOOS
	}
}

// executeCommandWithCache executes a command using the cached sudo password if available
// This is defined in engine.go and accessed here
var executeCommandWithCache func(string) (string, error)

// SetExecuteCommandFunc sets the execute command function
func SetExecuteCommandFunc(fn func(string) (string, error)) {
	executeCommandWithCache = fn
}
