package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
)

// RunHandler handles executing arbitrary shell commands
type RunHandler struct {
	BaseHandler
}

// NewRunHandler creates a new run handler
func NewRunHandler(rule parser.Rule, basePath string) *RunHandler {
	return &RunHandler{
		BaseHandler: BaseHandler{
			Rule:     rule,
			BasePath: basePath,
		},
	}
}

// Up executes the shell command, optionally skipping if the unless check passes
func (h *RunHandler) Up() (string, error) {
	if h.Rule.RunUnless != "" {
		cmd := exec.Command("sh", "-c", h.Rule.RunUnless) // #nosec G204 -- user-supplied unless check from blueprint
		if err := cmd.Run(); err == nil {
			return fmt.Sprintf("skipped (unless check passed): %s", h.Rule.RunUnless), nil
		}
	}

	runCmd := h.Rule.RunCommand
	if h.Rule.RunSudo {
		runCmd = "sudo " + runCmd
	}

	cmd := exec.Command("sh", "-c", runCmd) // #nosec G204 -- user-supplied command from blueprint
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command failed: %w\n%s", err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

// Down executes the undo command if set
func (h *RunHandler) Down() (string, error) {
	if h.Rule.RunUndo == "" {
		return "no undo command, skipping", nil
	}

	undoCmd := h.Rule.RunUndo
	if h.Rule.RunSudo {
		undoCmd = "sudo " + undoCmd
	}

	cmd := exec.Command("sh", "-c", undoCmd) // #nosec G204 -- user-supplied undo command from blueprint
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("undo command failed: %w\n%s", err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

// GetCommand returns the effective shell command string (used as the execution key)
func (h *RunHandler) GetCommand() string {
	if h.Rule.Action == "uninstall" {
		if h.Rule.RunUndo == "" {
			return "# no undo"
		}
		if h.Rule.RunSudo {
			return "sudo " + h.Rule.RunUndo
		}
		return h.Rule.RunUndo
	}
	if h.Rule.RunSudo {
		return "sudo " + h.Rule.RunCommand
	}
	return h.Rule.RunCommand
}

// UpdateStatus updates the blueprint status after executing run or uninstall-run
func (h *RunHandler) UpdateStatus(status *Status, records []ExecutionRecord, blueprint string, osName string) error {
	blueprint = normalizePath(blueprint)

	if h.Rule.Action == "run" {
		cmd := h.GetCommand()
		_, executed := commandSuccessfullyExecuted(cmd, records)

		// Also treat a "skipped" result as success (idempotent re-run)
		if !executed {
			skipMsg := fmt.Sprintf("skipped (unless check passed): %s", h.Rule.RunUnless)
			for _, record := range records {
				if record.Status == "success" && strings.Contains(record.Output, skipMsg) {
					executed = true
					break
				}
			}
		}

		if executed {
			status.Runs = removeRunStatus(status.Runs, h.Rule.RunCommand, blueprint, osName)
			status.Runs = append(status.Runs, RunStatus{
				Action:    "run",
				Command:   h.Rule.RunCommand,
				UndoCmd:   h.Rule.RunUndo,
				Sudo:      h.Rule.RunSudo,
				RanAt:     time.Now().Format(time.RFC3339),
				Blueprint: blueprint,
				OS:        osName,
			})
		}
	} else if h.Rule.Action == "uninstall" && DetectRuleType(h.Rule) == "run" {
		status.Runs = removeRunStatus(status.Runs, h.Rule.RunCommand, blueprint, osName)
	}

	return nil
}

// DisplayInfo displays handler-specific information
func (h *RunHandler) DisplayInfo() {
	formatFunc := ui.FormatInfo
	if h.Rule.Action == "uninstall" {
		formatFunc = ui.FormatDim
	}

	fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Command: %s", h.Rule.RunCommand)))
	if h.Rule.RunSudo {
		fmt.Printf("  %s\n", formatFunc("sudo: true"))
	}
	if h.Rule.RunUnless != "" {
		fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Unless: %s", h.Rule.RunUnless)))
	}
	if h.Rule.RunUndo != "" {
		fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Undo: %s", h.Rule.RunUndo)))
	}
}

// GetDependencyKey returns the unique key for this rule in dependency resolution
func (h *RunHandler) GetDependencyKey() string {
	return getDependencyKey(h.Rule, h.Rule.RunCommand)
}

// GetDisplayDetails returns a truncated command for display during execution
func (h *RunHandler) GetDisplayDetails(isUninstall bool) string {
	cmd := h.Rule.RunCommand
	if isUninstall {
		cmd = h.Rule.RunUndo
	}
	if len(cmd) > 60 {
		return cmd[:60] + "..."
	}
	return cmd
}

// GetState returns handler-specific state as key-value pairs
func (h *RunHandler) GetState(isUninstall bool) map[string]string {
	cmd := h.Rule.RunCommand
	summary := cmd
	if len(summary) > 60 {
		summary = summary[:60] + "..."
	}
	return map[string]string{
		"summary": summary,
		"command": cmd,
	}
}

// FindUninstallRules compares run status against current rules and returns uninstall rules
func (h *RunHandler) FindUninstallRules(status *Status, currentRules []parser.Rule, blueprintFile, osName string) []parser.Rule {
	normalizedBlueprint := normalizePath(blueprintFile)

	currentRunCmds := make(map[string]bool)
	for _, rule := range currentRules {
		if rule.Action == "run" && rule.RunCommand != "" {
			currentRunCmds[rule.RunCommand] = true
		}
	}

	var rules []parser.Rule
	if status.Runs != nil {
		for _, r := range status.Runs {
			if r.Action != "run" {
				continue
			}
			normalizedStatusBlueprint := normalizePath(r.Blueprint)
			if normalizedStatusBlueprint == normalizedBlueprint && r.OS == osName && !currentRunCmds[r.Command] {
				// Only emit uninstall if there's an undo command
				if r.UndoCmd != "" {
					rules = append(rules, parser.Rule{
						Action:     "uninstall",
						RunCommand: r.Command,
						RunUndo:    r.UndoCmd,
						RunSudo:    r.Sudo,
						OSList:     []string{osName},
					})
				}
			}
		}
	}

	return rules
}

// DisplayStatusFromStatus displays run status from Status object
func (h *RunHandler) DisplayStatusFromStatus(status *Status) {
	if status == nil || len(status.Runs) == 0 {
		return
	}

	fmt.Printf("\n%s\n", ui.FormatHighlight("Run Commands:"))
	for _, r := range status.Runs {
		t, err := time.Parse(time.RFC3339, r.RanAt)
		var timeStr string
		if err == nil {
			timeStr = t.Format("2006-01-02 15:04:05")
		} else {
			timeStr = r.RanAt
		}

		cmd := r.Command
		if len(cmd) > 60 {
			cmd = cmd[:60] + "..."
		}

		fmt.Printf("  %s %s (%s) [%s, %s]\n",
			ui.FormatSuccess("●"),
			ui.FormatInfo(cmd),
			ui.FormatDim(timeStr),
			ui.FormatDim(r.OS),
			ui.FormatDim(abbreviateBlueprintPath(r.Blueprint)),
		)
	}
}

// RunShHandler handles downloading and executing shell scripts from URLs
type RunShHandler struct {
	BaseHandler
}

// NewRunShHandler creates a new run-sh handler
func NewRunShHandler(rule parser.Rule, basePath string) *RunShHandler {
	return &RunShHandler{
		BaseHandler: BaseHandler{
			Rule:     rule,
			BasePath: basePath,
		},
	}
}

// Up downloads the script and executes it, optionally skipping if the unless check passes
// httpClient returns the HTTP client used for downloading scripts.
func (h *RunShHandler) httpClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

func (h *RunShHandler) Up() (string, error) {
	if h.Rule.RunUnless != "" {
		cmd := exec.Command("sh", "-c", h.Rule.RunUnless) // #nosec G204 -- user-supplied unless check from blueprint
		if err := cmd.Run(); err == nil {
			return fmt.Sprintf("skipped (unless check passed): %s", h.Rule.RunUnless), nil
		}
	}

	// Download script to a temp file
	tmpFile, err := os.CreateTemp("", "blueprint-run-sh-*.sh")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	resp, err := h.httpClient().Get(h.Rule.RunShURL) // #nosec G107 -- URL is user-supplied via blueprint file
	if err != nil {
		_ = tmpFile.Close()
		return "", fmt.Errorf("failed to download script %s: %w", h.Rule.RunShURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		_ = tmpFile.Close()
		return "", fmt.Errorf("download failed with status %d: %s", resp.StatusCode, h.Rule.RunShURL)
	}

	_, copyErr := io.Copy(tmpFile, resp.Body)
	if closeErr := tmpFile.Close(); closeErr != nil && copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		return "", fmt.Errorf("failed to write script: %w", copyErr)
	}

	if err := os.Chmod(tmpPath, 0700); err != nil { // #nosec G302 -- temp script needs execute permission
		return "", fmt.Errorf("failed to make script executable: %w", err)
	}

	runCmd := "sh " + tmpPath
	if h.Rule.RunSudo {
		runCmd = "sudo sh " + tmpPath
	}

	cmd := exec.Command("sh", "-c", runCmd) // #nosec G204 -- temp script path is internally generated
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("script failed: %w\n%s", err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

// Down executes the undo command if set
func (h *RunShHandler) Down() (string, error) {
	if h.Rule.RunUndo == "" {
		return "no undo command, skipping", nil
	}

	undoCmd := h.Rule.RunUndo
	if h.Rule.RunSudo {
		undoCmd = "sudo " + undoCmd
	}

	cmd := exec.Command("sh", "-c", undoCmd) // #nosec G204 -- user-supplied undo command from blueprint
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("undo command failed: %w\n%s", err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

// GetCommand returns the URL (used as the execution key)
func (h *RunShHandler) GetCommand() string {
	if h.Rule.Action == "uninstall" {
		if h.Rule.RunUndo == "" {
			return "# no undo"
		}
		if h.Rule.RunSudo {
			return "sudo " + h.Rule.RunUndo
		}
		return h.Rule.RunUndo
	}
	return h.Rule.RunShURL
}

// UpdateStatus updates the blueprint status after executing run-sh or uninstall-run-sh
func (h *RunShHandler) UpdateStatus(status *Status, records []ExecutionRecord, blueprint string, osName string) error {
	blueprint = normalizePath(blueprint)

	if h.Rule.Action == "run-sh" {
		cmd := h.GetCommand()
		_, executed := commandSuccessfullyExecuted(cmd, records)

		if !executed {
			skipMsg := fmt.Sprintf("skipped (unless check passed): %s", h.Rule.RunUnless)
			for _, record := range records {
				if record.Status == "success" && strings.Contains(record.Output, skipMsg) {
					executed = true
					break
				}
			}
		}

		if executed {
			status.Runs = removeRunStatus(status.Runs, h.Rule.RunShURL, blueprint, osName)
			status.Runs = append(status.Runs, RunStatus{
				Action:    "run-sh",
				Command:   h.Rule.RunShURL,
				UndoCmd:   h.Rule.RunUndo,
				Sudo:      h.Rule.RunSudo,
				RanAt:     time.Now().Format(time.RFC3339),
				Blueprint: blueprint,
				OS:        osName,
			})
		}
	} else if h.Rule.Action == "uninstall" && DetectRuleType(h.Rule) == "run-sh" {
		status.Runs = removeRunStatus(status.Runs, h.Rule.RunShURL, blueprint, osName)
	}

	return nil
}

// DisplayInfo displays handler-specific information
func (h *RunShHandler) DisplayInfo() {
	formatFunc := ui.FormatInfo
	if h.Rule.Action == "uninstall" {
		formatFunc = ui.FormatDim
	}

	fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Script URL: %s", h.Rule.RunShURL)))
	if h.Rule.RunSudo {
		fmt.Printf("  %s\n", formatFunc("sudo: true"))
	}
	if h.Rule.RunUnless != "" {
		fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Unless: %s", h.Rule.RunUnless)))
	}
	if h.Rule.RunUndo != "" {
		fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Undo: %s", h.Rule.RunUndo)))
	}
}

// GetDependencyKey returns the unique key for this rule in dependency resolution
func (h *RunShHandler) GetDependencyKey() string {
	return getDependencyKey(h.Rule, h.Rule.RunShURL)
}

// GetDisplayDetails returns the script URL for display during execution
func (h *RunShHandler) GetDisplayDetails(isUninstall bool) string {
	if isUninstall {
		return h.Rule.RunUndo
	}
	return h.Rule.RunShURL
}

// GetState returns handler-specific state as key-value pairs
func (h *RunShHandler) GetState(isUninstall bool) map[string]string {
	return map[string]string{
		"summary": h.Rule.RunShURL,
		"url":     h.Rule.RunShURL,
	}
}

// FindUninstallRules compares run-sh status against current rules and returns uninstall rules
func (h *RunShHandler) FindUninstallRules(status *Status, currentRules []parser.Rule, blueprintFile, osName string) []parser.Rule {
	normalizedBlueprint := normalizePath(blueprintFile)

	currentRunShURLs := make(map[string]bool)
	for _, rule := range currentRules {
		if rule.Action == "run-sh" && rule.RunShURL != "" {
			currentRunShURLs[rule.RunShURL] = true
		}
	}

	var rules []parser.Rule
	if status.Runs != nil {
		for _, r := range status.Runs {
			if r.Action != "run-sh" {
				continue
			}
			normalizedStatusBlueprint := normalizePath(r.Blueprint)
			if normalizedStatusBlueprint == normalizedBlueprint && r.OS == osName && !currentRunShURLs[r.Command] {
				// Only emit uninstall if there's an undo command
				if r.UndoCmd != "" {
					rules = append(rules, parser.Rule{
						Action:   "uninstall",
						RunShURL: r.Command,
						RunUndo:  r.UndoCmd,
						RunSudo:  r.Sudo,
						OSList:   []string{osName},
					})
				}
			}
		}
	}

	return rules
}

// DisplayStatusFromStatus displays run-sh status from Status object (delegates to RunHandler)
func (h *RunShHandler) DisplayStatusFromStatus(status *Status) {
	// run and run-sh share the same Runs slice; display is handled by RunHandler
}
