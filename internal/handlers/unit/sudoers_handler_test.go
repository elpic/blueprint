package unit

import (
	"testing"
	"time"

	"github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/parser"
)

// TestSudoersHandler_GetCommand_Pure tests command generation - pure function, no I/O.
func TestSudoersHandler_GetCommand_Pure(t *testing.T) {
	tests := []struct {
		name        string
		user        string
		isUninstall bool
		expected    string
	}{
		{
			name:        "install with specific user",
			user:        "johnsmith",
			isUninstall: false,
			expected:    "sudo install -m 0440 <entry> /etc/sudoers.d/johnsmith",
		},
		{
			name:        "install with default user",
			user:        "",
			isUninstall: false,
			expected:    "sudo install -m 0440 <entry> /etc/sudoers.d/$USER",
		},
		{
			name:        "uninstall specific user",
			user:        "alice",
			isUninstall: true,
			expected:    "sudo rm -f /etc/sudoers.d/alice",
		},
		{
			name:        "uninstall default user",
			user:        "",
			isUninstall: true,
			expected:    "sudo rm -f /etc/sudoers.d/$USER",
		},
		{
			name:        "install with service account user",
			user:        "deploy-user",
			isUninstall: false,
			expected:    "sudo install -m 0440 <entry> /etc/sudoers.d/deploy-user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Create rule manually
			rule := parser.Rule{
				Action:      "sudoers",
				SudoersUser: tt.user,
			}

			if tt.isUninstall {
				rule.Action = "uninstall"
			}

			// Create handler
			handler := handlers.NewSudoersHandler(rule, "/test/path")

			// Test command generation (pure function - no I/O)
			cmd := handler.GetCommand()

			duration := time.Since(start)

			// Verify command generation
			if cmd != tt.expected {
				t.Errorf("GetCommand() = %q, want %q", cmd, tt.expected)
			}

			// Verify that this is a fast unit test (< 200μs)
			if duration > 200*time.Microsecond {
				t.Errorf("Test took %v, expected < 200μs for pure unit test", duration)
			}
		})
	}
}

// TestSudoersHandler_NeedsSudo_Pure tests sudo requirement - always returns true.
func TestSudoersHandler_NeedsSudo_Pure(t *testing.T) {
	rule := parser.Rule{
		Action:      "sudoers",
		SudoersUser: "testuser",
	}

	handler := handlers.NewSudoersHandler(rule, "/test")

	start := time.Now()
	needsSudo := handler.NeedsSudo()
	duration := time.Since(start)

	// Sudoers handler should always need sudo
	if !needsSudo {
		t.Errorf("NeedsSudo() = false, want true (sudoers operations always need sudo)")
	}

	if duration > 10*time.Microsecond {
		t.Errorf("Test took %v, expected < 10μs for trivial function", duration)
	}
}

// TestSudoersHandler_GetDependencyKey_Pure tests dependency key generation
// without any I/O operations. Executes in microseconds.
func TestSudoersHandler_GetDependencyKey_Pure(t *testing.T) {
	tests := []struct {
		name        string
		ruleID      string
		user        string
		isUninstall bool
		expected    string
	}{
		{
			name:        "uses rule ID when present for install",
			ruleID:      "custom-sudoers-id",
			user:        "testuser",
			isUninstall: false,
			expected:    "custom-sudoers-id",
		},
		{
			name:        "falls back to 'sudoers' for install",
			ruleID:      "",
			user:        "testuser",
			isUninstall: false,
			expected:    "sudoers",
		},
		{
			name:        "uses rule ID when present for uninstall",
			ruleID:      "custom-id",
			user:        "testuser",
			isUninstall: true,
			expected:    "custom-id",
		},
		{
			name:        "falls back to 'uninstall-sudoers' for uninstall",
			ruleID:      "",
			user:        "testuser",
			isUninstall: true,
			expected:    "uninstall-sudoers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Build rule manually
			rule := parser.Rule{
				ID:          tt.ruleID,
				Action:      "sudoers",
				SudoersUser: tt.user,
			}

			if tt.isUninstall {
				rule.Action = "uninstall"
			}

			// Test dependency key generation
			handler := handlers.NewSudoersHandler(rule, "/test")
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

// TestSudoersHandler_GetDisplayDetails_Pure tests display information generation.
func TestSudoersHandler_GetDisplayDetails_Pure(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		expected string
	}{
		{
			name:     "specific user",
			user:     "johnsmith",
			expected: "/etc/sudoers.d/johnsmith",
		},
		{
			name:     "default user placeholder",
			user:     "",
			expected: "/etc/sudoers.d/$USER",
		},
		{
			name:     "service account user",
			user:     "deploy-bot",
			expected: "/etc/sudoers.d/deploy-bot",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			rule := parser.Rule{
				Action:      "sudoers",
				SudoersUser: tt.user,
			}

			handler := handlers.NewSudoersHandler(rule, "/test")

			details := handler.GetDisplayDetails(false)
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

// TestSudoersHandler_GetState_Pure tests state generation for the "blueprint ps" command.
func TestSudoersHandler_GetState_Pure(t *testing.T) {
	tests := []struct {
		name         string
		user         string
		expectedUser string
		expectedPath string
	}{
		{
			name:         "specific user",
			user:         "alice",
			expectedUser: "alice",
			expectedPath: "/etc/sudoers.d/alice",
		},
		{
			name:         "default user",
			user:         "",
			expectedUser: "$USER",
			expectedPath: "/etc/sudoers.d/$USER",
		},
		{
			name:         "system user",
			user:         "root",
			expectedUser: "root",
			expectedPath: "/etc/sudoers.d/root",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			rule := parser.Rule{
				Action:      "sudoers",
				SudoersUser: tt.user,
			}

			handler := handlers.NewSudoersHandler(rule, "/test")
			state := handler.GetState(false)

			duration := time.Since(start)

			// Verify required keys
			if state["summary"] != tt.expectedPath {
				t.Errorf("state[summary] = %q, want %q", state["summary"], tt.expectedPath)
			}
			if state["user"] != tt.expectedUser {
				t.Errorf("state[user] = %q, want %q", state["user"], tt.expectedUser)
			}

			if duration > 100*time.Microsecond {
				t.Errorf("Test took %v, expected < 100μs for pure logic test", duration)
			}
		})
	}
}
