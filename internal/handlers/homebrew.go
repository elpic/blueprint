package handlers

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
)

func init() {
	RegisterAction(ActionDef{
		Name:   "homebrew",
		Prefix: "homebrew",
		NewHandler: func(rule parser.Rule, basePath string, passwordCache map[string]string) Handler {
			return NewHomebrewHandler(rule, basePath)
		},
		RuleKey: func(rule parser.Rule) string {
			if len(rule.HomebrewPackages) > 0 {
				return rule.HomebrewPackages[0]
			}
			return "homebrew"
		},
		Detect: func(rule parser.Rule) bool {
			return len(rule.HomebrewPackages) > 0
		},
		Summary: func(rule parser.Rule) string {
			return strings.Join(append(rule.HomebrewPackages, rule.HomebrewCasks...), ", ")
		},
		OrphanIndex: func(rule parser.Rule, index func(string)) {
			for _, formula := range rule.HomebrewPackages {
				index(formula)
			}
			for _, cask := range rule.HomebrewCasks {
				index(caskKey(cask))
			}
		},
	})
}

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

// Up installs the homebrew formulas/casks and ensures homebrew is installed
func (h *HomebrewHandler) Up() (string, error) {
	// Homebrew only works on macOS and Linux
	targetOS := getOSName()
	if targetOS != "mac" && targetOS != "linux" {
		return "", fmt.Errorf("homebrew is not supported on %s", targetOS)
	}

	// First ensure homebrew is installed
	if err := h.ensureHomebrewInstalled(); err != nil {
		return "", fmt.Errorf("failed to ensure homebrew is installed: %w", err)
	}

	// Filter out already-installed formulas and casks.
	// Some packages (e.g. orbstack) are stored in Caskroom even when installed
	// via a plain `homebrew <name>` rule — check both formula and cask lists.
	brew := brewCmd()
	var missingFormulas []string
	for _, f := range h.Rule.HomebrewPackages {
		if !isBrewFormulaInstalled(brew, f) && !isBrewCaskInstalled(brew, f) {
			missingFormulas = append(missingFormulas, f)
		}
	}
	var missingCasks []string
	for _, c := range h.Rule.HomebrewCasks {
		if !isBrewCaskInstalled(brew, c) {
			missingCasks = append(missingCasks, c)
		}
	}

	if len(missingFormulas) == 0 && len(missingCasks) == 0 {
		return "already installed", nil
	}

	cmd := h.buildCommandForPackages(brew, missingFormulas, missingCasks)
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

// UpdateStatus updates the status after installing or uninstalling formulas/casks.
// It uses execution records to determine success rather than shelling out to brew,
// which avoids the significant overhead of running brew commands per-package
// (each brew invocation takes ~1s).
func (h *HomebrewHandler) UpdateStatus(status *Status, records []ExecutionRecord, blueprint string, osName string) error {
	blueprint = normalizeBlueprint(blueprint)

	switch h.Rule.Action {
	case "homebrew":
		// Check if the install command succeeded or packages were already installed.
		// The handler's Up() returns "already installed" when all packages are present,
		// or runs the install command which appears in records on success.
		cmd := h.buildCommand()
		_, commandExecuted := commandSuccessfullyExecuted(cmd, records)

		// Also check for "already installed" — Up() returns this with no error
		// when all packages are present, which means the record has status "success"
		// and output "already installed". In this case the command recorded is the
		// full install command, so commandSuccessfullyExecuted already catches it.
		alreadyInstalled := false
		for i := range records {
			if records[i].Command == cmd && records[i].Status == "success" && records[i].Output == "already installed" {
				alreadyInstalled = true
				break
			}
		}

		if !commandExecuted && !alreadyInstalled {
			return nil // command failed or didn't run — don't update status
		}

		// Record each formula/cask as installed
		for _, formulaStr := range h.Rule.HomebrewPackages {
			parts := strings.Split(formulaStr, "@")
			formula := parts[0]
			version := "latest"
			if len(parts) > 1 {
				version = parts[1]
			}
			status.Brews = removeHomebrewStatus(status.Brews, formula, blueprint, osName)
			status.Brews = append(status.Brews, HomebrewStatus{
				Formula:     formula,
				Version:     version,
				InstalledAt: time.Now().Format(time.RFC3339),
				Blueprint:   blueprint,
				OS:          osName,
			})
		}

		for _, cask := range h.Rule.HomebrewCasks {
			status.Brews = removeHomebrewStatus(status.Brews, caskKey(cask), blueprint, osName)
			status.Brews = append(status.Brews, HomebrewStatus{
				Formula:     caskKey(cask),
				Version:     "cask",
				InstalledAt: time.Now().Format(time.RFC3339),
				Blueprint:   blueprint,
				OS:          osName,
			})
		}
	case "uninstall":
		for _, formulaStr := range h.Rule.HomebrewPackages {
			parts := strings.Split(formulaStr, "@")
			status.Brews = removeHomebrewStatus(status.Brews, parts[0], blueprint, osName)
		}
		for _, cask := range h.Rule.HomebrewCasks {
			status.Brews = removeHomebrewStatus(status.Brews, caskKey(cask), blueprint, osName)
		}
	}

	return nil
}

// caskKey returns a storage key that distinguishes casks from formulas with the same name
func caskKey(name string) string {
	return "cask:" + name
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
	targetOS := getOSName()
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

// isBrewFormulaInstalled checks if a formula is already installed.
// Uses sh -c to support multi-word brew invocations (e.g. under Rosetta 2).
// Overridable for testing.
var isBrewFormulaInstalled = func(brew, formula string) bool {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("%s list --versions %s", brew, formula))
	cmd.Stdin = nil
	return cmd.Run() == nil
}

// isBrewCaskInstalled checks if a cask is already installed.
// Uses sh -c to support multi-word brew invocations (e.g. under Rosetta 2).
// Overridable for testing.
var isBrewCaskInstalled = func(brew, cask string) bool {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("%s list --cask %s", brew, cask))
	cmd.Stdin = nil
	return cmd.Run() == nil
}

// knownBrewPaths are the standard install locations for homebrew on each platform.
var knownBrewPaths = []string{
	"/opt/homebrew/bin/brew",              // macOS Apple Silicon
	"/usr/local/bin/brew",                 // macOS Intel
	"/home/linuxbrew/.linuxbrew/bin/brew", // Linux (system-wide)
}

// isHomebrewInstalled checks if homebrew is installed by checking known paths
// and falling back to PATH lookup. On Linux, brew is often not on PATH even
// when installed, so the path check is essential.
func (h *HomebrewHandler) isHomebrewInstalled() bool {
	for _, p := range knownBrewPaths {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return exec.Command("which", "brew").Run() == nil
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

// brewCmd returns the correct brew invocation for the current environment.
// On Apple Silicon macs, if the current process is running under Rosetta 2,
// sysctl.proc_translated returns 1. In that case we force ARM64 execution
// via /usr/bin/arch -arm64 with the full path to brew so it always runs
// natively regardless of the parent process architecture.
// brewCmdFunc is a variable that can be mocked during testing
var brewCmdFunc = realBrewCmd

// brewCmdCache caches the result of realBrewCmd so sysctl and path lookups
// are performed at most once per process lifetime.
var brewCmdCache struct {
	once sync.Once
	val  string
}

func brewCmd() string {
	return brewCmdFunc()
}

func realBrewCmd() string {
	brewCmdCache.once.Do(func() {
		brewCmdCache.val = detectBrewCmd()
	})
	return brewCmdCache.val
}

func detectBrewCmd() string {
	if getOSName() != "mac" {
		// On Linux brew is often not on PATH — check known locations first
		for _, p := range knownBrewPaths {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
		return "brew"
	}

	// sysctl.proc_translated == "1" means this process is running under Rosetta
	out, err := exec.Command("sysctl", "-n", "sysctl.proc_translated").Output()
	if err == nil && strings.TrimSpace(string(out)) == "1" {
		return "/usr/bin/arch -arm64 /opt/homebrew/bin/brew"
	}

	return "brew"
}

// SetBrewCmdFunc sets the brew command function (for testing)
func SetBrewCmdFunc(fn func() string) {
	brewCmdFunc = fn
}

// ResetBrewCmd resets the brew command function to default and clears the cache
func ResetBrewCmd() {
	brewCmdFunc = realBrewCmd
	brewCmdCache = struct {
		once sync.Once
		val  string
	}{}
}

// buildCommand builds the install command for formulas and/or casks
func (h *HomebrewHandler) buildCommand() string {
	return h.buildCommandForPackages(brewCmd(), h.Rule.HomebrewPackages, h.Rule.HomebrewCasks)
}

// buildCommandForPackages builds a brew install command for specific package lists.
func (h *HomebrewHandler) buildCommandForPackages(brew string, formulas, casks []string) string {
	var cmds []string
	if len(formulas) > 0 {
		cmds = append(cmds, fmt.Sprintf("%s install %s", brew, strings.Join(formulas, " ")))
	}
	if len(casks) > 0 {
		cmds = append(cmds, fmt.Sprintf("%s install --cask %s", brew, strings.Join(casks, " ")))
	}
	return strings.Join(cmds, " && ")
}

// buildUninstallCommand builds the uninstall command for formulas and/or casks
func (h *HomebrewHandler) buildUninstallCommand() string {
	brew := brewCmd()
	var cmds []string

	if len(h.Rule.HomebrewPackages) > 0 {
		var formulas []string
		for _, formulaStr := range h.Rule.HomebrewPackages {
			parts := strings.Split(formulaStr, "@")
			formulas = append(formulas, parts[0])
		}
		cmds = append(cmds, fmt.Sprintf("%s uninstall -y %s", brew, strings.Join(formulas, " ")))
	}
	if len(h.Rule.HomebrewCasks) > 0 {
		cmds = append(cmds, fmt.Sprintf("%s uninstall --cask -y %s", brew, strings.Join(h.Rule.HomebrewCasks, " ")))
	}

	return strings.Join(cmds, " && ")
}

// NeedsSudo returns true if homebrew installation requires sudo privileges.
// On macOS, cask installations frequently invoke sudo internally (e.g. to move
// apps to /Applications), so we signal sudo is needed whenever casks are present.
func (h *HomebrewHandler) NeedsSudo() bool {
	return len(h.Rule.HomebrewCasks) > 0
}

// GetDependencyKey returns the unique key for this rule in dependency resolution
func (h *HomebrewHandler) GetDependencyKey() string {
	fallback := "homebrew"
	if len(h.Rule.HomebrewPackages) > 0 {
		fallback = h.Rule.HomebrewPackages[0]
	}
	return getDependencyKey(h.Rule, fallback)
}

// GetDisplayDetails returns the formulas/casks to display during execution
func (h *HomebrewHandler) GetDisplayDetails(isUninstall bool) string {
	var parts []string
	if len(h.Rule.HomebrewPackages) > 0 {
		parts = append(parts, strings.Join(h.Rule.HomebrewPackages, ", "))
	}
	if len(h.Rule.HomebrewCasks) > 0 {
		parts = append(parts, "cask: "+strings.Join(h.Rule.HomebrewCasks, ", "))
	}
	return strings.Join(parts, " | ")
}

// DisplayInfo displays handler-specific information
func (h *HomebrewHandler) DisplayInfo() {
	formatFunc := ui.FormatInfo
	if h.Rule.Action == "uninstall" {
		formatFunc = ui.FormatDim
	}
	if len(h.Rule.HomebrewPackages) > 0 {
		fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Formulas: [%s]", strings.Join(h.Rule.HomebrewPackages, ", "))))
	}
	if len(h.Rule.HomebrewCasks) > 0 {
		fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Casks: [%s]", strings.Join(h.Rule.HomebrewCasks, ", "))))
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

// GetState returns handler-specific state as key-value pairs
func (h *HomebrewHandler) GetState(isUninstall bool) map[string]string {
	summary := h.GetDisplayDetails(isUninstall)
	state := map[string]string{
		"summary": summary,
	}
	if len(h.Rule.HomebrewPackages) > 0 {
		state["formulas"] = strings.Join(h.Rule.HomebrewPackages, ", ")
	}
	if len(h.Rule.HomebrewCasks) > 0 {
		state["casks"] = strings.Join(h.Rule.HomebrewCasks, ", ")
	}
	return state
}

// FindUninstallRules compares homebrew status against current rules and returns uninstall rules
func (h *HomebrewHandler) FindUninstallRules(status *Status, currentRules []parser.Rule, blueprintFile, osName string) []parser.Rule {
	normalizedBlueprint := normalizeBlueprint(blueprintFile)

	// Build set of current formula and cask keys
	currentKeys := make(map[string]bool)
	for _, rule := range currentRules {
		if rule.Action == "homebrew" {
			for _, formulaStr := range rule.HomebrewPackages {
				parts := strings.Split(formulaStr, "@")
				currentKeys[parts[0]] = true
			}
			for _, cask := range rule.HomebrewCasks {
				currentKeys[caskKey(cask)] = true
			}
		}
	}

	// Find formulas and casks to uninstall
	var formulasToUninstall []string
	var casksToUninstall []string
	if status.Brews != nil {
		for _, brew := range status.Brews {
			normalizedStatusBlueprint := normalizeBlueprint(brew.Blueprint)
			if normalizedStatusBlueprint == normalizedBlueprint && brew.OS == osName && !currentKeys[brew.Formula] {
				if strings.HasPrefix(brew.Formula, "cask:") {
					casksToUninstall = append(casksToUninstall, strings.TrimPrefix(brew.Formula, "cask:"))
				} else {
					formulasToUninstall = append(formulasToUninstall, brew.Formula)
				}
			}
		}
	}

	var rules []parser.Rule
	if len(formulasToUninstall) > 0 || len(casksToUninstall) > 0 {
		rules = append(rules, parser.Rule{
			Action:           "uninstall",
			HomebrewPackages: formulasToUninstall,
			HomebrewCasks:    casksToUninstall,
			OSList:           []string{osName},
		})
	}
	return rules
}

// IsInstalled returns true if all homebrew formulas and casks in this rule are already in status.
func (h *HomebrewHandler) IsInstalled(status *Status, blueprintFile, osName string) bool {
	normalizedBlueprint := normalizeBlueprint(blueprintFile)

	// Build set of stored formula keys for this blueprint+OS
	stored := make(map[string]bool)
	for _, brew := range status.Brews {
		if normalizeBlueprint(brew.Blueprint) == normalizedBlueprint && brew.OS == osName {
			stored[brew.Formula] = true
		}
	}

	for _, formulaStr := range h.Rule.HomebrewPackages {
		parts := strings.Split(formulaStr, "@")
		if !stored[parts[0]] {
			return false
		}
	}
	for _, cask := range h.Rule.HomebrewCasks {
		if !stored[caskKey(cask)] {
			return false
		}
	}
	return true
}

func removeHomebrewStatus(sl []HomebrewStatus, formula, blueprint, osName string) []HomebrewStatus {
	return removeStatusEntry[HomebrewStatus, *HomebrewStatus](sl, formula, blueprint, osName)
}
