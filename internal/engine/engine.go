package engine

import (
	"encoding/json"
	"fmt"
	cryptopkg "github.com/elpic/blueprint/internal/crypto"
	gitpkg "github.com/elpic/blueprint/internal/git"
	handlerskg "github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/logging"
	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
	"golang.org/x/term"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
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

// Re-export handler types for backward compatibility
type (
	PackageStatus    = handlerskg.PackageStatus
	CloneStatus      = handlerskg.CloneStatus
	DecryptStatus    = handlerskg.DecryptStatus
	MkdirStatus      = handlerskg.MkdirStatus
	KnownHostsStatus = handlerskg.KnownHostsStatus
	GPGKeyStatus     = handlerskg.GPGKeyStatus
	Status           = handlerskg.Status
)

// passwordCache stores decryption passwords by password-id to avoid re-prompting
var passwordCache = make(map[string]string)

// RunWithSkip runs blueprint with skip filters for group and id
func RunWithSkip(file string, dry bool, skipGroup string, skipID string) {
	var setupPath string
	var err error
	var runNumber int

	// Get next run number (only for non-dry runs)
	if !dry {
		runNumber, err = getNextRunNumber()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to get run number: %v\n", err)
			runNumber = 0 // Disable history saving if we can't get run number
		}
	}

	// Check if input is a git URL
	if gitpkg.IsGitURL(file) {
		// Clone the repository (show progress in dry run mode, hide in apply mode)
		tempDir, setupFile, err := gitpkg.CloneRepository(file, dry)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer func() { _ = gitpkg.CleanupRepository(tempDir) }()

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

	// Parse the setup file (with include support for both local and git repositories)
	var rules []parser.Rule
	// Use ParseFile for both local files and git repositories
	// This enables include directive support in both cases
	rules, err = parser.ParseFile(setupPath)
	if err != nil {
		fmt.Println("Parse error:", err)
		return
	}

	// Filter rules by skip group and id
	var filteredRules []parser.Rule
	for _, rule := range rules {
		// Skip if matches skip-group
		if skipGroup != "" && rule.Group == skipGroup {
			continue
		}
		// Skip if matches skip-id
		if skipID != "" && rule.ID == skipID {
			continue
		}
		filteredRules = append(filteredRules, rule)
	}
	rules = filteredRules

	// Filter rules by current OS
	filteredRules = filterRulesByOS(rules)
	currentOS := getOSName()

	// Check history and add auto-uninstall rules for removed packages
	autoUninstallRules := getAutoUninstallRules(filteredRules, file, currentOS)
	allRules := append(filteredRules, autoUninstallRules...)

	// Delete cloned repos and decrypted files only if not using skip options
	var numCleanups int
	if skipGroup == "" && skipID == "" {
		numClonedDeletions := deleteRemovedClones(filteredRules, file, currentOS)
		numDecryptedDeletions := deleteRemovedDecryptFiles(filteredRules, file, currentOS)

		// Count uninstall rules as cleanups
		numUninstalls := 0
		for _, rule := range autoUninstallRules {
			if rule.Action == "uninstall" {
				numUninstalls++
			}
		}
		numCleanups = numClonedDeletions + numDecryptedDeletions + numUninstalls
	}

	// Extract base directory from setupPath for resolving relative file paths
	basePath := filepath.Dir(setupPath)

	if dry {
		ui.PrintExecutionHeader(false, currentOS, file, len(filteredRules), len(autoUninstallRules), numCleanups)
		displayRules(filteredRules)
		if len(autoUninstallRules) > 0 {
			ui.PrintAutoUninstallSection()
			displayRules(autoUninstallRules)
		}
		ui.PrintPlanFooter()
	} else {
		ui.PrintExecutionHeader(true, currentOS, file, len(filteredRules), len(autoUninstallRules), numCleanups)

		// Prompt for sudo password upfront (before decrypt passwords)
		// Check all rules including auto-uninstall rules
		err := promptForSudoPasswordWithOS(allRules, currentOS)
		if err != nil {
			fmt.Printf("%s\n", ui.FormatError(fmt.Sprintf("Error prompting for sudo password: %v", err)))
			return
		}

		// Prompt for all decrypt passwords upfront
		err = promptForDecryptPasswords(allRules)
		if err != nil {
			fmt.Printf("%s\n", ui.FormatError(fmt.Sprintf("Error prompting for passwords: %v", err)))
			return
		}

		records := executeRules(allRules, file, currentOS, basePath, runNumber)
		if err := saveHistory(records); err != nil {
			fmt.Printf("Warning: Failed to save history: %v\n", err)
		}
		if err := saveStatus(allRules, records, file, currentOS); err != nil {
			fmt.Printf("Warning: Failed to save status: %v\n", err)
		}
	}
}

func Run(file string, dry bool) {
	var setupPath string
	var err error
	var runNumber int

	// Get next run number (only for non-dry runs)
	if !dry {
		runNumber, err = getNextRunNumber()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to get run number: %v\n", err)
			runNumber = 0 // Disable history saving if we can't get run number
		}
	}

	// Check if input is a git URL
	if gitpkg.IsGitURL(file) {
		// Clone the repository (show progress in dry run mode, hide in apply mode)
		tempDir, setupFile, err := gitpkg.CloneRepository(file, dry)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer func() { _ = gitpkg.CleanupRepository(tempDir) }()

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

	// Parse the setup file (with include support for both local and git repositories)
	var rules []parser.Rule
	// Use ParseFile for both local files and git repositories
	// This enables include directive support in both cases
	rules, err = parser.ParseFile(setupPath)
	if err != nil {
		fmt.Println("Parse error:", err)
		return
	}

	// Filter rules by current OS
	filteredRules := filterRulesByOS(rules)
	currentOS := getOSName()

	// Check history and add auto-uninstall rules for removed packages
	autoUninstallRules := getAutoUninstallRules(filteredRules, file, currentOS)
	allRules := append(filteredRules, autoUninstallRules...)

	// Delete cloned repos and decrypted files that are no longer in the blueprint
	numClonedDeletions := deleteRemovedClones(filteredRules, file, currentOS)
	numDecryptedDeletions := deleteRemovedDecryptFiles(filteredRules, file, currentOS)

	// Count uninstall rules as cleanups
	numUninstalls := 0
	for _, rule := range autoUninstallRules {
		if rule.Action == "uninstall" {
			numUninstalls++
		}
	}
	numCleanups := numClonedDeletions + numDecryptedDeletions + numUninstalls

	// Extract base directory from setupPath for resolving relative file paths
	basePath := filepath.Dir(setupPath)

	if dry {
		ui.PrintExecutionHeader(false, currentOS, file, len(filteredRules), len(autoUninstallRules), numCleanups)
		displayRules(filteredRules)
		if len(autoUninstallRules) > 0 {
			ui.PrintAutoUninstallSection()
			displayRules(autoUninstallRules)
		}
		ui.PrintPlanFooter()
	} else {
		ui.PrintExecutionHeader(true, currentOS, file, len(filteredRules), len(autoUninstallRules), numCleanups)

		// Prompt for sudo password upfront (before decrypt passwords)
		// Check all rules including auto-uninstall rules
		err := promptForSudoPasswordWithOS(allRules, currentOS)
		if err != nil {
			fmt.Printf("%s\n", ui.FormatError(fmt.Sprintf("Error prompting for sudo password: %v", err)))
			return
		}

		// Prompt for all decrypt passwords upfront
		err = promptForDecryptPasswords(allRules)
		if err != nil {
			fmt.Printf("%s\n", ui.FormatError(fmt.Sprintf("Error prompting for passwords: %v", err)))
			return
		}

		records := executeRules(allRules, file, currentOS, basePath, runNumber)
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

		// Display rule-specific information using handler
		handler := handlerskg.NewHandler(rule, "", make(map[string]string))
		if handler != nil {
			handler.DisplayInfo()
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

		// Display command that will be executed - use handler's GetCommand method
		if handler != nil {
			cmd := handler.GetCommand()
			if cmd != "" {
				fmt.Printf("  Command: %s\n", ui.FormatDim(cmd))
			}
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

	switch rule.Action {
	case "install":
		// Use brew for macOS, apt for Linux
		if targetOS == "mac" {
			return fmt.Sprintf("brew install %s", pkgNames)
		}
		return fmt.Sprintf("apt-get install -y %s", pkgNames)
	case "uninstall":
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
	// Note: Handlers can implement the SudoAwareHandler interface to declare their own sudo requirements
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

	// Check if this is a shell command that contains sudo
	// (e.g., "sh -c 'sudo gpg ...'")
	if cmdName == "sh" || cmdName == "bash" {
		if strings.Contains(command, "sudo") {
			return true
		}
	}

	return false
}

// executeCommand parses and executes a command string
func executeCommand(cmdStr string) (string, error) {
	// Check if sudo is needed
	if needsSudo(cmdStr) {
		// Check if user has passwordless sudo
		if exec.Command("sudo", "-n", "true").Run() == nil {
			// User has passwordless sudo, use -n flag
			cmdStr = "sudo -n " + cmdStr
		} else if sudoPassword, ok := passwordCache["sudo"]; ok {
			// Use cached sudo password if available
			// Use echo to pipe password to sudo with -S flag
			// This avoids interactive password prompts during execution
			cmd := exec.Command("sh", "-c", fmt.Sprintf("echo %s | sudo -S %s", shellEscape(sudoPassword), cmdStr))
			output, err := cmd.CombinedOutput()
			return string(output), err
		} else {
			// Fallback to regular sudo if no password cached
			cmdStr = "sudo " + cmdStr
		}
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

// executeRulesWithHandlers executes rules using the handler pattern
// This is the refactored version that uses the handlers package
func executeRulesWithHandlers(rules []parser.Rule, blueprint string, osName string, basePath string, runNumber int) []ExecutionRecord {
	var records []ExecutionRecord

	// Set up the handler package with our executeCommand function
	handlerskg.SetExecuteCommandFunc(executeCommand)

	// Sort rules by dependencies
	sortedRules, err := resolveDependencies(rules)
	if err != nil {
		fmt.Printf("%s\n", ui.FormatError(err.Error()))
		return records
	}

	for i, rule := range sortedRules {
		isUninstall := rule.Action == "uninstall"
		fmt.Printf("[%d/%d] %s", i+1, len(sortedRules), ui.FormatHighlight(rule.Action))

		var handler handlerskg.Handler
		var output string
		var err error
		var actualCmd string

		// Create handler for this rule
		handler = handlerskg.NewHandler(rule, basePath, passwordCache)

		if handler != nil {
			// Get display details from handler if it implements DisplayProvider
			if displayProvider, ok := handler.(handlerskg.DisplayProvider); ok {
				details := displayProvider.GetDisplayDetails(isUninstall)
				if details != "" {
					// Use error color for uninstall, info color for regular actions
					if isUninstall {
						fmt.Printf(" %s", ui.FormatError(details))
					} else {
						fmt.Printf(" %s", ui.FormatInfo(details))
					}
				}
			}

			// Get the actual command from the handler
			actualCmd = handler.GetCommand()

			// Execute handler
			if isUninstall {
				output, err = handler.Down()
			} else {
				output, err = handler.Up()
			}
		} else {
			// Unknown action - this shouldn't happen if parsing is correct
			fmt.Printf(" %s", ui.FormatError("unknown action"))
			output = fmt.Sprintf("unknown action: %s", rule.Action)
			err = fmt.Errorf("unknown action type")
		}

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
			fmt.Printf("       %s\n", ui.FormatError(err.Error()))
			if logging.IsDebug() {
				fmt.Printf("       %s: %s\n", ui.FormatDim("Command"), ui.FormatInfo(actualCmd))
			}
			record.Status = "error"
			record.Error = err.Error()
		} else {
			fmt.Printf(" %s\n", ui.FormatSuccess("Done"))
			if logging.IsDebug() {
				fmt.Printf("       %s: %s\n", ui.FormatDim("Command"), ui.FormatInfo(actualCmd))
			}
			record.Status = "success"
		}

		records = append(records, record)

		// Save output to history (only if runNumber > 0)
		if runNumber > 0 {
			if err := saveRuleOutput(runNumber, i, record.Output, record.Error); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to save rule output to history: %v\n", err)
			}
		}
	}

	return records
}

func executeRules(rules []parser.Rule, blueprint string, osName string, basePath string, runNumber int) []ExecutionRecord {
	// Use the refactored version with handlers
	return executeRulesWithHandlers(rules, blueprint, osName, basePath, runNumber)
}

// getHistoryPath returns the path to the history file in ~/.blueprint/
func getHistoryPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	blueprintDir := filepath.Join(homeDir, ".blueprint")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(blueprintDir, 0750); err != nil {
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
	if err := os.MkdirAll(blueprintDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create .blueprint directory: %w", err)
	}

	return filepath.Join(blueprintDir, "status.json"), nil
}

// validateBlueprintPath validates that a file path is within the blueprint directory
// This prevents path traversal attacks
func validateBlueprintPath(filePath string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	blueprintDir := filepath.Join(homeDir, ".blueprint")
	blueprintDirAbs, err := filepath.Abs(blueprintDir)
	if err != nil {
		return fmt.Errorf("invalid blueprint directory: %w", err)
	}

	filePathAbs, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}

	// Ensure the file path is within the blueprint directory
	relPath, err := filepath.Rel(blueprintDirAbs, filePathAbs)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return fmt.Errorf("path traversal attempt detected: %s", filePath)
	}

	return nil
}

// readBlueprintFile safely reads a file from the blueprint directory after validation
func readBlueprintFile(filePath string) ([]byte, error) {
	if err := validateBlueprintPath(filePath); err != nil {
		return nil, err
	}
	return os.ReadFile(filePath)
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
	if data, err := readBlueprintFile(historyPath); err == nil {
		_ = json.Unmarshal(data, &allRecords)
	}

	// Append new records
	allRecords = append(allRecords, records...)

	// Write back to file
	data, err := json.MarshalIndent(allRecords, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal history: %w", err)
	}

	if err := os.WriteFile(historyPath, data, 0600); err != nil {
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

	// Normalize blueprint path for consistent storage and comparison
	blueprint = normalizePath(blueprint)

	// Load existing status
	var status Status
	if data, err := readBlueprintFile(statusPath); err == nil {
		_ = json.Unmarshal(data, &status)
	}

	// Convert engine ExecutionRecords to handler ExecutionRecords
	handlerRecords := make([]handlerskg.ExecutionRecord, len(records))
	for i, record := range records {
		handlerRecords[i] = handlerskg.ExecutionRecord{
			Timestamp: record.Timestamp,
			Blueprint: record.Blueprint,
			OS:        record.OS,
			Command:   record.Command,
			Output:    record.Output,
			Status:    record.Status,
			Error:     record.Error,
		}
	}

	// Process each rule by creating appropriate handler and calling UpdateStatus
	for _, rule := range rules {
		// Create handler for the rule (handles both install and uninstall)
		handler := handlerskg.NewHandler(rule, "", passwordCache)
		if handler == nil {
			// Skip unknown actions
			continue
		}

		// Let the handler update status
		if err := handler.UpdateStatus(&status, handlerRecords, blueprint, osName); err != nil {
			// Log but don't fail on status update errors
			fmt.Fprintf(os.Stderr, "Warning: failed to update status for rule %v: %v\n", rule.Action, err)
		}
	}

	// Write status to file
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal status: %w", err)
	}

	if err := os.WriteFile(statusPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write status file: %w", err)
	}

	return nil
}


// getAutoUninstallRules compares status with current rules and generates uninstall rules for removed resources
// Each handler's FindUninstallRules method encapsulates all status comparison logic
func getAutoUninstallRules(currentRules []parser.Rule, blueprintFile string, osName string) []parser.Rule {
	var autoUninstallRules []parser.Rule

	// Load status file to check for removed resources
	statusPath, err := getStatusPath()
	if err != nil {
		return autoUninstallRules
	}

	statusData, err := readBlueprintFile(statusPath)
	if err != nil {
		// No status file yet, nothing to uninstall
		return autoUninstallRules
	}

	var status handlerskg.Status
	if err := json.Unmarshal(statusData, &status); err != nil {
		// Invalid status file, can't process
		return autoUninstallRules
	}

	// Create handlers - each implements StatusProvider.FindUninstallRules
	// which completely owns the logic for comparing its resources against current rules
	handlers := []handlerskg.Handler{
		handlerskg.NewInstallHandler(parser.Rule{}, ""),
		handlerskg.NewCloneHandler(parser.Rule{}, ""),
		handlerskg.NewDecryptHandler(parser.Rule{}, "", nil),
		handlerskg.NewAsdfHandler(parser.Rule{}, ""),
		handlerskg.NewMkdirHandler(parser.Rule{}, ""),
		handlerskg.NewKnownHostsHandler(parser.Rule{}, ""),
		handlerskg.NewGPGKeyHandler(parser.Rule{}, ""),
	}

	// Let each handler determine its own uninstall rules by comparing
	// its status records against current rules
	for _, handler := range handlers {
		if statusProvider, ok := handler.(handlerskg.StatusProvider); ok {
			uninstallRules := statusProvider.FindUninstallRules(&status, currentRules, blueprintFile, osName)
			autoUninstallRules = append(autoUninstallRules, uninstallRules...)
		}
	}

	return autoUninstallRules
}

// deleteRemovedClones checks status for cloned repos that are no longer in the blueprint and deletes them
// Uses CloneHandler's FindUninstallRules to identify clones to delete, then removes them from filesystem
// Returns the count of deleted directories
func deleteRemovedClones(currentRules []parser.Rule, blueprintFile string, osName string) int {
	// Load status
	statusPath, err := getStatusPath()
	if err != nil {
		return 0
	}

	// Read status file
	data, err := readBlueprintFile(statusPath)
	if err != nil {
		// No status yet, nothing to delete
		return 0
	}

	// Parse status
	var status handlerskg.Status
	if err := json.Unmarshal(data, &status); err != nil {
		return 0
	}

	// Normalize the blueprint file path for comparison
	normalizedBlueprintFile := normalizePath(blueprintFile)

	// Use CloneHandler to find clones that should be removed
	cloneHandler := handlerskg.NewCloneHandler(parser.Rule{}, "")
	var statusProvider handlerskg.StatusProvider
	statusProvider, ok := handlerskg.Handler(cloneHandler).(handlerskg.StatusProvider)
	if !ok {
		return 0
	}

	// Get uninstall rules for removed clones - this encapsulates all the logic
	// for comparing clone status against current rules
	uninstallRules := statusProvider.FindUninstallRules(&status, currentRules, blueprintFile, osName)

	// Find the actual CloneStatus records from status.Clones that match the uninstall rules
	// so we can delete them from the filesystem
	var directoriesToDelete []string
	var clonesToKeep []handlerskg.CloneStatus

	if status.Clones != nil {
		for _, clone := range status.Clones {
			normalizedStatusBlueprint := normalizePath(clone.Blueprint)
			// Check if this clone is marked for deletion by comparing against uninstall rules
			shouldDelete := false
			for _, rule := range uninstallRules {
				if rule.ClonePath == clone.Path {
					shouldDelete = true
					break
				}
			}

			if shouldDelete && normalizedStatusBlueprint == normalizedBlueprintFile && clone.OS == osName {
				directoriesToDelete = append(directoriesToDelete, clone.Path)
			} else {
				clonesToKeep = append(clonesToKeep, clone)
			}
		}
	}

	// Delete the directories from filesystem and count deletions
	deletedCount := 0
	for _, path := range directoriesToDelete {
		// Expand home directory
		expandedPath := path
		if strings.HasPrefix(expandedPath, "~") {
			usr, err := user.Current()
			if err == nil {
				expandedPath = strings.Replace(expandedPath, "~", usr.HomeDir, 1)
			}
		}

		// Delete the directory recursively
		if err := os.RemoveAll(expandedPath); err == nil {
			deletedCount++
			fmt.Printf("%s Removed cloned directory: %s\n",
				ui.FormatSuccess("✓"),
				ui.FormatInfo(path))
		} else {
			// Log error for debugging
			fmt.Printf("%s Failed to remove cloned directory %s: %v\n",
				ui.FormatError("✗"),
				ui.FormatInfo(path),
				err)
		}
	}

	// Update status file with remaining clones
	status.Clones = clonesToKeep

	// Write updated status to file
	updatedData, err := json.MarshalIndent(status, "", "  ")
	if err == nil {
		_ = os.WriteFile(statusPath, updatedData, 0600)
	}

	return deletedCount
}

// deleteRemovedDecryptFiles checks status for decrypted files that are no longer in the blueprint and deletes them
// Uses DecryptHandler's FindUninstallRules to identify files to delete, then removes them from filesystem
// Returns the count of deleted files
func deleteRemovedDecryptFiles(currentRules []parser.Rule, blueprintFile string, osName string) int {
	// Load status
	statusPath, err := getStatusPath()
	if err != nil {
		return 0
	}

	// Read status file
	data, err := readBlueprintFile(statusPath)
	if err != nil {
		// No status yet, nothing to delete
		return 0
	}

	// Parse status
	var status Status
	if err := json.Unmarshal(data, &status); err != nil {
		return 0
	}

	// Normalize the blueprint file path for comparison
	normalizedBlueprintFile := normalizePath(blueprintFile)

	// Use DecryptHandler to find decrypted files that should be removed
	decryptHandler := handlerskg.NewDecryptHandler(parser.Rule{}, "", nil)
	var statusProvider handlerskg.StatusProvider
	statusProvider, ok := handlerskg.Handler(decryptHandler).(handlerskg.StatusProvider)
	if !ok {
		return 0
	}

	// Get uninstall rules for removed decrypts - this encapsulates all the logic
	// for comparing decrypt status against current rules
	uninstallRules := statusProvider.FindUninstallRules(&status, currentRules, blueprintFile, osName)

	// Find the actual DecryptStatus records from status.Decrypts that match the uninstall rules
	// so we can delete them from the filesystem
	var filesToDelete []string
	var decryptsToKeep []DecryptStatus

	// Handle nil decrypts slice
	if status.Decrypts == nil {
		return 0
	}

	for _, decrypt := range status.Decrypts {
		normalizedStatusBlueprint := normalizePath(decrypt.Blueprint)
		// Check if this decrypt is marked for deletion by comparing against uninstall rules
		shouldDelete := false
		for _, rule := range uninstallRules {
			if rule.DecryptPath == decrypt.DestPath {
				shouldDelete = true
				break
			}
		}

		if shouldDelete && normalizedStatusBlueprint == normalizedBlueprintFile && decrypt.OS == osName {
			filesToDelete = append(filesToDelete, decrypt.DestPath)
		} else {
			decryptsToKeep = append(decryptsToKeep, decrypt)
		}
	}

	// Delete the files from filesystem and count deletions
	deletedCount := 0
	for _, path := range filesToDelete {
		// Expand home directory
		expandedPath := path
		if strings.HasPrefix(expandedPath, "~") {
			usr, err := user.Current()
			if err == nil {
				expandedPath = strings.Replace(expandedPath, "~", usr.HomeDir, 1)
			}
		}

		// Delete the file
		if err := os.Remove(expandedPath); err == nil {
			deletedCount++
		}
		// Silently ignore errors (file might already be deleted)
	}

	// Update status file with remaining decrypts
	status.Decrypts = decryptsToKeep

	// Write updated status to file
	updatedData, err := json.MarshalIndent(status, "", "  ")
	if err == nil {
		_ = os.WriteFile(statusPath, updatedData, 0600)
	}

	return deletedCount
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
			// Use handler's KeyProvider interface to get the key
			handler := handlerskg.NewHandler(*rule, "", nil)
			if handler != nil {
				if keyProvider, ok := handler.(handlerskg.KeyProvider); ok {
					ruleKey = keyProvider.GetDependencyKey()
				} else {
					// Fallback: use action as key if no KeyProvider implemented
					ruleKey = rule.Action
				}
			} else {
				// Fallback: use action as key if no handler found
				ruleKey = rule.Action
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

// EncryptFile encrypts a file and saves it with .enc extension
func EncryptFile(filePath string, passwordID string) {
	// Check if file exists
	if _, err := os.Stat(filePath); err != nil {
		fmt.Printf("%s\n", ui.FormatError(fmt.Sprintf("File not found: %s", filePath)))
		os.Exit(1)
	}

	// Read file content
	plaintext, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("%s\n", ui.FormatError(fmt.Sprintf("Failed to read file: %v", err)))
		os.Exit(1)
	}

	// Prompt for password
	fmt.Printf("Enter password for %s: ", filePath)
	password, err := readPassword()
	if err != nil {
		fmt.Printf("%s\n", ui.FormatError(fmt.Sprintf("Failed to read password: %v", err)))
		os.Exit(1)
	}

	// Encrypt file
	encryptedData, err := cryptopkg.EncryptFile(plaintext, password)
	if err != nil {
		fmt.Printf("%s\n", ui.FormatError(fmt.Sprintf("Encryption failed: %v", err)))
		os.Exit(1)
	}

	// Write encrypted file with .enc extension
	encryptedPath := filePath + ".enc"
	if err := os.WriteFile(encryptedPath, encryptedData, 0600); err != nil {
		fmt.Printf("%s\n", ui.FormatError(fmt.Sprintf("Failed to write encrypted file: %v", err)))
		os.Exit(1)
	}

	fmt.Printf("%s\n", ui.FormatSuccess(fmt.Sprintf("File encrypted: %s -> %s", filePath, encryptedPath)))
}

// promptForDecryptPasswords collects all unique password-ids from decrypt rules and prompts for passwords upfront
func promptForDecryptPasswords(rules []parser.Rule) error {
	// Collect unique password-ids from decrypt rules
	passwordIDsMap := make(map[string]bool)
	var passwordIDs []string

	for _, rule := range rules {
		if rule.Action == "decrypt" {
			passwordID := rule.DecryptPasswordID
			if passwordID == "" {
				passwordID = "default"
			}

			// Only add if we haven't seen this password-id before
			if !passwordIDsMap[passwordID] {
				passwordIDsMap[passwordID] = true
				passwordIDs = append(passwordIDs, passwordID)
			}
		}
	}

	// If there are no decrypt rules, return early
	if len(passwordIDs) == 0 {
		return nil
	}

	// Prompt for each unique password-id
	for _, passwordID := range passwordIDs {
		fmt.Printf("Enter password for %s: ", ui.FormatHighlight(passwordID))
		password, err := readPassword()
		if err != nil {
			return fmt.Errorf("failed to read password for %s: %w", passwordID, err)
		}
		// Cache the password
		passwordCache[passwordID] = password
	}

	return nil
}

// readPassword reads a password from stdin without echoing using x/term
func readPassword() (string, error) {
	// Read password from stdin with terminal echo disabled
	bytePassword, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}
	fmt.Println() // Print newline after password prompt
	return string(bytePassword), nil
}

// shellEscape escapes a string for safe use in shell commands
func shellEscape(s string) string {
	// Use single quotes to prevent shell interpretation
	// Replace single quotes with '\'' (end quote, escaped quote, start quote)
	escaped := strings.ReplaceAll(s, "'", "'\\''")
	return fmt.Sprintf("'%s'", escaped)
}

// promptForSudoPassword checks if any rules need sudo and prompts for password upfront
func promptForSudoPasswordWithOS(rules []parser.Rule, currentOS string) error {
	// Check if we're on Linux and not root
	if runtime.GOOS != "linux" {
		return nil
	}

	currentUser, err := user.Current()
	if err == nil {
		uid, err := strconv.Atoi(currentUser.Uid)
		if err == nil && uid == 0 {
			// Already root, no sudo needed
			return nil
		}
	}

	// Check if user has passwordless sudo (sudo -n true)
	// If this succeeds, user can run sudo without password
	if cmd := exec.Command("sudo", "-n", "true"); cmd.Run() == nil {
		// User has passwordless sudo, no need to prompt
		return nil
	}

	// Check if any rule needs sudo by building the actual command
	// Note: rules passed in are already filtered by OS, so we don't need to check ruleAppliesToOS()
	needsSudoPassword := false
	for _, rule := range rules {
		// First check if the handler implements SudoAwareHandler
		handler := handlerskg.NewHandler(rule, "", make(map[string]string))
		if sudoAwareHandler, ok := handler.(handlerskg.SudoAwareHandler); ok {
			if sudoAwareHandler.NeedsSudo() {
				needsSudoPassword = true
				break
			}
		} else {
			// Fall back to checking the command string
			// Check install/uninstall rules
			if rule.Action == "install" || rule.Action == "uninstall" {
				cmd := buildCommand(rule)
				if needsSudo(cmd) {
					needsSudoPassword = true
					break
				}
			}
		}
	}

	// If sudo is needed, prompt for password upfront
	if needsSudoPassword {
		fmt.Printf("Enter sudo password: ")
		password, err := readPassword()
		if err != nil {
			return fmt.Errorf("failed to read sudo password: %w", err)
		}
		// Cache the sudo password
		passwordCache["sudo"] = password
	}

	return nil
}

// PrintStatus displays the current status of installed packages and clones
func PrintStatus() {
	statusPath, err := getStatusPath()
	if err != nil {
		fmt.Printf("%s\n", ui.FormatError("Error getting status path"))
		return
	}

	// Read status file
	data, err := readBlueprintFile(statusPath)
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

	// Use handlers to display their respective status
	installHandler := &handlerskg.InstallHandler{}
	installHandler.DisplayStatus(status.Packages)

	cloneHandler := &handlerskg.CloneHandler{}
	cloneHandler.DisplayStatus(status.Clones)

	decryptHandler := &handlerskg.DecryptHandler{}
	decryptHandler.DisplayStatus(status.Decrypts)

	mkdirHandler := &handlerskg.MkdirHandler{}
	mkdirHandler.DisplayStatus(status.Mkdirs)

	knownHostsHandler := &handlerskg.KnownHostsHandler{}
	knownHostsHandler.DisplayStatus(status.KnownHosts)

	gpgKeyHandler := &handlerskg.GPGKeyHandler{}
	gpgKeyHandler.DisplayStatus(status.GPGKeys)

	if len(status.Packages) == 0 && len(status.Clones) == 0 && len(status.Decrypts) == 0 && len(status.Mkdirs) == 0 && len(status.KnownHosts) == 0 && len(status.GPGKeys) == 0 {
		fmt.Printf("\n%s\n", ui.FormatInfo("No packages, repositories, decrypted files, directories, known hosts, or GPG keys created"))
	}

	fmt.Printf("\n")
}

// getBlueprintDir returns the path to the .blueprint directory
func getBlueprintDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	blueprintDir := filepath.Join(homeDir, ".blueprint")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(blueprintDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create .blueprint directory: %w", err)
	}

	return blueprintDir, nil
}

// getNextRunNumber returns the next run number and increments the counter
func getNextRunNumber() (int, error) {
	blueprintDir, err := getBlueprintDir()
	if err != nil {
		return 0, err
	}

	runNumberFile := filepath.Join(blueprintDir, "run_number")

	// Read current run number
	var runNumber int
	if data, err := readBlueprintFile(runNumberFile); err == nil {
		_, _ = fmt.Sscanf(string(data), "%d", &runNumber)
	}

	// Increment for next run
	runNumber++

	// Write back
	if err := os.WriteFile(runNumberFile, []byte(fmt.Sprintf("%d", runNumber)), 0600); err != nil {
		return 0, err
	}

	return runNumber, nil
}

// saveRuleOutput saves the output of a rule execution to history
func saveRuleOutput(runNumber, ruleIndex int, output, stderr string) error {
	blueprintDir, err := getBlueprintDir()
	if err != nil {
		return err
	}

	historyDir := filepath.Join(blueprintDir, "history", fmt.Sprintf("%d", runNumber))
	if err := os.MkdirAll(historyDir, 0750); err != nil {
		return err
	}

	outputFile := filepath.Join(historyDir, fmt.Sprintf("%d.output", ruleIndex))
	content := fmt.Sprintf("=== STDOUT ===\n%s\n\n=== STDERR ===\n%s\n", output, stderr)

	return os.WriteFile(outputFile, []byte(content), 0600)
}

// getLatestRunNumber returns the latest run number from the history directory
func getLatestRunNumber() (int, error) {
	blueprintDir, err := getBlueprintDir()
	if err != nil {
		return 0, err
	}

	historyBaseDir := filepath.Join(blueprintDir, "history")
	entries, err := os.ReadDir(historyBaseDir)
	if err != nil {
		return 0, fmt.Errorf("no history found")
	}

	var latestRun int
	for _, entry := range entries {
		if entry.IsDir() {
			runNum := 0
			_, _ = fmt.Sscanf(entry.Name(), "%d", &runNum)
			if runNum > latestRun {
				latestRun = runNum
			}
		}
	}

	if latestRun == 0 {
		return 0, fmt.Errorf("no history found")
	}

	return latestRun, nil
}

// PrintHistory displays the history of a specific run
// If runNumber is 0, displays the latest run
// If stepNumber is >= 0, displays only that specific step
func PrintHistory(runNumber int, stepNumber int) {
	blueprintDir, err := getBlueprintDir()
	if err != nil {
		fmt.Printf("%s\n", ui.FormatError(fmt.Sprintf("Failed to get blueprint directory: %v", err)))
		return
	}

	// If runNumber is 0, get the latest run
	if runNumber == 0 {
		var err error
		runNumber, err = getLatestRunNumber()
		if err != nil {
			fmt.Printf("%s\n", ui.FormatError("No history found"))
			return
		}
	}

	historyDir := filepath.Join(blueprintDir, "history", fmt.Sprintf("%d", runNumber))

	// Check if history directory exists
	if _, err := os.Stat(historyDir); os.IsNotExist(err) {
		fmt.Printf("%s\n", ui.FormatError(fmt.Sprintf("No history found for run %d", runNumber)))
		return
	}

	fmt.Printf("\n%s\n", ui.FormatHighlight(fmt.Sprintf("=== RUN %d HISTORY ===", runNumber)))

	// List all output files
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		fmt.Printf("%s\n", ui.FormatError(fmt.Sprintf("Failed to read history: %v", err)))
		return
	}

	if len(entries) == 0 {
		fmt.Printf("%s\n", ui.FormatInfo("No rule outputs recorded for this run"))
		return
	}

	// Sort entries by filename (rule number)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".output" {
			ruleNum := strings.TrimSuffix(entry.Name(), ".output")

			// If stepNumber is specified, only show that step
			if stepNumber >= 0 {
				ruleNumInt := 0
				_, _ = fmt.Sscanf(ruleNum, "%d", &ruleNumInt)
				if ruleNumInt != stepNumber {
					continue
				}
			}

			outputPath := filepath.Join(historyDir, entry.Name())

			content, err := readBlueprintFile(outputPath)
			if err != nil {
				continue
			}

			fmt.Printf("\n%s\n", ui.FormatHighlight(fmt.Sprintf("Rule #%s:", ruleNum)))

			// Parse stdout and stderr sections
			contentStr := string(content)
			parts := strings.Split(contentStr, "\n=== STDERR ===\n")

			stdout := ""
			stderr := ""

			if len(parts) >= 1 {
				stdout = strings.TrimPrefix(parts[0], "=== STDOUT ===\n")
				stdout = strings.TrimSpace(stdout)
			}

			if len(parts) >= 2 {
				stderr = strings.TrimSpace(parts[1])
			}

			// Show stdout if not empty (with separator line instead of header)
			if stdout != "" {
				fmt.Printf("%s\n%s\n", "───────────────", stdout)
			}

			// Show stderr if not empty (in red)
			if stderr != "" {
				// Color each line of stderr red
				stderrLines := strings.Split(stderr, "\n")
				for _, line := range stderrLines {
					fmt.Printf("%s\n", ui.FormatError(line))
				}
			}

			// Show message if both are empty
			if stdout == "" && stderr == "" {
				fmt.Printf("%s\n", ui.FormatInfo("(no output)"))
			}
		}
	}

	fmt.Printf("\n")
}
