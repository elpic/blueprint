package engine

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"time"

	handlerskg "github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/logging"
	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
)

// RealCommandExecutor implements platform.CommandExecutor for production use
type RealCommandExecutor struct{}

// Execute runs a real command on the system
func (r *RealCommandExecutor) Execute(cmd string) (string, error) {
	return executeCommand(cmd)
}

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

// sudoRunWithPassword runs cmdStr under sudo by feeding password via stdin.
// The password never appears in the process argument list.
var sudoRunWithPassword = func(password, cmdStr string) (string, error) {
	cmd := exec.Command("sh", "-c", "sudo -S "+cmdStr) // #nosec G204 -- user-supplied command from blueprint
	cmd.Stdin = strings.NewReader(password + "\n")
	output, err := cmd.CombinedOutput()
	return string(output), err
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
		} else if sudoPassword, ok := passwordCache.get("sudo"); ok {
			// Use cached sudo password if available
			return sudoRunWithPassword(sudoPassword, cmdStr)
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

// ruleResult holds the output of a single rule execution, keyed by its
// position in the global sorted order so results can be reassembled later.
type ruleResult struct {
	globalIndex int             // 0-based position in the flattened sorted rules
	record      ExecutionRecord // the execution record
	output      string          // buffered terminal output (printed atomically)
}

// executeOneRule runs a single rule and returns the result without printing.
// All output is captured into ruleResult.output for atomic flushing.
func executeOneRule(
	rule parser.Rule,
	globalIndex int,
	totalRules int,
	blueprint string,
	osName string,
	basePath string,
	currentStatus *handlerskg.Status,
	priorRecords []ExecutionRecord,
) ruleResult {
	isUninstall := rule.Action == "uninstall"

	var buf strings.Builder
	fmt.Fprintf(&buf, "[%d/%d] %s", globalIndex+1, totalRules, ui.FormatHighlight(rule.Action))

	var handler handlerskg.Handler
	var output string
	var execErr error
	var actualCmd string
	var durationMs int64

	handler = handlerskg.NewHandler(rule, basePath, passwordCache.snapshot())

	if handler != nil {
		if displayProvider, ok := handler.(handlerskg.DisplayProvider); ok {
			details := displayProvider.GetDisplayDetails(isUninstall)
			if details != "" {
				if isUninstall {
					fmt.Fprintf(&buf, " %s", ui.FormatError(details))
				} else {
					fmt.Fprintf(&buf, " %s", ui.FormatInfo(details))
				}
			}
		}

		actualCmd = handler.GetCommand()

		// Give record-aware handlers access to records from prior waves
		if ra, ok := handler.(handlerskg.RecordAware); ok {
			ra.SetCurrentRecords(toHandlerRecords(priorRecords))
		}

		start := time.Now()
		if isUninstall {
			if !handler.IsInstalled(currentStatus, blueprint, osName) {
				output = "not installed"
			} else {
				output, execErr = handler.Down()
			}
		} else {
			alwaysRun := false
			if def := handlerskg.GetAction(rule.Action); def != nil {
				alwaysRun = def.AlwaysRunUp
			}
			if !alwaysRun && handler.IsInstalled(currentStatus, blueprint, osName) {
				output = "already installed"
			} else {
				output, execErr = handler.Up()
			}
		}
		durationMs = time.Since(start).Milliseconds()
	} else {
		fmt.Fprintf(&buf, " %s", ui.FormatError("unknown action"))
		output = fmt.Sprintf("unknown action: %s", rule.Action)
		execErr = fmt.Errorf("unknown action type")
	}

	record := ExecutionRecord{
		Timestamp:  time.Now().Format(time.RFC3339),
		Blueprint:  blueprint,
		OS:         osName,
		Command:    actualCmd,
		DurationMs: durationMs,
		Output:     strings.TrimSpace(output),
	}

	if execErr != nil {
		fmt.Fprintf(&buf, " %s\n", ui.FormatError("Failed"))
		fmt.Fprintf(&buf, "       %s\n", ui.FormatError(execErr.Error()))
		if logging.IsDebug() {
			fmt.Fprintf(&buf, "       %s: %s\n", ui.FormatDim("Command"), ui.FormatInfo(actualCmd))
		}
		record.Status = "error"
		record.Error = execErr.Error()
	} else {
		fmt.Fprintf(&buf, " %s\n", ui.FormatSuccess("Done"))
		if logging.IsDebug() {
			fmt.Fprintf(&buf, "       %s: %s\n", ui.FormatDim("Command"), ui.FormatInfo(actualCmd))
		}
		record.Status = "success"
	}

	return ruleResult{
		globalIndex: globalIndex,
		record:      record,
		output:      buf.String(),
	}
}

// executeRules executes rules using the handler pattern.
// Independent rules (those without mutual after: dependencies) run in parallel
// within the same "wave". Waves are executed sequentially.
func executeRules(rules []parser.Rule, blueprint string, osName string, basePath string, runNumber int) []ExecutionRecord {
	// Set up the handler package with our command executor
	handlerskg.SetCommandExecutor(&RealCommandExecutor{})

	// Load current status once — used for idempotency checks before Up()/Down()
	currentStatus := loadCurrentStatus()

	// Sort rules by dependencies
	sortedRules, err := resolveDependencies(rules)
	if err != nil {
		fmt.Printf("%s\n", ui.FormatError(err.Error()))
		return nil
	}

	totalRules := len(sortedRules)

	// Group sorted rules into waves for parallel execution.
	waves := groupIntoWaves(sortedRules)

	// Write initial process state and ensure cleanup
	psState := ProcessState{
		PID:           os.Getpid(),
		BlueprintFile: blueprint,
		OS:            osName,
		TotalRules:    totalRules,
		StartedAt:     time.Now().Format(time.RFC3339),
	}
	_ = writePSState(psState)
	defer clearPSState()

	// records accumulates results across all waves, in global sorted order.
	records := make([]ExecutionRecord, totalRules)
	globalIdx := 0 // tracks position in the flattened sorted order

	for _, wave := range waves {
		if len(wave) == 1 {
			// Single rule — run directly, no goroutine overhead.
			rule := wave[0]
			idx := globalIdx
			globalIdx++

			// Update process state
			psState.CurrentRule = idx + 1
			psState.CurrentAction = rule.Action
			psState.HandlerState = nil
			handler := handlerskg.NewHandler(rule, basePath, passwordCache.snapshot())
			if handler != nil {
				if sp, ok := handler.(handlerskg.StateProvider); ok {
					psState.HandlerState = sp.GetState(rule.Action == "uninstall")
				}
			}
			psState.RuleStartedAt = time.Now().Format(time.RFC3339)
			_ = writePSState(psState)

			res := executeOneRule(rule, idx, totalRules, blueprint, osName, basePath, &currentStatus, records[:idx])
			fmt.Print(res.output)
			records[idx] = res.record

			if runNumber > 0 {
				if err := saveRuleOutput(runNumber, idx+1, res.record.Output, res.record.Error); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to save rule output to history: %v\n", err)
				}
			}
			continue
		}

		// Multiple rules in this wave — run in parallel.
		// Snapshot records from prior waves for RecordAware handlers.
		priorRecords := make([]ExecutionRecord, globalIdx)
		copy(priorRecords, records[:globalIdx])

		results := make([]ruleResult, len(wave))
		var wg sync.WaitGroup

		for wi, rule := range wave {
			wg.Add(1)
			go func(wi int, rule parser.Rule, idx int) {
				defer wg.Done()
				results[wi] = executeOneRule(rule, idx, totalRules, blueprint, osName, basePath, &currentStatus, priorRecords)
			}(wi, rule, globalIdx+wi)
		}
		wg.Wait()

		// Flush output and collect records in deterministic order.
		for wi, res := range results {
			idx := globalIdx + wi
			fmt.Print(res.output)
			records[idx] = res.record

			if runNumber > 0 {
				if err := saveRuleOutput(runNumber, idx+1, res.record.Output, res.record.Error); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to save rule output to history: %v\n", err)
				}
			}
		}

		globalIdx += len(wave)
	}

	return records
}

func toHandlerRecords(records []ExecutionRecord) []handlerskg.ExecutionRecord {
	out := make([]handlerskg.ExecutionRecord, len(records))
	for i, r := range records {
		out[i] = handlerskg.ExecutionRecord{
			Timestamp: r.Timestamp,
			Blueprint: r.Blueprint,
			OS:        r.OS,
			Command:   r.Command,
			Output:    r.Output,
			Status:    r.Status,
			Error:     r.Error,
		}
	}
	return out
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
