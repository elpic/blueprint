package unit

import (
	"testing"
	"time"

	"github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/parser"
)

// TestDownloadHandler_GetCommand_Pure tests command generation - pure function, no I/O.
func TestDownloadHandler_GetCommand_Pure(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		path        string
		perms       string
		isUninstall bool
		expected    string
	}{
		{
			name:        "simple download without permissions",
			url:         "https://example.com/file.zip",
			path:        "/opt/app/file.zip",
			perms:       "",
			isUninstall: false,
			expected:    "curl -fsSL https://example.com/file.zip -o /opt/app/file.zip",
		},
		{
			name:        "download with permissions",
			url:         "https://github.com/user/repo/releases/download/v1.0/binary",
			path:        "/usr/local/bin/binary",
			perms:       "755",
			isUninstall: false,
			expected:    "curl -fsSL https://github.com/user/repo/releases/download/v1.0/binary -o /usr/local/bin/binary && chmod 755 /usr/local/bin/binary",
		},
		{
			name:        "uninstall removes file",
			url:         "https://example.com/file.zip",
			path:        "/opt/app/file.zip",
			perms:       "",
			isUninstall: true,
			expected:    "rm -f /opt/app/file.zip",
		},
		{
			name:        "download to home directory",
			url:         "https://raw.githubusercontent.com/user/repo/main/config.yaml",
			path:        "~/.config/app/config.yaml",
			perms:       "600",
			isUninstall: false,
			expected:    "curl -fsSL https://raw.githubusercontent.com/user/repo/main/config.yaml -o ~/.config/app/config.yaml && chmod 600 ~/.config/app/config.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Create rule manually since no builder method exists
			rule := parser.Rule{
				Action:        "download",
				DownloadURL:   tt.url,
				DownloadPath:  tt.path,
				DownloadPerms: tt.perms,
			}

			if tt.isUninstall {
				rule.Action = "uninstall"
			}

			// Create handler
			handler := handlers.NewDownloadHandler(rule, "/test/path")

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

// TestDownloadHandler_GetDependencyKey_Pure tests dependency key generation
// without any I/O operations. Executes in microseconds.
func TestDownloadHandler_GetDependencyKey_Pure(t *testing.T) {
	tests := []struct {
		name     string
		ruleID   string
		url      string
		path     string
		expected string
	}{
		{
			name:     "uses rule ID when present",
			ruleID:   "custom-download-id",
			url:      "https://example.com/file.zip",
			path:     "/opt/file.zip",
			expected: "custom-download-id",
		},
		{
			name:     "falls back to download path",
			ruleID:   "",
			url:      "https://example.com/binary",
			path:     "/usr/local/bin/binary",
			expected: "/usr/local/bin/binary",
		},
		{
			name:     "empty download path",
			ruleID:   "",
			url:      "https://example.com/file",
			path:     "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Build rule manually
			rule := parser.Rule{
				ID:           tt.ruleID,
				Action:       "download",
				DownloadURL:  tt.url,
				DownloadPath: tt.path,
			}

			// Test dependency key generation
			handler := handlers.NewDownloadHandler(rule, "/test")
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

// TestDownloadHandler_GetDisplayDetails_Pure tests display information generation.
func TestDownloadHandler_GetDisplayDetails_Pure(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		path     string
		expected string
	}{
		{
			name:     "simple paths",
			url:      "https://example.com/file.zip",
			path:     "/opt/app/file.zip",
			expected: "/opt/app/file.zip",
		},
		{
			name:     "binary download",
			url:      "https://github.com/user/repo/releases/latest/download/binary",
			path:     "/usr/local/bin/binary",
			expected: "/usr/local/bin/binary",
		},
		{
			name:     "config file to home",
			url:      "https://raw.githubusercontent.com/user/dotfiles/main/.vimrc",
			path:     "~/.vimrc",
			expected: "~/.vimrc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			rule := parser.Rule{
				Action:       "download",
				DownloadURL:  tt.url,
				DownloadPath: tt.path,
			}

			handler := handlers.NewDownloadHandler(rule, "/test")

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

// TestDownloadHandler_GetState_Pure tests state generation for the "blueprint ps" command.
func TestDownloadHandler_GetState_Pure(t *testing.T) {
	tests := []struct {
		name string
		url  string
		path string
	}{
		{
			name: "typical binary download",
			url:  "https://github.com/user/repo/releases/download/v1.0/app",
			path: "/usr/local/bin/app",
		},
		{
			name: "config file download",
			url:  "https://raw.githubusercontent.com/user/dotfiles/main/config.yaml",
			path: "~/.config/app/config.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			rule := parser.Rule{
				Action:       "download",
				DownloadURL:  tt.url,
				DownloadPath: tt.path,
			}

			handler := handlers.NewDownloadHandler(rule, "/test")
			state := handler.GetState(false)

			duration := time.Since(start)

			// Verify required keys
			if state["summary"] != tt.path {
				t.Errorf("state[summary] = %q, want %q", state["summary"], tt.path)
			}
			if state["url"] != tt.url {
				t.Errorf("state[url] = %q, want %q", state["url"], tt.url)
			}
			if state["path"] != tt.path {
				t.Errorf("state[path] = %q, want %q", state["path"], tt.path)
			}

			if duration > 10*time.Millisecond {
				t.Errorf("Test took %v, expected < 10ms for pure logic test", duration)
			}
		})
	}
}
