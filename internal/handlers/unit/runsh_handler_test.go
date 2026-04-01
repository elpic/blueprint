package unit

import (
	"testing"
	"time"

	"github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/platform/testutils"
)

// TestRunShHandler_GetCommand_Pure tests command generation - pure function, no I/O.
func TestRunShHandler_GetCommand_Pure(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		undo        string
		sudo        bool
		isUninstall bool
		expected    string
	}{
		{
			name:        "simple script URL",
			url:         "https://example.com/install.sh",
			isUninstall: false,
			expected:    "https://example.com/install.sh",
		},
		{
			name:        "GitHub raw script",
			url:         "https://raw.githubusercontent.com/user/repo/main/setup.sh",
			isUninstall: false,
			expected:    "https://raw.githubusercontent.com/user/repo/main/setup.sh",
		},
		{
			name:        "uninstall with undo command",
			url:         "https://example.com/install.sh",
			undo:        "systemctl stop service && rm -f /usr/local/bin/app",
			sudo:        false,
			isUninstall: true,
			expected:    "systemctl stop service && rm -f /usr/local/bin/app",
		},
		{
			name:        "uninstall with sudo undo command",
			url:         "https://example.com/install.sh",
			undo:        "systemctl stop docker",
			sudo:        true,
			isUninstall: true,
			expected:    "sudo systemctl stop docker",
		},
		{
			name:        "uninstall without undo command",
			url:         "https://example.com/install.sh",
			undo:        "",
			sudo:        false,
			isUninstall: true,
			expected:    "# no undo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Create rule using builder
			rule := testutils.NewRule().
				WithRunSh(tt.url).
				Build()

			// Set additional fields
			rule.RunUndo = tt.undo
			rule.RunSudo = tt.sudo
			if tt.isUninstall {
				rule.Action = "uninstall"
			}

			// Create handler
			handler := handlers.NewRunShHandler(rule, "/test/path")

			// Test command generation (pure function - no I/O)
			cmd := handler.GetCommand()

			duration := time.Since(start)

			// Verify command generation
			if cmd != tt.expected {
				t.Errorf("GetCommand() = %q, want %q", cmd, tt.expected)
			}

			// Verify that this is a fast unit test (< 10ms)
			if duration > 10*time.Millisecond {
				t.Errorf("Test took %v, expected < 10ms for pure unit test", duration)
			}
		})
	}
}

// TestRunShHandler_GetDependencyKey_Pure tests dependency key generation
// without any I/O operations. Executes in microseconds.
func TestRunShHandler_GetDependencyKey_Pure(t *testing.T) {
	tests := []struct {
		name     string
		ruleID   string
		url      string
		expected string
	}{
		{
			name:     "uses rule ID when present",
			ruleID:   "custom-runsh-id",
			url:      "https://example.com/script.sh",
			expected: "custom-runsh-id",
		},
		{
			name:     "falls back to URL",
			ruleID:   "",
			url:      "https://get.docker.com",
			expected: "https://get.docker.com",
		},
		{
			name:     "empty URL",
			ruleID:   "",
			url:      "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Build rule with test builder
			ruleBuilder := testutils.NewRule().
				WithRunSh(tt.url)

			if tt.ruleID != "" {
				ruleBuilder = ruleBuilder.WithID(tt.ruleID)
			}

			rule := ruleBuilder.Build()

			// Test dependency key generation
			handler := handlers.NewRunShHandler(rule, "/test")
			key := handler.GetDependencyKey()

			duration := time.Since(start)

			if key != tt.expected {
				t.Errorf("GetDependencyKey() = %q, want %q", key, tt.expected)
			}

			// This should be extremely fast (microseconds)
			if duration > 10*time.Millisecond {
				t.Errorf("Test took %v, expected < 10ms for pure logic test", duration)
			}
		})
	}
}

// TestRunShHandler_GetDisplayDetails_Pure tests display information generation.
func TestRunShHandler_GetDisplayDetails_Pure(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		undo        string
		isUninstall bool
		expected    string
	}{
		{
			name:        "install shows URL",
			url:         "https://example.com/install.sh",
			undo:        "cleanup command",
			isUninstall: false,
			expected:    "https://example.com/install.sh",
		},
		{
			name:        "uninstall shows undo command",
			url:         "https://example.com/install.sh",
			undo:        "rm -rf /opt/app",
			isUninstall: true,
			expected:    "rm -rf /opt/app",
		},
		{
			name:        "uninstall with empty undo",
			url:         "https://example.com/install.sh",
			undo:        "",
			isUninstall: true,
			expected:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			rule := testutils.NewRule().
				WithRunSh(tt.url).
				Build()
			rule.RunUndo = tt.undo

			handler := handlers.NewRunShHandler(rule, "/test")

			details := handler.GetDisplayDetails(tt.isUninstall)
			duration := time.Since(start)

			if details != tt.expected {
				t.Errorf("GetDisplayDetails() = %q, want %q", details, tt.expected)
			}

			if duration > 10*time.Millisecond {
				t.Errorf("Test took %v, expected < 10ms for pure logic test", duration)
			}
		})
	}
}

// TestRunShHandler_GetState_Pure tests state generation for the "blueprint ps" command.
func TestRunShHandler_GetState_Pure(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{
			name: "Docker install script",
			url:  "https://get.docker.com",
		},
		{
			name: "Custom script URL",
			url:  "https://raw.githubusercontent.com/user/repo/main/install.sh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			rule := testutils.NewRule().
				WithRunSh(tt.url).
				Build()

			handler := handlers.NewRunShHandler(rule, "/test")
			state := handler.GetState(false)

			duration := time.Since(start)

			// Verify required keys
			if state["summary"] != tt.url {
				t.Errorf("state[summary] = %q, want %q", state["summary"], tt.url)
			}
			if state["url"] != tt.url {
				t.Errorf("state[url] = %q, want %q", state["url"], tt.url)
			}

			if duration > 10*time.Millisecond {
				t.Errorf("Test took %v, expected < 10ms for pure logic test", duration)
			}
		})
	}
}
