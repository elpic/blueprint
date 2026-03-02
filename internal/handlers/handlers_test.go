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
		{
			name:    "OllamaHandler implements KeyProvider",
			handler: NewOllamaHandler(parser.Rule{OllamaModels: []string{"llama3"}}, ""),
		},
		{
			name:    "DownloadHandler implements KeyProvider",
			handler: NewDownloadHandler(parser.Rule{DownloadURL: "https://example.com/file.sh", DownloadPath: "~/bin/file.sh"}, ""),
		},
		{
			name:    "RunHandler implements KeyProvider",
			handler: NewRunHandler(parser.Rule{RunCommand: "echo hello"}, ""),
		},
		{
			name:    "RunShHandler implements KeyProvider",
			handler: NewRunShHandler(parser.Rule{RunShURL: "https://example.com/install.sh"}, ""),
		},
		{
			name:    "DotfilesHandler implements KeyProvider",
			handler: NewDotfilesHandler(parser.Rule{DotfilesURL: "https://github.com/user/dotfiles", DotfilesPath: "~/.blueprint/dotfiles/dotfiles"}, ""),
		},
		{
			name:    "MiseHandler implements KeyProvider",
			handler: NewMiseHandler(parser.Rule{Action: "mise"}, ""),
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
		{
			name:              "OllamaHandler provides model display",
			handler:           NewOllamaHandler(parser.Rule{OllamaModels: []string{"llama3", "codellama"}}, ""),
			expectedFormatted: "llama3, codellama",
			isUninstall:       false,
		},
		{
			name:              "DownloadHandler provides path display",
			handler:           NewDownloadHandler(parser.Rule{DownloadURL: "https://example.com/file.sh", DownloadPath: "~/bin/file.sh"}, ""),
			expectedFormatted: "~/bin/file.sh",
			isUninstall:       false,
		},
		{
			name:              "RunHandler provides command display",
			handler:           NewRunHandler(parser.Rule{RunCommand: "echo hello"}, ""),
			expectedFormatted: "echo hello",
			isUninstall:       false,
		},
		{
			name:              "RunShHandler provides URL display",
			handler:           NewRunShHandler(parser.Rule{RunShURL: "https://example.com/install.sh"}, ""),
			expectedFormatted: "https://example.com/install.sh",
			isUninstall:       false,
		},
		{
			name:              "DotfilesHandler provides URL display",
			handler:           NewDotfilesHandler(parser.Rule{DotfilesURL: "https://github.com/user/dotfiles", DotfilesPath: "~/.blueprint/dotfiles/dotfiles"}, ""),
			expectedFormatted: "https://github.com/user/dotfiles",
			isUninstall:       false,
		},
		{
			name:              "MiseHandler provides tools display",
			handler:           NewMiseHandler(parser.Rule{Action: "mise", MisePackages: []string{"node@20", "python@3.11"}}, ""),
			expectedFormatted: "node@20, python@3.11",
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

// TestStateProviderInterface verifies that all handlers implement StateProvider
func TestStateProviderInterface(t *testing.T) {
	tests := []struct {
		name            string
		handler         Handler
		expectedSummary string
		isUninstall     bool
		expectedKeys    []string
	}{
		{
			name:            "InstallHandler provides package state",
			handler:         NewInstallHandler(parser.Rule{Packages: []parser.Package{{Name: "vim"}, {Name: "curl"}}}, ""),
			expectedSummary: "vim, curl",
			isUninstall:     false,
			expectedKeys:    []string{"summary", "packages"},
		},
		{
			name:            "CloneHandler provides clone state",
			handler:         NewCloneHandler(parser.Rule{ClonePath: "~/my-repo", CloneURL: "https://github.com/user/repo"}, ""),
			expectedSummary: "~/my-repo",
			isUninstall:     false,
			expectedKeys:    []string{"summary", "url", "path"},
		},
		{
			name:            "DecryptHandler provides decrypt state",
			handler:         NewDecryptHandler(parser.Rule{DecryptPath: "~/.ssh/config", DecryptFile: "config.enc"}, "", nil),
			expectedSummary: "~/.ssh/config",
			isUninstall:     true,
			expectedKeys:    []string{"summary", "source", "dest"},
		},
		{
			name:            "AsdfHandler provides asdf state",
			handler:         NewAsdfHandler(parser.Rule{AsdfPackages: []string{"node@18", "python@3.11"}}, ""),
			expectedSummary: "node@18, python@3.11",
			isUninstall:     false,
			expectedKeys:    []string{"summary", "plugins"},
		},
		{
			name:            "MkdirHandler provides mkdir state",
			handler:         NewMkdirHandler(parser.Rule{Mkdir: "~/projects"}, ""),
			expectedSummary: "~/projects",
			isUninstall:     false,
			expectedKeys:    []string{"summary", "path"},
		},
		{
			name:            "KnownHostsHandler provides known_hosts state",
			handler:         NewKnownHostsHandler(parser.Rule{KnownHosts: "github.com"}, ""),
			expectedSummary: "github.com",
			isUninstall:     false,
			expectedKeys:    []string{"summary", "host"},
		},
		{
			name:            "GPGKeyHandler provides gpg state",
			handler:         NewGPGKeyHandler(parser.Rule{GPGKeyring: "ubuntu-keyring"}, ""),
			expectedSummary: "ubuntu-keyring",
			isUninstall:     false,
			expectedKeys:    []string{"summary", "keyring"},
		},
		{
			name:            "HomebrewHandler provides homebrew state",
			handler:         NewHomebrewHandler(parser.Rule{HomebrewPackages: []string{"wget", "jq"}}, ""),
			expectedSummary: "wget, jq",
			isUninstall:     false,
			expectedKeys:    []string{"summary", "formulas"},
		},
		{
			name:            "OllamaHandler provides ollama state",
			handler:         NewOllamaHandler(parser.Rule{OllamaModels: []string{"llama3", "codellama"}}, ""),
			expectedSummary: "llama3, codellama",
			isUninstall:     false,
			expectedKeys:    []string{"summary", "models"},
		},
		{
			name:            "DownloadHandler provides download state",
			handler:         NewDownloadHandler(parser.Rule{DownloadURL: "https://example.com/file.sh", DownloadPath: "~/bin/file.sh"}, ""),
			expectedSummary: "~/bin/file.sh",
			isUninstall:     false,
			expectedKeys:    []string{"summary", "url", "path"},
		},
		{
			name:            "RunHandler provides run state",
			handler:         NewRunHandler(parser.Rule{RunCommand: "echo hello"}, ""),
			expectedSummary: "echo hello",
			isUninstall:     false,
			expectedKeys:    []string{"summary", "command"},
		},
		{
			name:            "RunShHandler provides run-sh state",
			handler:         NewRunShHandler(parser.Rule{RunShURL: "https://example.com/install.sh"}, ""),
			expectedSummary: "https://example.com/install.sh",
			isUninstall:     false,
			expectedKeys:    []string{"summary", "url"},
		},
		{
			name:            "DotfilesHandler provides dotfiles state",
			handler:         NewDotfilesHandler(parser.Rule{DotfilesURL: "https://github.com/user/dotfiles", DotfilesPath: "~/.blueprint/dotfiles/dotfiles"}, ""),
			expectedSummary: "https://github.com/user/dotfiles",
			isUninstall:     false,
			expectedKeys:    []string{"summary", "url", "path"},
		},
		{
			name:            "MiseHandler provides mise state",
			handler:         NewMiseHandler(parser.Rule{MisePackages: []string{"node@20", "python@3.11"}}, ""),
			expectedSummary: "node@20, python@3.11",
			isUninstall:     false,
			expectedKeys:    []string{"summary", "tools"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Check that handler implements StateProvider
			psProvider, ok := tt.handler.(StateProvider)
			if !ok {
				t.Errorf("Handler does not implement StateProvider interface")
				return
			}

			// Verify GetState returns expected summary
			state := psProvider.GetState(tt.isUninstall)
			if state["summary"] != tt.expectedSummary {
				t.Errorf("GetState(%v)[\"summary\"] = %q, want %q", tt.isUninstall, state["summary"], tt.expectedSummary)
			}

			// Verify all expected keys are present
			for _, key := range tt.expectedKeys {
				if _, exists := state[key]; !exists {
					t.Errorf("GetState(%v) missing expected key %q", tt.isUninstall, key)
				}
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
		{
			name:              "OllamaHandler implements StatusProvider",
			handler:           NewOllamaHandler(parser.Rule{OllamaModels: []string{"llama3"}}, ""),
			currentRules:      []parser.Rule{},
			expectedRuleCount: 0,
		},
		{
			name:              "DownloadHandler implements StatusProvider",
			handler:           NewDownloadHandler(parser.Rule{DownloadURL: "https://example.com/file.sh", DownloadPath: "~/bin/file.sh"}, ""),
			currentRules:      []parser.Rule{},
			expectedRuleCount: 0,
		},
		{
			name:              "RunHandler implements StatusProvider",
			handler:           NewRunHandler(parser.Rule{RunCommand: "echo hello"}, ""),
			currentRules:      []parser.Rule{},
			expectedRuleCount: 0,
		},
		{
			name:              "RunShHandler implements StatusProvider",
			handler:           NewRunShHandler(parser.Rule{RunShURL: "https://example.com/install.sh"}, ""),
			currentRules:      []parser.Rule{},
			expectedRuleCount: 0,
		},
		{
			name:              "DotfilesHandler implements StatusProvider",
			handler:           NewDotfilesHandler(parser.Rule{DotfilesURL: "https://github.com/user/dotfiles", DotfilesPath: "~/.blueprint/dotfiles/dotfiles"}, ""),
			currentRules:      []parser.Rule{},
			expectedRuleCount: 0,
		},
		{
			name:              "MiseHandler implements StatusProvider",
			handler:           NewMiseHandler(parser.Rule{Action: "mise"}, ""),
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
