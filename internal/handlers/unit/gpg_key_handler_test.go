package unit

import (
	"testing"
	"time"

	"github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/parser"
)

// TestGPGKeyHandler_GetCommand_Pure tests command generation - pure function, no I/O.
func TestGPGKeyHandler_GetCommand_Pure(t *testing.T) {
	tests := []struct {
		name        string
		keyURL      string
		keyring     string
		debURL      string
		isUninstall bool
		expected    string
	}{
		{
			name:        "install GPG key and repository",
			keyURL:      "https://packages.microsoft.com/keys/microsoft.asc",
			keyring:     "packages-microsoft-com",
			debURL:      "https://packages.microsoft.com/repos/code",
			isUninstall: false,
			expected:    "curl -fsSL https://packages.microsoft.com/keys/microsoft.asc | sudo tee /etc/apt/keyrings/packages-microsoft-com.asc && echo 'deb [signed-by=/etc/apt/keyrings/packages-microsoft-com.asc] https://packages.microsoft.com/repos/code * *' | sudo tee /etc/apt/sources.list.d/packages-microsoft-com.list && sudo apt-get update",
		},
		{
			name:        "uninstall removes keyring and sources",
			keyURL:      "https://download.docker.com/linux/ubuntu/gpg",
			keyring:     "docker",
			debURL:      "https://download.docker.com/linux/ubuntu",
			isUninstall: true,
			expected:    "sudo rm -f /etc/apt/sources.list.d/docker.list && sudo rm -f /etc/apt/keyrings/docker.asc && sudo apt-get update",
		},
		{
			name:        "GitHub CLI GPG key",
			keyURL:      "https://cli.github.com/packages/githubcli-archive-keyring.gpg",
			keyring:     "githubcli-archive-keyring",
			debURL:      "https://cli.github.com/packages",
			isUninstall: false,
			expected:    "curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo tee /etc/apt/keyrings/githubcli-archive-keyring.asc && echo 'deb [signed-by=/etc/apt/keyrings/githubcli-archive-keyring.asc] https://cli.github.com/packages * *' | sudo tee /etc/apt/sources.list.d/githubcli-archive-keyring.list && sudo apt-get update",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Create rule manually
			rule := parser.Rule{
				Action:     "gpg_key",
				GPGKeyURL:  tt.keyURL,
				GPGKeyring: tt.keyring,
				GPGDebURL:  tt.debURL,
			}

			if tt.isUninstall {
				rule.Action = "uninstall"
			}

			// Create handler
			handler := handlers.NewGPGKeyHandler(rule, "/test/path")

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

// TestGPGKeyHandler_PathGeneration_Pure tests the pure path generation functions.
func TestGPGKeyHandler_PathGeneration_Pure(t *testing.T) {
	tests := []struct {
		name            string
		keyring         string
		expectedKeyPath string
		expectedSrcPath string
	}{
		{
			name:            "docker keyring",
			keyring:         "docker",
			expectedKeyPath: "/etc/apt/keyrings/docker.asc",
			expectedSrcPath: "/etc/apt/sources.list.d/docker.list",
		},
		{
			name:            "microsoft packages",
			keyring:         "packages-microsoft-com",
			expectedKeyPath: "/etc/apt/keyrings/packages-microsoft-com.asc",
			expectedSrcPath: "/etc/apt/sources.list.d/packages-microsoft-com.list",
		},
		{
			name:            "simple name",
			keyring:         "test",
			expectedKeyPath: "/etc/apt/keyrings/test.asc",
			expectedSrcPath: "/etc/apt/sources.list.d/test.list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			rule := parser.Rule{
				Action:     "gpg_key",
				GPGKeyring: tt.keyring,
			}

			handler := handlers.NewGPGKeyHandler(rule, "/test")

			// We can't directly test the private methods, but we can verify they work
			// correctly by checking the GetCommand output which uses them
			cmd := handler.GetCommand()

			duration := time.Since(start)

			// Verify the expected paths appear in the command
			if !containsString(cmd, tt.expectedKeyPath) {
				t.Errorf("GetCommand() should contain keyring path %q, but got: %q", tt.expectedKeyPath, cmd)
			}
			if !containsString(cmd, tt.expectedSrcPath) {
				t.Errorf("GetCommand() should contain sources path %q, but got: %q", tt.expectedSrcPath, cmd)
			}

			if duration > 10*time.Millisecond {
				t.Errorf("Test took %v, expected < 10ms for pure logic test", duration)
			}
		})
	}
}

// TestGPGKeyHandler_GetDependencyKey_Pure tests dependency key generation
// without any I/O operations. Executes in microseconds.
func TestGPGKeyHandler_GetDependencyKey_Pure(t *testing.T) {
	tests := []struct {
		name     string
		ruleID   string
		keyring  string
		expected string
	}{
		{
			name:     "uses rule ID when present",
			ruleID:   "custom-gpg-id",
			keyring:  "docker",
			expected: "custom-gpg-id",
		},
		{
			name:     "falls back to keyring name",
			ruleID:   "",
			keyring:  "packages-microsoft-com",
			expected: "packages-microsoft-com",
		},
		{
			name:     "empty keyring",
			ruleID:   "",
			keyring:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Build rule manually
			rule := parser.Rule{
				ID:         tt.ruleID,
				Action:     "gpg_key",
				GPGKeyring: tt.keyring,
			}

			// Test dependency key generation
			handler := handlers.NewGPGKeyHandler(rule, "/test")
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

// TestGPGKeyHandler_GetDisplayDetails_Pure tests display information generation.
func TestGPGKeyHandler_GetDisplayDetails_Pure(t *testing.T) {
	tests := []struct {
		name     string
		keyring  string
		expected string
	}{
		{
			name:     "docker keyring",
			keyring:  "docker",
			expected: "docker",
		},
		{
			name:     "microsoft packages",
			keyring:  "packages-microsoft-com",
			expected: "packages-microsoft-com",
		},
		{
			name:     "empty keyring",
			keyring:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			rule := parser.Rule{
				Action:     "gpg_key",
				GPGKeyring: tt.keyring,
			}

			handler := handlers.NewGPGKeyHandler(rule, "/test")

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

// TestGPGKeyHandler_GetState_Pure tests state generation for the "blueprint ps" command.
func TestGPGKeyHandler_GetState_Pure(t *testing.T) {
	tests := []struct {
		name    string
		keyring string
	}{
		{
			name:    "docker keyring state",
			keyring: "docker",
		},
		{
			name:    "github CLI keyring state",
			keyring: "githubcli-archive-keyring",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			rule := parser.Rule{
				Action:     "gpg_key",
				GPGKeyring: tt.keyring,
			}

			handler := handlers.NewGPGKeyHandler(rule, "/test")
			state := handler.GetState(false)

			duration := time.Since(start)

			// Verify required keys
			if state["summary"] != tt.keyring {
				t.Errorf("state[summary] = %q, want %q", state["summary"], tt.keyring)
			}
			if state["keyring"] != tt.keyring {
				t.Errorf("state[keyring] = %q, want %q", state["keyring"], tt.keyring)
			}

			if duration > 10*time.Millisecond {
				t.Errorf("Test took %v, expected < 10ms for pure logic test", duration)
			}
		})
	}
}

// Helper function to check if a string contains another string.
func containsString(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
