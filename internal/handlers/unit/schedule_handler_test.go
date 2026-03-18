package unit

import (
	"testing"

	"github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/parser"
)

func TestScheduleHandler_GetCommand_Pure(t *testing.T) {
	tests := []struct {
		name          string
		rule          parser.Rule
		expectedParts []string // Parts that should be present in the command
	}{
		{
			name: "daily preset schedule",
			rule: parser.Rule{
				Action:         "schedule",
				SchedulePreset: "daily",
				ScheduleSource: "/home/user/blueprint.yml",
			},
			expectedParts: []string{
				"{ crontab -l 2>/dev/null; echo \"@daily",
				"apply \"/home/user/blueprint.yml\"",
				"--skip-decrypt >>",
				"schedule.log 2>&1\"; } | crontab -",
			},
		},
		{
			name: "weekly preset schedule",
			rule: parser.Rule{
				Action:         "schedule",
				SchedulePreset: "weekly",
				ScheduleSource: "/home/user/app.yml",
			},
			expectedParts: []string{
				"{ crontab -l 2>/dev/null; echo \"@weekly",
				"apply \"/home/user/app.yml\"",
				"--skip-decrypt >>",
				"schedule.log 2>&1\"; } | crontab -",
			},
		},
		{
			name: "hourly preset schedule",
			rule: parser.Rule{
				Action:         "schedule",
				SchedulePreset: "hourly",
				ScheduleSource: "/etc/blueprint/system.yml",
			},
			expectedParts: []string{
				"{ crontab -l 2>/dev/null; echo \"@hourly",
				"apply \"/etc/blueprint/system.yml\"",
				"--skip-decrypt >>",
				"schedule.log 2>&1\"; } | crontab -",
			},
		},
		{
			name: "custom cron expression",
			rule: parser.Rule{
				Action:         "schedule",
				ScheduleCron:   "0 2 * * *",
				ScheduleSource: "/home/user/nightly.yml",
			},
			expectedParts: []string{
				"{ crontab -l 2>/dev/null; echo \"0 2 * * *",
				"apply \"/home/user/nightly.yml\"",
				"--skip-decrypt >>",
				"schedule.log 2>&1\"; } | crontab -",
			},
		},
		{
			name: "complex cron with special characters",
			rule: parser.Rule{
				Action:         "schedule",
				ScheduleCron:   "*/15 9-17 * * 1-5",
				ScheduleSource: "/work/monitoring.yml",
			},
			expectedParts: []string{
				"{ crontab -l 2>/dev/null; echo \"*/15 9-17 * * 1-5",
				"apply \"/work/monitoring.yml\"",
				"--skip-decrypt >>",
				"schedule.log 2>&1\"; } | crontab -",
			},
		},
		{
			name: "source with spaces",
			rule: parser.Rule{
				Action:         "schedule",
				SchedulePreset: "daily",
				ScheduleSource: "/home/user/my app/blueprint.yml",
			},
			expectedParts: []string{
				"{ crontab -l 2>/dev/null; echo \"@daily",
				"apply \"/home/user/my app/blueprint.yml\"",
				"--skip-decrypt >>",
				"schedule.log 2>&1\"; } | crontab -",
			},
		},
		{
			name: "preset takes precedence over cron",
			rule: parser.Rule{
				Action:         "schedule",
				SchedulePreset: "weekly",
				ScheduleCron:   "0 1 * * *", // This should be ignored
				ScheduleSource: "/home/user/test.yml",
			},
			expectedParts: []string{
				"{ crontab -l 2>/dev/null; echo \"@weekly",
				"apply \"/home/user/test.yml\"",
				"--skip-decrypt >>",
				"schedule.log 2>&1\"; } | crontab -",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlers.NewScheduleHandler(tt.rule, "/test/path")
			result := handler.GetCommand()

			// Check that all expected parts are present
			for _, part := range tt.expectedParts {
				if !containsStr(result, part) {
					t.Errorf("GetCommand() = %q should contain %q", result, part)
				}
			}
		})
	}
}

func TestScheduleHandler_GetDependencyKey_Pure(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		expected string
	}{
		{
			name: "uses rule ID when present",
			rule: parser.Rule{
				ID:             "custom-schedule",
				Action:         "schedule",
				ScheduleSource: "/home/user/app.yml",
			},
			expected: "custom-schedule",
		},
		{
			name: "uses schedule source as fallback",
			rule: parser.Rule{
				Action:         "schedule",
				ScheduleSource: "/home/user/app.yml",
			},
			expected: "schedule-/home/user/app.yml",
		},
		{
			name: "handles empty source gracefully",
			rule: parser.Rule{
				Action:         "schedule",
				ScheduleSource: "",
			},
			expected: "schedule",
		},
		{
			name: "uses basename of source path",
			rule: parser.Rule{
				Action:         "schedule",
				ScheduleSource: "/very/long/path/to/blueprint.yml",
			},
			expected: "schedule-/very/long/path/to/blueprint.yml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlers.NewScheduleHandler(tt.rule, "/test/path")
			result := handler.GetDependencyKey()
			if result != tt.expected {
				t.Errorf("GetDependencyKey() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestScheduleHandler_GetDisplayDetails_Pure(t *testing.T) {
	tests := []struct {
		name        string
		rule        parser.Rule
		isUninstall bool
		expected    string
	}{
		{
			name: "daily preset",
			rule: parser.Rule{
				SchedulePreset: "daily",
				ScheduleSource: "/home/user/blueprint.yml",
			},
			isUninstall: false,
			expected:    "@daily /home/user/blueprint.yml",
		},
		{
			name: "weekly preset",
			rule: parser.Rule{
				SchedulePreset: "weekly",
				ScheduleSource: "/home/user/app.yml",
			},
			isUninstall: false,
			expected:    "@weekly /home/user/app.yml",
		},
		{
			name: "hourly preset",
			rule: parser.Rule{
				SchedulePreset: "hourly",
				ScheduleSource: "/etc/system.yml",
			},
			isUninstall: false,
			expected:    "@hourly /etc/system.yml",
		},
		{
			name: "custom cron expression",
			rule: parser.Rule{
				ScheduleCron:   "0 2 * * *",
				ScheduleSource: "/home/user/nightly.yml",
			},
			isUninstall: false,
			expected:    "0 2 * * * /home/user/nightly.yml",
		},
		{
			name: "complex cron expression",
			rule: parser.Rule{
				ScheduleCron:   "*/15 9-17 * * 1-5",
				ScheduleSource: "/work/monitoring.yml",
			},
			isUninstall: false,
			expected:    "*/15 9-17 * * 1-5 /work/monitoring.yml",
		},
		{
			name: "uninstall operation (same format)",
			rule: parser.Rule{
				SchedulePreset: "daily",
				ScheduleSource: "/home/user/test.yml",
			},
			isUninstall: true,
			expected:    "@daily /home/user/test.yml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlers.NewScheduleHandler(tt.rule, "/test/path")
			result := handler.GetDisplayDetails(tt.isUninstall)
			if result != tt.expected {
				t.Errorf("GetDisplayDetails() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestScheduleHandler_GetState_Pure(t *testing.T) {
	tests := []struct {
		name        string
		rule        parser.Rule
		isUninstall bool
		expected    map[string]string
	}{
		{
			name: "daily preset schedule",
			rule: parser.Rule{
				SchedulePreset: "daily",
				ScheduleSource: "/home/user/blueprint.yml",
			},
			isUninstall: false,
			expected: map[string]string{
				"summary":  "@daily /home/user/blueprint.yml",
				"cron":     "@daily",
				"source":   "/home/user/blueprint.yml",
				"schedule": "@daily /home/user/blueprint.yml",
			},
		},
		{
			name: "custom cron expression",
			rule: parser.Rule{
				ScheduleCron:   "0 2 * * *",
				ScheduleSource: "/home/user/nightly.yml",
			},
			isUninstall: false,
			expected: map[string]string{
				"summary":  "0 2 * * * /home/user/nightly.yml",
				"cron":     "0 2 * * *",
				"source":   "/home/user/nightly.yml",
				"schedule": "0 2 * * * /home/user/nightly.yml",
			},
		},
		{
			name: "weekly preset",
			rule: parser.Rule{
				SchedulePreset: "weekly",
				ScheduleSource: "/work/backup.yml",
			},
			isUninstall: false,
			expected: map[string]string{
				"summary":  "@weekly /work/backup.yml",
				"cron":     "@weekly",
				"source":   "/work/backup.yml",
				"schedule": "@weekly /work/backup.yml",
			},
		},
		{
			name: "uninstall operation",
			rule: parser.Rule{
				SchedulePreset: "hourly",
				ScheduleSource: "/monitoring/check.yml",
			},
			isUninstall: true,
			expected: map[string]string{
				"summary":  "@hourly /monitoring/check.yml",
				"cron":     "@hourly",
				"source":   "/monitoring/check.yml",
				"schedule": "@hourly /monitoring/check.yml",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlers.NewScheduleHandler(tt.rule, "/test/path")
			result := handler.GetState(tt.isUninstall)

			if len(result) != len(tt.expected) {
				t.Errorf("GetState() returned %d keys, want %d", len(result), len(tt.expected))
			}

			for key, expectedValue := range tt.expected {
				if actualValue, exists := result[key]; !exists {
					t.Errorf("GetState() missing key %v", key)
				} else if actualValue != expectedValue {
					t.Errorf("GetState()[%v] = %v, want %v", key, actualValue, expectedValue)
				}
			}
		})
	}
}

func TestScheduleHandler_IsInstalled_Pure(t *testing.T) {
	// Create test status with various schedule entries
	status := &handlers.Status{
		Schedules: []handlers.ScheduleStatus{
			{
				CronExpr:  "@daily",
				Source:    "/home/user/blueprint.yml",
				Blueprint: "/test/blueprint.yml",
				OS:        "linux",
			},
			{
				CronExpr:  "@weekly",
				Source:    "/work/app.yml",
				Blueprint: "/test/blueprint.yml",
				OS:        "linux",
			},
			{
				CronExpr:  "0 2 * * *",
				Source:    "/home/user/nightly.yml",
				Blueprint: "/other/blueprint.yml", // Different blueprint
				OS:        "linux",
			},
			{
				CronExpr:  "@hourly",
				Source:    "/monitoring/check.yml",
				Blueprint: "/test/blueprint.yml",
				OS:        "darwin", // Different OS
			},
		},
	}

	tests := []struct {
		name          string
		rule          parser.Rule
		blueprintFile string
		osName        string
		expected      bool
	}{
		{
			name: "schedule installed for matching blueprint and OS",
			rule: parser.Rule{
				SchedulePreset: "daily",
				ScheduleSource: "/home/user/blueprint.yml",
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux",
			expected:      true,
		},
		{
			name: "schedule with custom cron installed",
			rule: parser.Rule{
				ScheduleCron:   "0 2 * * *",
				ScheduleSource: "/home/user/nightly.yml",
			},
			blueprintFile: "/other/blueprint.yml",
			osName:        "linux",
			expected:      true,
		},
		{
			name: "schedule not installed",
			rule: parser.Rule{
				SchedulePreset: "monthly",
				ScheduleSource: "/home/user/missing.yml",
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux",
			expected:      false,
		},
		{
			name: "schedule installed but wrong OS",
			rule: parser.Rule{
				SchedulePreset: "hourly",
				ScheduleSource: "/monitoring/check.yml",
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux", // schedule is for darwin, not linux
			expected:      false,
		},
		{
			name: "schedule installed but wrong blueprint",
			rule: parser.Rule{
				ScheduleCron:   "0 2 * * *",
				ScheduleSource: "/home/user/nightly.yml",
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux", // schedule is in /other/blueprint.yml, not /test/blueprint.yml
			expected:      false,
		},
		{
			name: "wrong cron expression",
			rule: parser.Rule{
				SchedulePreset: "weekly",                   // @weekly
				ScheduleSource: "/home/user/blueprint.yml", // source matches
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux",
			expected:      false, // @daily is installed, not @weekly
		},
		{
			name: "wrong source file",
			rule: parser.Rule{
				SchedulePreset: "daily",
				ScheduleSource: "/home/user/different.yml", // different source
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux",
			expected:      false, // /home/user/blueprint.yml is installed, not /home/user/different.yml
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlers.NewScheduleHandler(tt.rule, "/test/path")
			result := handler.IsInstalled(status, tt.blueprintFile, tt.osName)
			if result != tt.expected {
				t.Errorf("IsInstalled() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestScheduleHandler_FindUninstallRules_Pure(t *testing.T) {
	// Create test status with various schedule entries
	status := &handlers.Status{
		Schedules: []handlers.ScheduleStatus{
			{
				CronExpr:  "@daily",
				Source:    "/home/user/blueprint.yml",
				Blueprint: "/test/blueprint.yml",
				OS:        "linux",
			},
			{
				CronExpr:  "@weekly",
				Source:    "/work/app.yml",
				Blueprint: "/test/blueprint.yml",
				OS:        "linux",
			},
			{
				CronExpr:  "0 2 * * *",
				Source:    "/home/user/nightly.yml",
				Blueprint: "/other/blueprint.yml", // Different blueprint
				OS:        "linux",
			},
			{
				CronExpr:  "@hourly",
				Source:    "/monitoring/check.yml",
				Blueprint: "/test/blueprint.yml",
				OS:        "darwin", // Different OS
			},
		},
	}

	tests := []struct {
		name          string
		currentRules  []parser.Rule
		blueprintFile string
		osName        string
		expectedCount int                     // Expected number of uninstall rules
		expectedItems []scheduleUninstallItem // Expected items to uninstall
	}{
		{
			name:          "no current rules - all schedules should be uninstalled",
			currentRules:  []parser.Rule{},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux",
			expectedCount: 2,
			expectedItems: []scheduleUninstallItem{
				{cronExpr: "@daily", source: "/home/user/blueprint.yml"},
				{cronExpr: "@weekly", source: "/work/app.yml"},
			},
		},
		{
			name: "some schedules still in current rules",
			currentRules: []parser.Rule{
				{
					Action:         "schedule",
					SchedulePreset: "daily",
					ScheduleSource: "/home/user/blueprint.yml",
				},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux",
			expectedCount: 1,
			expectedItems: []scheduleUninstallItem{
				{cronExpr: "@weekly", source: "/work/app.yml"}, // daily is kept, weekly should be uninstalled
			},
		},
		{
			name: "all schedules still in current rules",
			currentRules: []parser.Rule{
				{
					Action:         "schedule",
					SchedulePreset: "daily",
					ScheduleSource: "/home/user/blueprint.yml",
				},
				{
					Action:         "schedule",
					SchedulePreset: "weekly",
					ScheduleSource: "/work/app.yml",
				},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux",
			expectedCount: 0,
			expectedItems: []scheduleUninstallItem{}, // Nothing to uninstall
		},
		{
			name:          "different OS - schedules from different OS",
			currentRules:  []parser.Rule{},
			blueprintFile: "/test/blueprint.yml",
			osName:        "darwin",
			expectedCount: 1,
			expectedItems: []scheduleUninstallItem{
				{cronExpr: "@hourly", source: "/monitoring/check.yml"}, // Only hourly for darwin
			},
		},
		{
			name:          "different blueprint - schedules from different blueprint",
			currentRules:  []parser.Rule{},
			blueprintFile: "/other/blueprint.yml",
			osName:        "linux",
			expectedCount: 1,
			expectedItems: []scheduleUninstallItem{
				{cronExpr: "0 2 * * *", source: "/home/user/nightly.yml"}, // Only custom cron belongs to this blueprint
			},
		},
		{
			name: "mixed scenarios with custom cron",
			currentRules: []parser.Rule{
				{
					Action:         "schedule",
					SchedulePreset: "daily",
					ScheduleSource: "/home/user/blueprint.yml",
				},
				{
					Action:         "schedule",
					ScheduleCron:   "0 1 * * *", // Different cron, new schedule not in status
					ScheduleSource: "/home/user/early.yml",
				},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux",
			expectedCount: 1,
			expectedItems: []scheduleUninstallItem{
				{cronExpr: "@weekly", source: "/work/app.yml"}, // weekly should be uninstalled, daily is kept
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a handler (the specific rule doesn't matter for this test)
			rule := parser.Rule{Action: "schedule"}
			handler := handlers.NewScheduleHandler(rule, "/test/path")

			result := handler.FindUninstallRules(status, tt.currentRules, tt.blueprintFile, tt.osName)

			if len(result) != tt.expectedCount {
				t.Errorf("FindUninstallRules() returned %d rules, want %d", len(result), tt.expectedCount)
				return
			}

			// Check each uninstall rule matches expected items
			for i, uninstallRule := range result {
				if uninstallRule.Action != "uninstall" {
					t.Errorf("FindUninstallRules()[%d] action = %v, want 'uninstall'", i, uninstallRule.Action)
				}

				// Check that the rule corresponds to one of the expected items
				found := false
				actualCron := uninstallRule.ScheduleCron
				actualSource := uninstallRule.ScheduleSource

				for _, expectedItem := range tt.expectedItems {
					if actualCron == expectedItem.cronExpr && actualSource == expectedItem.source {
						found = true
						break
					}
				}

				if !found {
					t.Errorf("FindUninstallRules()[%d] unexpected rule: cron=%v source=%v", i, actualCron, actualSource)
				}
			}
		})
	}
}

// Helper type for test expectations
type scheduleUninstallItem struct {
	cronExpr string
	source   string
}

// Test cron expression logic
func TestScheduleHandler_CronExpression_Pure(t *testing.T) {
	tests := []struct {
		name          string
		rule          parser.Rule
		expectedInCmd string // What should appear in the generated command
	}{
		{
			name: "daily preset",
			rule: parser.Rule{
				SchedulePreset: "daily",
			},
			expectedInCmd: "@daily",
		},
		{
			name: "weekly preset",
			rule: parser.Rule{
				SchedulePreset: "weekly",
			},
			expectedInCmd: "@weekly",
		},
		{
			name: "hourly preset",
			rule: parser.Rule{
				SchedulePreset: "hourly",
			},
			expectedInCmd: "@hourly",
		},
		{
			name: "custom cron expression",
			rule: parser.Rule{
				ScheduleCron: "0 2 * * *",
			},
			expectedInCmd: "0 2 * * *",
		},
		{
			name: "preset overrides cron",
			rule: parser.Rule{
				SchedulePreset: "daily",
				ScheduleCron:   "0 3 * * *", // Should be ignored
			},
			expectedInCmd: "@daily",
		},
		{
			name: "unknown preset falls back to cron",
			rule: parser.Rule{
				SchedulePreset: "unknown",
				ScheduleCron:   "*/5 * * * *",
			},
			expectedInCmd: "*/5 * * * *",
		},
		{
			name: "empty preset and cron",
			rule: parser.Rule{
				SchedulePreset: "",
				ScheduleCron:   "",
			},
			expectedInCmd: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Add a dummy source to get a valid command
			tt.rule.ScheduleSource = "/test/app.yml"
			handler := handlers.NewScheduleHandler(tt.rule, "/test/path")

			// Test through GetDisplayDetails which calls cronExpression()
			details := handler.GetDisplayDetails(false)
			if !containsStr(details, tt.expectedInCmd) && tt.expectedInCmd != "" {
				t.Errorf("GetDisplayDetails() = %q should contain %q", details, tt.expectedInCmd)
			}
		})
	}
}

// Test constructor and basic properties
func TestScheduleHandler_Constructor_Pure(t *testing.T) {
	rule := parser.Rule{
		Action:         "schedule",
		SchedulePreset: "daily",
		ScheduleSource: "/home/user/blueprint.yml",
	}

	handler := handlers.NewScheduleHandler(rule, "/test/base/path")

	// Test that the handler implements the expected interface methods
	if handler.GetCommand() == "" {
		t.Error("GetCommand() should return non-empty string")
	}

	if handler.GetDependencyKey() == "" {
		t.Error("GetDependencyKey() should return non-empty string")
	}

	if handler.GetDisplayDetails(false) == "" {
		t.Error("GetDisplayDetails() should return non-empty string")
	}

	state := handler.GetState(false)
	if len(state) == 0 {
		t.Error("GetState() should return non-empty map")
	}
}

// Helper function to check if a string contains a substring
func containsStr(s, substr string) bool {
	if substr == "" {
		return true
	}
	return len(s) >= len(substr) && findSubstr(s, substr) >= 0
}

func findSubstr(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
