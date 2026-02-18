package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/elpic/blueprint/internal"
	"github.com/elpic/blueprint/internal/ui"
)

// ProcessState represents the current execution state written to ~/.blueprint/ps.json
type ProcessState struct {
	PID           int    `json:"pid"`
	BlueprintFile string `json:"blueprint_file"`
	OS            string `json:"os"`
	TotalRules    int    `json:"total_rules"`
	CurrentRule   int    `json:"current_rule"`
	CurrentAction string `json:"current_action"`
	CurrentDetail string `json:"current_detail"`
	StartedAt     string `json:"started_at"`
	RuleStartedAt string `json:"rule_started_at"`
}

func getPSPath() (string, error) {
	blueprintDir, err := getBlueprintDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(blueprintDir, "ps.json"), nil
}

func writePSState(state ProcessState) error {
	psPath, err := getPSPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal process state: %w", err)
	}

	// Write atomically: write to temp file then rename
	tmpPath := psPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, internal.FilePermission); err != nil {
		return fmt.Errorf("failed to write process state: %w", err)
	}

	return os.Rename(tmpPath, psPath)
}

func readPSState() (*ProcessState, error) {
	psPath, err := getPSPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(psPath)
	if err != nil {
		return nil, err
	}

	var state ProcessState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse process state: %w", err)
	}

	return &state, nil
}

func clearPSState() {
	psPath, err := getPSPath()
	if err != nil {
		return
	}
	_ = os.Remove(psPath)
}

// isProcessAlive checks if a process with the given PID is still running
func isProcessAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 checks if the process exists without actually sending a signal
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// formatDuration formats a duration as a human-readable string
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		m := int(d.Minutes())
		s := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm %ds", m, s)
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%dh %dm %ds", h, m, s)
}

// PrintPS displays the current blueprint process state
func PrintPS() {
	state, err := readPSState()
	if err != nil {
		fmt.Println("No blueprint process running.")
		return
	}

	// Check if the process is still alive
	if !isProcessAlive(state.PID) {
		// Stale state file — clean it up
		clearPSState()
		fmt.Println("No blueprint process running.")
		return
	}

	// Parse timestamps
	startedAt, err := time.Parse(time.RFC3339, state.StartedAt)
	if err != nil {
		fmt.Println("No blueprint process running.")
		return
	}

	elapsed := time.Since(startedAt)

	fmt.Printf("\n%s\n\n", ui.FormatHighlight("=== Blueprint Process ==="))
	fmt.Printf("Blueprint: %s\n", ui.FormatInfo(state.BlueprintFile))
	fmt.Printf("OS:        %s\n", state.OS)
	fmt.Printf("PID:       %d\n", state.PID)
	fmt.Printf("Running:   %s\n", formatDuration(elapsed))

	// Show current rule progress
	if state.CurrentRule > 0 {
		detail := state.CurrentAction
		if state.CurrentDetail != "" {
			detail += " " + state.CurrentDetail
		}

		ruleStartedAt, err := time.Parse(time.RFC3339, state.RuleStartedAt)
		ruleElapsed := ""
		if err == nil {
			ruleElapsed = fmt.Sprintf(" (running for %s)", formatDuration(time.Since(ruleStartedAt)))
		}

		fmt.Printf("\n[%d/%d] %s%s\n", state.CurrentRule, state.TotalRules, detail, ruleElapsed)
	}

	fmt.Println()
}
