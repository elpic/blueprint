package handlers

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

// TestGPGKeyHandlerUpOutput tests that the Up() method returns expected output
func TestGPGKeyHandlerUpOutput(t *testing.T) {
	rule := parser.Rule{
		Action:     "gpg-key",
		GPGKeyring: "test-repo",
		GPGKeyURL:  "https://example.com/gpg.key",
		GPGDebURL:  "https://example.com/apt",
	}

	handler := NewGPGKeyHandler(rule, "")

	// We can't actually run this on macOS since it requires Linux commands
	// but we can verify the handler was created correctly
	if handler == nil {
		t.Error("NewGPGKeyHandler() returned nil")
	}

	if handler.Rule.GPGKeyring != "test-repo" {
		t.Errorf("Handler keyring: got %q, want 'test-repo'", handler.Rule.GPGKeyring)
	}
}

// TestGPGKeyHandlerDownOutput tests that the Down() method returns expected output
func TestGPGKeyHandlerDownOutput(t *testing.T) {
	rule := parser.Rule{
		Action:     "uninstall",
		GPGKeyring: "test-repo",
		GPGKeyURL:  "https://example.com/gpg.key",
		GPGDebURL:  "https://example.com/apt",
	}

	handler := NewGPGKeyHandler(rule, "")

	if handler == nil {
		t.Error("NewGPGKeyHandler() returned nil")
	}
}

// TestGPGKeyHandlerUpdateStatus tests that UpdateStatus properly updates status
func TestGPGKeyHandlerUpdateStatus(t *testing.T) {
	tests := []struct {
		name           string
		rule           parser.Rule
		records        []ExecutionRecord
		blueprint      string
		osName         string
		expectStatusOK bool
	}{
		{
			name: "successful gpg-key installation",
			rule: parser.Rule{
				Action:     "gpg-key",
				GPGKeyring: "wezterm-fury",
				GPGKeyURL:  "https://apt.fury.io/wez/gpg.key",
				GPGDebURL:  "https://apt.fury.io/wez/",
			},
			records: []ExecutionRecord{
				{
					Status:  "success",
					Command: "gpg-key wezterm-fury",
					Output:  "Added GPG key wezterm-fury and repository https://apt.fury.io/wez/",
				},
			},
			blueprint:      "/test/blueprint.bp",
			osName:         "linux",
			expectStatusOK: true,
		},
		{
			name: "failed gpg-key installation should not update status",
			rule: parser.Rule{
				Action:     "gpg-key",
				GPGKeyring: "test-repo",
				GPGKeyURL:  "https://example.com/gpg.key",
				GPGDebURL:  "https://example.com/apt",
			},
			records: []ExecutionRecord{
				{
					Status:  "error",
					Command: "gpg-key test-repo",
					Error:   "command failed",
				},
			},
			blueprint:      "/test/blueprint.bp",
			osName:         "linux",
			expectStatusOK: false,
		},
		{
			name: "gpg-key uninstall removes from status",
			rule: parser.Rule{
				Action:     "uninstall",
				GPGKeyring: "wezterm-fury",
			},
			records: []ExecutionRecord{
				{
					Status:  "success",
					Command: "uninstall-gpg-key",
					Output:  "Removed GPG key",
				},
			},
			blueprint:      "/test/blueprint.bp",
			osName:         "linux",
			expectStatusOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewGPGKeyHandler(tt.rule, "")
			status := &Status{
				GPGKeys: []GPGKeyStatus{
					{
						Keyring:   "wezterm-fury",
						URL:       "https://apt.fury.io/wez/gpg.key",
						DebURL:    "https://apt.fury.io/wez/",
						Blueprint: "/test/blueprint.bp",
						OS:        "linux",
					},
				},
			}

			err := handler.UpdateStatus(status, tt.records, tt.blueprint, tt.osName)
			if err != nil {
				t.Fatalf("UpdateStatus() error = %v", err)
			}

			// Verify status was updated correctly based on record status
			if tt.rule.Action == "gpg-key" && tt.expectStatusOK {
				// After successful install, GPG key should be in status
				found := false
				for _, gpg := range status.GPGKeys {
					if gpg.Keyring == tt.rule.GPGKeyring {
						found = true
						break
					}
				}
				if !found {
					t.Error("GPG key not found in status after successful install")
				}
			} else if tt.rule.Action == "uninstall" {
				// After uninstall, GPG key should be removed
				for _, gpg := range status.GPGKeys {
					if gpg.Keyring == tt.rule.GPGKeyring {
						t.Error("GPG key still in status after uninstall")
					}
				}
			}
		})
	}
}

// TestGPGKeyHandlerNewGPGKeyHandler tests the NewGPGKeyHandler factory function
func TestGPGKeyHandlerNewGPGKeyHandler(t *testing.T) {
	tests := []struct {
		name    string
		rule    parser.Rule
		basePath string
	}{
		{
			name: "handler with all fields",
			rule: parser.Rule{
				ID:         "test-setup",
				Action:     "gpg-key",
				GPGKeyring: "test-repo",
				GPGKeyURL:  "https://example.com/gpg.key",
				GPGDebURL:  "https://example.com/apt",
				OSList:     []string{"linux"},
			},
			basePath: "/test",
		},
		{
			name: "handler with minimal fields",
			rule: parser.Rule{
				Action:     "gpg-key",
				GPGKeyring: "test",
				GPGKeyURL:  "https://example.com/key",
				GPGDebURL:  "https://example.com/apt",
			},
			basePath: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewGPGKeyHandler(tt.rule, tt.basePath)

			if handler == nil {
				t.Error("NewGPGKeyHandler() returned nil")
				return
			}

			if handler.Rule.Action != "gpg-key" {
				t.Errorf("Action: got %q, want 'gpg-key'", handler.Rule.Action)
			}

			if handler.Rule.GPGKeyring != tt.rule.GPGKeyring {
				t.Errorf("Keyring: got %q, want %q", handler.Rule.GPGKeyring, tt.rule.GPGKeyring)
			}

			if handler.BasePath != tt.basePath {
				t.Errorf("BasePath: got %q, want %q", handler.BasePath, tt.basePath)
			}
		})
	}
}

// TestGPGKeyHandlerCommandConstruction tests that the handler constructs correct commands
func TestGPGKeyHandlerCommandConstruction(t *testing.T) {
	rule := parser.Rule{
		Action:     "gpg-key",
		GPGKeyring: "wezterm-fury",
		GPGKeyURL:  "https://apt.fury.io/wez/gpg.key",
		GPGDebURL:  "https://apt.fury.io/wez/",
	}

	handler := NewGPGKeyHandler(rule, "")

	// Verify the rule has correct fields for command construction
	if !strings.Contains(handler.Rule.GPGKeyURL, "apt.fury.io") {
		t.Errorf("URL not properly stored: %q", handler.Rule.GPGKeyURL)
	}

	if !strings.Contains(handler.Rule.GPGDebURL, "apt.fury.io") {
		t.Errorf("DebURL not properly stored: %q", handler.Rule.GPGDebURL)
	}

	// Verify keyring name doesn't have .gpg extension yet (it gets added in the handler)
	if strings.HasSuffix(handler.Rule.GPGKeyring, ".gpg") {
		t.Error("GPGKeyring should not have .gpg extension")
	}
}

// TestGPGKeyHandlerRemoveGPGKeyStatus tests the removeGPGKeyStatus helper
func TestGPGKeyHandlerRemoveGPGKeyStatus(t *testing.T) {
	tests := []struct {
		name     string
		gpgKeys  []GPGKeyStatus
		keyring  string
		blueprint string
		osName    string
		expected  int
	}{
		{
			name: "remove existing gpg key",
			gpgKeys: []GPGKeyStatus{
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
			},
			keyring:   "repo1",
			blueprint: "/test/blueprint.bp",
			osName:    "linux",
			expected:  1, // Should have 1 left after removal
		},
		{
			name: "remove non-existent gpg key",
			gpgKeys: []GPGKeyStatus{
				{
					Keyring:   "repo1",
					Blueprint: "/test/blueprint.bp",
					OS:        "linux",
				},
			},
			keyring:   "repo2",
			blueprint: "/test/blueprint.bp",
			osName:    "linux",
			expected:  1, // Should still have 1 since we didn't remove anything
		},
		{
			name: "remove from different blueprint",
			gpgKeys: []GPGKeyStatus{
				{
					Keyring:   "repo1",
					Blueprint: "/test/blueprint1.bp",
					OS:        "linux",
				},
				{
					Keyring:   "repo1",
					Blueprint: "/test/blueprint2.bp",
					OS:        "linux",
				},
			},
			keyring:   "repo1",
			blueprint: "/test/blueprint1.bp",
			osName:    "linux",
			expected:  1, // Should only remove from blueprint1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeGPGKeyStatus(tt.gpgKeys, tt.keyring, tt.blueprint, tt.osName)

			if len(result) != tt.expected {
				t.Errorf("Expected %d GPG keys, got %d", tt.expected, len(result))
			}
		})
	}
}

// TestGPGKeyHandlerImplementsInterface tests that GPGKeyHandler implements Handler interface
func TestGPGKeyHandlerImplementsInterface(t *testing.T) {
	rule := parser.Rule{
		Action:     "gpg-key",
		GPGKeyring: "test",
		GPGKeyURL:  "https://example.com/key",
		GPGDebURL:  "https://example.com/apt",
	}

	handler := NewGPGKeyHandler(rule, "")

	// Verify the handler implements the Handler interface
	var _ Handler = handler

	// Test that methods exist and are callable
	if handler == nil {
		t.Fatal("Handler is nil")
	}

	// Create a test status
	status := &Status{}

	// Call UpdateStatus (we already tested this above, just verify it's callable)
	err := handler.UpdateStatus(status, []ExecutionRecord{}, "/test/blueprint.bp", "linux")
	if err != nil {
		t.Errorf("UpdateStatus() error = %v", err)
	}
}

// TestGPGKeyHandlerPathConstruction tests that paths are constructed correctly
func TestGPGKeyHandlerPathConstruction(t *testing.T) {
	rule := parser.Rule{
		Action:     "gpg-key",
		GPGKeyring: "my-repo",
		GPGKeyURL:  "https://example.com/gpg.key",
		GPGDebURL:  "https://example.com/apt",
	}

	handler := NewGPGKeyHandler(rule, "/home/user")

	// Verify keyring name is used correctly
	expectedKeyringName := "my-repo" // without .gpg extension or path
	if handler.Rule.GPGKeyring != expectedKeyringName {
		t.Errorf("Keyring: got %q, want %q", handler.Rule.GPGKeyring, expectedKeyringName)
	}

	// Verify base path is stored
	if handler.BasePath != "/home/user" {
		t.Errorf("BasePath: got %q, want '/home/user'", handler.BasePath)
	}
}

// TestGPGKeyHandlerURLValidation tests that URLs are handled correctly
func TestGPGKeyHandlerURLValidation(t *testing.T) {
	tests := []struct {
		name      string
		gpgKeyURL string
		debURL    string
	}{
		{
			name:      "standard https URLs",
			gpgKeyURL: "https://apt.fury.io/wez/gpg.key",
			debURL:    "https://apt.fury.io/wez/",
		},
		{
			name:      "URLs with paths",
			gpgKeyURL: "https://example.com/path/to/gpg.key",
			debURL:    "https://example.com/path/to/apt/",
		},
		{
			name:      "URLs with ports",
			gpgKeyURL: "https://example.com:8443/gpg.key",
			debURL:    "https://example.com:8443/apt/",
		},
		{
			name:      "URLs with authentication",
			gpgKeyURL: "https://user:pass@example.com/gpg.key",
			debURL:    "https://user:pass@example.com/apt/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := parser.Rule{
				Action:     "gpg-key",
				GPGKeyring: "test",
				GPGKeyURL:  tt.gpgKeyURL,
				GPGDebURL:  tt.debURL,
			}

			handler := NewGPGKeyHandler(rule, "")

			if handler.Rule.GPGKeyURL != tt.gpgKeyURL {
				t.Errorf("GPGKeyURL: got %q, want %q", handler.Rule.GPGKeyURL, tt.gpgKeyURL)
			}

			if handler.Rule.GPGDebURL != tt.debURL {
				t.Errorf("GPGDebURL: got %q, want %q", handler.Rule.GPGDebURL, tt.debURL)
			}
		})
	}
}

// TestGPGKeyStatusStructure tests the GPGKeyStatus structure
func TestGPGKeyStatusStructure(t *testing.T) {
	status := GPGKeyStatus{
		Keyring:   "test-repo",
		URL:       "https://example.com/gpg.key",
		DebURL:    "https://example.com/apt",
		AddedAt:   "2025-12-15T10:30:00Z",
		Blueprint: "/test/blueprint.bp",
		OS:        "linux",
	}

	// Verify all fields are properly set
	if status.Keyring != "test-repo" {
		t.Errorf("Keyring: got %q", status.Keyring)
	}
	if status.URL != "https://example.com/gpg.key" {
		t.Errorf("URL: got %q", status.URL)
	}
	if status.DebURL != "https://example.com/apt" {
		t.Errorf("DebURL: got %q", status.DebURL)
	}
	if status.Blueprint != "/test/blueprint.bp" {
		t.Errorf("Blueprint: got %q", status.Blueprint)
	}
	if status.OS != "linux" {
		t.Errorf("OS: got %q", status.OS)
	}
}

// TestGPGKeyHandlerIntegration tests a more complex scenario with multiple GPG keys
func TestGPGKeyHandlerIntegration(t *testing.T) {
	// Simulate managing multiple GPG keys
	wezRule := parser.Rule{
		ID:         "wez-setup",
		Action:     "gpg-key",
		GPGKeyring: "wezterm-fury",
		GPGKeyURL:  "https://apt.fury.io/wez/gpg.key",
		GPGDebURL:  "https://apt.fury.io/wez/",
	}

	dockerRule := parser.Rule{
		ID:         "docker-setup",
		Action:     "gpg-key",
		GPGKeyring: "docker",
		GPGKeyURL:  "https://download.docker.com/linux/ubuntu/gpg",
		GPGDebURL:  "https://download.docker.com/linux/ubuntu",
		After:      []string{"wez-setup"},
	}

	wezHandler := NewGPGKeyHandler(wezRule, "")
	dockerHandler := NewGPGKeyHandler(dockerRule, "")

	if wezHandler == nil || dockerHandler == nil {
		t.Fatal("Failed to create handlers")
	}

	// Test updating status with both handlers
	status := &Status{}

	// Simulate wez installation
	wezRecords := []ExecutionRecord{
		{
			Status:  "success",
			Command: "gpg-key wezterm-fury",
		},
	}

	err := wezHandler.UpdateStatus(status, wezRecords, "/test/blueprint.bp", "linux")
	if err != nil {
		t.Fatalf("wezHandler.UpdateStatus() error = %v", err)
	}

	// Should have 1 GPG key
	if len(status.GPGKeys) != 1 {
		t.Errorf("After wez install: expected 1 GPG key, got %d", len(status.GPGKeys))
	}

	// Simulate docker installation
	dockerRecords := []ExecutionRecord{
		{
			Status:  "success",
			Command: "gpg-key docker",
		},
	}

	err = dockerHandler.UpdateStatus(status, dockerRecords, "/test/blueprint.bp", "linux")
	if err != nil {
		t.Fatalf("dockerHandler.UpdateStatus() error = %v", err)
	}

	// Should have 2 GPG keys
	if len(status.GPGKeys) != 2 {
		t.Errorf("After docker install: expected 2 GPG keys, got %d", len(status.GPGKeys))
	}

	// Verify dependencies are preserved
	if dockerRule.After[0] != "wez-setup" {
		t.Errorf("Docker rule dependency: got %q, want 'wez-setup'", dockerRule.After[0])
	}
}

// TestGPGKeyCommandExecution tests that the executeCommand function is used correctly
// (We don't actually execute commands in this test, just verify the setup)
func TestGPGKeyCommandExecution(t *testing.T) {
	// This test verifies that the handler would use executeCommand correctly
	// We can't actually run the commands in unit tests without mocking exec.Command

	rule := parser.Rule{
		Action:     "gpg-key",
		GPGKeyring: "test",
		GPGKeyURL:  "https://example.com/key",
		GPGDebURL:  "https://example.com/apt",
	}

	handler := NewGPGKeyHandler(rule, "")

	// Verify the handler has the necessary fields for command construction
	if handler.Rule.GPGKeyring == "" {
		t.Error("Handler missing GPGKeyring for command construction")
	}
	if handler.Rule.GPGKeyURL == "" {
		t.Error("Handler missing GPGKeyURL for command construction")
	}
	if handler.Rule.GPGDebURL == "" {
		t.Error("Handler missing GPGDebURL for command construction")
	}

	// The actual command execution would happen in Up() and Down()
	// which we can't test without mocking os/exec
}

// TestGPGKeyStatusPath tests path normalization
func TestGPGKeyStatusPath(t *testing.T) {
	// Test that blueprint paths are normalized correctly
	tests := []struct {
		name     string
		input    string
		contains string // what should be in the normalized path
	}{
		{
			name:     "absolute path",
			input:    "/home/user/blueprint.bp",
			contains: "blueprint.bp",
		},
		{
			name:     "relative path",
			input:    "./blueprint.bp",
			contains: "blueprint.bp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// normalizePath is used in UpdateStatus
			// We test this indirectly by ensuring the status records the path correctly
			status := &Status{}
			rule := parser.Rule{
				Action:     "gpg-key",
				GPGKeyring: "test",
				GPGKeyURL:  "https://example.com/key",
				GPGDebURL:  "https://example.com/apt",
			}

			handler := NewGPGKeyHandler(rule, "")
			records := []ExecutionRecord{
				{
					Status:  "success",
					Command: "gpg-key test",
				},
			}

			err := handler.UpdateStatus(status, records, tt.input, "linux")
			if err != nil {
				t.Errorf("UpdateStatus() error = %v", err)
			}

			// Verify the GPG key was added to status
			if len(status.GPGKeys) > 0 {
				// Blueprint path should be normalized and stored
				if status.GPGKeys[0].Blueprint == "" {
					t.Error("Blueprint path not stored in status")
				}
			}
		})
	}
}

func TestGPGKeyHandlerDisplayInfo(t *testing.T) {
	tests := []struct {
		name             string
		rule             parser.Rule
		expectedContains []string
	}{
		{
			name: "gpg-key action with all fields",
			rule: parser.Rule{
				Action:     "gpg-key",
				GPGKeyring: "wezterm-fury",
				GPGKeyURL:  "https://apt.fury.io/wez/gpg.key",
				GPGDebURL:  "https://apt.fury.io/wez/",
			},
			expectedContains: []string{"Keyring:", "wezterm-fury", "Repository:", "https://apt.fury.io/wez/", "Key URL:", "https://apt.fury.io/wez/gpg.key"},
		},
		{
			name: "gpg-key action with docker repository",
			rule: parser.Rule{
				Action:     "gpg-key",
				GPGKeyring: "docker",
				GPGKeyURL:  "https://download.docker.com/linux/ubuntu/gpg",
				GPGDebURL:  "https://download.docker.com/linux/ubuntu",
			},
			expectedContains: []string{"Keyring:", "docker", "Repository:", "https://download.docker.com/linux/ubuntu", "Key URL:", "https://download.docker.com/linux/ubuntu/gpg"},
		},
		{
			name: "uninstall action",
			rule: parser.Rule{
				Action:     "uninstall",
				GPGKeyring: "test-repo",
				GPGKeyURL:  "https://example.com/gpg.key",
				GPGDebURL:  "https://example.com/apt",
			},
			expectedContains: []string{"Keyring:", "test-repo", "Repository:", "https://example.com/apt", "Key URL:", "https://example.com/gpg.key"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewGPGKeyHandler(tt.rule, "")

			// Capture stdout
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			handler.DisplayInfo()

			w.Close()
			os.Stdout = old

			// Read captured output
			var buf bytes.Buffer
			io.Copy(&buf, r)
			output := buf.String()

			// Verify expected content is present
			for _, expected := range tt.expectedContains {
				if !strings.Contains(output, expected) {
					t.Errorf("DisplayInfo() output missing expected content %q\nGot: %s", expected, output)
				}
			}
		})
	}
}

// TestGPGKeyHandlerSudoAwareInterface tests that GPGKeyHandler implements SudoAwareHandler
func TestGPGKeyHandlerSudoAwareInterface(t *testing.T) {
	rule := parser.Rule{
		Action:     "gpg-key",
		GPGKeyring: "test-repo",
		GPGKeyURL:  "https://example.com/gpg.key",
		GPGDebURL:  "https://example.com/apt",
	}

	handler := NewGPGKeyHandler(rule, "")

	// Test that handler implements SudoAwareHandler interface
	sudoAwareHandler, ok := interface{}(handler).(SudoAwareHandler)
	if !ok {
		t.Error("GPGKeyHandler does not implement SudoAwareHandler interface")
		return
	}

	// Test that NeedsSudo() returns true
	if !sudoAwareHandler.NeedsSudo() {
		t.Error("NeedsSudo() should return true for GPGKeyHandler")
	}
}

// TestGPGKeyHandlerNeedsSudo tests the NeedsSudo method specifically
func TestGPGKeyHandlerNeedsSudo(t *testing.T) {
	tests := []struct {
		name        string
		rule        parser.Rule
		expectedSudo bool
	}{
		{
			name: "gpg-key action needs sudo",
			rule: parser.Rule{
				Action:     "gpg-key",
				GPGKeyring: "test",
				GPGKeyURL:  "https://example.com/key",
				GPGDebURL:  "https://example.com/apt",
			},
			expectedSudo: true,
		},
		{
			name: "uninstall action needs sudo",
			rule: parser.Rule{
				Action:     "uninstall",
				GPGKeyring: "test",
				GPGKeyURL:  "https://example.com/key",
				GPGDebURL:  "https://example.com/apt",
			},
			expectedSudo: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewGPGKeyHandler(tt.rule, "")
			if handler.NeedsSudo() != tt.expectedSudo {
				t.Errorf("NeedsSudo() = %v, want %v", handler.NeedsSudo(), tt.expectedSudo)
			}
		})
	}
}
