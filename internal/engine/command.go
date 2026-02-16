package engine

import (
	"fmt"
	handlerskg "github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/logging"
	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"time"
)

func needsSudo(command string) bool {
	// Only on Linux
	if getOSName() != "linux" {
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

	cmdName := strings.Fields(command)[0]

	// Check if command starts with sudo directly
	if cmdName == "sudo" {
		return true
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

func executeCommand(cmdStr string) (string, error) {
	// Check if the command needs shell processing (contains pipes, redirects, tilde expansion, etc.)
	needsShell := strings.ContainsAny(cmdStr, "|><&;$()~`")

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

	// If command needs shell processing or starts with sh -c, use shell
	if needsShell || strings.HasPrefix(strings.TrimSpace(cmdStr), "sh -c") {
		cmd := exec.Command("sh", "-c", cmdStr)
		// Explicitly set Stdin to nil to prevent blocking on input
		cmd.Stdin = nil
		output, err := cmd.CombinedOutput()
		return string(output), err
	}

	// Parse command string into parts for direct execution
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	// Create command
	cmd := exec.Command(parts[0], parts[1:]...)
	// Explicitly set Stdin to nil to prevent blocking on input
	cmd.Stdin = nil

	// Capture output
	output, err := cmd.CombinedOutput()

	return string(output), err
}

// executeRules executes rules using the handler pattern

func executeRules(rules []parser.Rule, blueprint string, osName string, basePath string, runNumber int) []ExecutionRecord {
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

func shellEscape(s string) string {
	// Use single quotes to prevent shell interpretation
	// Replace single quotes with '\'' (end quote, escaped quote, start quote)
	escaped := strings.ReplaceAll(s, "'", "'\\''")
	return fmt.Sprintf("'%s'", escaped)
}

// clearSudoCache clears the sudo password cache on all operating systems
// On Linux: runs 'sudo -K' to invalidate the sudo timestamp
// On macOS: runs 'sudo -K' to invalidate the sudo timestamp
func clearSudoCache() {
	// Run sudo -K to clear the cached sudo session on all operating systems
	cmd := exec.Command("sudo", "-K")
	// Ignore errors - this is a cleanup operation
	_ = cmd.Run()
}

// promptForSudoPassword checks if any rules need sudo and prompts for password upfront
