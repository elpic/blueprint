package unit

import (
	"testing"
	"time"

	"github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/platform/testutils"
)

// TestRunHandler_GetCommand_Pure tests command generation - pure function, no I/O.
func TestRunHandler_GetCommand_Pure(t *testing.T) {
	tests := []struct {
		name      string
		command   string
		undo      string
		sudo      bool
		isInstall bool
		expected  string
	}{
		{
			name:      "simple install command",
			command:   "echo 'hello world'",
			sudo:      false,
			isInstall: true,
			expected:  "echo 'hello world'",
		},
		{
			name:      "install command with sudo",
			command:   "systemctl start docker",
			sudo:      true,
			isInstall: true,
			expected:  "sudo systemctl start docker",
		},
		{
			name:      "uninstall command",
			command:   "start service",
			undo:      "stop service",
			sudo:      false,
			isInstall: false,
			expected:  "stop service",
		},
		{
			name:      "uninstall command with sudo",
			command:   "systemctl start docker",
			undo:      "systemctl stop docker",
			sudo:      true,
			isInstall: false,
			expected:  "sudo systemctl stop docker",
		},
		{
			name:      "uninstall without undo command",
			command:   "some command",
			undo:      "",
			sudo:      false,
			isInstall: false,
			expected:  "# no undo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Create rule
			ruleBuilder := testutils.NewRule().
				WithRun(tt.command)

			// Set additional fields using direct field access
			rule := ruleBuilder.Build()
			rule.RunUndo = tt.undo
			rule.RunSudo = tt.sudo
			if !tt.isInstall {
				rule.Action = "uninstall"
			}

			// Create handler using legacy constructor for pure logic test
			handler := handlers.NewRunHandler(rule, "/test/path")

			// Test command generation (pure function - no I/O)
			cmd := handler.GetCommand()

			duration := time.Since(start)

			// Verify command generation
			if cmd != tt.expected {
				t.Errorf("GetCommand() = %q, want %q", cmd, tt.expected)
			}

			// Verify that this is a fast unit test (< 100μs)
			if duration > 100*time.Microsecond {
				t.Errorf("Test took %v, expected < 100μs for pure unit test", duration)
			}
		})
	}
}

// TestRunHandler_GetDependencyKey_Pure tests dependency key generation
// without any I/O operations. Executes in microseconds.
func TestRunHandler_GetDependencyKey_Pure(t *testing.T) {
	tests := []struct {
		name     string
		ruleID   string
		command  string
		expected string
	}{
		{
			name:     "uses rule ID when present",
			ruleID:   "custom-run-id",
			command:  "echo test",
			expected: "custom-run-id",
		},
		{
			name:     "falls back to command",
			ruleID:   "",
			command:  "systemctl start nginx",
			expected: "systemctl start nginx",
		},
		{
			name:     "empty command",
			ruleID:   "",
			command:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Build rule with test builder
			ruleBuilder := testutils.NewRule().
				WithAction("run").
				WithRun(tt.command)

			if tt.ruleID != "" {
				ruleBuilder = ruleBuilder.WithID(tt.ruleID)
			}

			rule := ruleBuilder.Build()

			// Test dependency key generation using legacy constructor
			handler := handlers.NewRunHandler(rule, "/test")
			key := handler.GetDependencyKey()

			duration := time.Since(start)

			if key != tt.expected {
				t.Errorf("GetDependencyKey() = %q, want %q", key, tt.expected)
			}

			// This should be extremely fast (microseconds)
			if duration > 100*time.Microsecond {
				t.Errorf("Test took %v, expected < 100μs for pure logic test", duration)
			}
		})
	}
}

// TestRunHandler_GetDisplayDetails_Pure tests display information generation.
func TestRunHandler_GetDisplayDetails_Pure(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		undo        string
		isUninstall bool
		expected    string
	}{
		{
			name:        "short command install",
			command:     "echo hello",
			isUninstall: false,
			expected:    "echo hello",
		},
		{
			name:        "short command uninstall",
			command:     "start service",
			undo:        "stop service",
			isUninstall: true,
			expected:    "stop service",
		},
		{
			name:        "long command gets truncated",
			command:     "this is a very long command that should be truncated because it exceeds 60 characters",
			isUninstall: false,
			expected:    "this is a very long command that should be truncated because...",
		},
		{
			name:        "empty undo command",
			command:     "start service",
			undo:        "",
			isUninstall: true,
			expected:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			rule := testutils.NewRule().
				WithAction("run").
				WithRun(tt.command).
				Build()
			rule.RunUndo = tt.undo

			handler := handlers.NewRunHandler(rule, "/test")

			details := handler.GetDisplayDetails(tt.isUninstall)
			duration := time.Since(start)

			if details != tt.expected {
				t.Errorf("GetDisplayDetails() = %q, want %q", details, tt.expected)
			}

			if duration > 100*time.Microsecond {
				t.Errorf("Test took %v, expected < 100μs for pure logic test", duration)
			}
		})
	}
}

// TestRunHandler_GetState_Pure tests state generation for the "blueprint ps" command.
func TestRunHandler_GetState_Pure(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected string
	}{
		{
			name:     "short command",
			command:  "echo test",
			expected: "echo test",
		},
		{
			name:     "long command gets truncated",
			command:  "this is a very long command that should be truncated because it exceeds the 60 character limit",
			expected: "this is a very long command that should be truncated because...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			rule := testutils.NewRule().
				WithAction("run").
				WithRun(tt.command).
				Build()

			handler := handlers.NewRunHandler(rule, "/test")
			state := handler.GetState(false)

			duration := time.Since(start)

			// Verify required keys
			if state["summary"] != tt.expected {
				t.Errorf("state[summary] = %q, want %q", state["summary"], tt.expected)
			}

			if duration > 100*time.Microsecond {
				t.Errorf("Test took %v, expected < 100μs for pure logic test", duration)
			}
		})
	}
}
