package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

// TestValidateBlueprintPath tests the validateBlueprintPath function
func TestValidateBlueprintPath(t *testing.T) {
	// Get the blueprint directory path for testing
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get home directory, skipping test")
	}
	blueprintDir := filepath.Join(homeDir, ".blueprint")

	tests := []struct {
		name      string
		filePath  string
		wantError bool
	}{
		{
			name:      "valid path within blueprint directory",
			filePath:  filepath.Join(blueprintDir, "status.json"),
			wantError: false,
		},
		{
			name:      "path traversal attempt",
			filePath:  filepath.Join(blueprintDir, "..", "..", "etc", "passwd"),
			wantError: true,
		},
		{
			name:      "another path traversal attempt",
			filePath:  "/etc/passwd",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBlueprintPath(tt.filePath)
			if (err != nil) != tt.wantError {
				t.Errorf("validateBlueprintPath(%q) error = %v, wantError %v", tt.filePath, err, tt.wantError)
			}
		})
	}
}

// TestReadBlueprintFile tests the readBlueprintFile function
func TestReadBlueprintFile(t *testing.T) {
	// Get the blueprint directory path for testing
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get home directory, skipping test")
	}
	blueprintDir := filepath.Join(homeDir, ".blueprint")

	// Create a test file in the blueprint directory
	testFile := filepath.Join(blueprintDir, "test-validate.bp")
	testContent := []byte("test content")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer func() { _ = os.Remove(testFile) }()

	tests := []struct {
		name      string
		filePath  string
		wantError bool
	}{
		{
			name:      "read existing file",
			filePath:  testFile,
			wantError: false,
		},
		{
			name:      "read non-existent file",
			filePath:  filepath.Join(blueprintDir, "nonexistent.bp"),
			wantError: true,
		},
		{
			name:      "path traversal attempt",
			filePath:  "/etc/passwd",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := readBlueprintFile(tt.filePath)
			if (err != nil) != tt.wantError {
				t.Errorf("readBlueprintFile(%q) error = %v, wantError %v", tt.filePath, err, tt.wantError)
			}
			if !tt.wantError && string(data) != string(testContent) {
				t.Errorf("readBlueprintFile(%q) = %q, want %q", tt.filePath, data, testContent)
			}
		})
	}
}

// TestSaveHistory tests the saveHistory function
func TestSaveHistory(t *testing.T) {
	tests := []struct {
		name      string
		records   []ExecutionRecord
		wantError bool
	}{
		{
			name:      "save empty records",
			records:   []ExecutionRecord{},
			wantError: false, // Empty records should return nil (no-op)
		},
		{
			name: "save single record",
			records: []ExecutionRecord{
				{
					Timestamp: "2024-01-15T10:30:00Z",
					Blueprint: "/tmp/test.bp",
					OS:        "linux",
					Command:   "echo hello",
					Output:    "hello",
					Status:    "success",
				},
			},
			wantError: false,
		},
		{
			name: "save multiple records",
			records: []ExecutionRecord{
				{Command: "cmd1", Status: "success"},
				{Command: "cmd2", Status: "success"},
				{Command: "cmd3", Status: "error", Error: "failed"},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := saveHistory(tt.records)
			if (err != nil) != tt.wantError {
				t.Errorf("saveHistory() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// TestGetAutoUninstallRules tests the getAutoUninstallRules function
func TestGetAutoUninstallRules(t *testing.T) {
	tests := []struct {
		name          string
		currentRules  []parser.Rule
		blueprintFile string
		osName        string
	}{
		{
			name:          "empty rules",
			currentRules:  []parser.Rule{},
			blueprintFile: "/tmp/test.bp",
			osName:        "linux",
		},
		{
			name: "single install rule",
			currentRules: []parser.Rule{
				{Action: "install", Packages: []parser.Package{{Name: "vim"}}},
			},
			blueprintFile: "/tmp/test.bp",
			osName:        "linux",
		},
		{
			name: "multiple rules",
			currentRules: []parser.Rule{
				{Action: "install", Packages: []parser.Package{{Name: "vim"}}},
				{Action: "mkdir", Mkdir: "~/projects"},
				{Action: "clone", ClonePath: "~/repo", CloneURL: "https://github.com/user/repo"},
			},
			blueprintFile: "/tmp/test.bp",
			osName:        "linux",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This should not panic - may return nil if no status file exists
			_ = getAutoUninstallRules(tt.currentRules, tt.blueprintFile, tt.osName)
		})
	}
}

// TestSaveStatus tests the saveStatus function
func TestSaveStatus(t *testing.T) {
	// Create test records
	records := []ExecutionRecord{
		{
			Timestamp: "2024-01-15T10:30:00Z",
			Blueprint: "/tmp/test.bp",
			OS:        "linux",
			Command:   "echo hello",
			Output:    "hello",
			Status:    "success",
		},
	}

	// Test saving status - may fail but should not panic
	_ = saveStatus(nil, records, "/tmp/test.bp", "", "linux")
}

// TestSaveRuleOutput tests the saveRuleOutput function
func TestSaveRuleOutput(t *testing.T) {
	// Get a run number first
	runNum, _ := getNextRunNumber()
	err := saveRuleOutput(runNum, 1, "success", "")
	if err != nil {
		t.Errorf("saveRuleOutput() error = %v, expected nil", err)
	}
}

// TestGetNextRunNumber tests the getNextRunNumber function
func TestGetNextRunNumber(t *testing.T) {
	// First call should return 1 or higher
	num, err := getNextRunNumber()
	if err != nil {
		t.Errorf("getNextRunNumber() error = %v, expected nil", err)
	}
	if num < 1 {
		t.Errorf("getNextRunNumber() = %d, expected >= 1", num)
	}

	// Second call should return higher number
	num2, err := getNextRunNumber()
	if err != nil {
		t.Errorf("getNextRunNumber() error = %v, expected nil", err)
	}
	if num2 <= num {
		t.Errorf("getNextRunNumber() = %d, expected > %d", num2, num)
	}
}

// TestGetLatestRunNumber tests the getLatestRunNumber function
func TestGetLatestRunNumber(t *testing.T) {
	// With no history, should return 0
	num, err := getLatestRunNumber()
	if err != nil {
		t.Errorf("getLatestRunNumber() error = %v, expected nil", err)
	}
	// Number should be >= 0
	if num < 0 {
		t.Errorf("getLatestRunNumber() = %d, expected >= 0", num)
	}
}
