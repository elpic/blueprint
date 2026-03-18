package unit

import (
	"testing"
	"time"

	"github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/platform/testutils"
)

// TestMkdirHandler_GetDependencyKey_Pure tests dependency key generation
// without any I/O operations. Executes in microseconds.
func TestMkdirHandler_GetDependencyKey_Pure(t *testing.T) {
	tests := []struct {
		name      string
		ruleID    string
		mkdirPath string
		expected  string
	}{
		{
			name:      "uses rule ID when present",
			ruleID:    "custom-mkdir-id",
			mkdirPath: "/test/path",
			expected:  "custom-mkdir-id",
		},
		{
			name:      "falls back to mkdir path",
			ruleID:    "",
			mkdirPath: "/home/user/documents",
			expected:  "/home/user/documents",
		},
		{
			name:      "falls back to 'mkdir' when no path",
			ruleID:    "",
			mkdirPath: "",
			expected:  "mkdir",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Build rule with test builder
			ruleBuilder := testutils.NewRule().
				WithAction("mkdir").
				WithMkdir(tt.mkdirPath)

			if tt.ruleID != "" {
				ruleBuilder = ruleBuilder.WithID(tt.ruleID)
			}

			rule := ruleBuilder.Build()

			// Test dependency key generation using legacy constructor for pure logic
			handler := handlers.NewMkdirHandlerLegacy(rule, "/test")
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

// TestMkdirHandler_GetDisplayDetails_Pure tests display information generation.
func TestMkdirHandler_GetDisplayDetails_Pure(t *testing.T) {
	tests := []struct {
		name      string
		mkdirPath string
		expected  string
	}{
		{
			name:      "simple path",
			mkdirPath: "/home/user",
			expected:  "/home/user",
		},
		{
			name:      "nested path",
			mkdirPath: "/home/user/documents/projects",
			expected:  "/home/user/documents/projects",
		},
		{
			name:      "empty path",
			mkdirPath: "",
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			rule := testutils.NewRule().
				WithAction("mkdir").
				WithMkdir(tt.mkdirPath).
				Build()

			handler := handlers.NewMkdirHandlerLegacy(rule, "/test")

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

// TestMkdirHandler_GetState_Pure tests state generation for the "blueprint ps" command.
func TestMkdirHandler_GetState_Pure(t *testing.T) {
	mkdirPath := "/home/user/test"

	rule := testutils.NewRule().
		WithAction("mkdir").
		WithMkdir(mkdirPath).
		Build()

	handler := handlers.NewMkdirHandlerLegacy(rule, "/test")

	start := time.Now()
	state := handler.GetState(false)
	duration := time.Since(start)

	// Verify required keys
	if state["summary"] != mkdirPath {
		t.Errorf("state[summary] = %q, want %q", state["summary"], mkdirPath)
	}
	if state["path"] != mkdirPath {
		t.Errorf("state[path] = %q, want %q", state["path"], mkdirPath)
	}

	if duration > 100*time.Microsecond {
		t.Errorf("Test took %v, expected < 100μs for pure logic test", duration)
	}
}

// TestMkdirHandler_GetCommand_Pure tests command generation - pure function, no I/O.
func TestMkdirHandler_GetCommand_Pure(t *testing.T) {
	tests := []struct {
		name      string
		mkdirPath string
		expected  string
	}{
		{
			name:      "simple path",
			mkdirPath: "/home/user/test",
			expected:  "mkdir -p /home/user/test",
		},
		{
			name:      "path with spaces gets quoted",
			mkdirPath: "/home/user/My Documents",
			expected:  "mkdir -p '/home/user/My Documents'",
		},
		{
			name:      "path with special characters gets quoted",
			mkdirPath: "/home/user/test$dir",
			expected:  "mkdir -p '/home/user/test$dir'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Create rule
			rule := testutils.NewRule().
				WithAction("mkdir").
				WithMkdir(tt.mkdirPath).
				Build()

			// Create handler using legacy constructor for pure logic test
			handler := handlers.NewMkdirHandlerLegacy(rule, "/test/path")

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

// TestMkdirHandler_GetCommandWithPerms_Pure tests command generation with permissions.
func TestMkdirHandler_GetCommandWithPerms_Pure(t *testing.T) {
	tests := []struct {
		name      string
		mkdirPath string
		perms     string
		expected  string
	}{
		{
			name:      "simple path with 755 perms",
			mkdirPath: "/home/user/test",
			perms:     "755",
			expected:  "mkdir -p /home/user/test && chmod 755 /home/user/test",
		},
		{
			name:      "path with spaces and 644 perms",
			mkdirPath: "/home/user/My Documents",
			perms:     "644",
			expected:  "mkdir -p '/home/user/My Documents' && chmod 644 '/home/user/My Documents'",
		},
		{
			name:      "simple path no perms",
			mkdirPath: "/home/user/test",
			perms:     "",
			expected:  "mkdir -p /home/user/test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Create rule with permissions
			ruleBuilder := testutils.NewRule().
				WithAction("mkdir").
				WithMkdir(tt.mkdirPath)

			// Set permissions using direct field access
			rule := ruleBuilder.Build()
			rule.MkdirPerms = tt.perms

			// Create handler
			handler := handlers.NewMkdirHandlerLegacy(rule, "/test/path")

			// Test command generation
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
