package engine

import (
	"fmt"
	gitpkg "github.com/elpic/blueprint/internal/git"
	handlerskg "github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
	"os"
	"path/filepath"
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
func executeRules(rules []parser.Rule, blueprint string, osName string, basePath string, runNumber int) []ExecutionRecord {
	// Use the refactored version with handlers
	return executeRulesWithHandlers(rules, blueprint, osName, basePath, runNumber)
}

