package handlers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
)

// ScheduleHandler installs/removes a crontab entry that runs blueprint apply on a schedule.
type ScheduleHandler struct {
	BaseHandler
}

// NewScheduleHandler creates a new schedule handler
func NewScheduleHandler(rule parser.Rule, basePath string) *ScheduleHandler {
	return &ScheduleHandler{
		BaseHandler: BaseHandler{
			Rule:     rule,
			BasePath: basePath,
		},
	}
}

// cronExpression returns the cron expression for this rule
func (h *ScheduleHandler) cronExpression() string {
	switch h.Rule.SchedulePreset {
	case "daily":
		return "@daily"
	case "weekly":
		return "@weekly"
	case "hourly":
		return "@hourly"
	default:
		return h.Rule.ScheduleCron
	}
}

// blueprintBinary returns the absolute path to the current blueprint binary
func blueprintBinary() string {
	if path, err := os.Executable(); err == nil && path != "" {
		return path
	}
	return "blueprint"
}

// scheduleLogPath returns the path to the schedule log file
func scheduleLogPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "~/.blueprint/schedule.log"
	}
	return filepath.Join(homeDir, ".blueprint", "schedule.log")
}

// cronLine returns the full crontab line for this rule
func (h *ScheduleHandler) cronLine() string {
	log := scheduleLogPath()
	return fmt.Sprintf("%s %s apply %s --skip-decrypt >> %s 2>&1", h.cronExpression(), blueprintBinary(), h.Rule.ScheduleSource, log)
}

// isUserInPasswordlessSudoers returns true if the current user can sudo without a password
func isUserInPasswordlessSudoers() bool {
	cmd := exec.Command("sudo", "-n", "true")
	return cmd.Run() == nil
}

// readCrontab reads the current user's crontab, returning empty string if none exists
func readCrontab() (string, error) {
	out, err := exec.Command("crontab", "-l").Output()
	if err != nil {
		// Exit code 1 on empty crontab is normal on some systems
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", err
	}
	return string(out), nil
}

// writeCrontab installs a new crontab from the given content
func writeCrontab(content string) error {
	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(content)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("crontab install failed: %w\n%s", err, string(out))
	}
	return nil
}

// Up adds the crontab entry for this schedule rule
func (h *ScheduleHandler) Up() (string, error) {
	if !isUserInPasswordlessSudoers() {
		return "", fmt.Errorf("user must have passwordless sudo before scheduling; add a sudoers rule first")
	}

	current, err := readCrontab()
	if err != nil {
		return "", fmt.Errorf("failed to read crontab: %w", err)
	}

	line := h.cronLine()
	if strings.Contains(current, line) {
		return fmt.Sprintf("already scheduled: %s", line), nil
	}

	// Append line (ensure trailing newline before appending)
	newContent := current
	if newContent != "" && !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}
	newContent += line + "\n"

	if err := writeCrontab(newContent); err != nil {
		return "", err
	}

	return fmt.Sprintf("Scheduled: %s", line), nil
}

// Down removes the crontab entry for this schedule rule
func (h *ScheduleHandler) Down() (string, error) {
	current, err := readCrontab()
	if err != nil {
		return "", fmt.Errorf("failed to read crontab: %w", err)
	}

	line := h.cronLine()
	if !strings.Contains(current, line) {
		return fmt.Sprintf("crontab entry not found (already removed): %s", line), nil
	}

	var kept []string
	for _, l := range strings.Split(current, "\n") {
		if l != line {
			kept = append(kept, l)
		}
	}
	// Trim trailing empty lines but keep a single trailing newline
	newContent := strings.TrimRight(strings.Join(kept, "\n"), "\n")
	if newContent != "" {
		newContent += "\n"
	}

	if err := writeCrontab(newContent); err != nil {
		return "", err
	}

	return fmt.Sprintf("Removed schedule: %s", line), nil
}

// GetCommand returns a string representing the install operation (used for record matching)
func (h *ScheduleHandler) GetCommand() string {
	line := h.cronLine()
	return fmt.Sprintf(`{ crontab -l 2>/dev/null; echo "%s"; } | crontab -`, line)
}

// UpdateStatus updates the status after installing or removing a schedule
func (h *ScheduleHandler) UpdateStatus(status *Status, records []ExecutionRecord, blueprint string, osName string) error {
	blueprint = normalizePath(blueprint)

	if h.Rule.Action == "schedule" {
		_, commandExecuted := commandSuccessfullyExecuted(h.GetCommand(), records)
		if !commandExecuted {
			return nil
		}

		cronExpr := h.cronExpression()
		source := h.Rule.ScheduleSource

		// Skip duplicates
		for _, s := range status.Schedules {
			if s.CronExpr == cronExpr && s.Source == source && normalizePath(s.Blueprint) == blueprint && s.OS == osName {
				return nil
			}
		}

		status.Schedules = append(status.Schedules, ScheduleStatus{
			CronExpr:    cronExpr,
			Source:      source,
			InstalledAt: time.Now().Format(time.RFC3339),
			Blueprint:   blueprint,
			OS:          osName,
		})
	} else if h.Rule.Action == "uninstall" && DetectRuleType(h.Rule) == "schedule" {
		cronExpr := h.cronExpression()
		source := h.Rule.ScheduleSource
		var newSchedules []ScheduleStatus
		for _, s := range status.Schedules {
			if s.CronExpr != cronExpr || s.Source != source || normalizePath(s.Blueprint) != blueprint || s.OS != osName {
				newSchedules = append(newSchedules, s)
			}
		}
		status.Schedules = newSchedules
	}

	return nil
}

// DisplayInfo displays handler-specific information
func (h *ScheduleHandler) DisplayInfo() {
	formatFunc := ui.FormatInfo
	if h.Rule.Action == "uninstall" {
		formatFunc = ui.FormatDim
	}
	fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("%s → blueprint apply %s --skip-decrypt", h.cronExpression(), h.Rule.ScheduleSource)))
}

// GetDependencyKey returns the unique key for this rule in dependency resolution
func (h *ScheduleHandler) GetDependencyKey() string {
	fallback := "schedule-" + h.Rule.ScheduleSource
	if fallback == "schedule-" {
		fallback = "schedule"
	}
	return getDependencyKey(h.Rule, fallback)
}

// GetDisplayDetails returns the display detail for this rule during execution
func (h *ScheduleHandler) GetDisplayDetails(isUninstall bool) string {
	return fmt.Sprintf("%s %s", h.cronExpression(), h.Rule.ScheduleSource)
}

// GetState returns handler-specific state as key-value pairs
func (h *ScheduleHandler) GetState(isUninstall bool) map[string]string {
	summary := fmt.Sprintf("%s %s", h.cronExpression(), h.Rule.ScheduleSource)
	return map[string]string{
		"summary":  summary,
		"cron":     h.cronExpression(),
		"source":   h.Rule.ScheduleSource,
		"schedule": summary,
	}
}

// FindUninstallRules compares schedule status against current rules and returns uninstall rules
func (h *ScheduleHandler) FindUninstallRules(status *Status, currentRules []parser.Rule, blueprintFile, osName string) []parser.Rule {
	normalizedBlueprint := normalizePath(blueprintFile)

	// Build set of (cronExpr, file) pairs covered by current schedule rules
	type key struct{ cron, file string }
	current := make(map[key]bool)
	for _, rule := range currentRules {
		if rule.Action == "schedule" {
			// We need a temporary handler to resolve the cron expression
			tmp := NewScheduleHandler(rule, "")
			current[key{tmp.cronExpression(), rule.ScheduleSource}] = true
		}
	}

	var rules []parser.Rule
	for _, s := range status.Schedules {
		if normalizePath(s.Blueprint) == normalizedBlueprint && s.OS == osName {
			if !current[key{s.CronExpr, s.Source}] {
				rules = append(rules, parser.Rule{
					Action:         "uninstall",
					ScheduleCron:   s.CronExpr, // stored as raw cron; cronExpression() returns it via default branch
					ScheduleSource: s.Source,
					OSList:         []string{osName},
				})
			}
		}
	}

	return rules
}
