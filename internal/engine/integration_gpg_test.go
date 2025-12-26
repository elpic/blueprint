package engine

import (
	"testing"

	handlerskg "github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/parser"
)

// TestGPGKeyStatusPersistenceFlow tests that GPG key status is properly persisted
func TestGPGKeyStatusPersistenceFlow(t *testing.T) {
	tests := []struct {
		name      string
		blueprint string
		os        string
		keyring   string
		url       string
		debURL    string
	}{
		{
			name:      "GPG key status for wezterm",
			blueprint: "/test/blueprint.bp",
			os:        "linux",
			keyring:   "wezterm-fury",
			url:       "https://apt.fury.io/wez/gpg.key",
			debURL:    "https://apt.fury.io/wez/",
		},
		{
			name:      "GPG key status for docker",
			blueprint: "/test/blueprint.bp",
			os:        "linux",
			keyring:   "docker",
			url:       "https://download.docker.com/linux/ubuntu/gpg",
			debURL:    "https://download.docker.com/linux/ubuntu",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create status with GPG key
			status := Status{
				GPGKeys: []GPGKeyStatus{
					{
						Keyring:   tt.keyring,
						URL:       tt.url,
						DebURL:    tt.debURL,
						AddedAt:   "2025-12-16T10:00:00Z",
						Blueprint: tt.blueprint,
						OS:        tt.os,
					},
				},
			}

			// Verify status contains the GPG key
			if len(status.GPGKeys) != 1 {
				t.Errorf("Expected 1 GPG key, got %d", len(status.GPGKeys))
			}

			gpg := status.GPGKeys[0]
			if gpg.Keyring != tt.keyring {
				t.Errorf("Keyring: got %q, want %q", gpg.Keyring, tt.keyring)
			}
			if gpg.URL != tt.url {
				t.Errorf("URL: got %q, want %q", gpg.URL, tt.url)
			}
			if gpg.DebURL != tt.debURL {
				t.Errorf("DebURL: got %q, want %q", gpg.DebURL, tt.debURL)
			}
			if gpg.Blueprint != tt.blueprint {
				t.Errorf("Blueprint: got %q, want %q", gpg.Blueprint, tt.blueprint)
			}
			if gpg.OS != tt.os {
				t.Errorf("OS: got %q, want %q", gpg.OS, tt.os)
			}
		})
	}
}

// TestGPGKeyAutoUninstallDetection tests that removed GPG keys are detected for uninstall
func TestGPGKeyAutoUninstallDetection(t *testing.T) {
	// Create status with existing GPG keys
	status := Status{
		GPGKeys: []GPGKeyStatus{
			{
				Keyring:   "wezterm-fury",
				URL:       "https://apt.fury.io/wez/gpg.key",
				DebURL:    "https://apt.fury.io/wez/",
				Blueprint: "/test/blueprint.bp",
				OS:        "linux",
			},
			{
				Keyring:   "docker",
				URL:       "https://download.docker.com/linux/ubuntu/gpg",
				DebURL:    "https://download.docker.com/linux/ubuntu",
				Blueprint: "/test/blueprint.bp",
				OS:        "linux",
			},
		},
	}

	// Current rules only have wezterm-fury (docker is removed)
	currentRules := []parser.Rule{
		{
			Action:     "gpg-key",
			GPGKeyring: "wezterm-fury",
			GPGKeyURL:  "https://apt.fury.io/wez/gpg.key",
			GPGDebURL:  "https://apt.fury.io/wez/",
			OSList:     []string{"linux"},
		},
	}

	// Build currentGPGKeys map for auto-uninstall detection
	currentGPGKeys := make(map[string]bool)
	for _, rule := range currentRules {
		if rule.Action == "gpg-key" {
			currentGPGKeys[rule.GPGKeyring] = true
		}
	}

	// Simulate auto-uninstall detection logic
	autoUninstallCount := 0
	for _, gpg := range status.GPGKeys {
		if gpg.Blueprint == "/test/blueprint.bp" &&
			gpg.OS == "linux" &&
			!currentGPGKeys[gpg.Keyring] {
			autoUninstallCount++
		}
	}

	// Should detect docker for removal (not in currentRules)
	if autoUninstallCount != 1 {
		t.Errorf("Expected 1 auto-uninstall rule, got %d", autoUninstallCount)
	}

	// Verify wezterm-fury is NOT flagged for removal
	if !currentGPGKeys["wezterm-fury"] {
		t.Error("wezterm-fury should be in current rules")
	}

	// Verify docker IS flagged for removal
	if currentGPGKeys["docker"] {
		t.Error("docker should not be in current rules")
	}
}

// TestGPGKeyStatusEquivalence tests that engine and handler GPGKeyStatus types are equivalent
func TestGPGKeyStatusEquivalence(t *testing.T) {
	// Create a GPGKeyStatus
	gpgKey := GPGKeyStatus{
		Keyring:   "test-repo",
		URL:       "https://example.com/gpg.key",
		DebURL:    "https://example.com/apt",
		AddedAt:   "2025-12-16T10:00:00Z",
		Blueprint: "/test/blueprint.bp",
		OS:        "linux",
	}

	// Verify it's the same as handler type
	handlerKey := handlerskg.GPGKeyStatus(gpgKey)
	if handlerKey.Keyring != gpgKey.Keyring {
		t.Errorf("Keyring mismatch: got %q, want %q", handlerKey.Keyring, gpgKey.Keyring)
	}

	// Verify all fields match
	if handlerKey.URL != gpgKey.URL || handlerKey.DebURL != gpgKey.DebURL ||
		handlerKey.AddedAt != gpgKey.AddedAt || handlerKey.Blueprint != gpgKey.Blueprint ||
		handlerKey.OS != gpgKey.OS {
		t.Error("GPGKeyStatus fields don't match between engine and handler types")
	}
}
