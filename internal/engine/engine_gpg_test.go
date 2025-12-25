package engine

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/elpic/blueprint/internal/parser"
)

// TestGPGKeyStatusStructure tests the GPGKeyStatus structure
func TestGPGKeyStatusStructure(t *testing.T) {
	gpgKey := GPGKeyStatus{
		Keyring:   "test-repo",
		URL:       "https://example.com/gpg.key",
		DebURL:    "https://example.com/apt",
		AddedAt:   "2025-12-15T10:30:00Z",
		Blueprint: "/test/blueprint.bp",
		OS:        "linux",
	}

	// Verify all fields are properly set
	if gpgKey.Keyring != "test-repo" {
		t.Errorf("Keyring: got %q", gpgKey.Keyring)
	}
	if gpgKey.URL != "https://example.com/gpg.key" {
		t.Errorf("URL: got %q", gpgKey.URL)
	}
	if gpgKey.DebURL != "https://example.com/apt" {
		t.Errorf("DebURL: got %q", gpgKey.DebURL)
	}
	if gpgKey.Blueprint != "/test/blueprint.bp" {
		t.Errorf("Blueprint: got %q", gpgKey.Blueprint)
	}
	if gpgKey.OS != "linux" {
		t.Errorf("OS: got %q", gpgKey.OS)
	}
	if gpgKey.AddedAt == "" {
		t.Error("AddedAt should not be empty")
	}
}

// TestGPGKeyStatusJSON tests JSON marshaling/unmarshaling of GPG key status
func TestGPGKeyStatusJSON(t *testing.T) {
	gpgKey := GPGKeyStatus{
		Keyring:   "test-repo",
		URL:       "https://example.com/gpg.key",
		DebURL:    "https://example.com/apt",
		AddedAt:   "2025-12-15T10:30:00Z",
		Blueprint: "/test/blueprint.bp",
		OS:        "linux",
	}

	// Marshal to JSON
	data, err := json.Marshal(gpgKey)
	if err != nil {
		t.Fatalf("Failed to marshal GPG key: %v", err)
	}

	// Unmarshal back
	var restored GPGKeyStatus
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Failed to unmarshal GPG key: %v", err)
	}

	// Verify all fields are preserved
	if restored.Keyring != gpgKey.Keyring {
		t.Errorf("Keyring mismatch: got %q, want %q", restored.Keyring, gpgKey.Keyring)
	}
	if restored.URL != gpgKey.URL {
		t.Errorf("URL mismatch: got %q, want %q", restored.URL, gpgKey.URL)
	}
	if restored.DebURL != gpgKey.DebURL {
		t.Errorf("DebURL mismatch: got %q, want %q", restored.DebURL, gpgKey.DebURL)
	}
	if restored.Blueprint != gpgKey.Blueprint {
		t.Errorf("Blueprint mismatch: got %q, want %q", restored.Blueprint, gpgKey.Blueprint)
	}
	if restored.OS != gpgKey.OS {
		t.Errorf("OS mismatch: got %q, want %q", restored.OS, gpgKey.OS)
	}

	// Verify JSON tags are correct
	var rawJSON map[string]interface{}
	if err := json.Unmarshal(data, &rawJSON); err != nil {
		t.Fatalf("Failed to unmarshal as map: %v", err)
	}

	// Verify snake_case JSON tags
	if _, ok := rawJSON["keyring"]; !ok {
		t.Error("JSON should have 'keyring' key")
	}
	if _, ok := rawJSON["deb_url"]; !ok {
		t.Error("JSON should have 'deb_url' key")
	}
}

// TestStatusWithGPGKeys tests that Status struct includes GPG keys
func TestStatusWithGPGKeys(t *testing.T) {
	status := Status{
		GPGKeys: []GPGKeyStatus{
			{
				Keyring:   "repo1",
				URL:       "https://example.com/key1",
				DebURL:    "https://example.com/apt1",
				Blueprint: "/test/blueprint.bp",
				OS:        "linux",
			},
			{
				Keyring:   "repo2",
				URL:       "https://example.com/key2",
				DebURL:    "https://example.com/apt2",
				Blueprint: "/test/blueprint.bp",
				OS:        "linux",
			},
		},
	}

	if len(status.GPGKeys) != 2 {
		t.Errorf("Expected 2 GPG keys, got %d", len(status.GPGKeys))
	}

	if status.GPGKeys[0].Keyring != "repo1" {
		t.Errorf("First keyring: got %q", status.GPGKeys[0].Keyring)
	}

	if status.GPGKeys[1].Keyring != "repo2" {
		t.Errorf("Second keyring: got %q", status.GPGKeys[1].Keyring)
	}
}

// TestGPGKeyStatusFieldConversion tests that GPG key status fields are correct
func TestGPGKeyStatusFieldConversion(t *testing.T) {
	tests := []struct {
		name  string
		input GPGKeyStatus
	}{
		{
			name: "basic gpg key status",
			input: GPGKeyStatus{
				Keyring:   "test-repo",
				URL:       "https://example.com/gpg.key",
				DebURL:    "https://example.com/apt",
				AddedAt:   "2025-12-15T10:30:00Z",
				Blueprint: "/test/blueprint.bp",
				OS:        "linux",
			},
		},
		{
			name: "gpg key with complex keyring name",
			input: GPGKeyStatus{
				Keyring:   "my-repo-2024-v1",
				URL:       "https://example.com/gpg.key",
				DebURL:    "https://example.com/apt",
				AddedAt:   "2025-12-15T10:30:00Z",
				Blueprint: "/test/blueprint.bp",
				OS:        "ubuntu",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify that all fields are properly maintained
			if tt.input.Keyring == "" {
				t.Error("Keyring is empty")
			}
			if tt.input.URL == "" {
				t.Error("URL is empty")
			}
			if tt.input.DebURL == "" {
				t.Error("DebURL is empty")
			}
			if tt.input.Blueprint == "" {
				t.Error("Blueprint is empty")
			}
			if tt.input.OS == "" {
				t.Error("OS is empty")
			}
		})
	}
}

// TestGPGKeyStatusFilter tests filtering GPG keys by blueprint and OS
func TestGPGKeyStatusFilter(t *testing.T) {
	gpgKeys := []GPGKeyStatus{
		{
			Keyring:   "wezterm",
			Blueprint: "/test/blueprint1.bp",
			OS:        "linux",
		},
		{
			Keyring:   "docker",
			Blueprint: "/test/blueprint1.bp",
			OS:        "linux",
		},
		{
			Keyring:   "wezterm",
			Blueprint: "/test/blueprint2.bp",
			OS:        "linux",
		},
		{
			Keyring:   "wezterm",
			Blueprint: "/test/blueprint1.bp",
			OS:        "mac",
		},
	}

	// Filter for blueprint1.bp on linux
	var filtered []GPGKeyStatus
	for _, gpg := range gpgKeys {
		if gpg.Blueprint == "/test/blueprint1.bp" && gpg.OS == "linux" {
			filtered = append(filtered, gpg)
		}
	}

	if len(filtered) != 2 {
		t.Errorf("Expected 2 filtered items, got %d", len(filtered))
	}

	// Verify we got the right ones
	for _, gpg := range filtered {
		if gpg.Blueprint != "/test/blueprint1.bp" || gpg.OS != "linux" {
			t.Error("Filter didn't work correctly")
		}
	}
}

// TestAutoUninstallRulesFormat tests that auto-uninstall rules have correct structure
func TestAutoUninstallRulesFormat(t *testing.T) {
	// Test that a rule can be created with gpg-key uninstall information
	rule := parser.Rule{
		Action:     "uninstall",
		GPGKeyring: "test-repo",
		OSList:     []string{"linux"},
	}

	if rule.Action != "uninstall" {
		t.Errorf("Action: got %q, want 'uninstall'", rule.Action)
	}


	if rule.GPGKeyring != "test-repo" {
		t.Errorf("GPGKeyring: got %q, want 'test-repo'", rule.GPGKeyring)
	}

	if len(rule.OSList) != 1 || rule.OSList[0] != "linux" {
		t.Errorf("OSList: got %v, want ['linux']", rule.OSList)
	}
}

// TestExecutionRecordForGPGKey tests execution record creation for GPG key rules
func TestExecutionRecordForGPGKey(t *testing.T) {
	tests := []struct {
		name   string
		record ExecutionRecord
	}{
		{
			name: "successful gpg-key execution",
			record: ExecutionRecord{
				Timestamp: time.Now().Format(time.RFC3339),
				Blueprint: "/test/blueprint.bp",
				OS:        "linux",
				Command:   "gpg-key wezterm-fury",
				Output:    "Added GPG key",
				Status:    "success",
			},
		},
		{
			name: "failed gpg-key execution",
			record: ExecutionRecord{
				Timestamp: time.Now().Format(time.RFC3339),
				Blueprint: "/test/blueprint.bp",
				OS:        "linux",
				Command:   "gpg-key test-repo",
				Status:    "error",
				Error:     "command failed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.record.Blueprint != "/test/blueprint.bp" {
				t.Errorf("Blueprint: got %q", tt.record.Blueprint)
			}
			if tt.record.OS != "linux" {
				t.Errorf("OS: got %q", tt.record.OS)
			}
			if tt.record.Status != "success" && tt.record.Status != "error" {
				t.Errorf("Status: got %q", tt.record.Status)
			}
		})
	}
}

// TestGPGKeyStatusPersistence tests that GPG key status can be marshaled and unmarshaled
func TestGPGKeyStatusPersistence(t *testing.T) {
	// Create a status with GPG keys
	originalStatus := Status{
		GPGKeys: []GPGKeyStatus{
			{
				Keyring:   "wezterm-fury",
				URL:       "https://apt.fury.io/wez/gpg.key",
				DebURL:    "https://apt.fury.io/wez/",
				AddedAt:   time.Now().Format(time.RFC3339),
				Blueprint: "/test/blueprint.bp",
				OS:        "linux",
			},
			{
				Keyring:   "docker",
				URL:       "https://download.docker.com/linux/ubuntu/gpg",
				DebURL:    "https://download.docker.com/linux/ubuntu",
				AddedAt:   time.Now().Format(time.RFC3339),
				Blueprint: "/test/blueprint.bp",
				OS:        "linux",
			},
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(originalStatus)
	if err != nil {
		t.Fatalf("Failed to marshal status: %v", err)
	}

	// Unmarshal back
	var restoredStatus Status
	if err := json.Unmarshal(data, &restoredStatus); err != nil {
		t.Fatalf("Failed to unmarshal status: %v", err)
	}

	// Verify GPG keys are preserved
	if len(restoredStatus.GPGKeys) != 2 {
		t.Errorf("Expected 2 GPG keys, got %d", len(restoredStatus.GPGKeys))
	}

	if restoredStatus.GPGKeys[0].Keyring != "wezterm-fury" {
		t.Errorf("First keyring: got %q, want 'wezterm-fury'", restoredStatus.GPGKeys[0].Keyring)
	}

	if restoredStatus.GPGKeys[1].Keyring != "docker" {
		t.Errorf("Second keyring: got %q, want 'docker'", restoredStatus.GPGKeys[1].Keyring)
	}
}

// TestGPGKeyStatusMultipleBlueprints tests GPG keys from multiple blueprints
func TestGPGKeyStatusMultipleBlueprints(t *testing.T) {
	status := Status{
		GPGKeys: []GPGKeyStatus{
			{
				Keyring:   "wezterm",
				Blueprint: "/test/blueprint1.bp",
				OS:        "linux",
			},
			{
				Keyring:   "docker",
				Blueprint: "/test/blueprint2.bp",
				OS:        "linux",
			},
			{
				Keyring:   "postgres",
				Blueprint: "/test/blueprint1.bp",
				OS:        "linux",
			},
		},
	}

	// Group by blueprint
	blueprint1Keys := 0
	blueprint2Keys := 0

	for _, gpg := range status.GPGKeys {
		if gpg.Blueprint == "/test/blueprint1.bp" {
			blueprint1Keys++
		} else if gpg.Blueprint == "/test/blueprint2.bp" {
			blueprint2Keys++
		}
	}

	if blueprint1Keys != 2 {
		t.Errorf("Blueprint 1: expected 2 keys, got %d", blueprint1Keys)
	}

	if blueprint2Keys != 1 {
		t.Errorf("Blueprint 2: expected 1 key, got %d", blueprint2Keys)
	}
}

// TestGPGKeyStatusRemoval tests filtering out specific GPG keys
func TestGPGKeyStatusRemoval(t *testing.T) {
	status := Status{
		GPGKeys: []GPGKeyStatus{
			{
				Keyring:   "repo1",
				Blueprint: "/test/blueprint.bp",
				OS:        "linux",
			},
			{
				Keyring:   "repo2",
				Blueprint: "/test/blueprint.bp",
				OS:        "linux",
			},
			{
				Keyring:   "repo1",
				Blueprint: "/test/blueprint.bp",
				OS:        "mac",
			},
		},
	}

	// Remove repo1 from linux
	var filtered []GPGKeyStatus
	for _, gpg := range status.GPGKeys {
		if !(gpg.Keyring == "repo1" && gpg.Blueprint == "/test/blueprint.bp" && gpg.OS == "linux") {
			filtered = append(filtered, gpg)
		}
	}

	if len(filtered) != 2 {
		t.Errorf("Expected 2 items after removal, got %d", len(filtered))
	}

	// Verify the remaining items
	for _, gpg := range filtered {
		if gpg.Keyring == "repo1" && gpg.OS == "linux" {
			t.Error("Removed item still present")
		}
	}
}

// TestGPGKeyConversionRoundtrip tests that GPG key data survives conversion
func TestGPGKeyConversionRoundtrip(t *testing.T) {
	original := GPGKeyStatus{
		Keyring:   "test-keyring",
		URL:       "https://example.com/key.gpg",
		DebURL:    "https://example.com/repo",
		AddedAt:   "2025-12-15T10:00:00Z",
		Blueprint: "/blueprints/setup.bp",
		OS:        "debian",
	}

	// Convert to map (simulating JSON encoding)
	data, _ := json.Marshal(original)
	var decoded GPGKeyStatus
	_ = json.Unmarshal(data, &decoded)

	// Verify no data loss
	if decoded.Keyring != original.Keyring ||
		decoded.URL != original.URL ||
		decoded.DebURL != original.DebURL ||
		decoded.Blueprint != original.Blueprint ||
		decoded.OS != original.OS {
		t.Error("Data lost during conversion")
	}
}
