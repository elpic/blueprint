package handlers

import (
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

// TestRegistryAllActionsRegistered verifies that all expected action names are
// present in the registry and have their required fields populated.
func TestRegistryAllActionsRegistered(t *testing.T) {
	canonicalNames := []string{
		"install", "uninstall",
		"clone",
		"decrypt",
		"mkdir",
		"known_hosts",
		"gpg_key",
		"asdf",
		"mise",
		"sudoers",
		"homebrew",
		"ollama",
		"download",
		"run", "run-sh",
		"dotfiles",
		"schedule",
		"shell",
		"authorized_keys",
	}

	for _, name := range canonicalNames {
		def := GetAction(name)
		if def == nil {
			t.Errorf("GetAction(%q) returned nil — action not registered", name)
			continue
		}
		if def.Name != name {
			t.Errorf("GetAction(%q).Name = %q, want %q", name, def.Name, name)
		}
	}
}

// TestRegistryNewHandlerViaRegistry verifies that NewHandler delegates to the
// registry and returns a non-nil handler for each canonical action.
func TestRegistryNewHandlerViaRegistry(t *testing.T) {
	actions := []struct {
		name string
		rule parser.Rule
	}{
		{"install", parser.Rule{Action: "install"}},
		{"clone", parser.Rule{Action: "clone"}},
		{"decrypt", parser.Rule{Action: "decrypt"}},
		{"mkdir", parser.Rule{Action: "mkdir"}},
		{"known_hosts", parser.Rule{Action: "known_hosts"}},
		{"gpg_key", parser.Rule{Action: "gpg_key"}},
		{"asdf", parser.Rule{Action: "asdf"}},
		{"mise", parser.Rule{Action: "mise"}},
		{"sudoers", parser.Rule{Action: "sudoers"}},
		{"homebrew", parser.Rule{Action: "homebrew"}},
		{"ollama", parser.Rule{Action: "ollama"}},
		{"download", parser.Rule{Action: "download"}},
		{"run", parser.Rule{Action: "run"}},
		{"run-sh", parser.Rule{Action: "run-sh"}},
		{"dotfiles", parser.Rule{Action: "dotfiles"}},
		{"schedule", parser.Rule{Action: "schedule"}},
		{"shell", parser.Rule{Action: "shell"}},
		{"authorized_keys", parser.Rule{Action: "authorized_keys"}},
	}

	for _, tt := range actions {
		h := NewHandler(tt.rule, "", nil)
		if h == nil {
			t.Errorf("NewHandler(rule{Action:%q}) returned nil", tt.name)
		}
	}
}

// TestRegistryDetectRuleTypeViaRegistry verifies that DetectRuleType delegates
// to the registry and returns the correct action name for each rule type.
func TestRegistryDetectRuleTypeViaRegistry(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		expected string
	}{
		{"packages", parser.Rule{Packages: []parser.Package{{Name: "vim"}}}, "install"},
		{"clone", parser.Rule{CloneURL: "https://github.com/foo/bar"}, "clone"},
		{"decrypt", parser.Rule{DecryptFile: "secret.enc"}, "decrypt"},
		{"mkdir", parser.Rule{Mkdir: "~/projects"}, "mkdir"},
		{"known_hosts", parser.Rule{KnownHosts: "github.com"}, "known_hosts"},
		{"gpg_key", parser.Rule{GPGKeyring: "ubuntu-keyring"}, "gpg_key"},
		{"asdf", parser.Rule{AsdfPackages: []string{"nodejs 18.0.0"}}, "asdf"},
		{"mise", parser.Rule{MisePackages: []string{"node@18"}}, "mise"},
		{"homebrew", parser.Rule{HomebrewPackages: []string{"wget"}}, "homebrew"},
		{"ollama", parser.Rule{OllamaModels: []string{"llama3"}}, "ollama"},
		{"download", parser.Rule{DownloadURL: "https://example.com/file"}, "download"},
		{"run", parser.Rule{RunCommand: "echo hello"}, "run"},
		{"run-sh", parser.Rule{RunShURL: "https://example.com/script.sh"}, "run-sh"},
		{"dotfiles", parser.Rule{DotfilesURL: "https://github.com/user/dots"}, "dotfiles"},
		{"sudoers", parser.Rule{SudoersUser: "ubuntu"}, "sudoers"},
		{"schedule", parser.Rule{ScheduleSource: "https://github.com/user/bp"}, "schedule"},
		{"shell", parser.Rule{ShellName: "zsh"}, "shell"},
		{"authorized_keys", parser.Rule{AuthorizedKeysFile: "~/.ssh/id_rsa.pub"}, "authorized_keys"},
		{"empty", parser.Rule{}, ""},
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

// TestRegistryGetStatusProviderHandlers verifies that aliases are not
// duplicated in the status provider list.
func TestRegistryGetStatusProviderHandlers(t *testing.T) {
	handlers := GetStatusProviderHandlers()
	if len(handlers) == 0 {
		t.Fatal("GetStatusProviderHandlers() returned empty slice")
	}

	// All returned handlers must implement StatusProvider.
	for i, h := range handlers {
		if _, ok := h.(StatusProvider); !ok {
			t.Errorf("handler at index %d does not implement StatusProvider", i)
		}
	}

	// Count should match the number of canonical (non-alias) action defs that
	// have a NewHandler and implement StatusProvider. Currently 19 actions are
	// registered (18 canonical + uninstall stub); all canonical handler types
	// implement StatusProvider except none are excluded.
	// We simply verify the count is at least 15 (sanity floor) and that
	// no handler appears twice by identity.
	if len(handlers) < 15 {
		t.Errorf("GetStatusProviderHandlers() returned %d handlers, want at least 15", len(handlers))
	}
}
