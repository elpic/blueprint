package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
)

// asdfVersionCache stores the fetched asdf version to avoid multiple API calls
var (
	asdfVersionCache string
	asdfVersionMutex sync.Mutex
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
	needsInstall := false
	var latestVersion string

	if !isInstalled {
		needsInstall = true
	} else {
		// If asdf is installed, check if we need to update it
		// Get the latest version available once
		version, err := getLatestAsdfVersion()
		if err == nil {
			latestVersion = version
			// Get currently installed version
			installedVersion, err := h.getInstalledAsdfVersion()
			if err == nil {
				// Compare versions (simple string comparison for semver)
				if installedVersion != latestVersion {
					// Newer version available, remove old one first
					if err := h.removeOldAsdf(); err != nil {
						return "", fmt.Errorf("failed to remove old asdf: %w", err)
					}
					needsInstall = true
				}
			}
		}
	}

	// Install asdf if needed (pass latestVersion to avoid fetching again)
	if needsInstall {
		if err := h.installAsdfWithVersion(latestVersion); err != nil {
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
		// Only add plugin and install version, don't set local version
		allCmds = append(allCmds,
			fmt.Sprintf("asdf plugin add %s 2>/dev/null || true", plugin),
			fmt.Sprintf("asdf install %s %s", plugin, version),
		)
	}

	// Execute all commands in a single shell session with asdf in PATH
	if len(allCmds) > 0 {
		// Build command with asdf in PATH - symlink and PATH are sufficient for asdf to work
		combinedCmd := strings.Join(allCmds, " && ")
		// Set PATH to include asdf bin directory
		fullCmd := fmt.Sprintf("export PATH=\"$HOME/.asdf/bin:$PATH\" && %s", combinedCmd)
		cmd := exec.Command("bash", "-c", fullCmd)
		// Ensure stdin is completely disconnected
		cmd.Stdin = nil
		// Capture output for debugging
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Sprintf("Installation output:\n%s", string(output)), fmt.Errorf("failed to install asdf packages: %w", err)
		}
	}

	if isInstalled {
		return "Installed plugins and versions", nil
	}
	return "Installed asdf and plugins", nil
}

// Down uninstalls asdf packages and optionally asdf itself
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

		// Check if there are any other versions of this plugin installed
		// Only remove the plugin if no other versions exist
		checkCmd := fmt.Sprintf(". ~/.bashrc 2>/dev/null || true && asdf list %s 2>/dev/null | grep -v '^ ' | wc -l", plugin)
		output, err := exec.Command("sh", "-c", checkCmd).Output()
		if err == nil {
			// Count of installed versions (excluding system version)
			versionCount := 0
			_, _ = fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &versionCount)

			// If no other versions exist, remove the plugin
			if versionCount == 0 {
				removeCmd := fmt.Sprintf(". ~/.bashrc 2>/dev/null || true && asdf plugin remove %s 2>/dev/null || true", plugin)
				_ = exec.Command("sh", "-c", removeCmd).Run() // Continue even if remove fails
			}
		}
	}

	// Only uninstall asdf completely if there are no more plugins installed
	// Check if asdf has any plugins left
	checkPluginsCmd := `. ~/.bashrc 2>/dev/null || true && asdf plugin list 2>/dev/null | wc -l`
	output, err := exec.Command("sh", "-c", checkPluginsCmd).Output()
	pluginCount := 0
	if err == nil {
		_, _ = fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &pluginCount)
	}

	// Only uninstall asdf if there are no plugins left
	if pluginCount == 0 {
		if err := h.uninstallAsdf(); err != nil {
			return "", fmt.Errorf("failed to uninstall asdf: %w", err)
		}
		return "Uninstalled asdf and all plugins", nil
	}

	return "Uninstalled asdf packages", nil
}

// isAsdfInstalled checks if asdf is installed in /usr/local/bin
func (h *AsdfHandler) isAsdfInstalled() bool {
	// Check if /usr/local/bin/asdf exists
	_, err := os.Stat("/usr/local/bin/asdf")
	return err == nil
}

// getInstalledAsdfVersion returns the currently installed asdf version
func (h *AsdfHandler) getInstalledAsdfVersion() (string, error) {
	// Try to get version from asdf
	cmd := exec.Command("sh", "-c", ". ~/.bashrc 2>/dev/null || true && asdf --version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get asdf version: %w", err)
	}

	// Parse version from output (format: "v0.18.0" or "asdf 0.18.0")
	versionStr := strings.TrimSpace(string(output))
	if versionStr == "" {
		return "", fmt.Errorf("empty version output from asdf --version")
	}

	// Remove 'v' prefix and 'asdf' prefix if present
	versionStr = strings.TrimPrefix(versionStr, "v")
	versionStr = strings.TrimPrefix(versionStr, "asdf ")

	// Take first word if multiple words
	fields := strings.Fields(versionStr)
	if len(fields) > 0 {
		versionStr = fields[0]
	}

	return versionStr, nil
}

// removeOldAsdf removes the old asdf installation from /usr/local/bin
func (h *AsdfHandler) removeOldAsdf() error {
	asdfPath := "/usr/local/bin/asdf"

	// Try to remove without sudo first
	if err := os.Remove(asdfPath); err == nil {
		return nil
	}

	// If that fails, try with sudo
	removeCmd := fmt.Sprintf("sudo rm -f %s", asdfPath)
	if _, err := executeCommandWithCache(removeCmd); err != nil {
		return fmt.Errorf("failed to remove old asdf: %w", err)
	}

	return nil
}

// installAsdfWithVersion installs asdf using the best available method with a cached version
// Pass empty string for version to fetch it automatically
func (h *AsdfHandler) installAsdfWithVersion(version string) error {
	switch runtime.GOOS {
	case "darwin":
		return h.installAsdfMacOS()

	case "linux":
		return h.installAsdfLinuxWithVersion(version)

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

// installAsdfLinuxWithVersion installs asdf on Linux with a cached version
// Pass empty string for version to fetch it automatically
func (h *AsdfHandler) installAsdfLinuxWithVersion(version string) error {
	// Get system architecture using uname -m (maps to asdf release names)
	asdfArch, err := getSystemArchitecture()
	if err != nil {
		return fmt.Errorf("failed to detect system architecture: %w", err)
	}

	// If version is empty, fetch the latest release version from GitHub API
	if version == "" {
		fetchedVersion, err := getLatestAsdfVersion()
		if err != nil {
			return fmt.Errorf("failed to get latest asdf version: %w", err)
		}
		version = fetchedVersion
	}

	downloadURL := fmt.Sprintf("https://github.com/asdf-vm/asdf/releases/download/v%s/asdf-v%s-linux-%s.tar.gz", version, version, asdfArch)

	// Create temporary directory for download and extraction
	tmpDir, err := os.MkdirTemp("", "asdf-install-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	// Download the binary
	asdfTarPath := filepath.Join(tmpDir, fmt.Sprintf("asdf-v%s-linux-%s.tar.gz", version, asdfArch))
	downloadCmd := fmt.Sprintf("curl -fsSL -o %s %s", asdfTarPath, downloadURL)
	if _, err := executeCommandWithCache(downloadCmd); err != nil {
		return fmt.Errorf("failed to download asdf binary: %w", err)
	}

	// Extract tar.gz to temp directory
	extractCmd := fmt.Sprintf("tar -xzf %s -C %s", asdfTarPath, tmpDir)
	if _, err := executeCommandWithCache(extractCmd); err != nil {
		return fmt.Errorf("failed to extract asdf binary: %w", err)
	}

	// The extracted file is just 'asdf' in the temp directory
	extractedBinary := filepath.Join(tmpDir, "asdf")
	if _, err := os.Stat(extractedBinary); err != nil {
		return fmt.Errorf("asdf binary not found in archive: %w", err)
	}

	// Install directly to /usr/local/bin/asdf
	asdfBinPath := "/usr/local/bin/asdf"

	// Try to copy the binary directly first (for root or if /usr/local/bin is writable)
	if err := os.Rename(extractedBinary, asdfBinPath); err == nil {
		// Successfully moved, make it executable
		// 0755 is standard for system-wide binaries in /usr/local/bin
		// #nosec G302
		if err := os.Chmod(asdfBinPath, 0o755); err != nil {
			return fmt.Errorf("failed to make asdf executable: %w", err)
		}
	} else {
		// Need sudo, use cp with sudo
		cpCmd := fmt.Sprintf("sudo cp %s %s", extractedBinary, asdfBinPath)
		if _, err := executeCommandWithCache(cpCmd); err != nil {
			return fmt.Errorf("failed to install asdf to %s: %w", asdfBinPath, err)
		}

		// Make it executable with sudo
		// 0755 is standard for system-wide binaries in /usr/local/bin
		chmodCmd := fmt.Sprintf("sudo chmod 755 %s", asdfBinPath) // nosec G302
		if _, err := executeCommandWithCache(chmodCmd); err != nil {
			return fmt.Errorf("failed to make asdf executable: %w", err)
		}
	}

	// asdf is now installed to /usr/local/bin/asdf which is in default PATH
	// No additional setup needed - asdf will create ~/.asdf on first use

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

// uninstallAsdfLinux removes asdf from Linux
func (h *AsdfHandler) uninstallAsdfLinux() error {
	asdfPath := "/usr/local/bin/asdf"

	// Try to remove without sudo first
	if err := os.Remove(asdfPath); err == nil {
		return nil
	}

	// If that fails, try with sudo
	removeCmd := fmt.Sprintf("sudo rm -f %s", asdfPath)
	if _, err := executeCommandWithCache(removeCmd); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to remove asdf from %s\n", asdfPath)
	}

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
		for _, record := range records {
			// Check if this is an asdf install command that succeeded
			if record.Status == "success" && strings.Contains(record.Command, "asdf install") {
				commandExecuted = true
				break
			}
		}

		if commandExecuted {
			// Store individual asdf packages/plugins in dedicated status
			// Just add entries for each package (don't remove, to allow multiple versions per plugin)
			for _, pkg := range h.Rule.AsdfPackages {
				parts := strings.Split(pkg, "@")
				if len(parts) == 2 {
					plugin := strings.TrimSpace(parts[0])
					version := strings.TrimSpace(parts[1])

					// Check if this exact plugin@version already exists
					exists := false
					for _, asdf := range status.Asdfs {
						if asdf.Plugin == plugin && asdf.Version == version &&
							normalizePath(asdf.Blueprint) == blueprint && asdf.OS == osName {
							exists = true
							break
						}
					}

					// Only add if it doesn't already exist
					if !exists {
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
		}
	} else if h.Rule.Action == "uninstall" && DetectRuleType(h.Rule) == "asdf" {
		// Check if asdf was uninstalled successfully
		if succeededAsdfUninstall(records) {
			// Remove only the specific packages (plugin@version) specified in this rule
			for _, pkg := range h.Rule.AsdfPackages {
				parts := strings.Split(pkg, "@")
				if len(parts) == 2 {
					plugin := strings.TrimSpace(parts[0])
					version := strings.TrimSpace(parts[1])

					// Remove this specific plugin@version entry
					var newAsdfs []AsdfStatus
					for _, asdf := range status.Asdfs {
						// Keep all entries that are NOT this specific plugin@version
						if asdf.Plugin != plugin || asdf.Version != version ||
							normalizePath(asdf.Blueprint) != blueprint || asdf.OS != osName {
							newAsdfs = append(newAsdfs, asdf)
						}
					}
					status.Asdfs = newAsdfs
				}
			}
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
	if len(status.Asdfs) > 0 {
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

// GetDisplayDetails returns the packages to display during execution
// Shows comma-separated package@version pairs (e.g., "node@18, python@3.11")
func (h *AsdfHandler) GetDisplayDetails(isUninstall bool) string {
	if len(h.Rule.AsdfPackages) > 0 {
		return strings.Join(h.Rule.AsdfPackages, ", ")
	}
	return "asdf"
}

// FindUninstallRules compares asdf status against current rules and returns uninstall rules
func (h *AsdfHandler) FindUninstallRules(status *Status, currentRules []parser.Rule, blueprintFile, osName string) []parser.Rule {
	normalizedBlueprint := normalizePath(blueprintFile)

	// Build set of current asdf packages from rules (plugin@version format)
	currentPackages := make(map[string]bool)
	for _, rule := range currentRules {
		if rule.Action == "asdf" {
			for _, pkg := range rule.AsdfPackages {
				currentPackages[pkg] = true
			}
		}
	}

	// Find packages to uninstall (in status but not in current rules)
	var asdfPackagesToRemove []string
	if status.Asdfs != nil {
		for _, asdf := range status.Asdfs {
			normalizedStatusBlueprint := normalizePath(asdf.Blueprint)
			if normalizedStatusBlueprint == normalizedBlueprint && asdf.OS == osName {
				pkgKey := fmt.Sprintf("%s@%s", asdf.Plugin, asdf.Version)
				if !currentPackages[pkgKey] {
					// This package is in status but not in current rules, so it should be uninstalled
					asdfPackagesToRemove = append(asdfPackagesToRemove, pkgKey)
				}
			}
		}
	}

	// Return uninstall rule if there are packages to uninstall
	var rules []parser.Rule
	if len(asdfPackagesToRemove) > 0 {
		rules = append(rules, parser.Rule{
			Action:       "uninstall",
			AsdfPackages: asdfPackagesToRemove,
			OSList:       []string{osName},
		})
	}

	return rules
}

// succeededAsdfUninstall checks if asdf uninstall was successful
func succeededAsdfUninstall(records []ExecutionRecord) bool {
	for _, record := range records {
		// Check if any asdf uninstall command succeeded
		if record.Status == "success" && strings.HasPrefix(record.Command, "asdf uninstall") {
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

// getSystemArchitecture detects the system architecture using uname -m
// and maps it to asdf release names (amd64, arm64, 386, etc.)
func getSystemArchitecture() (string, error) {
	// Get architecture using uname -m
	cmd := exec.Command("uname", "-m")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to detect architecture: %w", err)
	}

	arch := strings.TrimSpace(string(output))

	// Map system architecture to asdf release names
	switch arch {
	case "x86_64":
		return "amd64", nil
	case "aarch64":
		return "arm64", nil
	case "arm64":
		return "arm64", nil
	case "i386", "i686":
		return "386", nil
	default:
		return "", fmt.Errorf("unsupported architecture: %s", arch)
	}
}

// getLatestAsdfVersion fetches the latest asdf version from GitHub API
func getLatestAsdfVersion() (string, error) {
	// Check cache first to avoid repeated API calls
	asdfVersionMutex.Lock()
	if asdfVersionCache != "" {
		defer asdfVersionMutex.Unlock()
		return asdfVersionCache, nil
	}
	asdfVersionMutex.Unlock()

	// Fetch from GitHub API
	resp, err := http.Get("https://api.github.com/repos/asdf-vm/asdf/releases/latest")
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest asdf version: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github api returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}

	if err := json.Unmarshal(body, &release); err != nil {
		return "", fmt.Errorf("failed to parse github response: %w", err)
	}

	// Remove 'v' prefix if present
	version := strings.TrimPrefix(release.TagName, "v")
	if version == "" {
		return "", fmt.Errorf("invalid version from github api")
	}

	// Store in cache for future calls
	asdfVersionMutex.Lock()
	asdfVersionCache = version
	asdfVersionMutex.Unlock()

	return version, nil
}
