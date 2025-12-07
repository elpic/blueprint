package handlers

import (
	"fmt"
	"runtime"
	"strings"

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
