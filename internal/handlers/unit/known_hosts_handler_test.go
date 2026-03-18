package unit

import (
	"testing"
	"time"

	"github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/parser"
)

// TestKnownHostsHandler_GetCommand_Pure tests command generation - pure function, no I/O.
func TestKnownHostsHandler_GetCommand_Pure(t *testing.T) {
	tests := []struct {
		name        string
		host        string
		keyType     string
		isUninstall bool
		expectedCmd string
	}{
		{
			name:        "add GitHub with default ed25519 key",
			host:        "github.com",
			keyType:     "",
			isUninstall: false,
			expectedCmd: "ssh-keyscan -t ed25519 github.com",
		},
		{
			name:        "add host with specific rsa key type",
			host:        "gitlab.com",
			keyType:     "rsa",
			isUninstall: false,
			expectedCmd: "ssh-keyscan -t rsa gitlab.com",
		},
		{
			name:        "add host with ecdsa key",
			host:        "bitbucket.org",
			keyType:     "ecdsa",
			isUninstall: false,
			expectedCmd: "ssh-keyscan -t ecdsa bitbucket.org",
		},
		{
			name:        "remove host from known_hosts",
			host:        "github.com",
			keyType:     "",
			isUninstall: true,
			expectedCmd: "sed -i.bak '/^github\\.com[, ]/d' ~/.ssh/known_hosts && rm -f ~/.ssh/known_hosts.bak",
		},
		{
			name:        "remove host with special chars",
			host:        "my-server.example.com",
			keyType:     "",
			isUninstall: true,
			expectedCmd: "sed -i.bak '/^my-server\\.example\\.com[, ]/d' ~/.ssh/known_hosts && rm -f ~/.ssh/known_hosts.bak",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Create rule manually
			rule := parser.Rule{
				Action:        "known_hosts",
				KnownHosts:    tt.host,
				KnownHostsKey: tt.keyType,
			}

			if tt.isUninstall {
				rule.Action = "uninstall"
			}

			// Create handler
			handler := handlers.NewKnownHostsHandler(rule, "/test/path")

			// Test command generation (pure function - no I/O)
			cmd := handler.GetCommand()

			duration := time.Since(start)

			// Verify command generation
			if cmd != tt.expectedCmd {
				t.Errorf("GetCommand() = %q, want %q", cmd, tt.expectedCmd)
			}

			// Verify that this is a fast unit test (< 200μs)
			if duration > 200*time.Microsecond {
				t.Errorf("Test took %v, expected < 200μs for pure unit test", duration)
			}
		})
	}
}

// TestKnownHostsHandler_GetDependencyKey_Pure tests dependency key generation
// without any I/O operations. Executes in microseconds.
func TestKnownHostsHandler_GetDependencyKey_Pure(t *testing.T) {
	tests := []struct {
		name     string
		ruleID   string
		host     string
		expected string
	}{
		{
			name:     "uses rule ID when present",
			ruleID:   "custom-known-hosts-id",
			host:     "github.com",
			expected: "custom-known-hosts-id",
		},
		{
			name:     "falls back to hostname",
			ruleID:   "",
			host:     "gitlab.com",
			expected: "gitlab.com",
		},
		{
			name:     "empty hostname",
			ruleID:   "",
			host:     "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Build rule manually
			rule := parser.Rule{
				ID:         tt.ruleID,
				Action:     "known_hosts",
				KnownHosts: tt.host,
			}

			// Test dependency key generation
			handler := handlers.NewKnownHostsHandler(rule, "/test")
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

// TestKnownHostsHandler_GetDisplayDetails_Pure tests display information generation.
func TestKnownHostsHandler_GetDisplayDetails_Pure(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		expected string
	}{
		{
			name:     "GitHub hostname",
			host:     "github.com",
			expected: "github.com",
		},
		{
			name:     "GitLab hostname",
			host:     "gitlab.com",
			expected: "gitlab.com",
		},
		{
			name:     "custom server",
			host:     "my-server.example.com",
			expected: "my-server.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			rule := parser.Rule{
				Action:     "known_hosts",
				KnownHosts: tt.host,
			}

			handler := handlers.NewKnownHostsHandler(rule, "/test")

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

// TestKnownHostsHandler_GetState_Pure tests state generation for the "blueprint ps" command.
func TestKnownHostsHandler_GetState_Pure(t *testing.T) {
	tests := []struct {
		name string
		host string
	}{
		{
			name: "GitHub known host state",
			host: "github.com",
		},
		{
			name: "custom server state",
			host: "my-build-server.company.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			rule := parser.Rule{
				Action:     "known_hosts",
				KnownHosts: tt.host,
			}

			handler := handlers.NewKnownHostsHandler(rule, "/test")
			state := handler.GetState(false)

			duration := time.Since(start)

			// Verify required keys
			if state["summary"] != tt.host {
				t.Errorf("state[summary] = %q, want %q", state["summary"], tt.host)
			}
			if state["host"] != tt.host {
				t.Errorf("state[host] = %q, want %q", state["host"], tt.host)
			}

			if duration > 100*time.Microsecond {
				t.Errorf("Test took %v, expected < 100μs for pure logic test", duration)
			}
		})
	}
}

// TestEscapeForSed_Pure tests the sed escaping helper function indirectly through GetCommand.
func TestEscapeForSed_Pure(t *testing.T) {
	tests := []struct {
		name         string
		host         string
		expectedPart string // part of the sed command that should contain the escaped host
	}{
		{
			name:         "simple hostname",
			host:         "example.com",
			expectedPart: "example\\.com",
		},
		{
			name:         "hostname with multiple dots",
			host:         "sub.example.com",
			expectedPart: "sub\\.example\\.com",
		},
		{
			name:         "hostname with hyphen (no escaping needed)",
			host:         "my-server.com",
			expectedPart: "my-server\\.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			rule := parser.Rule{
				Action:     "uninstall", // uninstall uses sed escaping
				KnownHosts: tt.host,
			}

			handler := handlers.NewKnownHostsHandler(rule, "/test")
			cmd := handler.GetCommand()

			duration := time.Since(start)

			// Verify the escaped hostname appears in the sed command
			if !containsStringKH(cmd, tt.expectedPart) {
				t.Errorf("GetCommand() = %q should contain escaped host %q", cmd, tt.expectedPart)
			}

			if duration > 100*time.Microsecond {
				t.Errorf("Test took %v, expected < 100μs for pure logic test", duration)
			}
		})
	}
}

// TestIsValidHostname_Pure tests hostname validation indirectly by checking error handling.
// We can't call isValidHostname directly since it's not exported, but we can test the validation
// behavior through the handler methods.
func TestHostnameValidation_Behavior(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		valid    bool
	}{
		{
			name:     "valid simple hostname",
			hostname: "example.com",
			valid:    true,
		},
		{
			name:     "valid hostname with subdomain",
			hostname: "api.example.com",
			valid:    true,
		},
		{
			name:     "valid hostname with hyphen",
			hostname: "my-server.com",
			valid:    true,
		},
		{
			name:     "valid hostname with underscore",
			hostname: "my_server.com",
			valid:    true,
		},
		{
			name:     "valid IP address",
			hostname: "192.168.1.100",
			valid:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			rule := parser.Rule{
				Action:     "known_hosts",
				KnownHosts: tt.hostname,
			}

			handler := handlers.NewKnownHostsHandler(rule, "/test")

			// Test that the handler can generate commands for valid hostnames
			// (this tests the validation indirectly)
			cmd := handler.GetCommand()

			duration := time.Since(start)

			if tt.valid {
				// For valid hostnames, command should contain the hostname
				if !containsStringKH(cmd, tt.hostname) {
					t.Errorf("GetCommand() = %q should contain hostname %q", cmd, tt.hostname)
				}
			}

			if duration > 100*time.Microsecond {
				t.Errorf("Test took %v, expected < 100μs for pure logic test", duration)
			}
		})
	}
}

// Helper function to check if a string contains another string.
func containsStringKH(haystack, needle string) bool {
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
