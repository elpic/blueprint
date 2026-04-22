package handlers

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
)

func init() {
	RegisterAction(ActionDef{
		Name:   "schedule",
		Prefix: "schedule ",
		NewHandler: func(rule parser.Rule, basePath string, passwordCache map[string]string) Handler {
			return NewScheduleHandler(rule, basePath)
		},
		RuleKey: func(rule parser.Rule) string {
			if rule.ScheduleSource != "" {
				return "schedule-" + rule.ScheduleSource
			}
			return "schedule"
		},
		Detect: func(rule parser.Rule) bool {
			return rule.ScheduleSource != ""
		},
		Summary: func(rule parser.Rule) string {
			return rule.ScheduleSource
		},
		OrphanIndex: func(rule parser.Rule, index func(string)) {
			index(rule.ScheduleSource)
			index(NormalizeBlueprint(rule.ScheduleSource))
		},
		ShellExport: func(rule parser.Rule, _, _ string) []string {
			cron := rule.ScheduleCron
			if cron == "" {
				switch rule.SchedulePreset {
				case "daily":
					cron = "0 9 * * *"
				case "weekly":
					cron = "0 9 * * 1"
				case "hourly":
					cron = "0 * * * *"
				}
			}
			source := rule.ScheduleSource
			cronLine := fmt.Sprintf(`%s blueprint apply %s --skip-decrypt >> ~/.blueprint/schedule.log 2>&1`, cron, shellQ(source))
			return []string{
				fmt.Sprintf(`(crontab -l 2>/dev/null | grep -v %s; echo %s) | crontab -`, shellQ(source), shellQ(cronLine)),
			}
		},
	})
}

// ScheduleHandler installs/removes a crontab entry that runs blueprint apply on a schedule.
type ScheduleHandler struct {
	BaseHandler
	currentRecords []ExecutionRecord
}

// SetCurrentRecords provides the handler with the execution records accumulated
// so far in the current run. The engine calls this before Up() so that the
// handler can check whether a sudoers rule already ran successfully in this run.
func (h *ScheduleHandler) SetCurrentRecords(records []ExecutionRecord) {
	h.currentRecords = records
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
	return fmt.Sprintf(`%s %s apply "%s" --skip-decrypt >> %s 2>&1`, h.cronExpression(), blueprintBinary(), h.Rule.ScheduleSource, log)
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

// sudoersRanSuccessfully returns true if a sudoers rule succeeded in the
// provided current-run execution records.
func sudoersRanSuccessfully(records []ExecutionRecord) bool {
	sudoersCmd := NewSudoersHandler(parser.Rule{Action: "sudoers"}, "").GetCommand()
	for _, r := range records {
		if r.Status == "success" && r.Command == sudoersCmd {
			return true
		}
	}
	return false
}

// UpWithStatus adds the crontab entry using the provided status, current-run
// execution records, and injectable crontab functions. This is the testable core of Up().
func (h *ScheduleHandler) UpWithStatus(status *Status, records []ExecutionRecord, readCron func() (string, error), writeCron func(string) error) (string, error) {
	if len(status.Sudoers) == 0 && !sudoersRanSuccessfully(records) {
		return "", fmt.Errorf("user must have a sudoers entry before scheduling; add a sudoers rule first")
	}

	current, err := readCron()
	if err != nil {
		return "", fmt.Errorf("failed to read crontab: %w", err)
	}

	line := h.cronLine()
	if strings.Contains(current, line) {
		return fmt.Sprintf("already scheduled: %s", line), nil
	}

	newContent := current
	if newContent != "" && !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}
	newContent += line + "\n"

	if err := writeCron(newContent); err != nil {
		return "", err
	}

	return fmt.Sprintf("Scheduled: %s", line), nil
}

// loadStatus reads the current blueprint status from ~/.blueprint/status.json.
// Returns an empty Status if the file does not exist or cannot be parsed.
func loadStatus() *Status {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return &Status{}
	}
	data, err := os.ReadFile(filepath.Join(homeDir, ".blueprint", "status.json"))
	if err != nil {
		return &Status{}
	}
	var s Status
	if err := json.Unmarshal(data, &s); err != nil {
		return &Status{}
	}
	return &s
}

// Up adds the crontab entry for this schedule rule
func (h *ScheduleHandler) Up() (string, error) {
	return h.UpWithStatus(loadStatus(), h.currentRecords, readCrontab, writeCrontab)
}

// DownWithCrontab removes the crontab entry using injectable crontab functions.
func (h *ScheduleHandler) DownWithCrontab(readCron func() (string, error), writeCron func(string) error) (string, error) {
	current, err := readCron()
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
	newContent := strings.TrimRight(strings.Join(kept, "\n"), "\n")
	if newContent != "" {
		newContent += "\n"
	}

	if err := writeCron(newContent); err != nil {
		return "", err
	}

	return fmt.Sprintf("Removed schedule: %s", line), nil
}

// Down removes the crontab entry for this schedule rule
func (h *ScheduleHandler) Down() (string, error) {
	return h.DownWithCrontab(readCrontab, writeCrontab)
}

// GetCommand returns a string representing the install operation (used for record matching)
func (h *ScheduleHandler) GetCommand() string {
	line := h.cronLine()
	return fmt.Sprintf(`{ crontab -l 2>/dev/null; echo "%s"; } | crontab -`, line)
}

// UpdateStatus updates the status after installing or removing a schedule
func (h *ScheduleHandler) UpdateStatus(status *Status, records []ExecutionRecord, blueprint string, osName string) error {
	blueprint = normalizeBlueprint(blueprint)

	if h.Rule.Action == "schedule" {
		_, commandExecuted := commandSuccessfullyExecuted(h.GetCommand(), records)
		if !commandExecuted {
			return nil
		}

		cronExpr := h.cronExpression()
		source := h.Rule.ScheduleSource

		// Skip duplicates
		for _, s := range status.Schedules {
			if s.CronExpr == cronExpr && s.Source == source && normalizeBlueprint(s.Blueprint) == blueprint && s.OS == osName {
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
			if s.CronExpr != cronExpr || s.Source != source || normalizeBlueprint(s.Blueprint) != blueprint || s.OS != osName {
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
	normalizedBlueprint := normalizeBlueprint(blueprintFile)

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
		if normalizeBlueprint(s.Blueprint) == normalizedBlueprint && s.OS == osName {
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

// IsInstalled returns true if the schedule entry in this rule is already in status.
func (h *ScheduleHandler) IsInstalled(status *Status, blueprintFile, osName string) bool {
	normalizedBlueprint := normalizeBlueprint(blueprintFile)
	cronExpr := h.cronExpression()
	for _, s := range status.Schedules {
		if s.CronExpr == cronExpr && s.Source == h.Rule.ScheduleSource && normalizeBlueprint(s.Blueprint) == normalizedBlueprint && s.OS == osName {
			return true
		}
	}
	return false
}
