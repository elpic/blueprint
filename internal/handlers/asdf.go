package handlers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	gitpkg "github.com/elpic/blueprint/internal/git"
	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
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
	// Build a single shell command that sources asdf first, then installs all packages
	// This ensures asdf is available even if freshly installed
	var allCmds []string

	for _, pkg := range h.Rule.AsdfPackages {
		parts := strings.Split(pkg, "@")
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid asdf package format: %s, expected format: plugin@version", pkg)
		}

		plugin := strings.TrimSpace(parts[0])
		version := strings.TrimSpace(parts[1])

		// Validate plugin and version names (reject shell metacharacters)
		if !isValidAsdfIdentifier(plugin) {
			return "", fmt.Errorf("invalid plugin name: %s (contains invalid characters)", plugin)
		}
		if !isValidAsdfIdentifier(version) {
			return "", fmt.Errorf("invalid version: %s (contains invalid characters)", version)
		}

		// Add each command to the list
		allCmds = append(allCmds,
			fmt.Sprintf("asdf plugin add %s 2>/dev/null || true", plugin),
			fmt.Sprintf("asdf install %s %s", plugin, version),
			fmt.Sprintf("asdf local %s %s", plugin, version),
		)
	}

	// Execute all commands in a single shell session with asdf sourced once
	if len(allCmds) > 0 {
		// Source bashrc once at the beginning to make asdf available for all commands
		combinedCmd := strings.Join(allCmds, " && ")
		fullCmd := fmt.Sprintf(". ~/.bashrc 2>/dev/null || true && %s", combinedCmd)
		if err := exec.Command("sh", "-c", fullCmd).Run(); err != nil {
			return "", fmt.Errorf("failed to install asdf packages: %w", err)
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

		plugin := strings.TrimSpace(parts[0])
		version := strings.TrimSpace(parts[1])

		// Validate plugin and version names
		if !isValidAsdfIdentifier(plugin) || !isValidAsdfIdentifier(version) {
			continue
		}

		// Uninstall version
		uninstallCmd := fmt.Sprintf("asdf uninstall %s %s", plugin, version)
		_ = exec.Command("sh", "-c", uninstallCmd).Run() // Continue even if uninstall fails
	}

	// Remove plugins
	for _, pkg := range h.Rule.AsdfPackages {
		parts := strings.Split(pkg, "@")
		if len(parts) != 2 {
			continue
		}

		plugin := strings.TrimSpace(parts[0])

		// Validate plugin name
		if !isValidAsdfIdentifier(plugin) {
			continue
		}

		// Remove plugin
		removeCmd := fmt.Sprintf("asdf plugin remove %s 2>/dev/null || true", plugin)
		_ = exec.Command("sh", "-c", removeCmd).Run() // Continue even if remove fails
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
	_, _ = executeCommandWithCache(depCmd) // Continue even if coreutils fails

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
	_, _ = executeCommandWithCache(depCmd) // Don't fail - bash might already be installed

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
	// First ensure bashrc exists
	touchCmd := `touch ~/.bashrc`
	if _, err := executeCommandWithCache(touchCmd); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not create ~/.bashrc\n")
	}

	// Check if already present using grep
	checkCmd := `grep -q '. $HOME/.asdf/asdf.sh' ~/.bashrc 2>/dev/null`
	_, checkErr := executeCommandWithCache(checkCmd)
	hasAsdfInit := checkErr == nil

	if !hasAsdfInit {
		// Add asdf initialization to bashrc
		addCmd := `echo ". $HOME/.asdf/asdf.sh" >> ~/.bashrc`
		if _, err := executeCommandWithCache(addCmd); err != nil {
			// Don't fail if bashrc update fails
			fmt.Fprintf(os.Stderr, "Warning: could not update ~/.bashrc\n")
		}
	}

	// Source bashrc to load asdf in current shell
	sourceCmd := `bash -c 'source ~/.bashrc'`
	if _, err := executeCommandWithCache(sourceCmd); err != nil {
		// Don't fail if sourcing fails - asdf will be available in new shell
		fmt.Fprintf(os.Stderr, "Warning: asdf will be available in your next shell session\n")
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
	removeAsdfCmd := `rm -rf ~/.asdf`
	if _, err := executeCommandWithCache(removeAsdfCmd); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to remove ~/.asdf\n")
	}

	return nil
}

// uninstallAsdfLinux removes asdf from Linux (cloned from GitHub repository)
func (h *AsdfHandler) uninstallAsdfLinux() error {
	// Remove asdf directory that was cloned from GitHub
	removeAsdfCmd := `rm -rf ~/.asdf`
	if _, err := executeCommandWithCache(removeAsdfCmd); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to remove ~/.asdf\n")
	}

	// Remove asdf initialization from bashrc using sed
	removeBashrcCmd := `sed -i.bak '/. $HOME\/.asdf\/asdf.sh/d' ~/.bashrc 2>/dev/null || true`
	if _, err := executeCommandWithCache(removeBashrcCmd); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not remove asdf initialization from ~/.bashrc\n")
	}

	// Clean up sed backup file if it was created
	cleanupCmd := `rm -f ~/.bashrc.bak`
	_, _ = executeCommandWithCache(cleanupCmd) // Ignore errors on cleanup

	return nil
}

// GetCommand returns the actual command(s) that will be executed
func (h *AsdfHandler) GetCommand() string {
	if h.Rule.Action == "uninstall" {
		return "asdf uninstall"
	}

	// Asdf action - return installation command with all packages
	if len(h.Rule.AsdfPackages) > 0 {
		var commands []string
		for _, pkg := range h.Rule.AsdfPackages {
			parts := strings.Split(pkg, "@")
			if len(parts) == 2 {
				plugin := parts[0]
				version := parts[1]
				commands = append(commands, fmt.Sprintf("asdf install %s %s", plugin, version))
			}
		}
		if len(commands) > 0 {
			return strings.Join(commands, " && ")
		}
	}

	return "asdf-init"
}

// UpdateStatus updates the status after installing or uninstalling asdf
func (h *AsdfHandler) UpdateStatus(status *Status, records []ExecutionRecord, blueprint string, osName string) error {
	// Normalize blueprint path for consistent storage and comparison
	blueprint = normalizePath(blueprint)

	if h.Rule.Action == "asdf" {
		// Check if asdf was executed successfully
		commandExecuted := false
		var asdfSHA string
		for _, record := range records {
			// Check if this is an asdf install command that succeeded
			if record.Status == "success" && strings.Contains(record.Command, "asdf install") {
				commandExecuted = true
				// Extract SHA from output using regex
				asdfSHA = extractSHAFromOutput(record.Output)
				break
			}
		}

		if commandExecuted {
			// Use "~/.asdf" as the path identifier for status tracking
			asdfPath := "~/.asdf"
			// Remove existing asdf entry if present
			status.Clones = removeCloneStatus(status.Clones, asdfPath, blueprint, osName)
			// Add new entry
			status.Clones = append(status.Clones, CloneStatus{
				URL:       "https://github.com/asdf-vm/asdf.git",
				Path:      asdfPath,
				SHA:       asdfSHA,
				ClonedAt:  time.Now().Format(time.RFC3339),
				Blueprint: blueprint,
				OS:        osName,
			})

			// Also store individual asdf packages/plugins
			// Clear existing entries for this blueprint/OS
			status.Asdfs = removeAsdfStatus(status.Asdfs, "", blueprint, osName)

			// Add new entries for each package
			for _, pkg := range h.Rule.AsdfPackages {
				parts := strings.Split(pkg, "@")
				if len(parts) == 2 {
					plugin := strings.TrimSpace(parts[0])
					version := strings.TrimSpace(parts[1])
					status.Asdfs = append(status.Asdfs, AsdfStatus{
						Plugin:      plugin,
						Version:     version,
						InstalledAt: time.Now().Format(time.RFC3339),
						Blueprint:   blueprint,
						OS:          osName,
					})
				}
			}
		}
	} else if h.Rule.Action == "uninstall" && DetectRuleType(h.Rule) == "asdf" {
		// Check if asdf was uninstalled successfully
		if succeededAsdfUninstall(records) {
			// Remove asdf from status
			asdfPath := "~/.asdf"
			status.Clones = removeCloneStatus(status.Clones, asdfPath, blueprint, osName)
			// Also remove all asdf packages for this blueprint/OS
			status.Asdfs = removeAsdfStatus(status.Asdfs, "", blueprint, osName)
		}
	}

	return nil
}

// DisplayInfo displays handler-specific information
func (h *AsdfHandler) DisplayInfo() {
	formatFunc := ui.FormatInfo
	if h.Rule.Action == "uninstall" {
		formatFunc = ui.FormatDim
	}

	if len(h.Rule.AsdfPackages) > 0 {
		fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Plugins: [%s]", strings.Join(h.Rule.AsdfPackages, ", "))))
	} else {
		fmt.Printf("  %s\n", formatFunc("Description: Installs asdf version manager"))
	}
}

// DisplayStatusFromStatus displays asdf handler status from Status object
// Displays both the asdf installation and the individual plugins/versions installed
func (h *AsdfHandler) DisplayStatusFromStatus(status *Status) {
	if status == nil {
		return
	}

	// Display asdf installation header if there are any asdf entries
	if status.Asdfs != nil && len(status.Asdfs) > 0 {
		fmt.Printf("\n%s\n", ui.FormatHighlight("ASDF Version Manager:"))

		// Group packages by their installation date (usually all at once)
		// Display each installed plugin/version
		for _, asdf := range status.Asdfs {
			// Parse timestamp for display
			t, err := time.Parse(time.RFC3339, asdf.InstalledAt)
			var timeStr string
			if err == nil {
				timeStr = t.Format("2006-01-02 15:04:05")
			} else {
				timeStr = asdf.InstalledAt
			}

			// Display plugin@version format
			pluginVersion := fmt.Sprintf("%s@%s", asdf.Plugin, asdf.Version)
			fmt.Printf("  %s %s (%s) [%s, %s]\n",
				ui.FormatSuccess("●"),
				ui.FormatInfo(pluginVersion),
				ui.FormatDim(timeStr),
				ui.FormatDim(asdf.OS),
				ui.FormatDim(abbreviateBlueprintPath(asdf.Blueprint)),
			)
		}
	}
}

// GetDependencyKey returns the unique key for this rule in dependency resolution
func (h *AsdfHandler) GetDependencyKey() string {
	fallback := "asdf"
	if h.Rule.Action == "uninstall" {
		if DetectRuleType(h.Rule) == "asdf" {
			fallback = "uninstall-asdf"
		}
	}
	return getDependencyKey(h.Rule, fallback)
}

// GetDisplayDetails returns "asdf" to display during execution
func (h *AsdfHandler) GetDisplayDetails(isUninstall bool) string {
	return "asdf"
}

// FindUninstallRules compares asdf status against current rules and returns uninstall rules
// Note: asdf is tracked as a clone with path ~/.asdf
func (h *AsdfHandler) FindUninstallRules(status *Status, currentRules []parser.Rule, blueprintFile, osName string) []parser.Rule {
	normalizedBlueprint := normalizePath(blueprintFile)

	// Check if asdf is in current rules
	asdfInCurrentRules := false
	for _, rule := range currentRules {
		if rule.Action == "asdf" {
			asdfInCurrentRules = true
			break
		}
	}

	// If asdf is not in current rules but is in status, uninstall it
	var rules []parser.Rule
	if !asdfInCurrentRules && status.Clones != nil {
		for _, clone := range status.Clones {
			if clone.Path == "~/.asdf" {
				normalizedStatusBlueprint := normalizePath(clone.Blueprint)
				if normalizedStatusBlueprint == normalizedBlueprint && clone.OS == osName {
					rules = append(rules, parser.Rule{
						Action:    "uninstall",
						ClonePath: clone.Path,
						CloneURL:  clone.URL,
						OSList:    []string{osName},
					})
				}
			}
		}
	}

	return rules
}

// succeededAsdfUninstall checks if asdf uninstall was successful
func succeededAsdfUninstall(records []ExecutionRecord) bool {
	for _, record := range records {
		// Check if any asdf uninstall command succeeded
		if record.Status == "success" && record.Command == "asdf uninstall" {
			return true
		}
	}
	return false
}

// isValidAsdfIdentifier validates that a plugin or version name is safe to use in shell commands
// It only allows alphanumeric characters, dots, hyphens, and underscores
func isValidAsdfIdentifier(identifier string) bool {
	if identifier == "" {
		return false
	}
	// Match pattern: alphanumeric, dots, hyphens, underscores, plus sign
	matched, err := regexp.MatchString(`^[a-zA-Z0-9._\-+]+$`, identifier)
	if err != nil {
		return false
	}
	return matched
}
