package handlers

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	gitpkg "github.com/elpic/blueprint/internal/git"
	"github.com/elpic/blueprint/internal/parser"
)

// TestGetDependencyKeyHelper tests the helper function that centralizes ID checking
func TestGetDependencyKeyHelper(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		fallback string
		expected string
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
			handler: NewInstallHandlerLegacy(parser.Rule{Packages: []parser.Package{{Name: "pkg"}}}, ""),
		},
		{
			name:    "CloneHandler implements KeyProvider",
			handler: NewCloneHandlerLegacy(parser.Rule{ClonePath: "path"}, ""),
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
			handler: NewMkdirHandlerLegacy(parser.Rule{Mkdir: "path"}, ""),
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
		{
			name:    "SudoersHandler implements KeyProvider",
			handler: NewSudoersHandler(parser.Rule{Action: "sudoers", SudoersUser: "alice"}, ""),
		},
		{
			name:    "ScheduleHandler implements KeyProvider",
			handler: NewScheduleHandler(parser.Rule{Action: "schedule", SchedulePreset: "daily", ScheduleSource: "setup.bp"}, ""),
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
			handler:           NewInstallHandlerLegacy(parser.Rule{Packages: []parser.Package{{Name: "vim"}, {Name: "curl"}}}, ""),
			expectedFormatted: "vim, curl",
			isUninstall:       false,
		},
		{
			name:              "CloneHandler provides path display",
			handler:           NewCloneHandlerLegacy(parser.Rule{ClonePath: "~/my-repo"}, ""),
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
			handler:           NewMkdirHandlerLegacy(parser.Rule{Mkdir: "~/projects"}, ""),
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
		{
			name:              "SudoersHandler provides user display",
			handler:           NewSudoersHandler(parser.Rule{Action: "sudoers", SudoersUser: "alice"}, ""),
			expectedFormatted: "/etc/sudoers.d/alice",
			isUninstall:       false,
		},
		{
			name:              "ScheduleHandler provides cron+file display",
			handler:           NewScheduleHandler(parser.Rule{Action: "schedule", SchedulePreset: "daily", ScheduleSource: "setup.bp"}, ""),
			expectedFormatted: "@daily setup.bp",
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
			handler:         NewInstallHandlerLegacy(parser.Rule{Packages: []parser.Package{{Name: "vim"}, {Name: "curl"}}}, ""),
			expectedSummary: "vim, curl",
			isUninstall:     false,
			expectedKeys:    []string{"summary", "packages"},
		},
		{
			name:            "CloneHandler provides clone state",
			handler:         NewCloneHandlerLegacy(parser.Rule{ClonePath: "~/my-repo", CloneURL: "https://github.com/user/repo"}, ""),
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
			handler:         NewMkdirHandlerLegacy(parser.Rule{Mkdir: "~/projects"}, ""),
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
		{
			name:            "SudoersHandler provides sudoers state",
			handler:         NewSudoersHandler(parser.Rule{Action: "sudoers", SudoersUser: "alice"}, ""),
			expectedSummary: "/etc/sudoers.d/alice",
			isUninstall:     false,
			expectedKeys:    []string{"summary", "user"},
		},
		{
			name:            "ScheduleHandler provides schedule state",
			handler:         NewScheduleHandler(parser.Rule{Action: "schedule", SchedulePreset: "daily", ScheduleSource: "setup.bp"}, ""),
			expectedSummary: "@daily setup.bp",
			isUninstall:     false,
			expectedKeys:    []string{"summary", "cron", "source"},
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

// TestRuleKey tests the RuleKey function for different rule types
func TestRuleKey(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		expected string
	}{
		{
			name: "returns ID when present",
			rule: parser.Rule{
				ID:     "custom-id-123",
				Action: "clone",
			},
			expected: "custom-id-123",
		},
		{
			name: "install with package returns package name",
			rule: parser.Rule{
				Action:   "install",
				Packages: []parser.Package{{Name: "vim"}},
			},
			expected: "vim",
		},
		{
			name: "install with multiple packages returns first",
			rule: parser.Rule{
				Action:   "install",
				Packages: []parser.Package{{Name: "vim"}, {Name: "curl"}},
			},
			expected: "vim",
		},
		{
			name: "uninstall with package returns package name",
			rule: parser.Rule{
				Action:   "uninstall",
				Packages: []parser.Package{{Name: "vim"}},
			},
			expected: "vim",
		},
		{
			name: "clone returns clone path",
			rule: parser.Rule{
				Action:    "clone",
				ClonePath: "~/projects/repo",
			},
			expected: "~/projects/repo",
		},
		{
			name: "decrypt returns decrypt path",
			rule: parser.Rule{
				Action:      "decrypt",
				DecryptPath: "~/.ssh/config",
				DecryptFile: "config.enc",
			},
			expected: "~/.ssh/config",
		},
		{
			name: "download returns download path",
			rule: parser.Rule{
				Action:       "download",
				DownloadURL:  "https://example.com/file.sh",
				DownloadPath: "~/bin/file.sh",
			},
			expected: "~/bin/file.sh",
		},
		{
			name: "known_hosts returns hostname",
			rule: parser.Rule{
				Action:     "known_hosts",
				KnownHosts: "github.com",
			},
			expected: "github.com",
		},
		{
			name: "mkdir returns directory path",
			rule: parser.Rule{
				Action: "mkdir",
				Mkdir:  "~/projects",
			},
			expected: "~/projects",
		},
		{
			name: "run returns command",
			rule: parser.Rule{
				Action:     "run",
				RunCommand: "echo hello",
			},
			expected: "echo hello",
		},
		{
			name: "run-sh returns URL",
			rule: parser.Rule{
				Action:   "run-sh",
				RunShURL: "https://example.com/install.sh",
			},
			expected: "https://example.com/install.sh",
		},
		{
			name: "gpg_key returns keyring",
			rule: parser.Rule{
				Action:     "gpg_key",
				GPGKeyring: "ubuntu-keyring",
			},
			expected: "ubuntu-keyring",
		},
		{
			name: "homebrew with package returns package name",
			rule: parser.Rule{
				Action:           "homebrew",
				HomebrewPackages: []string{"wget"},
			},
			expected: "wget",
		},
		{
			name: "asdf returns asdf",
			rule: parser.Rule{
				Action:       "asdf",
				AsdfPackages: []string{"nodejs"},
			},
			expected: "asdf",
		},
		{
			name: "mise returns mise",
			rule: parser.Rule{
				Action:       "mise",
				MisePackages: []string{"node@20"},
			},
			expected: "mise",
		},
		{
			name: "ollama with model returns model name",
			rule: parser.Rule{
				Action:       "ollama",
				OllamaModels: []string{"llama3"},
			},
			expected: "llama3",
		},
		{
			name: "schedule with source returns schedule-source",
			rule: parser.Rule{
				Action:         "schedule",
				ScheduleSource: "setup.bp",
			},
			expected: "schedule-setup.bp",
		},
		{
			name: "shell returns shell name",
			rule: parser.Rule{
				Action:    "shell",
				ShellName: "zsh",
			},
			expected: "zsh",
		},
		{
			name: "unknown action returns action name",
			rule: parser.Rule{
				Action: "unknown_action",
			},
			expected: "unknown_action",
		},
		{
			name: "install with no packages returns install",
			rule: parser.Rule{
				Action: "install",
			},
			expected: "install",
		},
		{
			name: "homebrew with no packages returns homebrew",
			rule: parser.Rule{
				Action: "homebrew",
			},
			expected: "homebrew",
		},
		{
			name: "schedule with no source returns schedule",
			rule: parser.Rule{
				Action: "schedule",
			},
			expected: "schedule",
		},
		{
			name: "ollama with no models returns ollama",
			rule: parser.Rule{
				Action: "ollama",
			},
			expected: "ollama",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RuleKey(tt.rule)
			if got != tt.expected {
				t.Errorf("RuleKey() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestDetectRuleType tests the DetectRuleType function
func TestDetectRuleType(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		expected string
	}{
		{
			name: "detects install from packages",
			rule: parser.Rule{
				Packages: []parser.Package{{Name: "vim"}},
			},
			expected: "install",
		},
		{
			name: "detects clone from clone URL",
			rule: parser.Rule{
				CloneURL: "https://github.com/user/repo",
			},
			expected: "clone",
		},
		{
			name: "detects decrypt from decrypt file",
			rule: parser.Rule{
				DecryptFile: "config.enc",
			},
			expected: "decrypt",
		},
		{
			name: "detects mkdir from mkdir field",
			rule: parser.Rule{
				Mkdir: "~/projects",
			},
			expected: "mkdir",
		},
		{
			name: "detects asdf from asdf packages",
			rule: parser.Rule{
				AsdfPackages: []string{"nodejs"},
			},
			expected: "asdf",
		},
		{
			name: "detects mise from mise packages",
			rule: parser.Rule{
				MisePackages: []string{"node@20"},
			},
			expected: "mise",
		},
		{
			name: "detects homebrew from homebrew packages",
			rule: parser.Rule{
				HomebrewPackages: []string{"wget"},
			},
			expected: "homebrew",
		},
		{
			name: "detects ollama from ollama models",
			rule: parser.Rule{
				OllamaModels: []string{"llama3"},
			},
			expected: "ollama",
		},
		{
			name: "detects known_hosts from known hosts field",
			rule: parser.Rule{
				KnownHosts: "github.com",
			},
			expected: "known_hosts",
		},
		{
			name: "detects gpg_key from keyring field",
			rule: parser.Rule{
				GPGKeyring: "ubuntu-keyring",
			},
			expected: "gpg-key",
		},
		{
			name: "detects download from download URL",
			rule: parser.Rule{
				DownloadURL: "https://example.com/file.sh",
			},
			expected: "download",
		},
		{
			name: "detects run from run command",
			rule: parser.Rule{
				RunCommand: "echo hello",
			},
			expected: "run",
		},
		{
			name: "detects run-sh from run-sh URL",
			rule: parser.Rule{
				RunShURL: "https://example.com/install.sh",
			},
			expected: "run-sh",
		},
		{
			name: "detects dotfiles from dotfiles URL",
			rule: parser.Rule{
				DotfilesURL: "https://github.com/user/dotfiles",
			},
			expected: "dotfiles",
		},
		{
			name: "detects sudoers from sudoers user",
			rule: parser.Rule{
				SudoersUser: "alice",
			},
			expected: "sudoers",
		},
		{
			name: "detects schedule from schedule source",
			rule: parser.Rule{
				ScheduleSource: "setup.bp",
			},
			expected: "schedule",
		},
		{
			name: "detects shell from shell name",
			rule: parser.Rule{
				ShellName: "zsh",
			},
			expected: "shell",
		},
		{
			name: "returns empty for unknown rule type",
			rule: parser.Rule{
				Action: "unknown",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectRuleType(tt.rule)
			if got != tt.expected {
				t.Errorf("DetectRuleType() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestNormalizePath tests the normalizePath function
func TestNormalizePath(t *testing.T) {
	cwd, _ := os.Getwd()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "returns absolute path unchanged",
			input:    "/Users/test/file.txt",
			expected: "/Users/test/file.txt",
		},
		{
			name:     "converts relative path to absolute",
			input:    "relative/../relative/file.txt",
			expected: filepath.Join(cwd, "relative/file.txt"),
		},
		{
			name:     "handles empty path",
			input:    "",
			expected: cwd,
		},
		{
			name:     "removes trailing slashes",
			input:    "/path/to/dir//",
			expected: "/path/to/dir",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizePath(tt.input)
			if got != tt.expected {
				t.Errorf("normalizePath(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// TestRemovePackageStatus tests the removePackageStatus function
func TestRemovePackageStatus(t *testing.T) {
	tests := []struct {
		name      string
		packages  []PackageStatus
		target    string
		blueprint string
		osName    string
		expected  int
	}{
		{
			name: "removes matching package",
			packages: []PackageStatus{
				{Name: "vim", Blueprint: "/bp/setup.bp", OS: "mac"},
				{Name: "curl", Blueprint: "/bp/setup.bp", OS: "mac"},
			},
			target:    "vim",
			blueprint: "/bp/setup.bp",
			osName:    "mac",
			expected:  1,
		},
		{
			name: "keeps package with different name",
			packages: []PackageStatus{
				{Name: "vim", Blueprint: "/bp/setup.bp", OS: "mac"},
				{Name: "curl", Blueprint: "/bp/setup.bp", OS: "mac"},
			},
			target:    "wget",
			blueprint: "/bp/setup.bp",
			osName:    "mac",
			expected:  2,
		},
		{
			name: "keeps package with different blueprint",
			packages: []PackageStatus{
				{Name: "vim", Blueprint: "/bp/setup.bp", OS: "mac"},
				{Name: "vim", Blueprint: "/other/setup.bp", OS: "mac"},
			},
			target:    "vim",
			blueprint: "/bp/setup.bp",
			osName:    "mac",
			expected:  1,
		},
		{
			name: "keeps package with different OS",
			packages: []PackageStatus{
				{Name: "vim", Blueprint: "/bp/setup.bp", OS: "mac"},
				{Name: "vim", Blueprint: "/bp/setup.bp", OS: "linux"},
			},
			target:    "vim",
			blueprint: "/bp/setup.bp",
			osName:    "mac",
			expected:  1,
		},
		{
			name:      "empty packages returns empty",
			packages:  []PackageStatus{},
			target:    "vim",
			blueprint: "/bp/setup.bp",
			osName:    "mac",
			expected:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := removePackageStatus(tt.packages, tt.target, tt.blueprint, tt.osName)
			if len(got) != tt.expected {
				t.Errorf("removePackageStatus() returned %d items, want %d", len(got), tt.expected)
			}
		})
	}
}

// TestRemoveRunStatus tests the removeRunStatus function
func TestRemoveRunStatus(t *testing.T) {
	tests := []struct {
		name      string
		runs      []RunStatus
		command   string
		blueprint string
		osName    string
		expected  int
	}{
		{
			name: "removes matching run",
			runs: []RunStatus{
				{Command: "echo hello", Blueprint: "/bp/setup.bp", OS: "mac"},
				{Command: "echo world", Blueprint: "/bp/setup.bp", OS: "mac"},
			},
			command:   "echo hello",
			blueprint: "/bp/setup.bp",
			osName:    "mac",
			expected:  1,
		},
		{
			name: "keeps run with different command",
			runs: []RunStatus{
				{Command: "echo hello", Blueprint: "/bp/setup.bp", OS: "mac"},
			},
			command:   "echo world",
			blueprint: "/bp/setup.bp",
			osName:    "mac",
			expected:  1,
		},
		{
			name: "keeps run with different blueprint",
			runs: []RunStatus{
				{Command: "echo hello", Blueprint: "/bp/setup.bp", OS: "mac"},
				{Command: "echo hello", Blueprint: "/other/setup.bp", OS: "mac"},
			},
			command:   "echo hello",
			blueprint: "/bp/setup.bp",
			osName:    "mac",
			expected:  1,
		},
		{
			name: "keeps run with different OS",
			runs: []RunStatus{
				{Command: "echo hello", Blueprint: "/bp/setup.bp", OS: "mac"},
				{Command: "echo hello", Blueprint: "/bp/setup.bp", OS: "linux"},
			},
			command:   "echo hello",
			blueprint: "/bp/setup.bp",
			osName:    "mac",
			expected:  1,
		},
		{
			name:      "empty runs returns empty",
			runs:      []RunStatus{},
			command:   "echo hello",
			blueprint: "/bp/setup.bp",
			osName:    "mac",
			expected:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := removeRunStatus(tt.runs, tt.command, tt.blueprint, tt.osName)
			if len(got) != tt.expected {
				t.Errorf("removeRunStatus() returned %d items, want %d", len(got), tt.expected)
			}
		})
	}
}

// TestRemoveDotfilesStatus tests the removeDotfilesStatus function
func TestRemoveDotfilesStatus(t *testing.T) {
	tests := []struct {
		name      string
		dotfiles  []DotfilesStatus
		url       string
		blueprint string
		osName    string
		expected  int
	}{
		{
			name: "removes matching dotfiles",
			dotfiles: []DotfilesStatus{
				{URL: "https://github.com/user/dotfiles", Blueprint: "/bp/setup.bp", OS: "mac"},
				{URL: "https://github.com/other/dotfiles", Blueprint: "/bp/setup.bp", OS: "mac"},
			},
			url:       "https://github.com/user/dotfiles",
			blueprint: "/bp/setup.bp",
			osName:    "mac",
			expected:  1,
		},
		{
			name: "keeps dotfiles with different URL",
			dotfiles: []DotfilesStatus{
				{URL: "https://github.com/user/dotfiles", Blueprint: "/bp/setup.bp", OS: "mac"},
			},
			url:       "https://github.com/other/dotfiles",
			blueprint: "/bp/setup.bp",
			osName:    "mac",
			expected:  1,
		},
		{
			name: "keeps dotfiles with different blueprint",
			dotfiles: []DotfilesStatus{
				{URL: "https://github.com/user/dotfiles", Blueprint: "/bp/setup.bp", OS: "mac"},
				{URL: "https://github.com/user/dotfiles", Blueprint: "/other/setup.bp", OS: "mac"},
			},
			url:       "https://github.com/user/dotfiles",
			blueprint: "/bp/setup.bp",
			osName:    "mac",
			expected:  1,
		},
		{
			name:      "empty dotfiles returns empty",
			dotfiles:  []DotfilesStatus{},
			url:       "https://github.com/user/dotfiles",
			blueprint: "/bp/setup.bp",
			osName:    "mac",
			expected:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := removeDotfilesStatus(tt.dotfiles, tt.url, tt.blueprint, tt.osName)
			if len(got) != tt.expected {
				t.Errorf("removeDotfilesStatus() returned %d items, want %d", len(got), tt.expected)
			}
		})
	}
}

// TestRemoveDownloadStatus tests the removeDownloadStatus function
func TestRemoveDownloadStatus(t *testing.T) {
	tests := []struct {
		name      string
		downloads []DownloadStatus
		path      string
		blueprint string
		osName    string
		expected  int
	}{
		{
			name: "removes matching download",
			downloads: []DownloadStatus{
				{Path: "~/bin/file.sh", Blueprint: "/bp/setup.bp", OS: "mac"},
				{Path: "~/bin/other.sh", Blueprint: "/bp/setup.bp", OS: "mac"},
			},
			path:      "~/bin/file.sh",
			blueprint: "/bp/setup.bp",
			osName:    "mac",
			expected:  1,
		},
		{
			name: "keeps download with different path",
			downloads: []DownloadStatus{
				{Path: "~/bin/file.sh", Blueprint: "/bp/setup.bp", OS: "mac"},
			},
			path:      "~/bin/other.sh",
			blueprint: "/bp/setup.bp",
			osName:    "mac",
			expected:  1,
		},
		{
			name:      "empty downloads returns empty",
			downloads: []DownloadStatus{},
			path:      "~/bin/file.sh",
			blueprint: "/bp/setup.bp",
			osName:    "mac",
			expected:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := removeDownloadStatus(tt.downloads, tt.path, tt.blueprint, tt.osName)
			if len(got) != tt.expected {
				t.Errorf("removeDownloadStatus() returned %d items, want %d", len(got), tt.expected)
			}
		})
	}
}

// TestAbbreviateBlueprintPath tests the abbreviateBlueprintPath function
func TestAbbreviateBlueprintPath(t *testing.T) {
	// Get the current working directory for testing
	cwd, _ := os.Getwd()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "returns path unchanged when outside cwd",
			input:    "/Users/other/project/file.bp",
			expected: "/Users/other/project/file.bp",
		},
		{
			name:     "returns path unchanged when error getting cwd",
			input:    "/unknown/path/file.bp",
			expected: "/unknown/path/file.bp",
		},
	}

	// Add test for path within cwd
	pathWithinCwd := filepath.Join(cwd, "setup.bp")
	relPath := "setup.bp"
	tests = append(tests, struct {
		name     string
		input    string
		expected string
	}{
		name:     "returns relative path for file within cwd",
		input:    pathWithinCwd,
		expected: relPath,
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := abbreviateBlueprintPath(tt.input)
			if got != tt.expected {
				t.Errorf("abbreviateBlueprintPath(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// TestNewHandler tests the NewHandler function
func TestNewHandler(t *testing.T) {
	tests := []struct {
		name       string
		rule       parser.Rule
		basePath   string
		passwords  map[string]string
		expectNil  bool
		expectType string
	}{
		{
			name: "creates install handler",
			rule: parser.Rule{
				Action:   "install",
				Packages: []parser.Package{{Name: "vim"}},
			},
			basePath:   "",
			passwords:  nil,
			expectNil:  false,
			expectType: "*handlers.InstallHandler",
		},
		{
			name: "creates clone handler",
			rule: parser.Rule{
				Action:    "clone",
				ClonePath: "~/repo",
				CloneURL:  "https://github.com/user/repo",
			},
			basePath:   "",
			passwords:  nil,
			expectNil:  false,
			expectType: "*handlers.CloneHandler",
		},
		{
			name: "creates decrypt handler",
			rule: parser.Rule{
				Action:      "decrypt",
				DecryptPath: "~/.ssh/config",
				DecryptFile: "config.enc",
			},
			basePath:   "",
			passwords:  nil,
			expectNil:  false,
			expectType: "*handlers.DecryptHandler",
		},
		{
			name: "creates mkdir handler",
			rule: parser.Rule{
				Action: "mkdir",
				Mkdir:  "~/projects",
			},
			basePath:   "",
			passwords:  nil,
			expectNil:  false,
			expectType: "*handlers.MkdirHandler",
		},
		{
			name: "creates run handler",
			rule: parser.Rule{
				Action:     "run",
				RunCommand: "echo hello",
			},
			basePath:   "",
			passwords:  nil,
			expectNil:  false,
			expectType: "*handlers.RunHandler",
		},
		{
			name: "creates run-sh handler",
			rule: parser.Rule{
				Action:   "run-sh",
				RunShURL: "https://example.com/install.sh",
			},
			basePath:   "",
			passwords:  nil,
			expectNil:  false,
			expectType: "*handlers.RunShHandler",
		},
		{
			name: "creates download handler",
			rule: parser.Rule{
				Action:       "download",
				DownloadURL:  "https://example.com/file.sh",
				DownloadPath: "~/bin/file.sh",
			},
			basePath:   "",
			passwords:  nil,
			expectNil:  false,
			expectType: "*handlers.DownloadHandler",
		},
		{
			name: "returns nil for unknown action",
			rule: parser.Rule{
				Action: "unknown_action",
			},
			basePath:   "",
			passwords:  nil,
			expectNil:  true,
			expectType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewHandler(tt.rule, tt.basePath, tt.passwords)
			if tt.expectNil {
				if got != nil {
					t.Errorf("NewHandler() = %v, want nil", got)
				}
			} else {
				if got == nil {
					t.Errorf("NewHandler() = nil, want non-nil handler")
				}
			}
		})
	}
}

// TestGetStatusProviderHandlers tests the GetStatusProviderHandlers function
func TestGetStatusProviderHandlers(t *testing.T) {
	handlers := GetStatusProviderHandlers()

	// Verify we get handlers for all types
	if len(handlers) == 0 {
		t.Error("GetStatusProviderHandlers() returned empty slice")
	}

	// Verify each handler implements StatusProvider
	for i, h := range handlers {
		if _, ok := h.(StatusProvider); !ok {
			t.Errorf("Handler at index %d does not implement StatusProvider", i)
		}
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
			handler:           NewInstallHandlerLegacy(parser.Rule{Packages: []parser.Package{{Name: "vim"}}}, ""),
			currentRules:      []parser.Rule{}, // No current rules means resources should be uninstalled
			expectedRuleCount: 0,               // No uninstall rules from empty status
		},
		{
			name:              "CloneHandler implements StatusProvider",
			handler:           NewCloneHandlerLegacy(parser.Rule{ClonePath: "~/repo"}, ""),
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
			handler:           NewMkdirHandlerLegacy(parser.Rule{Mkdir: "~/projects"}, ""),
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
		{
			name:              "SudoersHandler implements StatusProvider",
			handler:           NewSudoersHandler(parser.Rule{Action: "sudoers", SudoersUser: "alice"}, ""),
			currentRules:      []parser.Rule{},
			expectedRuleCount: 0,
		},
		{
			name:              "ScheduleHandler implements StatusProvider",
			handler:           NewScheduleHandler(parser.Rule{Action: "schedule", SchedulePreset: "daily", ScheduleSource: "setup.bp"}, ""),
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

// TestNormalizeBlueprintHandlesGitURLs tests that normalizeBlueprint correctly
// normalizes git URLs using NormalizeGitURL instead of filepath-based normalizePath.
func TestNormalizeBlueprintHandlesGitURLs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "SSH git URL normalized to HTTPS",
			input:    "git@github.com:user/repo.git",
			expected: gitpkg.NormalizeGitURL("git@github.com:user/repo.git"),
		},
		{
			name:     "HTTPS git URL normalized",
			input:    "https://github.com/user/repo.git",
			expected: gitpkg.NormalizeGitURL("https://github.com/user/repo.git"),
		},
		{
			name:     "SSH and HTTPS normalize to the same value",
			input:    "git@github.com:user/repo.git",
			expected: gitpkg.NormalizeGitURL("https://github.com/user/repo.git"),
		},
		{
			name:     "local path still works",
			input:    "/tmp/setup.bp",
			expected: "/tmp/setup.bp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeBlueprint(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeBlueprint(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// TestIsInstalledSSHvsHTTPS tests that IsInstalled matches resources installed via
// SSH git URL when queried with the HTTPS URL for the same repo (and vice versa).
func TestIsInstalledSSHvsHTTPS(t *testing.T) {
	sshURL := "git@github.com:user/dotfiles.git"
	httpsURL := "https://github.com/user/dotfiles.git"

	// Simulate a package installed via SSH URL blueprint
	status := &Status{
		Packages: []PackageStatus{
			{Name: "vim", Blueprint: sshURL, OS: "linux"},
		},
	}

	handler := NewInstallHandlerLegacy(parser.Rule{
		Action:   "install",
		Packages: []parser.Package{{Name: "vim", Version: "latest"}},
	}, "")

	// Query with HTTPS URL — should still find it
	installed := handler.IsInstalled(status, httpsURL, "linux")
	if !installed {
		t.Error("IsInstalled should return true when status has SSH URL and query uses HTTPS URL")
	}

	// Query with SSH URL — should still find it
	installed = handler.IsInstalled(status, sshURL, "linux")
	if !installed {
		t.Error("IsInstalled should return true when status and query both use SSH URL")
	}

	// Now test reverse: installed via HTTPS, query with SSH
	status2 := &Status{
		Packages: []PackageStatus{
			{Name: "vim", Blueprint: httpsURL, OS: "linux"},
		},
	}
	installed = handler.IsInstalled(status2, sshURL, "linux")
	if !installed {
		t.Error("IsInstalled should return true when status has HTTPS URL and query uses SSH URL")
	}
}

// TestFindUninstallRulesSSHvsHTTPS tests that FindUninstallRules correctly matches
// resources across SSH and HTTPS git URL forms for the same repository.
func TestFindUninstallRulesSSHvsHTTPS(t *testing.T) {
	sshURL := "git@github.com:user/dotfiles.git"
	httpsURL := "https://github.com/user/dotfiles.git"

	// Status has a package installed via SSH URL
	status := &Status{
		Packages: []PackageStatus{
			{Name: "vim", Blueprint: sshURL, OS: "linux"},
		},
	}

	// Current rules still have vim — should NOT generate uninstall rules
	currentRules := []parser.Rule{
		{Action: "install", Packages: []parser.Package{{Name: "vim", Version: "latest"}}},
	}

	handler := NewInstallHandlerLegacy(parser.Rule{}, "")
	// Query with HTTPS — should match the SSH-stored status and NOT uninstall
	rules := handler.FindUninstallRules(status, currentRules, httpsURL, "linux")
	if len(rules) != 0 {
		t.Errorf("FindUninstallRules should return 0 rules when SSH/HTTPS refer to same repo, got %d", len(rules))
	}
}

// TestRemovePackageStatusWithGitURLs tests that removePackageStatus correctly
// matches blueprints stored as SSH URLs when queried with HTTPS URLs.
func TestRemovePackageStatusWithGitURLs(t *testing.T) {
	sshURL := "git@github.com:user/dotfiles.git"
	httpsURL := "https://github.com/user/dotfiles.git"

	packages := []PackageStatus{
		{Name: "vim", Blueprint: sshURL, OS: "linux"},
		{Name: "curl", Blueprint: "/local/setup.bp", OS: "linux"},
	}

	// Remove vim using HTTPS URL — should match the SSH-stored blueprint
	result := removePackageStatus(packages, "vim", httpsURL, "linux")
	if len(result) != 1 {
		t.Errorf("removePackageStatus with HTTPS URL should remove SSH-stored package, got %d remaining (want 1)", len(result))
	}
	if len(result) == 1 && result[0].Name != "curl" {
		t.Errorf("remaining package should be curl, got %s", result[0].Name)
	}
}

// TestAllEntriesCoversAllSlices verifies that AllEntries() covers every
// StatusEntry-typed slice field on Status. If a new action is added with a new
// slice field but AllEntries() and FilterEntries() are not updated, this test
// fails — a test-time safety net equivalent to a compile-time check.
func TestAllEntriesCoversAllSlices(t *testing.T) {
	statusEntryType := reflect.TypeOf((*StatusEntry)(nil)).Elem()
	st := reflect.TypeOf(Status{})

	// Count []T fields on Status where *T implements StatusEntry.
	sliceCount := 0
	for i := 0; i < st.NumField(); i++ {
		f := st.Field(i)
		if f.Type.Kind() == reflect.Slice &&
			reflect.PointerTo(f.Type.Elem()).Implements(statusEntryType) {
			sliceCount++
		}
	}

	// Build a Status with one zero-value element in each such slice.
	sv := reflect.New(st).Elem()
	for i := 0; i < st.NumField(); i++ {
		f := st.Field(i)
		if f.Type.Kind() == reflect.Slice &&
			reflect.PointerTo(f.Type.Elem()).Implements(statusEntryType) {
			sv.Field(i).Set(reflect.MakeSlice(f.Type, 1, 1))
		}
	}
	status := sv.Addr().Interface().(*Status)

	got := len(status.AllEntries())
	if got != sliceCount {
		t.Errorf("AllEntries() returned %d entries but Status has %d StatusEntry slices; "+
			"update AllEntries() and FilterEntries() to include the new slice",
			got, sliceCount)
	}

	// Verify FilterEntries(keep=false) clears everything it knows about.
	status.FilterEntries(func(StatusEntry) bool { return false })
	if remaining := len(status.AllEntries()); remaining != 0 {
		t.Errorf("FilterEntries(false) left %d entries; update FilterEntries() to include the new slice",
			remaining)
	}
}
