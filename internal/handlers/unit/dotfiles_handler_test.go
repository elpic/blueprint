package unit

import (
	"testing"
	"time"

	"github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/parser"
)

// TestDotfilesHandler_GetCommand_Pure tests command generation - pure function, no I/O.
func TestDotfilesHandler_GetCommand_Pure(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		path     string
		branch   string
		expected string
	}{
		{
			name:     "simple clone without branch",
			url:      "https://github.com/user/dotfiles.git",
			path:     "~/.blueprint/dotfiles/dotfiles",
			branch:   "",
			expected: "git clone https://github.com/user/dotfiles.git ~/.blueprint/dotfiles/dotfiles",
		},
		{
			name:     "clone with specific branch",
			url:      "https://github.com/user/dotfiles.git",
			path:     "~/.config/dotfiles",
			branch:   "main",
			expected: "git clone -b main https://github.com/user/dotfiles.git ~/.config/dotfiles",
		},
		{
			name:     "clone with custom branch",
			url:      "git@github.com:user/dotfiles.git",
			path:     "/opt/dotfiles",
			branch:   "dev",
			expected: "git clone -b dev git@github.com:user/dotfiles.git /opt/dotfiles",
		},
		{
			name:     "GitLab repo with branch",
			url:      "https://gitlab.com/user/mydotfiles.git",
			path:     "~/dotfiles",
			branch:   "personal",
			expected: "git clone -b personal https://gitlab.com/user/mydotfiles.git ~/dotfiles",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Create rule manually
			rule := parser.Rule{
				Action:         "dotfiles",
				DotfilesURL:    tt.url,
				DotfilesPath:   tt.path,
				DotfilesBranch: tt.branch,
			}

			// Create handler
			handler := handlers.NewDotfilesHandler(rule, "/test/path")

			// Test command generation (pure function - no I/O)
			cmd := handler.GetCommand()

			duration := time.Since(start)

			// Verify command generation
			if cmd != tt.expected {
				t.Errorf("GetCommand() = %q, want %q", cmd, tt.expected)
			}

			// Verify that this is a fast unit test (< 200μs to account for string operations)
			if duration > 200*time.Microsecond {
				t.Errorf("Test took %v, expected < 200μs for pure unit test", duration)
			}
		})
	}
}

// TestDotfilesHandler_GetDependencyKey_Pure tests dependency key generation
// without any I/O operations. Executes in microseconds.
func TestDotfilesHandler_GetDependencyKey_Pure(t *testing.T) {
	tests := []struct {
		name     string
		ruleID   string
		url      string
		expected string
	}{
		{
			name:     "uses rule ID when present",
			ruleID:   "custom-dotfiles-id",
			url:      "https://github.com/user/dotfiles.git",
			expected: "custom-dotfiles-id",
		},
		{
			name:     "falls back to URL",
			ruleID:   "",
			url:      "https://github.com/user/mydotfiles.git",
			expected: "https://github.com/user/mydotfiles.git",
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

			// Build rule manually
			rule := parser.Rule{
				ID:          tt.ruleID,
				Action:      "dotfiles",
				DotfilesURL: tt.url,
			}

			// Test dependency key generation
			handler := handlers.NewDotfilesHandler(rule, "/test")
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

// TestDotfilesHandler_GetDisplayDetails_Pure tests display information generation.
func TestDotfilesHandler_GetDisplayDetails_Pure(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "GitHub repo",
			url:      "https://github.com/user/dotfiles.git",
			expected: "https://github.com/user/dotfiles.git",
		},
		{
			name:     "GitLab repo",
			url:      "https://gitlab.com/user/mydotfiles.git",
			expected: "https://gitlab.com/user/mydotfiles.git",
		},
		{
			name:     "SSH URL",
			url:      "git@github.com:user/dotfiles.git",
			expected: "git@github.com:user/dotfiles.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			rule := parser.Rule{
				Action:      "dotfiles",
				DotfilesURL: tt.url,
			}

			handler := handlers.NewDotfilesHandler(rule, "/test")

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

// TestDotfilesHandler_GetState_Pure tests state generation for the "blueprint ps" command.
func TestDotfilesHandler_GetState_Pure(t *testing.T) {
	tests := []struct {
		name string
		url  string
		path string
	}{
		{
			name: "typical dotfiles setup",
			url:  "https://github.com/user/dotfiles.git",
			path: "~/.blueprint/dotfiles/dotfiles",
		},
		{
			name: "custom dotfiles location",
			url:  "git@github.com:user/mydotfiles.git",
			path: "~/dotfiles",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			rule := parser.Rule{
				Action:       "dotfiles",
				DotfilesURL:  tt.url,
				DotfilesPath: tt.path,
			}

			handler := handlers.NewDotfilesHandler(rule, "/test")
			state := handler.GetState(false)

			duration := time.Since(start)

			// Verify required keys
			if state["summary"] != tt.url {
				t.Errorf("state[summary] = %q, want %q", state["summary"], tt.url)
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
