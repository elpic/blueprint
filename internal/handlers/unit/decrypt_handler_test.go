package unit

import (
	"testing"
	"time"

	"github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/platform/testutils"
)

// TestDecryptHandler_GetCommand_Pure tests command generation - pure function, no I/O.
func TestDecryptHandler_GetCommand_Pure(t *testing.T) {
	tests := []struct {
		name        string
		sourceFile  string
		destPath    string
		isUninstall bool
		expected    string
	}{
		{
			name:        "decrypt command shows file transfer",
			sourceFile:  "secrets.enc",
			destPath:    "/etc/app/config.yaml",
			isUninstall: false,
			expected:    "Decrypt file: secrets.enc → /etc/app/config.yaml",
		},
		{
			name:        "uninstall command shows rm",
			sourceFile:  "secrets.enc",
			destPath:    "/etc/app/config.yaml",
			isUninstall: true,
			expected:    "rm -f /etc/app/config.yaml",
		},
		{
			name:        "decrypt with relative paths",
			sourceFile:  "./data/secret.gpg",
			destPath:    "~/.ssh/key",
			isUninstall: false,
			expected:    "Decrypt file: ./data/secret.gpg → ~/.ssh/key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Create rule
			rule := testutils.NewRule().
				WithDecrypt(tt.sourceFile, tt.destPath).
				Build()

			if tt.isUninstall {
				rule.Action = "uninstall"
			}

			// Create handler (password cache not needed for pure function tests)
			handler := handlers.NewDecryptHandler(rule, "/test/path", make(map[string]string))

			// Test command generation (pure function - no I/O)
			cmd := handler.GetCommand()

			duration := time.Since(start)

			// Verify command generation
			if cmd != tt.expected {
				t.Errorf("GetCommand() = %q, want %q", cmd, tt.expected)
			}

			// Verify that this is a fast unit test (< 1ms — generous bound for CI)
			if duration > 1*time.Millisecond {
				t.Errorf("Test took %v, expected < 1ms for pure unit test", duration)
			}
		})
	}
}

// TestDecryptHandler_GetDependencyKey_Pure tests dependency key generation
// without any I/O operations. Executes in microseconds.
func TestDecryptHandler_GetDependencyKey_Pure(t *testing.T) {
	tests := []struct {
		name       string
		ruleID     string
		sourceFile string
		destPath   string
		expected   string
	}{
		{
			name:       "uses rule ID when present",
			ruleID:     "custom-decrypt-id",
			sourceFile: "secret.enc",
			destPath:   "/etc/config",
			expected:   "custom-decrypt-id",
		},
		{
			name:       "falls back to destination path",
			ruleID:     "",
			sourceFile: "secret.enc",
			destPath:   "/etc/app/config.yaml",
			expected:   "/etc/app/config.yaml",
		},
		{
			name:       "empty destination path",
			ruleID:     "",
			sourceFile: "secret.enc",
			destPath:   "",
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Build rule with test builder
			ruleBuilder := testutils.NewRule().
				WithDecrypt(tt.sourceFile, tt.destPath)

			if tt.ruleID != "" {
				ruleBuilder = ruleBuilder.WithID(tt.ruleID)
			}

			rule := ruleBuilder.Build()

			// Test dependency key generation
			handler := handlers.NewDecryptHandler(rule, "/test", make(map[string]string))
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

// TestDecryptHandler_GetDisplayDetails_Pure tests display information generation.
func TestDecryptHandler_GetDisplayDetails_Pure(t *testing.T) {
	tests := []struct {
		name       string
		sourceFile string
		destPath   string
		expected   string
	}{
		{
			name:       "simple paths",
			sourceFile: "secret.enc",
			destPath:   "/etc/config.yaml",
			expected:   "/etc/config.yaml",
		},
		{
			name:       "relative paths",
			sourceFile: "./secrets/key.gpg",
			destPath:   "~/.ssh/private_key",
			expected:   "~/.ssh/private_key",
		},
		{
			name:       "empty destination",
			sourceFile: "secret.enc",
			destPath:   "",
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			rule := testutils.NewRule().
				WithDecrypt(tt.sourceFile, tt.destPath).
				Build()

			handler := handlers.NewDecryptHandler(rule, "/test", make(map[string]string))

			details := handler.GetDisplayDetails(false)
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

// TestDecryptHandler_GetState_Pure tests state generation for the "blueprint ps" command.
func TestDecryptHandler_GetState_Pure(t *testing.T) {
	tests := []struct {
		name       string
		sourceFile string
		destPath   string
	}{
		{
			name:       "typical decrypt operation",
			sourceFile: "secrets/app.enc",
			destPath:   "/etc/app/config.yaml",
		},
		{
			name:       "ssh key decryption",
			sourceFile: "./ssh/private.gpg",
			destPath:   "~/.ssh/id_rsa",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			rule := testutils.NewRule().
				WithDecrypt(tt.sourceFile, tt.destPath).
				Build()

			handler := handlers.NewDecryptHandler(rule, "/test", make(map[string]string))
			state := handler.GetState(false)

			duration := time.Since(start)

			// Verify required keys
			if state["summary"] != tt.destPath {
				t.Errorf("state[summary] = %q, want %q", state["summary"], tt.destPath)
			}
			if state["source"] != tt.sourceFile {
				t.Errorf("state[source] = %q, want %q", state["source"], tt.sourceFile)
			}
			if state["dest"] != tt.destPath {
				t.Errorf("state[dest] = %q, want %q", state["dest"], tt.destPath)
			}

			if duration > 10*time.Millisecond {
				t.Errorf("Test took %v, expected < 10ms for pure logic test", duration)
			}
		})
	}
}
