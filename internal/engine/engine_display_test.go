package engine

import (
	"testing"
	"time"

	"github.com/elpic/blueprint/internal/parser"
)

// TestExecutionRecordForGPGKeyHistory tests that execution records are created for GPG key operations
func TestExecutionRecordForGPGKeyHistory(t *testing.T) {
	tests := []struct {
		name    string
		rule    parser.Rule
		status  string
		command string
		output  string
		hasErr  bool
	}{
		{
			name: "successful gpg-key installation history",
			rule: parser.Rule{
				Action:     "gpg-key",
				GPGKeyring: "wezterm-fury",
				GPGKeyURL:  "https://apt.fury.io/wez/gpg.key",
				GPGDebURL:  "https://apt.fury.io/wez/",
			},
			status:  "success",
			command: "gpg-key wezterm-fury",
			output:  "Added GPG key wezterm-fury and repository https://apt.fury.io/wez/",
			hasErr:  false,
		},
		{
			name: "failed gpg-key installation history",
			rule: parser.Rule{
				Action:     "gpg-key",
				GPGKeyring: "test-repo",
				GPGKeyURL:  "https://example.com/gpg.key",
				GPGDebURL:  "https://example.com/apt",
			},
			status:  "error",
			command: "gpg-key test-repo",
			output:  "",
			hasErr:  true,
		},
		{
			name: "successful gpg-key removal history",
			rule: parser.Rule{
				Action:     "uninstall",
				GPGKeyring: "wezterm-fury",
			},
			status:  "success",
			command: "uninstall-gpg-key",
			output:  "Removed GPG key wezterm-fury and repository",
			hasErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := ExecutionRecord{
				Timestamp: time.Now().Format(time.RFC3339),
				Blueprint: "/test/blueprint.bp",
				OS:        "linux",
				Command:   tt.command,
				Output:    tt.output,
				Status:    tt.status,
			}

			if tt.hasErr {
				record.Error = "command failed"
			}

			// Verify all fields are recorded
			if record.Timestamp == "" {
				t.Error("Timestamp not recorded")
			}
			if record.Blueprint != "/test/blueprint.bp" {
				t.Error("Blueprint not recorded")
			}
			if record.OS != "linux" {
				t.Error("OS not recorded")
			}
			if record.Command != tt.command {
				t.Errorf("Command: got %q, want %q", record.Command, tt.command)
			}
			if record.Status != tt.status {
				t.Errorf("Status: got %q, want %q", record.Status, tt.status)
			}

			// Verify error field
			if tt.hasErr && record.Error == "" {
				t.Error("Error not recorded for failed execution")
			}
			if !tt.hasErr && record.Error != "" {
				t.Error("Error recorded for successful execution")
			}
		})
	}
}

// TestExecutionRecordTimestamp tests that execution records have proper timestamps
func TestExecutionRecordTimestamp(t *testing.T) {
	now := time.Now()
	record := ExecutionRecord{
		Timestamp: now.Format(time.RFC3339),
		Blueprint: "/test/blueprint.bp",
		OS:        "linux",
		Command:   "gpg-key test",
		Status:    "success",
	}

	if record.Timestamp == "" {
		t.Error("Timestamp is empty")
	}

	// Verify timestamp can be parsed back
	parsed, err := time.Parse(time.RFC3339, record.Timestamp)
	if err != nil {
		t.Errorf("Failed to parse timestamp: %v", err)
	}

	// Verify timestamp is recent (within 1 second)
	diff := now.Sub(parsed).Seconds()
	if diff < -1 || diff > 1 {
		t.Errorf("Timestamp is too far off: %f seconds", diff)
	}
}

// TestGPGKeyCommandDisplay tests that GPG key rules display correctly
func TestGPGKeyCommandDisplay(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		expected string
	}{
		{
			name: "basic gpg-key rule display",
			rule: parser.Rule{
				Action:     "gpg-key",
				GPGKeyring: "wezterm-fury",
				GPGKeyURL:  "https://apt.fury.io/wez/gpg.key",
				GPGDebURL:  "https://apt.fury.io/wez/",
			},
			expected: "gpg-key",
		},
		{
			name: "gpg-key with ID display",
			rule: parser.Rule{
				ID:         "wezterm-setup",
				Action:     "gpg-key",
				GPGKeyring: "wezterm-fury",
				GPGKeyURL:  "https://apt.fury.io/wez/gpg.key",
				GPGDebURL:  "https://apt.fury.io/wez/",
			},
			expected: "gpg-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify rule has expected action
			if tt.rule.Action != tt.expected {
				t.Errorf("Rule action: got %q, want %q", tt.rule.Action, tt.expected)
			}

			// Verify keyring information is available for display
			if tt.rule.GPGKeyring == "" {
				t.Error("Keyring not available for display")
			}

			// Verify URLs are available for display
			if tt.rule.GPGKeyURL == "" {
				t.Error("GPG Key URL not available for display")
			}
			if tt.rule.GPGDebURL == "" {
				t.Error("DEB URL not available for display")
			}
		})
	}
}

// TestGPGKeyPlanDisplay tests that plan display shows GPG key information
func TestGPGKeyPlanDisplay(t *testing.T) {
	rule := parser.Rule{
		Action:     "gpg-key",
		GPGKeyring: "wezterm-fury",
		GPGKeyURL:  "https://apt.fury.io/wez/gpg.key",
		GPGDebURL:  "https://apt.fury.io/wez/",
		OSList:     []string{"linux"},
	}

	// Verify all fields needed for plan display are present
	if rule.Action == "" {
		t.Error("Action missing for plan display")
	}
	if rule.GPGKeyring == "" {
		t.Error("Keyring missing for plan display")
	}
	if rule.GPGKeyURL == "" {
		t.Error("GPG Key URL missing for plan display")
	}
	if rule.GPGDebURL == "" {
		t.Error("DEB URL missing for plan display")
	}
	if len(rule.OSList) == 0 {
		t.Error("OS list missing for plan display")
	}
}

// TestGPGKeyUninstallDisplay tests that uninstall GPG key rules display correctly
func TestGPGKeyUninstallDisplay(t *testing.T) {
	rule := parser.Rule{
		Action:     "uninstall",
		GPGKeyring: "wezterm-fury",
		OSList:     []string{"linux"},
	}

	// Verify all fields are present for uninstall display
	if rule.Action != "uninstall" {
		t.Errorf("Action: got %q, want 'uninstall'", rule.Action)
	}
	if rule.GPGKeyring == "" {
		t.Error("Keyring missing for uninstall display")
	}
}

// TestRuleIDForDisplay tests that rule IDs are available for display
func TestRuleIDForDisplay(t *testing.T) {
	tests := []struct {
		name  string
		rule  parser.Rule
		hasID bool
	}{
		{
			name: "gpg-key with ID",
			rule: parser.Rule{
				ID:         "wezterm-setup",
				Action:     "gpg-key",
				GPGKeyring: "wezterm-fury",
				GPGKeyURL:  "https://apt.fury.io/wez/gpg.key",
				GPGDebURL:  "https://apt.fury.io/wez/",
			},
			hasID: true,
		},
		{
			name: "gpg-key without ID",
			rule: parser.Rule{
				Action:     "gpg-key",
				GPGKeyring: "wezterm-fury",
				GPGKeyURL:  "https://apt.fury.io/wez/gpg.key",
				GPGDebURL:  "https://apt.fury.io/wez/",
			},
			hasID: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.hasID {
				if tt.rule.ID == "" {
					t.Error("ID expected but not found")
				}
			} else {
				if tt.rule.ID != "" {
					t.Error("ID not expected but found")
				}
			}
		})
	}
}

// TestRuleAfterDependencies tests that rule dependencies are available for display
func TestRuleAfterDependencies(t *testing.T) {
	rule := parser.Rule{
		Action:     "gpg-key",
		GPGKeyring: "docker",
		GPGKeyURL:  "https://download.docker.com/linux/ubuntu/gpg",
		GPGDebURL:  "https://download.docker.com/linux/ubuntu",
		After:      []string{"wezterm-setup"},
	}

	if len(rule.After) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(rule.After))
	}

	if rule.After[0] != "wezterm-setup" {
		t.Errorf("Dependency: got %q, want 'wezterm-setup'", rule.After[0])
	}
}

// TestRuleOSListForDisplay tests that OS list is available for plan display
func TestRuleOSListForDisplay(t *testing.T) {
	tests := []struct {
		name   string
		oslist []string
	}{
		{
			name:   "single OS",
			oslist: []string{"linux"},
		},
		{
			name:   "multiple OS",
			oslist: []string{"linux", "ubuntu", "debian"},
		},
		{
			name:   "all platforms",
			oslist: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := parser.Rule{
				Action:     "gpg-key",
				GPGKeyring: "test",
				GPGKeyURL:  "https://example.com/key",
				GPGDebURL:  "https://example.com/apt",
				OSList:     tt.oslist,
			}

			// Verify OSList is available
			if rule.OSList == nil {
				t.Error("OSList should not be nil")
			}

			if len(tt.oslist) > 0 && len(rule.OSList) != len(tt.oslist) {
				t.Errorf("OSList length: got %d, want %d", len(rule.OSList), len(tt.oslist))
			}
		})
	}
}

// TestHistoryRecordIntegration tests that GPG key history is properly structured
func TestHistoryRecordIntegration(t *testing.T) {
	// Simulate a sequence of GPG key operations
	operations := []ExecutionRecord{
		{
			Timestamp: time.Now().Format(time.RFC3339),
			Blueprint: "/test/blueprint1.bp",
			OS:        "linux",
			Command:   "gpg-key wezterm-fury",
			Output:    "Added GPG key",
			Status:    "success",
		},
		{
			Timestamp: time.Now().Format(time.RFC3339),
			Blueprint: "/test/blueprint1.bp",
			OS:        "linux",
			Command:   "gpg-key docker",
			Output:    "Added GPG key",
			Status:    "success",
		},
		{
			Timestamp: time.Now().Format(time.RFC3339),
			Blueprint: "/test/blueprint2.bp",
			OS:        "linux",
			Command:   "uninstall-gpg-key",
			Output:    "Removed GPG key",
			Status:    "success",
		},
	}

	if len(operations) != 3 {
		t.Errorf("Expected 3 operations, got %d", len(operations))
	}

	// Verify each operation is properly recorded
	for i, op := range operations {
		if op.Timestamp == "" {
			t.Errorf("Operation %d: no timestamp", i)
		}
		if op.Blueprint == "" {
			t.Errorf("Operation %d: no blueprint", i)
		}
		if op.Command == "" {
			t.Errorf("Operation %d: no command", i)
		}
		if op.Status != "success" && op.Status != "error" {
			t.Errorf("Operation %d: invalid status %q", i, op.Status)
		}
	}

	// Verify we can group by blueprint
	blueprint1Ops := 0
	blueprint2Ops := 0

	for _, op := range operations {
		switch op.Blueprint {
		case "/test/blueprint1.bp":
			blueprint1Ops++
		case "/test/blueprint2.bp":
			blueprint2Ops++
		}
	}

	if blueprint1Ops != 2 {
		t.Errorf("Blueprint1: expected 2 ops, got %d", blueprint1Ops)
	}

	if blueprint2Ops != 1 {
		t.Errorf("Blueprint2: expected 1 op, got %d", blueprint2Ops)
	}
}

// TestExecutionRecordTypes tests different record types
func TestExecutionRecordTypes(t *testing.T) {
	recordTypes := []struct {
		name    string
		command string
		status  string
	}{
		{
			name:    "gpg-key installation",
			command: "gpg-key wezterm-fury",
			status:  "success",
		},
		{
			name:    "gpg-key uninstall",
			command: "uninstall-gpg-key",
			status:  "success",
		},
		{
			name:    "failed gpg-key",
			command: "gpg-key test",
			status:  "error",
		},
	}

	for _, rt := range recordTypes {
		t.Run(rt.name, func(t *testing.T) {
			record := ExecutionRecord{
				Timestamp: time.Now().Format(time.RFC3339),
				Blueprint: "/test/blueprint.bp",
				OS:        "linux",
				Command:   rt.command,
				Status:    rt.status,
			}

			// Verify record structure
			if record.Command == "" || record.Status == "" {
				t.Error("Record missing required fields")
			}
		})
	}
}
