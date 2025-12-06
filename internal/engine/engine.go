package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	gitpkg "github.com/elpic/blueprint/internal/git"
	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
)

type ExecutionRecord struct {
	Timestamp string `json:"timestamp"`
	Blueprint string `json:"blueprint"`
	OS        string `json:"os"`
	Command   string `json:"command"`
	Status    string `json:"status"`
	Output    string `json:"output,omitempty"`
	Error     string `json:"error,omitempty"`
}

// PackageStatus tracks an installed package
type PackageStatus struct {
	Name        string `json:"name"`
	InstalledAt string `json:"installed_at"`
	Blueprint   string `json:"blueprint"`
	OS          string `json:"os"`
}

// CloneStatus tracks a cloned repository
type CloneStatus struct {
	URL       string `json:"url"`
	Path      string `json:"path"`
	SHA       string `json:"sha"`
	ClonedAt  string `json:"cloned_at"`
	Blueprint string `json:"blueprint"`
	OS        string `json:"os"`
}

// Status represents the current state of installed packages and clones
type Status struct {
	Packages []PackageStatus `json:"packages"`
	Clones   []CloneStatus   `json:"clones"`
}

func Run(file string, dry bool) {
	var setupPath string
	var err error

	// Check if input is a git URL
	if gitpkg.IsGitURL(file) {
		// Clone the repository (show progress in dry run mode, hide in apply mode)
		tempDir, setupFile, err := gitpkg.CloneRepository(file, dry)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer gitpkg.CleanupRepository(tempDir)

		// Find setup file in the cloned repo
		setupPath, err = gitpkg.FindSetupFile(tempDir, setupFile)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
	} else {
		// Local file
		setupPath = file
	}

	// Parse the setup file (with include support for local files)
	var rules []parser.Rule
	if gitpkg.IsGitURL(file) {
		// For git URLs, read content and parse with string parsing
		data, err := os.ReadFile(setupPath)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		rules, err = parser.Parse(string(data))
		if err != nil {
			fmt.Println("Parse error:", err)
			return
		}
	} else {
		// For local files, use ParseFile which supports includes
		rules, err = parser.ParseFile(setupPath)
		if err != nil {
			fmt.Println("Parse error:", err)
			return
		}
	}

	// Filter rules by current OS
	filteredRules := filterRulesByOS(rules)
	currentOS := getOSName()

	// Check history and add auto-uninstall rules for removed packages
	autoUninstallRules := getAutoUninstallRules(filteredRules, file, currentOS)
	allRules := append(filteredRules, autoUninstallRules...)

	if dry {
		ui.PrintExecutionHeader(false, currentOS, file, len(filteredRules), len(autoUninstallRules))
		displayRules(filteredRules)
		if len(autoUninstallRules) > 0 {
			ui.PrintAutoUninstallSection()
			displayRules(autoUninstallRules)
		}
		ui.PrintPlanFooter()
	} else {
		ui.PrintExecutionHeader(true, currentOS, file, len(filteredRules), len(autoUninstallRules))
		records := executeRules(allRules, file, currentOS)
		if err := saveHistory(records); err != nil {
			fmt.Printf("Warning: Failed to save history: %v\n", err)
		}
		if err := saveStatus(allRules, records, file, currentOS); err != nil {
			fmt.Printf("Warning: Failed to save status: %v\n", err)
		}
	}
}

// getOSName returns the current operating system name
func getOSName() string {
	switch runtime.GOOS {
	case "darwin":
		return "mac"
	case "linux":
		return "linux"
	case "windows":
		return "windows"
	default:
		return runtime.GOOS
	}
}

// filterRulesByOS filters rules to only include those for the current OS
func filterRulesByOS(rules []parser.Rule) []parser.Rule {
	currentOS := getOSName()
	var filtered []parser.Rule

	for _, rule := range rules {
		// If no OS is specified, rule applies to all systems
		if len(rule.OSList) == 0 {
			filtered = append(filtered, rule)
			continue
		}

		// Check if rule applies to current OS
		for _, os := range rule.OSList {
			if strings.TrimSpace(os) == currentOS {
				filtered = append(filtered, rule)
				break
			}
		}
	}

	return filtered
}

func displayRules(rules []parser.Rule) {
	for i, rule := range rules {
		fmt.Printf("Rule #%s:\n", ui.FormatHighlight(fmt.Sprint(i+1)))
		fmt.Printf("  Action: %s\n", ui.FormatHighlight(rule.Action))

		if rule.ID != "" {
			fmt.Printf("  ID: %s\n", ui.FormatDim(rule.ID))
		}

		// Display rule-specific information
		if rule.Action == "clone" {
			fmt.Printf("  URL: %s\n", ui.FormatInfo(rule.CloneURL))
			fmt.Printf("  Path: %s\n", ui.FormatInfo(rule.ClonePath))
			if rule.Branch != "" {
				fmt.Printf("  Branch: %s\n", ui.FormatDim(rule.Branch))
			}
		} else if rule.Action == "asdf" {
			if len(rule.AsdfPackages) > 0 {
				fmt.Print("  Plugins: ")
				for j, pkg := range rule.AsdfPackages {
					if j > 0 {
						fmt.Print(", ")
					}
					fmt.Print(ui.FormatInfo(pkg))
				}
				fmt.Println()
			} else {
				fmt.Printf("  Description: %s\n", ui.FormatInfo("Installs asdf version manager"))
			}
		} else if rule.Action == "uninstall-asdf" {
			fmt.Printf("  Description: %s\n", ui.FormatError("Uninstalls asdf version manager"))
		} else {
			if len(rule.Packages) > 0 {
				fmt.Print("  Packages: ")
				for j, pkg := range rule.Packages {
					if j > 0 {
						fmt.Print(", ")
					}
					fmt.Print(ui.FormatInfo(pkg.Name))
				}
				fmt.Println()
			}
		}

		if len(rule.After) > 0 {
			fmt.Print("  After: ")
			for j, dep := range rule.After {
				if j > 0 {
					fmt.Print(", ")
				}
				fmt.Print(ui.FormatHighlight(dep))
			}
			fmt.Println()
		}

		if len(rule.OSList) > 0 {
			fmt.Print("  On: ")
			for j, os := range rule.OSList {
				if j > 0 {
					fmt.Print(", ")
				}
				fmt.Print(ui.FormatDim(os))
			}
			fmt.Println()
		}

		if rule.Tool != "" {
			fmt.Printf("  Tool: %s\n", ui.FormatDim(rule.Tool))
		}

		// Display command that will be executed (for install/uninstall only)
		if rule.Action != "clone" && rule.Action != "asdf" && rule.Action != "uninstall-asdf" {
			cmd := buildCommand(rule)
			fmt.Printf("  Command: %s\n", ui.FormatDim(cmd))
		} else if rule.Action == "uninstall-asdf" {
			// For uninstall-asdf, show what command will be executed
			fmt.Printf("  Command: %s\n", ui.FormatDim("Remove ~/.asdf directory and clean shell configs"))
		}
		fmt.Println()
	}
}

func buildCommand(rule parser.Rule) string {
	if len(rule.Packages) == 0 {
		return fmt.Sprintf("%s %v", rule.Action, rule.Packages)
	}

	pkgNames := ""
	for i, pkg := range rule.Packages {
		if i > 0 {
			pkgNames += " "
		}
		pkgNames += pkg.Name
	}

	// Determine target OS: use first in OSList if specified, otherwise use current OS
	targetOS := getOSName()
	if len(rule.OSList) > 0 {
		targetOS = strings.TrimSpace(rule.OSList[0])
	}

	if rule.Action == "install" {
		// Use brew for macOS, apt for Linux
		if targetOS == "mac" {
			return fmt.Sprintf("brew install %s", pkgNames)
		}
		return fmt.Sprintf("apt-get install -y %s", pkgNames)
	} else if rule.Action == "uninstall" {
		// Uninstall commands
		if targetOS == "mac" {
			return fmt.Sprintf("brew uninstall -y %s", pkgNames)
		}
		return fmt.Sprintf("apt-get remove -y %s", pkgNames)
	}

	return fmt.Sprintf("%s %v", rule.Action, rule.Packages)
}

// needsSudo checks if a command requires sudo elevation
func needsSudo(command string) bool {
	// Only on Linux
	if runtime.GOOS != "linux" {
		return false
	}

	// Check if current user is root
	currentUser, err := user.Current()
	if err == nil {
		uid, err := strconv.Atoi(currentUser.Uid)
		if err == nil && uid == 0 {
			// Already root, no sudo needed
			return false
		}
	}

	// Package managers that require sudo on Linux
	// Both install and remove/uninstall commands need sudo
	sudoRequired := []string{
		"apt", "apt-get", "aptitude",
		"yum", "dnf",
		"pacman", "pamac",
		"zypper",
		"emerge",
		"opkg",
		"apk",
		"pkg",
	}

	cmdName := strings.Fields(command)[0]
	for _, pm := range sudoRequired {
		if cmdName == pm {
			return true
		}
	}

	return false
}

// executeCommand parses and executes a command string
func executeCommand(cmdStr string) (string, error) {
	// Check if sudo is needed
	if needsSudo(cmdStr) {
		cmdStr = "sudo " + cmdStr
	}

	// Parse command string into parts
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	// Create command
	cmd := exec.Command(parts[0], parts[1:]...)

	// Capture output
	output, err := cmd.CombinedOutput()

	return string(output), err
}

func executeRules(rules []parser.Rule, blueprint string, osName string) []ExecutionRecord {
	var records []ExecutionRecord

	// Sort rules by dependencies
	sortedRules, err := resolveDependencies(rules)
	if err != nil {
		fmt.Printf("%s\n", ui.FormatError(err.Error()))
		return records
	}

	for i, rule := range sortedRules {
		fmt.Printf("[%d/%d] %s", i+1, len(sortedRules), ui.FormatHighlight(rule.Action))

		var output string
		var err error
		var actualCmd string
		var cloneInfo string

		if rule.Action == "clone" {
			// Handle clone operation
			fmt.Printf(" %s", ui.FormatInfo(rule.ClonePath))
			output, cloneInfo, err = executeClone(rule)
			actualCmd = fmt.Sprintf("git clone %s %s", rule.CloneURL, rule.ClonePath)
			if rule.Branch != "" {
				actualCmd = fmt.Sprintf("git clone -b %s %s %s", rule.Branch, rule.CloneURL, rule.ClonePath)
			}
		} else if rule.Action == "asdf" {
			// Handle asdf installation
			output, cloneInfo, err = executeAsdf(rule)
			actualCmd = "asdf-init"
		} else if rule.Action == "uninstall-asdf" {
			// Handle asdf uninstallation
			output, err = executeUninstallAsdf()
			actualCmd = "asdf-uninstall"
		} else {
			// Handle install/uninstall operation
			cmd := buildCommand(rule)

			// Show actual command including sudo if needed
			actualCmd = cmd
			if needsSudo(cmd) {
				actualCmd = "sudo " + cmd
			}

			// Build package list string
			packages := ""
			for j, pkg := range rule.Packages {
				if j > 0 {
					packages += ", "
				}
				packages += pkg.Name
			}

			if packages != "" {
				fmt.Printf(" %s", ui.FormatInfo(packages))
			}

			// Execute the command
			output, err = executeCommand(cmd)
		}

		// Create execution record
		recordOutput := strings.TrimSpace(output)
		if cloneInfo != "" {
			recordOutput = cloneInfo
		}

		record := ExecutionRecord{
			Timestamp: time.Now().Format(time.RFC3339),
			Blueprint: blueprint,
			OS:        osName,
			Command:   actualCmd,
			Output:    recordOutput,
		}

		if err != nil {
			fmt.Printf(" %s\n", ui.FormatError("Failed"))
			// Print error details on next line
			fmt.Printf("       %s\n", ui.FormatError(err.Error()))
			record.Status = "error"
			record.Error = err.Error()
		} else {
			if cloneInfo != "" {
				fmt.Printf(" %s\n", ui.FormatSuccess(cloneInfo))
			} else {
				fmt.Printf(" %s\n", ui.FormatSuccess("Done"))
			}
			record.Status = "success"
		}

		records = append(records, record)
	}

	return records
}

// getHistoryPath returns the path to the history file in ~/.blueprint/
func getHistoryPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	blueprintDir := filepath.Join(homeDir, ".blueprint")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(blueprintDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create .blueprint directory: %w", err)
	}

	return filepath.Join(blueprintDir, "history.json"), nil
}

// getStatusPath returns the path to the status file in ~/.blueprint/
func getStatusPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	blueprintDir := filepath.Join(homeDir, ".blueprint")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(blueprintDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create .blueprint directory: %w", err)
	}

	return filepath.Join(blueprintDir, "status.json"), nil
}

// saveHistory saves execution records to ~/.blueprint/history.json
func saveHistory(records []ExecutionRecord) error {
	if len(records) == 0 {
		return nil
	}

	historyPath, err := getHistoryPath()
	if err != nil {
		return err
	}

	// Read existing history
	var allRecords []ExecutionRecord
	if data, err := os.ReadFile(historyPath); err == nil {
		json.Unmarshal(data, &allRecords)
	}

	// Append new records
	allRecords = append(allRecords, records...)

	// Write back to file
	data, err := json.MarshalIndent(allRecords, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal history: %w", err)
	}

	if err := os.WriteFile(historyPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write history file: %w", err)
	}

	return nil
}

// saveStatus saves the current status of installed packages and clones to ~/.blueprint/status.json
func saveStatus(rules []parser.Rule, records []ExecutionRecord, blueprint string, osName string) error {
	statusPath, err := getStatusPath()
	if err != nil {
		return err
	}

	// Load existing status
	var status Status
	if data, err := os.ReadFile(statusPath); err == nil {
		json.Unmarshal(data, &status)
	}

	// Create a map for quick lookup of succeeded records
	succeededCommands := make(map[string]bool)
	for _, record := range records {
		if record.Status == "success" {
			succeededCommands[record.Command] = true
		}
	}

	// Process each rule
	for _, rule := range rules {
		if rule.Action == "install" {
			// Check if this rule's command was executed successfully
			cmd := buildCommand(rule)
			if needsSudo(cmd) {
				cmd = "sudo " + cmd
			}

			if succeededCommands[cmd] {
				// Add or update package status
				for _, pkg := range rule.Packages {
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
		} else if rule.Action == "clone" {
			// Check if this rule's command was executed successfully
			cloneCmd := fmt.Sprintf("git clone %s %s", rule.CloneURL, rule.ClonePath)
			if rule.Branch != "" {
				cloneCmd = fmt.Sprintf("git clone -b %s %s %s", rule.Branch, rule.CloneURL, rule.ClonePath)
			}

			if succeededCommands[cloneCmd] {
				// Find the SHA from the records
				var cloneSHA string
				for _, record := range records {
					if record.Status == "success" && record.Command == cloneCmd {
						// Extract SHA from cloneInfo in output
						if strings.Contains(record.Output, "SHA:") {
							parts := strings.Split(record.Output, "SHA:")
							if len(parts) > 1 {
								cloneSHA = strings.TrimSpace(strings.Split(parts[1], ")")[0])
								break
							}
						}
					}
				}

				// Remove existing entry if present
				status.Clones = removeCloneStatus(status.Clones, rule.ClonePath, blueprint, osName)
				// Add new entry
				status.Clones = append(status.Clones, CloneStatus{
					URL:       rule.CloneURL,
					Path:      rule.ClonePath,
					SHA:       cloneSHA,
					ClonedAt:  time.Now().Format(time.RFC3339),
					Blueprint: blueprint,
					OS:        osName,
				})
			}
		} else if rule.Action == "uninstall" {
			// Remove uninstalled packages
			for _, pkg := range rule.Packages {
				status.Packages = removePackageStatus(status.Packages, pkg.Name, blueprint, osName)
			}
		} else if rule.Action == "asdf" {
			// Check if asdf was executed successfully
			if succeededCommands["asdf-init"] {
				// Find the SHA from the records
				var asdfSHA string
				for _, record := range records {
					if record.Status == "success" && record.Command == "asdf-init" {
						// Extract SHA from output
						if strings.Contains(record.Output, "SHA:") {
							parts := strings.Split(record.Output, "SHA:")
							if len(parts) > 1 {
								asdfSHA = strings.TrimSpace(strings.Split(parts[1], ")")[0])
								break
							}
						}
					}
				}

				// Use "asdf" as the path identifier for status tracking
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
			}
		}
	}

	// Write status to file
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal status: %w", err)
	}

	if err := os.WriteFile(statusPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write status file: %w", err)
	}

	return nil
}

// removePackageStatus removes a package from the status packages list
func removePackageStatus(packages []PackageStatus, name string, blueprint string, osName string) []PackageStatus {
	var result []PackageStatus
	for _, pkg := range packages {
		if !(pkg.Name == name && pkg.Blueprint == blueprint && pkg.OS == osName) {
			result = append(result, pkg)
		}
	}
	return result
}

// removeCloneStatus removes a clone from the status clones list
func removeCloneStatus(clones []CloneStatus, path string, blueprint string, osName string) []CloneStatus {
	var result []CloneStatus
	for _, clone := range clones {
		if !(clone.Path == path && clone.Blueprint == blueprint && clone.OS == osName) {
			result = append(result, clone)
		}
	}
	return result
}

// getAutoUninstallRules compares history with current rules and generates uninstall rules for removed packages
func getAutoUninstallRules(currentRules []parser.Rule, blueprintFile string, osName string) []parser.Rule {
	var autoUninstallRules []parser.Rule

	// Load history
	historyPath, err := getHistoryPath()
	if err != nil {
		return autoUninstallRules
	}

	// Read history file
	data, err := os.ReadFile(historyPath)
	if err != nil {
		// No history yet, nothing to uninstall
		return autoUninstallRules
	}

	// Parse history
	var records []ExecutionRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return autoUninstallRules
	}

	// Normalize the blueprint file path for comparison
	normalizedBlueprintFile := normalizePath(blueprintFile)

	// Extract historically installed packages for this blueprint and OS
	historicalPackages := make(map[string]bool)
	for _, record := range records {
		// Only consider successful commands from this blueprint on this OS
		// Use normalized paths for comparison to handle relative vs absolute paths
		if record.Status == "success" && normalizePath(record.Blueprint) == normalizedBlueprintFile && record.OS == osName {
			// Check if it's an install command
			if strings.Contains(record.Command, "install") && !strings.Contains(record.Command, "uninstall") {
				// Extract package names from command
				// Format: "brew install <packages>" or "apt-get install -y <packages>"
				pkgs := extractPackagesFromCommand(record.Command, "install")
				for _, pkg := range pkgs {
					historicalPackages[pkg] = true
				}
			}
		}
	}

	// Remove packages that have been successfully uninstalled
	for _, record := range records {
		if record.Status == "success" && normalizePath(record.Blueprint) == normalizedBlueprintFile && record.OS == osName {
			// Check if it's an uninstall command
			if strings.Contains(record.Command, "uninstall") || (strings.Contains(record.Command, "remove") && strings.Contains(record.Command, "apt-get")) {
				// Extract package names from command
				pkgs := extractPackagesFromCommand(record.Command, "uninstall")
				for _, pkg := range pkgs {
					delete(historicalPackages, pkg)
				}
			}
		}
	}

	// Get current packages from blueprint rules
	currentPackages := make(map[string]bool)
	for _, rule := range currentRules {
		if rule.Action == "install" {
			for _, pkg := range rule.Packages {
				currentPackages[pkg.Name] = true
			}
		}
	}

	// Find packages to uninstall (in history but not in current rules)
	var packagesToUninstall []parser.Package
	for pkg := range historicalPackages {
		if !currentPackages[pkg] {
			packagesToUninstall = append(packagesToUninstall, parser.Package{
				Name:    pkg,
				Version: "latest",
			})
		}
	}

	// If there are packages to uninstall, create a rule
	if len(packagesToUninstall) > 0 {
		autoUninstallRules = append(autoUninstallRules, parser.Rule{
			Action:   "uninstall",
			Packages: packagesToUninstall,
			OSList:   []string{osName},
			Tool:     "package-manager",
		})
	}

	// Check if asdf was historically used but is not in current rules
	asdfWasUsed := false
	for _, record := range records {
		if record.Status == "success" && normalizePath(record.Blueprint) == normalizedBlueprintFile && record.OS == osName {
			if record.Command == "asdf-init" || record.Command == "asdf-uninstall" {
				// If we find asdf-init, it was used; if we only find asdf-uninstall, it was already cleaned up
				if record.Command == "asdf-init" {
					asdfWasUsed = true
				} else if record.Command == "asdf-uninstall" {
					// If we find uninstall after init, mark that asdf is no longer used
					asdfWasUsed = false
				}
			}
		}
	}

	// Check if asdf is in current rules
	asdfIsInCurrentRules := false
	for _, rule := range currentRules {
		if rule.Action == "asdf" {
			asdfIsInCurrentRules = true
			break
		}
	}

	// If asdf was used but is not in current rules, create an uninstall rule
	if asdfWasUsed && !asdfIsInCurrentRules {
		autoUninstallRules = append(autoUninstallRules, parser.Rule{
			Action: "uninstall-asdf",
			OSList: []string{osName},
			Tool:   "asdf-vm",
		})
	}

	return autoUninstallRules
}

// normalizePath normalizes a file path to allow comparison of relative and absolute paths
// It converts to absolute path and normalizes separators
func normalizePath(filePath string) string {
	// Try to get absolute path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		// If conversion fails, just return the normalized version of the input
		return filepath.Clean(filePath)
	}
	return filepath.Clean(absPath)
}

// extractPackagesFromCommand extracts package names from a command string
func extractPackagesFromCommand(command string, action string) []string {
	var packages []string

	if action == "install" {
		// Handle different install commands
		if strings.Contains(command, "brew install") {
			// Extract packages after "brew install"
			parts := strings.Split(command, "brew install")
			if len(parts) > 1 {
				pkgStr := strings.TrimSpace(parts[1])
				packages = strings.Fields(pkgStr)
			}
		} else if strings.Contains(command, "apt-get install") {
			// Extract packages after "apt-get install -y"
			// Remove sudo prefix if present
			cmd := strings.TrimPrefix(command, "sudo ")
			parts := strings.Split(cmd, "apt-get install")
			if len(parts) > 1 {
				pkgStr := strings.TrimSpace(parts[1])
				// Remove -y flag
				pkgStr = strings.ReplaceAll(pkgStr, "-y", "")
				packages = strings.Fields(pkgStr)
			}
		}
	} else if action == "uninstall" {
		// Handle different uninstall commands
		if strings.Contains(command, "brew uninstall") {
			// Extract packages after "brew uninstall"
			parts := strings.Split(command, "brew uninstall")
			if len(parts) > 1 {
				pkgStr := strings.TrimSpace(parts[1])
				// Remove -y flag
				pkgStr = strings.ReplaceAll(pkgStr, "-y", "")
				packages = strings.Fields(pkgStr)
			}
		} else if strings.Contains(command, "apt-get remove") {
			// Extract packages after "apt-get remove -y"
			// Remove sudo prefix if present
			cmd := strings.TrimPrefix(command, "sudo ")
			parts := strings.Split(cmd, "apt-get remove")
			if len(parts) > 1 {
				pkgStr := strings.TrimSpace(parts[1])
				// Remove -y flag
				pkgStr = strings.ReplaceAll(pkgStr, "-y", "")
				packages = strings.Fields(pkgStr)
			}
		}
	}

	return packages
}

// resolveDependencies performs topological sort on rules based on dependencies
func resolveDependencies(rules []parser.Rule) ([]parser.Rule, error) {
	if len(rules) == 0 {
		return rules, nil
	}

	// Build maps for lookup by ID and package name
	rulesByID := make(map[string]*parser.Rule)
	rulesByPackage := make(map[string]*parser.Rule)

	for i := range rules {
		if rules[i].ID != "" {
			rulesByID[rules[i].ID] = &rules[i]
		}
		for _, pkg := range rules[i].Packages {
			rulesByPackage[pkg.Name] = &rules[i]
		}
		// Also allow clone rules to be referenced by their path
		if rules[i].Action == "clone" && rules[i].ClonePath != "" {
			rulesByPackage[rules[i].ClonePath] = &rules[i]
		}
	}

	// Track visited and recursion stack for cycle detection
	visited := make(map[string]bool)
	recursionStack := make(map[string]bool)
	var sorted []parser.Rule

	// Helper function for DFS
	var visit func(rule *parser.Rule) error
	visit = func(rule *parser.Rule) error {
		ruleKey := rule.ID
		if ruleKey == "" {
			// Use first package name as key if no ID
			if len(rule.Packages) > 0 {
				ruleKey = rule.Packages[0].Name
			} else if rule.Action == "clone" {
				// For clone rules without ID, use the clone path as key
				ruleKey = rule.ClonePath
			} else {
				return nil
			}
		}

		if recursionStack[ruleKey] {
			return fmt.Errorf("circular dependency detected involving rule %s", ruleKey)
		}

		if visited[ruleKey] {
			return nil
		}

		recursionStack[ruleKey] = true

		// Visit dependencies first
		for _, depName := range rule.After {
			var depRule *parser.Rule

			// First try to find by ID
			if depRule = rulesByID[depName]; depRule == nil {
				// Then try to find by package name
				depRule = rulesByPackage[depName]
			}

			if depRule != nil {
				if err := visit(depRule); err != nil {
					return err
				}
			}
		}

		recursionStack[ruleKey] = false
		visited[ruleKey] = true
		sorted = append(sorted, *rule)

		return nil
	}

	// Visit all rules
	for i := range rules {
		if err := visit(&rules[i]); err != nil {
			return nil, err
		}
	}

	return sorted, nil
}

// executeClone handles git clone operations with SHA tracking
func executeClone(rule parser.Rule) (string, string, error) {
	oldSHA, newSHA, status, err := gitpkg.CloneOrUpdateRepository(rule.CloneURL, rule.ClonePath, rule.Branch)

	if err != nil {
		return "", "", err
	}

	// Format status message with SHA info
	statusMsg := status
	if oldSHA != "" && newSHA != "" && oldSHA != newSHA {
		statusMsg = fmt.Sprintf("Updated (SHA changed: %s → %s)", oldSHA[:8], newSHA[:8])
	} else if newSHA != "" && status == "Cloned" {
		statusMsg = fmt.Sprintf("Cloned (SHA: %s)", newSHA[:8])
	}

	return "", statusMsg, nil
}

// getShellInfo detects the user's current shell and returns shell executable and config file
func getShellInfo() (shellExe, configFile string, err error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}

	// Get the user's shell from $SHELL environment variable
	userShell := os.Getenv("SHELL")
	if userShell == "" {
		// Fallback: try to detect from common shell executables
		if _, err := exec.LookPath("zsh"); err == nil {
			userShell = "zsh"
		} else if _, err := exec.LookPath("bash"); err == nil {
			userShell = "bash"
		} else {
			return "bash", filepath.Join(homeDir, ".bashrc"), nil // Default fallback
		}
	}

	// Extract shell name from path (e.g., "/bin/zsh" -> "zsh")
	shellName := filepath.Base(userShell)

	// Determine the appropriate config file based on shell
	var configPath string
	switch shellName {
	case "zsh":
		configPath = filepath.Join(homeDir, ".zshrc")
	case "bash":
		configPath = filepath.Join(homeDir, ".bashrc")
	case "fish":
		configPath = filepath.Join(homeDir, ".config/fish/config.fish")
	default:
		// Default to bash
		configPath = filepath.Join(homeDir, ".bashrc")
		shellName = "bash"
	}

	return shellName, configPath, nil
}

// executeAsdf handles asdf installation and shell integration
func executeAsdf(rule parser.Rule) (string, string, error) {
	asdfPath := os.ExpandEnv("$HOME/.asdf")
	var asdfStatusMsg string

	// Detect user's shell
	shellName, _, err := getShellInfo()
	if err != nil {
		return "", "", fmt.Errorf("failed to detect shell: %w", err)
	}

	// Check if asdf is already installed
	if _, err := os.Stat(asdfPath); err == nil {
		// asdf already exists, check for updates
		oldSHA, newSHA, _, err := gitpkg.CloneOrUpdateRepository(
			"https://github.com/asdf-vm/asdf.git",
			asdfPath,
			"",
		)

		if err != nil {
			return "", "", err
		}

		// Format status message
		if oldSHA != "" && newSHA != "" && oldSHA != newSHA {
			asdfStatusMsg = fmt.Sprintf("Updated (SHA changed: %s → %s)", oldSHA[:8], newSHA[:8])
		} else {
			asdfStatusMsg = "Already installed"
		}
	} else {
		// Clone asdf if not installed
		_, newSHA, cloneStatus, err := gitpkg.CloneOrUpdateRepository(
			"https://github.com/asdf-vm/asdf.git",
			asdfPath,
			"",
		)

		if err != nil {
			return "", "", err
		}

		// Setup asdf in shell configuration files
		if err := setupAsdfInShells(asdfPath); err != nil {
			return "", "", fmt.Errorf("asdf installed but failed to setup in shells: %w", err)
		}

		// Format status message with SHA
		if newSHA != "" && cloneStatus == "Cloned" {
			asdfStatusMsg = fmt.Sprintf("Installed (SHA: %s)", newSHA[:8])
		} else {
			asdfStatusMsg = cloneStatus
		}
	}

	// Now handle plugin installations if any
	if len(rule.AsdfPackages) == 0 {
		return "", asdfStatusMsg, nil
	}

	// Build source command based on detected shell
	var sourceCmd string
	switch shellName {
	case "fish":
		sourceCmd = fmt.Sprintf("source %s/asdf.fish", asdfPath)
	case "zsh":
		sourceCmd = fmt.Sprintf(". %s/asdf.sh", asdfPath)
	default:
		sourceCmd = fmt.Sprintf(". %s/asdf.sh", asdfPath)
	}

	// Process each plugin@version entry
	var pluginOutput strings.Builder
	for _, pkg := range rule.AsdfPackages {
		// Parse plugin@version format
		var plugin string
		var version string

		if strings.Contains(pkg, "@") {
			parts := strings.Split(pkg, "@")
			plugin = parts[0]
			version = strings.Join(parts[1:], "@") // Handle versions with @ (rare but possible)
		} else {
			plugin = pkg
			version = "latest"
		}

		// Build commands with proper shell sourcing
		// Step 1: Add plugin if not already added
		addCmd := fmt.Sprintf("%s && asdf plugin add %s 2>/dev/null || true", sourceCmd, plugin)
		cmd := exec.Command(shellName, "-c", addCmd)
		if err := cmd.Run(); err != nil {
			// Some plugins might fail to add if already added, that's okay
			fmt.Printf("Note: Could not add plugin %s (may already exist)\n", plugin)
		}

		// Step 2: Install specific version
		if version != "latest" && version != "" {
			installCmd := fmt.Sprintf("%s && asdf install %s %s", sourceCmd, plugin, version)
			cmd = exec.Command(shellName, "-c", installCmd)
			if output, err := cmd.CombinedOutput(); err != nil {
				return string(output), "", fmt.Errorf("failed to install %s@%s: %w", plugin, version, err)
			}
		}

		// Step 3: Set global version
		if version != "latest" && version != "" {
			globalCmd := fmt.Sprintf("%s && asdf global %s %s", sourceCmd, plugin, version)
			cmd = exec.Command(shellName, "-c", globalCmd)
			if output, err := cmd.CombinedOutput(); err != nil {
				return string(output), "", fmt.Errorf("failed to set global version for %s: %w", plugin, err)
			}
			pluginOutput.WriteString(fmt.Sprintf("\n  - %s@%s", plugin, version))
		} else {
			pluginOutput.WriteString(fmt.Sprintf("\n  - %s (latest)", plugin))
		}
	}

	// Combine asdf status with plugin installation status
	statusMsg := fmt.Sprintf("%s, plugins installed:%s", asdfStatusMsg, pluginOutput.String())
	return "", statusMsg, nil
}

// setupAsdfInShells adds asdf initialization to shell configuration files
func setupAsdfInShells(asdfPath string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Detect user's shell to know which config files to update
	shellName, _, err := getShellInfo()
	if err != nil {
		shellName = "bash" // Fallback to bash
	}

	// Determine which config files to update based on the detected shell
	var configFiles []string
	switch shellName {
	case "zsh":
		configFiles = []string{
			filepath.Join(homeDir, ".zshrc"),
			filepath.Join(homeDir, ".zsh_profile"),
		}
	case "fish":
		configFiles = []string{
			filepath.Join(homeDir, ".config/fish/config.fish"),
		}
	default: // bash
		configFiles = []string{
			filepath.Join(homeDir, ".bashrc"),
			filepath.Join(homeDir, ".bash_profile"),
		}
	}

	// Determine the source command based on shell
	var sourceCmd string
	switch shellName {
	case "fish":
		sourceCmd = fmt.Sprintf("source %s/asdf.fish", asdfPath)
	case "zsh":
		sourceCmd = fmt.Sprintf(". %s/asdf.sh", asdfPath)
	default: // bash
		sourceCmd = fmt.Sprintf(". %s/asdf.sh", asdfPath)
	}

	// Add to all relevant config files
	for _, configFile := range configFiles {
		// Create parent directory if needed (for fish)
		if strings.Contains(configFile, ".config") {
			configDir := filepath.Dir(configFile)
			if _, err := os.Stat(configDir); os.IsNotExist(err) {
				if err := os.MkdirAll(configDir, 0755); err != nil {
					return err
				}
			}
		}

		// Add to config file if it exists, or create it
		if _, err := os.Stat(configFile); err == nil {
			if err := addAsdfSourceToFile(configFile, sourceCmd); err != nil {
				return err
			}
		} else if configFile == filepath.Join(homeDir, ".zshrc") || configFile == filepath.Join(homeDir, ".bashrc") {
			// For main shell config files, create them if they don't exist
			if err := addAsdfSourceToFile(configFile, sourceCmd); err != nil {
				return err
			}
		}
	}

	return nil
}

// addAsdfSourceToFile adds the asdf source command to a shell config file if not already present
func addAsdfSourceToFile(filePath string, sourceCmd string) error {
	// Read existing content if file exists
	var content string
	if data, err := os.ReadFile(filePath); err == nil {
		content = string(data)
		// Check if asdf is already sourced in this file
		if strings.Contains(content, "asdf.sh") || strings.Contains(content, "asdf.fish") {
			return nil // Already configured
		}
	} else if !os.IsNotExist(err) {
		return err // Return error if it's not a "file not found" error
	}

	// Add source command at the end
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += "\n# asdf initialization\n" + sourceCmd + "\n"

	return os.WriteFile(filePath, []byte(content), 0644)
}

// executeUninstallAsdf handles asdf uninstallation and cleanup
func executeUninstallAsdf() (string, error) {
	asdfPath := os.ExpandEnv("$HOME/.asdf")

	// Check if asdf is installed
	if _, err := os.Stat(asdfPath); err != nil {
		// asdf not found, nothing to uninstall
		return "asdf was not installed", nil
	}

	// Remove asdf directory
	if err := os.RemoveAll(asdfPath); err != nil {
		return "", fmt.Errorf("failed to remove asdf directory: %w", err)
	}

	// Clean up shell configuration files
	if err := removeAsdfFromShells(); err != nil {
		return "", fmt.Errorf("asdf removed but failed to clean up shells: %w", err)
	}

	return "asdf uninstalled", nil
}

// removeAsdfFromShells removes asdf initialization from shell configuration files
func removeAsdfFromShells() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Detect user's shell to know which config files to clean up
	shellName, _, err := getShellInfo()
	if err != nil {
		shellName = "bash" // Fallback to bash
	}

	// Determine which config files to clean up based on the detected shell
	var configFiles []string
	switch shellName {
	case "zsh":
		configFiles = []string{
			filepath.Join(homeDir, ".zshrc"),
			filepath.Join(homeDir, ".zsh_profile"),
		}
	case "fish":
		configFiles = []string{
			filepath.Join(homeDir, ".config/fish/config.fish"),
		}
	default: // bash
		configFiles = []string{
			filepath.Join(homeDir, ".bashrc"),
			filepath.Join(homeDir, ".bash_profile"),
		}
	}

	// Remove from all relevant config files
	for _, configFile := range configFiles {
		if _, err := os.Stat(configFile); err == nil {
			if err := removeAsdfSourceFromFile(configFile); err != nil {
				return err
			}
		}
	}

	return nil
}

// removeAsdfSourceFromFile removes asdf initialization from a shell config file
func removeAsdfSourceFromFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	content := string(data)

	// Check if asdf is sourced in this file
	if !strings.Contains(content, "asdf.sh") && !strings.Contains(content, "asdf.fish") {
		return nil // Not configured
	}

	// Remove asdf-related lines
	lines := strings.Split(content, "\n")
	var result []string
	skipNextBlank := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip asdf initialization comment and source lines
		if strings.Contains(trimmed, "asdf initialization") {
			skipNextBlank = true
			continue
		}
		if strings.Contains(trimmed, "asdf.sh") || strings.Contains(trimmed, "asdf.fish") {
			skipNextBlank = true
			continue
		}

		// Skip blank line after asdf lines if needed
		if skipNextBlank && trimmed == "" {
			skipNextBlank = false
			continue
		}

		result = append(result, line)
	}

	newContent := strings.Join(result, "\n")
	// Clean up multiple consecutive blank lines
	for strings.Contains(newContent, "\n\n\n") {
		newContent = strings.ReplaceAll(newContent, "\n\n\n", "\n\n")
	}

	return os.WriteFile(filePath, []byte(newContent), 0644)
}

// PrintStatus displays the current status of installed packages and clones
func PrintStatus() {
	statusPath, err := getStatusPath()
	if err != nil {
		fmt.Printf("%s\n", ui.FormatError("Error getting status path"))
		return
	}

	// Read status file
	data, err := os.ReadFile(statusPath)
	if err != nil {
		fmt.Printf("%s\n", ui.FormatInfo("No status file found. Run 'blueprint apply' to create one."))
		return
	}

	// Parse status
	var status Status
	if err := json.Unmarshal(data, &status); err != nil {
		fmt.Printf("%s\n", ui.FormatError("Error parsing status file"))
		return
	}

	// Display header
	fmt.Printf("\n%s\n", ui.FormatHighlight("=== Blueprint Status ==="))

	// Display packages
	if len(status.Packages) > 0 {
		fmt.Printf("\n%s\n", ui.FormatHighlight("Installed Packages:"))
		for _, pkg := range status.Packages {
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
				ui.FormatDim(pkg.Blueprint),
			)
		}
	}

	// Display clones
	if len(status.Clones) > 0 {
		fmt.Printf("\n%s\n", ui.FormatHighlight("Cloned Repositories:"))
		for _, clone := range status.Clones {
			// Parse timestamp for display
			t, err := time.Parse(time.RFC3339, clone.ClonedAt)
			var timeStr string
			if err == nil {
				timeStr = t.Format("2006-01-02 15:04:05")
			} else {
				timeStr = clone.ClonedAt
			}

			shaStr := clone.SHA
			if len(shaStr) > 8 {
				shaStr = shaStr[:8]
			}

			fmt.Printf("  %s %s (%s) [%s, %s]\n",
				ui.FormatSuccess("●"),
				ui.FormatInfo(clone.Path),
				ui.FormatDim(timeStr),
				ui.FormatDim(clone.OS),
				ui.FormatDim(clone.Blueprint),
			)
			fmt.Printf("     %s %s\n",
				ui.FormatDim("URL:"),
				ui.FormatInfo(clone.URL),
			)
			if shaStr != "" {
				fmt.Printf("     %s %s\n",
					ui.FormatDim("SHA:"),
					ui.FormatDim(shaStr),
				)
			}
		}
	}

	if len(status.Packages) == 0 && len(status.Clones) == 0 {
		fmt.Printf("\n%s\n", ui.FormatInfo("No packages or repositories installed"))
	}

	fmt.Printf("\n")
}

