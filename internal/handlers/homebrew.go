package handlers

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
)

// HomebrewHandler handles homebrew package installation and uninstallation
type HomebrewHandler struct {
	BaseHandler
}

// homebrewInstallMutex prevents concurrent homebrew installation attempts
var homebrewInstallMutex = &sync.Mutex{}

// NewHomebrewHandler creates a new homebrew handler
func NewHomebrewHandler(rule parser.Rule, basePath string) *HomebrewHandler {
	return &HomebrewHandler{
		BaseHandler: BaseHandler{
			Rule:     rule,
			BasePath: basePath,
		},
	}
}

// Up installs the homebrew formulas and ensures homebrew is installed
func (h *HomebrewHandler) Up() (string, error) {
	// Homebrew only works on macOS and Linux
	targetOS := getOSNameForAction()
	if targetOS != "mac" && targetOS != "linux" {
		return "", fmt.Errorf("homebrew is not supported on %s", targetOS)
	}

	// First ensure homebrew is installed
	if err := h.ensureHomebrewInstalled(); err != nil {
		return "", fmt.Errorf("failed to ensure homebrew is installed: %w", err)
	}

	// Then install the formulas
	cmd := h.buildCommand()
	if cmd == "" {
		return "", fmt.Errorf("unable to build install command")
	}

	return executeCommandWithCache(cmd)
}

// Down uninstalls the homebrew formulas
func (h *HomebrewHandler) Down() (string, error) {
	cmd := h.buildUninstallCommand()
	if cmd == "" {
		return "", fmt.Errorf("unable to build uninstall command")
	}

	return executeCommandWithCache(cmd)
}

// GetCommand returns the actual command(s) that will be executed
func (h *HomebrewHandler) GetCommand() string {
	if h.Rule.Action == "uninstall" {
		return h.buildUninstallCommand()
	}

	// Install action
	return h.buildCommand()
}

// UpdateStatus updates the status after installing or uninstalling formulas
func (h *HomebrewHandler) UpdateStatus(status *Status, records []ExecutionRecord, blueprint string, osName string) error {
	// Normalize blueprint path for consistent storage and comparison
	blueprint = normalizePath(blueprint)

	switch h.Rule.Action {
	case "install":
		// Check if this rule's command was executed successfully
		cmd := h.buildCommand()
		_, commandExecuted := commandSuccessfullyExecuted(cmd, records)

		if commandExecuted {
			// Add or update formula status
			for _, formulaStr := range h.Rule.HomebrewPackages {
				// Parse formula@version or just formula
				parts := strings.Split(formulaStr, "@")
				formula := parts[0]

				// Remove existing entry if present
				status.Brews = removeHomebrewStatus(status.Brews, formula, blueprint, osName)

				// Get installed version
				version := "latest"
				if versionStr, err := h.getInstalledFormulaVersion(formula); err == nil && versionStr != "" {
					version = versionStr
				}

				// Add new entry
				status.Brews = append(status.Brews, HomebrewStatus{
					Formula:     formula,
					Version:     version,
					InstalledAt: time.Now().Format(time.RFC3339),
					Blueprint:   blueprint,
					OS:          osName,
				})
			}
		}
	case "uninstall":
		// Remove uninstalled formulas from status
		for _, formulaStr := range h.Rule.HomebrewPackages {
			parts := strings.Split(formulaStr, "@")
			formula := parts[0]
			status.Brews = removeHomebrewStatus(status.Brews, formula, blueprint, osName)
		}
	}

	return nil
}

// getInstalledFormulaVersion gets the installed version of a formula
func (h *HomebrewHandler) getInstalledFormulaVersion(formula string) (string, error) {
	cmd := exec.Command("brew", "list", "--versions", formula)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	// Output format: "formula version"
	parts := strings.Fields(string(output))
	if len(parts) > 1 {
		return parts[1], nil
	}

	return "", nil
}

// ensureHomebrewInstalled ensures homebrew is installed on the system
// Uses mutex to prevent concurrent installation attempts that could cause conflicts
func (h *HomebrewHandler) ensureHomebrewInstalled() error {
	// Check if homebrew is already installed (fast path without lock)
	if h.isHomebrewInstalled() {
		return nil
	}

	// Use mutex to prevent concurrent installation attempts
	homebrewInstallMutex.Lock()
	defer homebrewInstallMutex.Unlock()

	// Double-check after acquiring lock (another goroutine might have installed it)
	if h.isHomebrewInstalled() {
		return nil
	}

	// Determine OS and install accordingly
	targetOS := getOSNameForAction()
	if len(h.Rule.OSList) > 0 {
		targetOS = strings.TrimSpace(h.Rule.OSList[0])
	}

	switch targetOS {
	case "mac":
		return h.installHomebrewMacOS()
	case "linux":
		return h.installHomebrewLinux()
	default:
		return fmt.Errorf("homebrew installation not supported on %s", targetOS)
	}
}

// isHomebrewInstalled checks if homebrew is installed
func (h *HomebrewHandler) isHomebrewInstalled() bool {
	cmd := exec.Command("which", "brew")
	return cmd.Run() == nil
}

// installHomebrewMacOS installs homebrew on macOS using the official script
func (h *HomebrewHandler) installHomebrewMacOS() error {
	// Use the official Homebrew installation script
	installCmd := `curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh | bash`

	if _, err := executeCommandWithCache(installCmd); err != nil {
		return fmt.Errorf("failed to install homebrew on macOS: %w", err)
	}

	return nil
}

// installHomebrewLinux installs homebrew on Linux
func (h *HomebrewHandler) installHomebrewLinux() error {
	// Homebrew on Linux requires some dependencies and a specific installation process
	// First ensure we have git and curl
	depCmd := "apt-get update && apt-get install -y git curl build-essential"
	if _, err := executeCommandWithCache(fmt.Sprintf("sudo %s", depCmd)); err != nil {
		// Try without sudo if it fails (user might have permissions)
		if _, err := executeCommandWithCache(depCmd); err != nil {
			return fmt.Errorf("failed to install homebrew dependencies: %w", err)
		}
	}

	// Download and run Homebrew installation script
	installCmd := `curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh | bash`
	if _, err := executeCommandWithCache(installCmd); err != nil {
		return fmt.Errorf("failed to install homebrew on Linux: %w", err)
	}

	return nil
}

// buildCommand builds the install command based on formula list
func (h *HomebrewHandler) buildCommand() string {
	if len(h.Rule.HomebrewPackages) == 0 {
		return ""
	}

	formulaNames := strings.Join(h.Rule.HomebrewPackages, " ")
	return fmt.Sprintf("brew install %s", formulaNames)
}

// buildUninstallCommand builds the uninstall command based on formula list
func (h *HomebrewHandler) buildUninstallCommand() string {
	if len(h.Rule.HomebrewPackages) == 0 {
		return ""
	}

	// Extract just the formula names (without versions) for uninstall
	var formulas []string
	for _, formulaStr := range h.Rule.HomebrewPackages {
		parts := strings.Split(formulaStr, "@")
		formulas = append(formulas, parts[0])
	}

	formulaNames := strings.Join(formulas, " ")
	return fmt.Sprintf("brew uninstall -y %s", formulaNames)
}

// NeedsSudo returns true if homebrew installation requires sudo privileges
func (h *HomebrewHandler) NeedsSudo() bool {
	// Homebrew on Linux might need sudo for dependencies, but not for brew itself
	// On macOS, homebrew handles its own permissions
	return false
}

// GetDependencyKey returns the unique key for this rule in dependency resolution
func (h *HomebrewHandler) GetDependencyKey() string {
	fallback := "homebrew"
	if len(h.Rule.HomebrewPackages) > 0 {
		fallback = h.Rule.HomebrewPackages[0]
	}
	return getDependencyKey(h.Rule, fallback)
}

// GetDisplayDetails returns the formulas to display during execution
func (h *HomebrewHandler) GetDisplayDetails(isUninstall bool) string {
	return strings.Join(h.Rule.HomebrewPackages, ", ")
}

// DisplayInfo displays handler-specific information
func (h *HomebrewHandler) DisplayInfo() {
	if h.Rule.Action == "uninstall" {
		fmt.Printf("  %s\n", ui.FormatDim(fmt.Sprintf("Formulas: [%s]", strings.Join(h.Rule.HomebrewPackages, ", "))))
	} else {
		fmt.Printf("  %s\n", ui.FormatInfo(fmt.Sprintf("Formulas: [%s]", strings.Join(h.Rule.HomebrewPackages, ", "))))
	}
}

// DisplayStatusFromStatus displays homebrew handler status from Status object
func (h *HomebrewHandler) DisplayStatusFromStatus(status *Status) {
	if status == nil || status.Brews == nil {
		return
	}
	h.DisplayStatus(status.Brews)
}

// DisplayStatus displays installed homebrew formula status information
func (h *HomebrewHandler) DisplayStatus(brews []HomebrewStatus) {
	if len(brews) == 0 {
		return
	}

	fmt.Printf("\n%s\n", ui.FormatHighlight("Installed Homebrew Formulas:"))
	for _, brew := range brews {
		// Parse timestamp for display
		t, err := time.Parse(time.RFC3339, brew.InstalledAt)
		var timeStr string
		if err == nil {
			timeStr = t.Format("2006-01-02 15:04:05")
		} else {
			timeStr = brew.InstalledAt
		}

		fmt.Printf("  %s %s (%s) [%s, %s]\n",
			ui.FormatSuccess("●"),
			ui.FormatInfo(brew.Formula),
			ui.FormatDim(timeStr),
			ui.FormatDim(brew.OS),
			ui.FormatDim(abbreviateBlueprintPath(brew.Blueprint)),
		)
	}
}

// FindUninstallRules compares homebrew status against current rules and returns uninstall rules
func (h *HomebrewHandler) FindUninstallRules(status *Status, currentRules []parser.Rule, blueprintFile, osName string) []parser.Rule {
	normalizedBlueprint := normalizePath(blueprintFile)

	// Build set of current formula names from homebrew rules
	currentFormulas := make(map[string]bool)
	for _, rule := range currentRules {
		if rule.Action == "homebrew" {
			for _, formulaStr := range rule.HomebrewPackages {
				parts := strings.Split(formulaStr, "@")
				currentFormulas[parts[0]] = true
			}
		}
	}

	// Find formulas to uninstall (in status but not in current rules)
	var formulasToUninstall []string
	if status.Brews != nil {
		for _, brew := range status.Brews {
			normalizedStatusBlueprint := normalizePath(brew.Blueprint)
			if normalizedStatusBlueprint == normalizedBlueprint && brew.OS == osName && !currentFormulas[brew.Formula] {
				formulasToUninstall = append(formulasToUninstall, brew.Formula)
			}
		}
	}

	// Return uninstall rule if there are formulas to uninstall
	var rules []parser.Rule
	if len(formulasToUninstall) > 0 {
		rules = append(rules, parser.Rule{
			Action:           "uninstall",
			HomebrewPackages: formulasToUninstall,
			OSList:           []string{osName},
		})
	}
	return rules
}

// getOSNameForAction returns the current operating system name for action
func getOSNameForAction() string {
	switch runtime.GOOS {
	case "darwin":
		return "mac"
	case "linux":
		return "linux"
	default:
		return runtime.GOOS
	}
}

