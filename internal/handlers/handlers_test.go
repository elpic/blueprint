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
		currentRules      []parser.Rule
		expectedRuleCount int
	}{
		{
			name:              "InstallHandler implements StatusProvider",
			handler:           NewInstallHandler(parser.Rule{Packages: []parser.Package{{Name: "vim"}}}, ""),
			currentRules:      []parser.Rule{}, // No current rules means resources should be uninstalled
			expectedRuleCount: 0,               // No uninstall rules from empty status
		},
		{
			name:              "CloneHandler implements StatusProvider",
			handler:           NewCloneHandler(parser.Rule{ClonePath: "~/repo"}, ""),
			currentRules:      []parser.Rule{},
			expectedRuleCount: 0,
		},
		{
			name:              "DecryptHandler implements StatusProvider",
			handler:           NewDecryptHandler(parser.Rule{DecryptPath: "~/.ssh/key"}, "", nil),
			currentRules:      []parser.Rule{},
			expectedRuleCount: 0,
		},
		{
			name:              "AsdfHandler implements StatusProvider",
			handler:           NewAsdfHandler(parser.Rule{Action: "asdf"}, ""),
			currentRules:      []parser.Rule{},
			expectedRuleCount: 0,
		},
		{
			name:              "MkdirHandler implements StatusProvider",
			handler:           NewMkdirHandler(parser.Rule{Mkdir: "~/projects"}, ""),
			currentRules:      []parser.Rule{},
			expectedRuleCount: 0,
		},
		{
			name:              "KnownHostsHandler implements StatusProvider",
			handler:           NewKnownHostsHandler(parser.Rule{KnownHosts: "github.com"}, ""),
			currentRules:      []parser.Rule{},
			expectedRuleCount: 0,
		},
		{
			name:              "GPGKeyHandler implements StatusProvider",
			handler:           NewGPGKeyHandler(parser.Rule{GPGKeyring: "ubuntu"}, ""),
			currentRules:      []parser.Rule{},
			expectedRuleCount: 0,
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

			// Verify FindUninstallRules method works with empty status
			emptyStatus := &Status{}
			rules := statusProvider.FindUninstallRules(emptyStatus, tt.currentRules, "/tmp/test.bp", "mac")
			if len(rules) != tt.expectedRuleCount {
				t.Errorf("FindUninstallRules(empty) = %d rules, want %d", len(rules), tt.expectedRuleCount)
			}
		})
	}
}
