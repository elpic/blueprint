package engine

import (
	"encoding/json"
	"fmt"
	handlerskg "github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

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

	// Get all status provider handlers from the factory (single place where handlers are instantiated)
	handlers := handlerskg.GetStatusProviderHandlers()

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


// normalizePath normalizes a file path to allow comparison of relative and absolute paths
// It converts to absolute path and normalizes separators

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