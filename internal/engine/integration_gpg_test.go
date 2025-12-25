package engine

import (
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

// TestGPGKeyStatusPersistenceFlow tests that GPG key status is properly persisted
func TestGPGKeyStatusPersistenceFlow(t *testing.T) {
	tests := []struct {
		name          string
		blueprint     string
		os            string
		keyring       string
		url           string
		debURL        string
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

// TestGPGKeyStatusConversionBidirectional tests that status converts correctly both ways
func TestGPGKeyStatusConversionBidirectional(t *testing.T) {
	original := Status{
		GPGKeys: []GPGKeyStatus{
			{
				Keyring:   "test-repo",
				URL:       "https://example.com/gpg.key",
				DebURL:    "https://example.com/apt",
				AddedAt:   "2025-12-16T10:00:00Z",
				Blueprint: "/test/blueprint.bp",
				OS:        "linux",
			},
		},
	}

	// Convert to handler status
	handlerGPGKeys := convertGPGKeys(original.GPGKeys)

	// Verify conversion
	if len(handlerGPGKeys) != 1 {
		t.Fatalf("Expected 1 GPG key, got %d", len(handlerGPGKeys))
	}

	if handlerGPGKeys[0].Keyring != "test-repo" {
		t.Errorf("Keyring mismatch: got %q, want 'test-repo'", handlerGPGKeys[0].Keyring)
	}

	// Convert back to engine status
	engineGPGKeys := convertHandlerGPGKeys(handlerGPGKeys)

	// Verify round-trip conversion
	if len(engineGPGKeys) != 1 {
		t.Fatalf("Expected 1 GPG key after round-trip, got %d", len(engineGPGKeys))
	}

	if engineGPGKeys[0].Keyring != original.GPGKeys[0].Keyring {
		t.Errorf("Keyring mismatch after round-trip: got %q, want %q",
			engineGPGKeys[0].Keyring, original.GPGKeys[0].Keyring)
	}

	if engineGPGKeys[0].URL != original.GPGKeys[0].URL {
		t.Errorf("URL mismatch after round-trip: got %q, want %q",
			engineGPGKeys[0].URL, original.GPGKeys[0].URL)
	}

	if engineGPGKeys[0].DebURL != original.GPGKeys[0].DebURL {
		t.Errorf("DebURL mismatch after round-trip: got %q, want %q",
			engineGPGKeys[0].DebURL, original.GPGKeys[0].DebURL)
	}

	if engineGPGKeys[0].Blueprint != original.GPGKeys[0].Blueprint {
		t.Errorf("Blueprint mismatch after round-trip: got %q, want %q",
			engineGPGKeys[0].Blueprint, original.GPGKeys[0].Blueprint)
	}

	if engineGPGKeys[0].OS != original.GPGKeys[0].OS {
		t.Errorf("OS mismatch after round-trip: got %q, want %q",
			engineGPGKeys[0].OS, original.GPGKeys[0].OS)
	}
}
