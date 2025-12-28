package handlers

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
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

// GetCommand returns the actual command(s) that will be executed
func (h *InstallHandler) GetCommand() string {
	if h.Rule.Action == "uninstall" {
		cmd := h.buildUninstallCommand(h.Rule)
		if needsSudo(cmd) {
			cmd = "sudo " + cmd
		}
		return cmd
	}

	// Install action
	cmd := h.buildCommand()
	if needsSudo(cmd) {
		cmd = "sudo " + cmd
	}
	return cmd
}

// UpdateStatus updates the status after installing or uninstalling packages
func (h *InstallHandler) UpdateStatus(status *Status, records []ExecutionRecord, blueprint string, osName string) error {
	// Normalize blueprint path for consistent storage and comparison
	blueprint = normalizePath(blueprint)

	switch h.Rule.Action {
	case "install":
		// Check if this rule's command was executed successfully
		cmd := h.buildCommand()
		if needsSudo(cmd) {
			cmd = "sudo " + cmd
		}

		_, commandExecuted := commandSuccessfullyExecuted(cmd, records)

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
	case "uninstall":
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

// DisplayInfo displays handler-specific information
func (h *InstallHandler) DisplayInfo() {
	if h.Rule.Action == "uninstall" {
		// For uninstall, display packages in a dimmed format
		packageNames := make([]string, len(h.Rule.Packages))
		for i, pkg := range h.Rule.Packages {
			packageNames[i] = pkg.Name
		}
		fmt.Printf("  %s\n", ui.FormatDim(fmt.Sprintf("Packages: [%s]", strings.Join(packageNames, ", "))))
	} else {
		// For install, display packages in info format
		packageNames := make([]string, len(h.Rule.Packages))
		for i, pkg := range h.Rule.Packages {
			packageNames[i] = pkg.Name
		}
		fmt.Printf("  %s\n", ui.FormatInfo(fmt.Sprintf("Packages: [%s]", strings.Join(packageNames, ", "))))
	}
}

// executeCommandWithCache executes a command using the cached sudo password if available
// This is defined in engine.go and accessed here
var executeCommandWithCache func(string) (string, error)

// SetExecuteCommandFunc sets the execute command function
func SetExecuteCommandFunc(fn func(string) (string, error)) {
	executeCommandWithCache = fn
}

// DisplayStatus displays installed package status information
func (h *InstallHandler) DisplayStatus(packages []PackageStatus) {
	if len(packages) == 0 {
		return
	}

	fmt.Printf("\n%s\n", ui.FormatHighlight("Installed Packages:"))
	for _, pkg := range packages {
		// Parse timestamp for display
		t, err := time.Parse(time.RFC3339, pkg.InstalledAt)
		var timeStr string
		if err == nil {
			timeStr = t.Format("2006-01-02 15:04:05")
		} else {
			timeStr = pkg.InstalledAt
		}

		fmt.Printf("  %s %s (%s) [%s, %s]\n",
			ui.FormatSuccess("●"),
			ui.FormatInfo(pkg.Name),
			ui.FormatDim(timeStr),
			ui.FormatDim(pkg.OS),
			ui.FormatDim(abbreviateBlueprintPath(pkg.Blueprint)),
		)
	}
}

// GetDependencyKey returns the unique key for this rule in dependency resolution
func (h *InstallHandler) GetDependencyKey() string {
	fallback := "install"
	if len(h.Rule.Packages) > 0 {
		fallback = h.Rule.Packages[0].Name
	}
	return getDependencyKey(h.Rule, fallback)
}

// GetDisplayDetails returns the packages to display during execution
func (h *InstallHandler) GetDisplayDetails(isUninstall bool) string {
	// Build package list string
	packages := ""
	for j, pkg := range h.Rule.Packages {
		if j > 0 {
			packages += ", "
		}
		packages += pkg.Name
	}
	return packages
}
