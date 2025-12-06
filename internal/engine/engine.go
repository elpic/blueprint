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
	"github.com/elpic/blueprint/internal/git"
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

func Run(file string, dry bool) {
	var setupPath string
	var err error

	// Check if input is a git URL
	if git.IsGitURL(file) {
		// Clone the repository (show progress in dry run mode, hide in apply mode)
		tempDir, setupFile, err := git.CloneRepository(file, dry)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer git.CleanupRepository(tempDir)

		// Find setup file in the cloned repo
		setupPath, err = git.FindSetupFile(tempDir, setupFile)
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
	if git.IsGitURL(file) {
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

		// Display command that will be executed
		cmd := buildCommand(rule)
		fmt.Printf("  Command: %s\n", ui.FormatDim(cmd))
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

	if rule.Action == "install" {
		// Use brew for macOS, apt for Linux
		if len(rule.OSList) > 0 && rule.OSList[0] == "mac" {
			return fmt.Sprintf("brew install %s", pkgNames)
		}
		return fmt.Sprintf("apt-get install -y %s", pkgNames)
	} else if rule.Action == "uninstall" {
		// Uninstall commands
		if len(rule.OSList) > 0 && rule.OSList[0] == "mac" {
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

	for i, rule := range rules {
		cmd := buildCommand(rule)

		// Show actual command including sudo if needed
		actualCmd := cmd
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

		fmt.Printf("[%d/%d] %s", i+1, len(rules), ui.FormatHighlight(rule.Action))
		if packages != "" {
			fmt.Printf(" %s", ui.FormatInfo(packages))
		}

		// Execute the command
		output, err := executeCommand(cmd)

		// Create execution record
		record := ExecutionRecord{
			Timestamp: time.Now().Format(time.RFC3339),
			Blueprint: blueprint,
			OS:        osName,
			Command:   actualCmd,
			Output:    strings.TrimSpace(output),
		}

		if err != nil {
			fmt.Printf(" %s\n", ui.FormatError("Failed"))
			record.Status = "error"
			record.Error = err.Error()
		} else {
			fmt.Printf(" %s\n", ui.FormatSuccess("Done"))
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

	// Extract historically installed packages for this blueprint and OS
	historicalPackages := make(map[string]bool)
	for _, record := range records {
		// Only consider successful commands from this blueprint on this OS
		if record.Status == "success" && record.Blueprint == blueprintFile && record.OS == osName {
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
		if record.Status == "success" && record.Blueprint == blueprintFile && record.OS == osName {
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

	return autoUninstallRules
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

