package handlers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
)

// MiseHandler handles mise version manager operations
type MiseHandler struct {
	BaseHandler
}

// NewMiseHandler creates a new mise handler
func NewMiseHandler(rule parser.Rule, basePath string) *MiseHandler {
	return &MiseHandler{
		BaseHandler: BaseHandler{
			Rule:     rule,
			BasePath: basePath,
		},
	}
}

// isMiseInstalled checks if mise is installed
func (h *MiseHandler) isMiseInstalled() bool {
	// Check default curl install location first
	homeDir, err := os.UserHomeDir()
	if err == nil {
		misePath := filepath.Join(homeDir, ".local", "bin", "mise")
		if _, err := os.Stat(misePath); err == nil {
			return true
		}
	}

	// Fall back to PATH lookup
	_, err = exec.LookPath("mise")
	return err == nil
}

// miseCmd returns the full path to the mise binary
func (h *MiseHandler) miseCmd() string {
	homeDir, err := os.UserHomeDir()
	if err == nil {
		misePath := filepath.Join(homeDir, ".local", "bin", "mise")
		if _, err := os.Stat(misePath); err == nil {
			return misePath
		}
	}
	return "mise"
}

// installMise installs mise using the platform-appropriate method
func (h *MiseHandler) installMise() error {
	switch runtime.GOOS {
	case "darwin":
		return h.installMiseMacOS()
	case "linux":
		return h.installMiseLinux()
	default:
		return fmt.Errorf("mise installation not supported on %s", runtime.GOOS)
	}
}

// installMiseMacOS installs mise on macOS using Homebrew
func (h *MiseHandler) installMiseMacOS() error {
	installCmd := "brew install mise"
	if _, err := executeCommandWithCache(installCmd); err != nil {
		return fmt.Errorf("failed to install mise: %w", err)
	}
	return nil
}

// installMiseLinux installs mise on Linux using the official install script
func (h *MiseHandler) installMiseLinux() error {
	// Ensure ~/.local/bin exists
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	localBin := filepath.Join(homeDir, ".local", "bin")
	// 0755 is standard for user bin directories
	// #nosec G301
	if err := os.MkdirAll(localBin, 0o755); err != nil {
		return fmt.Errorf("failed to create ~/.local/bin: %w", err)
	}

	installCmd := "curl https://mise.run | sh"
	if _, err := executeCommandWithCache(installCmd); err != nil {
		return fmt.Errorf("failed to install mise: %w", err)
	}
	return nil
}

// uninstallMise completely removes mise from the system
func (h *MiseHandler) uninstallMise() error {
	switch runtime.GOOS {
	case "darwin":
		uninstallCmd := "brew uninstall mise 2>/dev/null || true"
		_, _ = executeCommandWithCache(uninstallCmd)
	case "linux":
		homeDir, err := os.UserHomeDir()
		if err == nil {
			misePath := filepath.Join(homeDir, ".local", "bin", "mise")
			_ = os.Remove(misePath)
		}
	}
	return nil
}

// isGlobal returns true when no project path is set (default behaviour)
func (h *MiseHandler) isGlobal() bool {
	return h.Rule.MisePath == ""
}

// resolvedMisePath expands ~ in MisePath to the actual home directory
func (h *MiseHandler) resolvedMisePath() (string, error) {
	p := h.Rule.MisePath
	if strings.HasPrefix(p, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		p = filepath.Join(homeDir, p[2:])
	}
	return p, nil
}

// Up installs mise (if not present) and then installs specified tool versions
func (h *MiseHandler) Up() (string, error) {
	isInstalled := h.isMiseInstalled()

	if !isInstalled {
		if err := h.installMise(); err != nil {
			return "", fmt.Errorf("failed to install mise: %w", err)
		}
	}

	if len(h.Rule.MisePackages) == 0 {
		if isInstalled {
			return "mise already installed", nil
		}
		return "Installed mise", nil
	}

	miseBin := h.miseCmd()
	global := h.isGlobal()

	// Build a single combined command for all packages
	var allCmds []string
	for _, pkg := range h.Rule.MisePackages {
		if strings.Contains(pkg, "@") {
			parts := strings.SplitN(pkg, "@", 2)
			tool := strings.TrimSpace(parts[0])
			version := strings.TrimSpace(parts[1])

			if !isValidAsdfIdentifier(tool) {
				return "", fmt.Errorf("invalid tool name: %s (contains invalid characters)", tool)
			}
			if !isValidAsdfIdentifier(version) {
				return "", fmt.Errorf("invalid version: %s (contains invalid characters)", version)
			}

			if global {
				allCmds = append(allCmds, fmt.Sprintf("%s use -g %s@%s", miseBin, tool, version))
			} else {
				allCmds = append(allCmds, fmt.Sprintf("%s use %s@%s", miseBin, tool, version))
			}
		} else {
			tool := strings.TrimSpace(pkg)
			if !isValidAsdfIdentifier(tool) {
				return "", fmt.Errorf("invalid tool name: %s (contains invalid characters)", tool)
			}
			if global {
				allCmds = append(allCmds, fmt.Sprintf("%s use -g %s", miseBin, tool))
			} else {
				allCmds = append(allCmds, fmt.Sprintf("%s use %s", miseBin, tool))
			}
		}
	}

	if len(allCmds) > 0 {
		homeDir, _ := os.UserHomeDir()
		localBin := filepath.Join(homeDir, ".local", "bin")
		combinedCmd := strings.Join(allCmds, " && ")
		fullCmd := fmt.Sprintf(`export PATH="%s:$PATH" && %s`, localBin, combinedCmd)
		cmd := exec.Command("bash", "-c", fullCmd)
		cmd.Stdin = nil

		if !global {
			projectPath, err := h.resolvedMisePath()
			if err != nil {
				return "", err
			}
			// Ensure the project directory exists
			// #nosec G301
			if err := os.MkdirAll(projectPath, 0o755); err != nil {
				return "", fmt.Errorf("failed to create project directory %s: %w", projectPath, err)
			}
			cmd.Dir = projectPath
		}

		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Sprintf("Installation output:\n%s", string(output)), fmt.Errorf("failed to install mise packages: %w", err)
		}
	}

	if isInstalled {
		return "Installed tools", nil
	}
	return "Installed mise and tools", nil
}

// Down uninstalls mise tools and optionally mise itself
func (h *MiseHandler) Down() (string, error) {
	miseBin := h.miseCmd()

	// Resolve project path once if needed
	var projectPath string
	if !h.isGlobal() {
		var err error
		projectPath, err = h.resolvedMisePath()
		if err != nil {
			return "", err
		}
	}

	for _, pkg := range h.Rule.MisePackages {
		var uninstallCmd string
		if strings.Contains(pkg, "@") {
			parts := strings.SplitN(pkg, "@", 2)
			tool := strings.TrimSpace(parts[0])
			version := strings.TrimSpace(parts[1])

			if !isValidAsdfIdentifier(tool) || !isValidAsdfIdentifier(version) {
				continue
			}
			uninstallCmd = fmt.Sprintf("%s uninstall %s@%s", miseBin, tool, version)
		} else {
			tool := strings.TrimSpace(pkg)
			if !isValidAsdfIdentifier(tool) {
				continue
			}
			uninstallCmd = fmt.Sprintf("%s uninstall %s", miseBin, tool)
		}
		cmd := exec.Command("sh", "-c", uninstallCmd)
		if projectPath != "" {
			cmd.Dir = projectPath
		}
		_ = cmd.Run() // Continue even if uninstall fails
	}

	// Only auto-remove mise itself for global installs with no remaining tools
	if h.isGlobal() {
		checkCmd := fmt.Sprintf("%s ls 2>/dev/null | wc -l", miseBin)
		output, err := exec.Command("sh", "-c", checkCmd).Output()
		if err == nil {
			toolCount := 0
			_, _ = fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &toolCount)
			if toolCount == 0 {
				_ = h.uninstallMise()
				return "Uninstalled mise tools and mise", nil
			}
		}
	}

	return "Uninstalled mise tools", nil
}

// GetCommand returns the actual command(s) that will be executed
func (h *MiseHandler) GetCommand() string {
	if h.Rule.Action == "uninstall" {
		return "mise uninstall"
	}

	miseBin := h.miseCmd()
	if len(h.Rule.MisePackages) > 0 {
		global := h.isGlobal()
		var commands []string
		for _, pkg := range h.Rule.MisePackages {
			if global {
				commands = append(commands, fmt.Sprintf("%s use -g %s", miseBin, pkg))
			} else {
				commands = append(commands, fmt.Sprintf("%s use %s", miseBin, pkg))
			}
		}
		if !global {
			return fmt.Sprintf("(in %s) %s", h.Rule.MisePath, strings.Join(commands, " && "))
		}
		return strings.Join(commands, " && ")
	}
	return "mise-init"
}

// UpdateStatus updates the status after installing or uninstalling mise tools
func (h *MiseHandler) UpdateStatus(status *Status, records []ExecutionRecord, blueprint string, osName string) error {
	blueprint = normalizePath(blueprint)

	if h.Rule.Action == "mise" {
		// Check if mise use was executed successfully by matching the recorded command
		expectedCmd := h.GetCommand()
		_, commandExecuted := commandSuccessfullyExecuted(expectedCmd, records)

		if commandExecuted {
			for _, pkg := range h.Rule.MisePackages {
				var tool, version string
				if strings.Contains(pkg, "@") {
					parts := strings.SplitN(pkg, "@", 2)
					tool = strings.TrimSpace(parts[0])
					version = strings.TrimSpace(parts[1])
				} else {
					tool = strings.TrimSpace(pkg)
					version = "latest"
				}

				// Skip duplicates
				exists := false
				for _, mise := range status.Mises {
					if mise.Tool == tool && mise.Version == version &&
						normalizePath(mise.Blueprint) == blueprint && mise.OS == osName {
						exists = true
						break
					}
				}

				if !exists {
					status.Mises = append(status.Mises, MiseStatus{
						Tool:        tool,
						Version:     version,
						InstalledAt: time.Now().Format(time.RFC3339),
						Blueprint:   blueprint,
						OS:          osName,
					})
				}
			}
		}
	} else if h.Rule.Action == "uninstall" && DetectRuleType(h.Rule) == "mise" {
		if succeededMiseUninstall(records) {
			for _, pkg := range h.Rule.MisePackages {
				var tool, version string
				if strings.Contains(pkg, "@") {
					parts := strings.SplitN(pkg, "@", 2)
					tool = strings.TrimSpace(parts[0])
					version = strings.TrimSpace(parts[1])
				} else {
					tool = strings.TrimSpace(pkg)
					version = "latest"
				}

				var newMises []MiseStatus
				for _, mise := range status.Mises {
					if mise.Tool != tool || mise.Version != version ||
						normalizePath(mise.Blueprint) != blueprint || mise.OS != osName {
						newMises = append(newMises, mise)
					}
				}
				status.Mises = newMises
			}
		}
	}

	return nil
}

// DisplayInfo displays handler-specific information
func (h *MiseHandler) DisplayInfo() {
	formatFunc := ui.FormatInfo
	if h.Rule.Action == "uninstall" {
		formatFunc = ui.FormatDim
	}

	if len(h.Rule.MisePackages) > 0 {
		fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Tools: [%s]", strings.Join(h.Rule.MisePackages, ", "))))
	} else {
		fmt.Printf("  %s\n", formatFunc("Description: Installs mise version manager"))
	}
}

// DisplayStatusFromStatus displays mise handler status from Status object
func (h *MiseHandler) DisplayStatusFromStatus(status *Status) {
	if status == nil {
		return
	}

	if len(status.Mises) > 0 {
		fmt.Printf("\n%s\n", ui.FormatHighlight("Mise Version Manager:"))

		for _, mise := range status.Mises {
			t, err := time.Parse(time.RFC3339, mise.InstalledAt)
			var timeStr string
			if err == nil {
				timeStr = t.Format("2006-01-02 15:04:05")
			} else {
				timeStr = mise.InstalledAt
			}

			toolVersion := fmt.Sprintf("%s@%s", mise.Tool, mise.Version)
			fmt.Printf("  %s %s (%s) [%s, %s]\n",
				ui.FormatSuccess("●"),
				ui.FormatInfo(toolVersion),
				ui.FormatDim(timeStr),
				ui.FormatDim(mise.OS),
				ui.FormatDim(abbreviateBlueprintPath(mise.Blueprint)),
			)
		}
	}
}

// GetDependencyKey returns the unique key for this rule in dependency resolution
func (h *MiseHandler) GetDependencyKey() string {
	fallback := "mise"
	if h.Rule.Action == "uninstall" {
		if DetectRuleType(h.Rule) == "mise" {
			fallback = "uninstall-mise"
		}
	}
	return getDependencyKey(h.Rule, fallback)
}

// GetDisplayDetails returns the tools to display during execution
func (h *MiseHandler) GetDisplayDetails(isUninstall bool) string {
	if len(h.Rule.MisePackages) > 0 {
		return strings.Join(h.Rule.MisePackages, ", ")
	}
	return "mise"
}

// GetState returns handler-specific state as key-value pairs
func (h *MiseHandler) GetState(isUninstall bool) map[string]string {
	summary := h.GetDisplayDetails(isUninstall)
	return map[string]string{
		"summary": summary,
		"tools":   strings.Join(h.Rule.MisePackages, ", "),
	}
}

// FindUninstallRules compares mise status against current rules and returns uninstall rules
func (h *MiseHandler) FindUninstallRules(status *Status, currentRules []parser.Rule, blueprintFile, osName string) []parser.Rule {
	normalizedBlueprint := normalizePath(blueprintFile)

	// Build set of current mise packages from rules (tool@version format)
	currentPackages := make(map[string]bool)
	for _, rule := range currentRules {
		if rule.Action == "mise" {
			for _, pkg := range rule.MisePackages {
				currentPackages[pkg] = true
			}
		}
	}

	// Find packages to uninstall (in status but not in current rules)
	var misePackagesToRemove []string
	if status.Mises != nil {
		for _, mise := range status.Mises {
			normalizedStatusBlueprint := normalizePath(mise.Blueprint)
			if normalizedStatusBlueprint == normalizedBlueprint && mise.OS == osName {
				pkgKey := fmt.Sprintf("%s@%s", mise.Tool, mise.Version)
				if !currentPackages[pkgKey] {
					misePackagesToRemove = append(misePackagesToRemove, pkgKey)
				}
			}
		}
	}

	var rules []parser.Rule
	if len(misePackagesToRemove) > 0 {
		rules = append(rules, parser.Rule{
			Action:       "uninstall",
			MisePackages: misePackagesToRemove,
			OSList:       []string{osName},
		})
	}

	return rules
}

// succeededMiseUninstall checks if mise uninstall was successful
func succeededMiseUninstall(records []ExecutionRecord) bool {
	for _, record := range records {
		if record.Status == "success" && strings.HasPrefix(record.Command, "mise uninstall") {
			return true
		}
	}
	return false
}
