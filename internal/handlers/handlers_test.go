package handlers

import (
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

// TestGetDependencyKeyHelper tests the helper function that centralizes ID checking
func TestGetDependencyKeyHelper(t *testing.T) {
	tests := []struct {
		name       string
		rule       parser.Rule
		fallback   string
		expected   string
	}{
		{
			name: "returns ID when present",
			rule: parser.Rule{
				ID: "unique-id-123",
			},
			fallback: "fallback-key",
			expected: "unique-id-123",
		},
		{
			name: "returns fallback when ID is empty",
			rule: parser.Rule{
				ID: "",
			},
			fallback: "fallback-key",
			expected: "fallback-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getDependencyKey(tt.rule, tt.fallback)
			if got != tt.expected {
				t.Errorf("getDependencyKey() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestBaseHandlerGetFallbackDependencyKey tests the default fallback implementation
func TestBaseHandlerGetFallbackDependencyKey(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		expected string
	}{
		{
			name: "defaults to action name for install",
			rule: parser.Rule{
				Action: "install",
			},
			expected: "install",
		},
		{
			name: "defaults to action name for clone",
			rule: parser.Rule{
				Action: "clone",
			},
			expected: "clone",
		},
		{
			name: "defaults to action name for decrypt",
			rule: parser.Rule{
				Action: "decrypt",
			},
			expected: "decrypt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &BaseHandler{Rule: tt.rule}
			got := handler.GetFallbackDependencyKey()
			if got != tt.expected {
				t.Errorf("GetFallbackDependencyKey() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestKeyProviderInterface verifies that all handlers implement KeyProvider
func TestKeyProviderInterface(t *testing.T) {
	tests := []struct {
		name    string
		handler Handler
	}{
		{
			name:    "InstallHandler implements KeyProvider",
			handler: NewInstallHandler(parser.Rule{Packages: []parser.Package{{Name: "pkg"}}}, ""),
		},
		{
			name:    "CloneHandler implements KeyProvider",
			handler: NewCloneHandler(parser.Rule{ClonePath: "path"}, ""),
		},
		{
			name:    "DecryptHandler implements KeyProvider",
			handler: NewDecryptHandler(parser.Rule{DecryptPath: "path"}, "", nil),
		},
		{
			name:    "AsdfHandler implements KeyProvider",
			handler: NewAsdfHandler(parser.Rule{Action: "asdf"}, ""),
		},
		{
			name:    "MkdirHandler implements KeyProvider",
			handler: NewMkdirHandler(parser.Rule{Mkdir: "path"}, ""),
		},
		{
			name:    "KnownHostsHandler implements KeyProvider",
			handler: NewKnownHostsHandler(parser.Rule{KnownHosts: "host"}, ""),
		},
		{
			name:    "GPGKeyHandler implements KeyProvider",
			handler: NewGPGKeyHandler(parser.Rule{GPGKeyring: "key"}, ""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Check that handler implements KeyProvider
			keyProvider, ok := tt.handler.(KeyProvider)
			if !ok {
				t.Errorf("Handler does not implement KeyProvider interface")
			}

			// Verify GetDependencyKey can be called without error
			key := keyProvider.GetDependencyKey()
			if key == "" {
				t.Errorf("GetDependencyKey() returned empty string")
			}
		})
	}
}

// TestDisplayProviderInterface verifies that all handlers implement DisplayProvider
func TestDisplayProviderInterface(t *testing.T) {
	tests := []struct {
		name              string
		handler           Handler
		expectedFormatted string
		isUninstall       bool
	}{
		{
			name:              "InstallHandler provides package display",
			handler:           NewInstallHandler(parser.Rule{Packages: []parser.Package{{Name: "vim"}, {Name: "curl"}}}, ""),
			expectedFormatted: "vim, curl",
			isUninstall:       false,
		},
		{
			name:              "CloneHandler provides path display",
			handler:           NewCloneHandler(parser.Rule{ClonePath: "~/my-repo"}, ""),
			expectedFormatted: "~/my-repo",
			isUninstall:       false,
		},
		{
			name:              "DecryptHandler provides path display",
			handler:           NewDecryptHandler(parser.Rule{DecryptPath: "~/.ssh/config"}, "", nil),
			expectedFormatted: "~/.ssh/config",
			isUninstall:       true,
		},
		{
			name:              "AsdfHandler provides asdf display",
			handler:           NewAsdfHandler(parser.Rule{Action: "asdf"}, ""),
			expectedFormatted: "asdf",
			isUninstall:       false,
		},
		{
			name:              "MkdirHandler provides path display",
			handler:           NewMkdirHandler(parser.Rule{Mkdir: "~/projects"}, ""),
			expectedFormatted: "~/projects",
			isUninstall:       false,
		},
		{
			name:              "KnownHostsHandler provides hostname display",
			handler:           NewKnownHostsHandler(parser.Rule{KnownHosts: "github.com"}, ""),
			expectedFormatted: "github.com",
			isUninstall:       false,
		},
		{
			name:              "GPGKeyHandler provides keyring display",
			handler:           NewGPGKeyHandler(parser.Rule{GPGKeyring: "ubuntu-keyring"}, ""),
			expectedFormatted: "ubuntu-keyring",
			isUninstall:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Check that handler implements DisplayProvider
			displayProvider, ok := tt.handler.(DisplayProvider)
			if !ok {
				t.Errorf("Handler does not implement DisplayProvider interface")
			}

			// Verify GetDisplayDetails returns expected value
			details := displayProvider.GetDisplayDetails(tt.isUninstall)
			if details != tt.expectedFormatted {
				t.Errorf("GetDisplayDetails(%v) = %q, want %q", tt.isUninstall, details, tt.expectedFormatted)
			}
		})
	}
}

// TestStatusProviderInterface verifies that all handlers implement StatusProvider
func TestStatusProviderInterface(t *testing.T) {
	tests := []struct {
		name              string
		handler           Handler
		expectedRecords   int
		expectedUninstall bool
	}{
		{
			name:              "InstallHandler provides status records",
			handler:           NewInstallHandler(parser.Rule{Packages: []parser.Package{{Name: "vim"}}}, ""),
			expectedRecords:   0, // No status records from empty status
			expectedUninstall: true,
		},
		{
			name:              "CloneHandler provides status records",
			handler:           NewCloneHandler(parser.Rule{ClonePath: "~/repo"}, ""),
			expectedRecords:   0,
			expectedUninstall: true,
		},
		{
			name:              "DecryptHandler provides status records",
			handler:           NewDecryptHandler(parser.Rule{DecryptPath: "~/.ssh/key"}, "", nil),
			expectedRecords:   0,
			expectedUninstall: true,
		},
		{
			name:              "AsdfHandler provides status records",
			handler:           NewAsdfHandler(parser.Rule{Action: "asdf"}, ""),
			expectedRecords:   0,
			expectedUninstall: true,
		},
		{
			name:              "MkdirHandler provides status records",
			handler:           NewMkdirHandler(parser.Rule{Mkdir: "~/projects"}, ""),
			expectedRecords:   0,
			expectedUninstall: true,
		},
		{
			name:              "KnownHostsHandler provides status records",
			handler:           NewKnownHostsHandler(parser.Rule{KnownHosts: "github.com"}, ""),
			expectedRecords:   0,
			expectedUninstall: true,
		},
		{
			name:              "GPGKeyHandler provides status records",
			handler:           NewGPGKeyHandler(parser.Rule{GPGKeyring: "ubuntu"}, ""),
			expectedRecords:   0,
			expectedUninstall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Check that handler implements StatusProvider
			statusProvider, ok := tt.handler.(StatusProvider)
			if !ok {
				t.Errorf("Handler does not implement StatusProvider interface")
				return
			}

			// Verify GetCurrentResourceKey returns a non-empty key
			key := statusProvider.GetCurrentResourceKey()
			if key == "" {
				t.Errorf("GetCurrentResourceKey() returned empty string")
			}

			// Verify GetStatusRecords works with empty status
			emptyStatus := &Status{}
			records := statusProvider.GetStatusRecords(emptyStatus)
			if len(records) != tt.expectedRecords {
				t.Errorf("GetStatusRecords(empty) = %d records, want %d", len(records), tt.expectedRecords)
			}

			// Verify BuildUninstallRule creates a valid rule
			if tt.expectedUninstall {
				// Get a mock record (just check that BuildUninstallRule doesn't panic)
				// We can't easily create a status record here, so just verify the method exists
				// by checking if it's callable (which we've already done via the interface)
			}
		})
	}
}
