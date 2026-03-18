package handlers

import (
	"fmt"
	"strings"
	"time"

	"github.com/elpic/blueprint/internal"
	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/platform"
	"github.com/elpic/blueprint/internal/ui"
)

// InstallHandler handles package installation and uninstallation
type InstallHandler struct {
	BaseHandler
}

// NewInstallHandler creates a new install handler
func NewInstallHandler(rule parser.Rule, basePath string, container platform.Container) *InstallHandler {
	return &InstallHandler{
		BaseHandler: BaseHandler{
			Rule:      rule,
			BasePath:  basePath,
			Container: container,
		},
	}
}

// NewInstallHandlerLegacy creates a new install handler without container (for backward compatibility)
func NewInstallHandlerLegacy(rule parser.Rule, basePath string) *InstallHandler {
	return NewInstallHandler(rule, basePath, platform.NewContainer())
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
		return h.buildUninstallCommand(h.Rule)
	}

	// Install action
	return h.buildCommand()
}

// UpdateStatus updates the status after installing or uninstalling packages
func (h *InstallHandler) UpdateStatus(status *Status, records []ExecutionRecord, blueprint string, osName string) error {
	// Normalize blueprint path for consistent storage and comparison
	blueprint = normalizePath(blueprint)

	switch h.Rule.Action {
	case "install":
		// Check if this rule's command was executed successfully
		cmd := h.buildCommand()
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
	return strings.Contains(cmd, "sudo")
}

// shouldAddSudo checks if sudo should be added for package installation on this OS
func (h *InstallHandler) shouldAddSudo() bool {
	// Use injected dependencies for OS and user detection
	osDetector := h.Container.SystemProvider().OS()

	// Only Linux requires sudo for package managers
	if osDetector.Name() != "linux" {
		return false
	}

	// Check if current user is root using injected dependency
	return !osDetector.IsRoot()
}

// getBrewCommand returns the appropriate brew command using dependency injection
func (h *InstallHandler) getBrewCommand() string {
	osDetector := h.Container.SystemProvider().OS()
	processExecutor := h.Container.SystemProvider().Process()
	filesystemProvider := h.Container.SystemProvider().Filesystem()

	if osDetector.Name() != "mac" {
		// On Linux brew is often not on PATH — check known locations first
		knownBrewPaths := []string{
			"/home/linuxbrew/.linuxbrew/bin/brew",
			"/opt/homebrew/bin/brew",
			"/usr/local/bin/brew",
		}

		for _, p := range knownBrewPaths {
			if filesystemProvider.Exists(p) {
				return p
			}
		}
		return "brew"
	}

	// On Mac, check for Rosetta 2 using dependency injection
	result, err := processExecutor.Execute("sysctl -n sysctl.proc_translated", platform.ExecuteOptions{})
	if err == nil && strings.TrimSpace(result.Stdout) == "1" {
		return "/usr/bin/arch -arm64 /opt/homebrew/bin/brew"
	}

	return "brew"
}

// buildCommand builds the install command based on OS and package manager
func (h *InstallHandler) buildCommand() string {
	if len(h.Rule.Packages) == 0 {
		return ""
	}

	// Use injected OS detector instead of hardcoded getOSName
	targetOS := h.Container.SystemProvider().OS().Name()

	// Group packages by package manager
	packagesByManager := h.groupPackagesByManager()

	// Build commands for each package manager
	var commands []string
	for manager, pkgList := range packagesByManager {
		cmd := h.buildInstallCommandForManager(manager, pkgList, targetOS)
		if cmd != "" {
			commands = append(commands, cmd)
		}
	}

	// If multiple package managers, join with && to run sequentially
	if len(commands) > 1 {
		return strings.Join(commands, " && ")
	} else if len(commands) == 1 {
		return commands[0]
	}
	return ""
}

// groupPackagesByManager groups packages by their package manager
func (h *InstallHandler) groupPackagesByManager() map[string][]string {
	groups := make(map[string][]string)
	for _, pkg := range h.Rule.Packages {
		manager := pkg.PackageManager
		if manager == "" {
			// Default to system package manager
			manager = "default"
		}
		groups[manager] = append(groups[manager], pkg.Name)
	}
	return groups
}

// buildInstallCommandForManager builds install command for a specific package manager
func (h *InstallHandler) buildInstallCommandForManager(manager string, pkgNames []string, targetOS string) string {
	if len(pkgNames) == 0 {
		return ""
	}

	pkgStr := strings.Join(pkgNames, " ")

	// Handle specific package managers
	switch manager {
	case "snap":
		// snap install - for multiple packages, create a shell command with all installs
		if targetOS == "linux" {
			if len(pkgNames) == 1 {
				// Single package: simple snap install
				cmd := fmt.Sprintf("snap install %s", pkgNames[0])
				if h.shouldAddSudo() {
					cmd = fmt.Sprintf("sudo %s", cmd)
				}
				return cmd
			}
			// Multiple packages: chain them with && in a single shell invocation
			// This way sudo only prompts once
			var snapCmds []string
			for _, pkg := range pkgNames {
				snapCmds = append(snapCmds, fmt.Sprintf("snap install %s", pkg))
			}
			chainedCmd := strings.Join(snapCmds, " && ")
			if h.shouldAddSudo() {
				// Wrap in sudo with shell so we only authenticate once
				// The command executor will handle this properly via shell
				return fmt.Sprintf("sudo %s", chainedCmd)
			}
			return chainedCmd
		}
		return ""

	case "homebrew", "brew":
		// Homebrew (macOS and Linux) — use dependency injection for brew command
		return fmt.Sprintf("%s install %s", h.getBrewCommand(), pkgStr)

	case "apt", "apt-get", "default":
		// apt-get (Linux default)
		if targetOS == "mac" {
			// Fallback to brew on macOS if apt is specified — use dependency injection for brew command
			return fmt.Sprintf("%s install %s", h.getBrewCommand(), pkgStr)
		}

		cmd := fmt.Sprintf("apt-get install -y %s", pkgStr)
		if h.shouldAddSudo() {
			cmd = fmt.Sprintf("sudo %s", cmd)
		}
		return cmd

	default:
		// Unknown package manager, try to use it directly
		cmd := fmt.Sprintf("%s install %s", manager, pkgStr)
		if h.shouldAddSudo() {
			cmd = fmt.Sprintf("sudo %s", cmd)
		}
		return cmd
	}
}

// buildUninstallCommand builds the uninstall command based on OS and package manager
func (h *InstallHandler) buildUninstallCommand(rule parser.Rule) string {
	if len(rule.Packages) == 0 {
		return ""
	}

	targetOS := h.Container.SystemProvider().OS().Name()

	// Group packages by package manager
	packagesByManager := make(map[string][]string)
	for _, pkg := range rule.Packages {
		manager := pkg.PackageManager
		if manager == "" {
			manager = "default"
		}
		packagesByManager[manager] = append(packagesByManager[manager], pkg.Name)
	}

	// Build uninstall commands for each package manager
	var commands []string
	for manager, pkgList := range packagesByManager {
		cmd := h.buildUninstallCommandForManager(manager, pkgList, targetOS)
		if cmd != "" {
			commands = append(commands, cmd)
		}
	}

	// If multiple package managers, join with && to run sequentially
	if len(commands) > 1 {
		return strings.Join(commands, " && ")
	} else if len(commands) == 1 {
		return commands[0]
	}
	return ""
}

// buildUninstallCommandForManager builds uninstall command for a specific package manager
func (h *InstallHandler) buildUninstallCommandForManager(manager string, pkgNames []string, targetOS string) string {
	if len(pkgNames) == 0 {
		return ""
	}

	pkgStr := strings.Join(pkgNames, " ")

	// Handle specific package managers
	switch manager {
	case "snap":
		// snap remove command
		if targetOS == "linux" {
			if len(pkgNames) == 1 {
				// Single package: simple snap remove
				cmd := fmt.Sprintf("snap remove %s", pkgNames[0])
				if h.shouldAddSudo() {
					cmd = fmt.Sprintf("sudo %s", cmd)
				}
				return cmd
			}
			// Multiple packages: chain them with && in a single shell invocation
			// This way sudo only prompts once
			var snapCmds []string
			for _, pkg := range pkgNames {
				snapCmds = append(snapCmds, fmt.Sprintf("snap remove %s", pkg))
			}
			chainedCmd := strings.Join(snapCmds, " && ")
			if h.shouldAddSudo() {
				// Wrap in sudo so we only authenticate once
				return fmt.Sprintf("sudo %s", chainedCmd)
			}
			return chainedCmd
		}
		return ""

	case "homebrew", "brew":
		// Homebrew uninstall — use dependency injection for brew command
		return fmt.Sprintf("%s uninstall -y %s", h.getBrewCommand(), pkgStr)

	case "apt", "apt-get", "default":
		// apt-get (Linux default)
		if targetOS == "mac" {
			// Fallback to brew on macOS if apt is specified — use dependency injection for brew command
			return fmt.Sprintf("%s uninstall -y %s", h.getBrewCommand(), pkgStr)
		}

		cmd := fmt.Sprintf("apt-get remove -y %s", pkgStr)
		if h.shouldAddSudo() {
			cmd = fmt.Sprintf("sudo %s", cmd)
		}
		return cmd

	default:
		// Unknown package manager, try to use it directly
		cmd := fmt.Sprintf("%s remove %s", manager, pkgStr)
		if h.shouldAddSudo() {
			cmd = fmt.Sprintf("sudo %s", cmd)
		}
		return cmd
	}
}

// getOSName returns the current OS name. Defined as a var to allow stubbing in tests.
// DEPRECATED: Use dependency injection via Container.SystemProvider().OS().Name() instead
var getOSName = func() string {
	return internal.OSName()
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

// DisplayStatusFromStatus displays install handler status from Status object
func (h *InstallHandler) DisplayStatusFromStatus(status *Status) {
	if status == nil || status.Packages == nil {
		return
	}
	h.DisplayStatus(status.Packages)
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

// GetState returns handler-specific state as key-value pairs
func (h *InstallHandler) GetState(isUninstall bool) map[string]string {
	packages := h.GetDisplayDetails(isUninstall)
	return map[string]string{
		"summary":  packages,
		"packages": packages,
	}
}

// FindUninstallRules compares package status against current rules and returns uninstall rules
func (h *InstallHandler) FindUninstallRules(status *Status, currentRules []parser.Rule, blueprintFile, osName string) []parser.Rule {
	normalizedBlueprint := normalizePath(blueprintFile)

	// Build set of current package names from install rules
	currentPackages := make(map[string]bool)
	for _, rule := range currentRules {
		if rule.Action == "install" {
			for _, pkg := range rule.Packages {
				currentPackages[pkg.Name] = true
			}
		}
	}

	// Find packages to uninstall (in status but not in current rules)
	var packagesToUninstall []parser.Package
	if status.Packages != nil {
		for _, pkg := range status.Packages {
			normalizedStatusBlueprint := normalizePath(pkg.Blueprint)
			if normalizedStatusBlueprint == normalizedBlueprint && pkg.OS == osName && !currentPackages[pkg.Name] {
				packagesToUninstall = append(packagesToUninstall, parser.Package{
					Name:    pkg.Name,
					Version: "latest",
				})
			}
		}
	}

	// Return uninstall rule if there are packages to uninstall
	var rules []parser.Rule
	if len(packagesToUninstall) > 0 {
		rules = append(rules, parser.Rule{
			Action:   "uninstall",
			Packages: packagesToUninstall,
			OSList:   []string{osName},
		})
	}
	return rules
}

// IsInstalled returns true if all packages in this rule are already in status.
func (h *InstallHandler) IsInstalled(status *Status, blueprintFile, osName string) bool {
	normalizedBlueprint := normalizePath(blueprintFile)
	for _, pkg := range h.Rule.Packages {
		found := false
		for _, s := range status.Packages {
			if s.Name == pkg.Name && normalizePath(s.Blueprint) == normalizedBlueprint && s.OS == osName {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// NeedsSudo returns true if package installation/uninstallation requires sudo privileges.
// This checks if the actual command that will be executed contains "sudo".
func (h *InstallHandler) NeedsSudo() bool {
	// Check the command that will be executed
	var cmd string
	if h.Rule.Action == "uninstall" {
		cmd = h.buildUninstallCommand(h.Rule)
	} else {
		cmd = h.buildCommand()
	}

	return needsSudo(cmd)
}
