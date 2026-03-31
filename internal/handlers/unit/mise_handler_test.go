package unit

import (
	"testing"

	"github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/parser"
)

func TestMiseHandler_GetCommand_Pure(t *testing.T) {
	// Mock mise command to return consistent path across environments
	handlers.SetMiseCmdFunc(func() string { return "/Users/elpic/.local/bin/mise" })
	defer handlers.ResetMiseCmd()
	tests := []struct {
		name     string
		rule     parser.Rule
		expected string
	}{
		{
			name: "install single package globally",
			rule: parser.Rule{
				Action:       "mise",
				MisePackages: []string{"node@18.0.0"},
			},
			expected: "/Users/elpic/.local/bin/mise use -g node@18.0.0",
		},
		{
			name: "install multiple packages globally",
			rule: parser.Rule{
				Action:       "mise",
				MisePackages: []string{"node@18.0.0", "python@3.11.0"},
			},
			expected: "/Users/elpic/.local/bin/mise use -g node@18.0.0 && /Users/elpic/.local/bin/mise use -g python@3.11.0",
		},
		{
			name: "install package with project path",
			rule: parser.Rule{
				Action:       "mise",
				MisePackages: []string{"node@18.0.0"},
				MisePath:     "~/projects/myapp",
			},
			expected: "(in ~/projects/myapp) /Users/elpic/.local/bin/mise use node@18.0.0",
		},
		{
			name: "install multiple packages with project path",
			rule: parser.Rule{
				Action:       "mise",
				MisePackages: []string{"node@18.0.0", "ruby@3.2.1"},
				MisePath:     "/workspace/app",
			},
			expected: "(in /workspace/app) /Users/elpic/.local/bin/mise use node@18.0.0 && /Users/elpic/.local/bin/mise use ruby@3.2.1",
		},
		{
			name: "install package without version globally",
			rule: parser.Rule{
				Action:       "mise",
				MisePackages: []string{"golang"},
			},
			expected: "/Users/elpic/.local/bin/mise use -g golang",
		},
		{
			name: "install package without version with project",
			rule: parser.Rule{
				Action:       "mise",
				MisePackages: []string{"terraform"},
				MisePath:     "~/infra",
			},
			expected: "(in ~/infra) /Users/elpic/.local/bin/mise use terraform",
		},
		{
			name: "empty packages list",
			rule: parser.Rule{
				Action:       "mise",
				MisePackages: []string{},
			},
			expected: "mise-init",
		},
		{
			name: "uninstall action",
			rule: parser.Rule{
				Action:       "uninstall",
				MisePackages: []string{"node@18.0.0"},
			},
			expected: "mise uninstall",
		},
		{
			name: "mixed packages with and without versions",
			rule: parser.Rule{
				Action:       "mise",
				MisePackages: []string{"node@18.0.0", "python", "terraform@1.5.0"},
			},
			expected: "/Users/elpic/.local/bin/mise use -g node@18.0.0 && /Users/elpic/.local/bin/mise use -g python && /Users/elpic/.local/bin/mise use -g terraform@1.5.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlers.NewMiseHandler(tt.rule, "/test/path")
			result := handler.GetCommand()
			if result != tt.expected {
				t.Errorf("GetCommand() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestMiseHandler_GetDependencyKey_Pure(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		expected string
	}{
		{
			name: "uses rule ID when present for install",
			rule: parser.Rule{
				ID:     "custom-mise-setup",
				Action: "mise",
			},
			expected: "custom-mise-setup",
		},
		{
			name: "falls back to 'mise' for install",
			rule: parser.Rule{
				Action: "mise",
			},
			expected: "mise",
		},
		{
			name: "uses rule ID when present for uninstall",
			rule: parser.Rule{
				ID:           "custom-mise-setup",
				Action:       "uninstall",
				MisePackages: []string{"node@18"}, // Makes it detect as mise type
			},
			expected: "custom-mise-setup",
		},
		{
			name: "falls back to 'uninstall-mise' for uninstall",
			rule: parser.Rule{
				Action:       "uninstall",
				MisePackages: []string{"node@18"}, // Makes it detect as mise type
			},
			expected: "uninstall-mise",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlers.NewMiseHandler(tt.rule, "/test/path")
			result := handler.GetDependencyKey()
			if result != tt.expected {
				t.Errorf("GetDependencyKey() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMiseHandler_GetDisplayDetails_Pure(t *testing.T) {
	tests := []struct {
		name        string
		rule        parser.Rule
		isUninstall bool
		expected    string
	}{
		{
			name: "single package",
			rule: parser.Rule{
				MisePackages: []string{"node@18.0.0"},
			},
			isUninstall: false,
			expected:    "node@18.0.0",
		},
		{
			name: "multiple packages",
			rule: parser.Rule{
				MisePackages: []string{"node@18.0.0", "python@3.11.0", "ruby@3.2.1"},
			},
			isUninstall: false,
			expected:    "node@18.0.0, python@3.11.0, ruby@3.2.1",
		},
		{
			name: "empty packages list",
			rule: parser.Rule{
				MisePackages: []string{},
			},
			isUninstall: false,
			expected:    "mise",
		},
		{
			name: "single package for uninstall",
			rule: parser.Rule{
				MisePackages: []string{"golang@1.21.3"},
			},
			isUninstall: true,
			expected:    "golang@1.21.3",
		},
		{
			name: "packages without versions",
			rule: parser.Rule{
				MisePackages: []string{"terraform", "kubectl"},
			},
			isUninstall: false,
			expected:    "terraform, kubectl",
		},
		{
			name: "mixed packages with and without versions",
			rule: parser.Rule{
				MisePackages: []string{"node@18.0.0", "python", "terraform@1.5.0"},
			},
			isUninstall: false,
			expected:    "node@18.0.0, python, terraform@1.5.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlers.NewMiseHandler(tt.rule, "/test/path")
			result := handler.GetDisplayDetails(tt.isUninstall)
			if result != tt.expected {
				t.Errorf("GetDisplayDetails() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMiseHandler_GetState_Pure(t *testing.T) {
	tests := []struct {
		name        string
		rule        parser.Rule
		isUninstall bool
		expected    map[string]string
	}{
		{
			name: "single package",
			rule: parser.Rule{
				MisePackages: []string{"node@18.0.0"},
			},
			isUninstall: false,
			expected: map[string]string{
				"summary": "node@18.0.0",
				"tools":   "node@18.0.0",
			},
		},
		{
			name: "multiple packages",
			rule: parser.Rule{
				MisePackages: []string{"node@18.0.0", "python@3.11.0"},
			},
			isUninstall: false,
			expected: map[string]string{
				"summary": "node@18.0.0, python@3.11.0",
				"tools":   "node@18.0.0, python@3.11.0",
			},
		},
		{
			name: "empty packages list",
			rule: parser.Rule{
				MisePackages: []string{},
			},
			isUninstall: false,
			expected: map[string]string{
				"summary": "mise",
				"tools":   "",
			},
		},
		{
			name: "uninstall operation",
			rule: parser.Rule{
				MisePackages: []string{"ruby@3.2.1", "golang@1.21.3"},
			},
			isUninstall: true,
			expected: map[string]string{
				"summary": "ruby@3.2.1, golang@1.21.3",
				"tools":   "ruby@3.2.1, golang@1.21.3",
			},
		},
		{
			name: "packages without versions",
			rule: parser.Rule{
				MisePackages: []string{"terraform", "kubectl"},
			},
			isUninstall: false,
			expected: map[string]string{
				"summary": "terraform, kubectl",
				"tools":   "terraform, kubectl",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlers.NewMiseHandler(tt.rule, "/test/path")
			result := handler.GetState(tt.isUninstall)

			if len(result) != len(tt.expected) {
				t.Errorf("GetState() returned %d keys, want %d", len(result), len(tt.expected))
			}

			for key, expectedValue := range tt.expected {
				if actualValue, exists := result[key]; !exists {
					t.Errorf("GetState() missing key %v", key)
				} else if actualValue != expectedValue {
					t.Errorf("GetState()[%v] = %v, want %v", key, actualValue, expectedValue)
				}
			}
		})
	}
}

func TestMiseHandler_IsInstalled_Pure(t *testing.T) {
	// Create test status with various mise packages
	status := &handlers.Status{
		Mises: []handlers.MiseStatus{
			{
				Tool:      "node",
				Version:   "18.0.0",
				Blueprint: "/test/blueprint.yml",
				OS:        "linux",
			},
			{
				Tool:      "python",
				Version:   "3.11.0",
				Blueprint: "/test/blueprint.yml",
				OS:        "linux",
			},
			{
				Tool:      "ruby",
				Version:   "3.2.1",
				Blueprint: "/other/blueprint.yml", // Different blueprint
				OS:        "linux",
			},
			{
				Tool:      "golang",
				Version:   "1.21.3",
				Blueprint: "/test/blueprint.yml",
				OS:        "darwin", // Different OS
			},
			{
				Tool:      "terraform",
				Version:   "latest",
				Blueprint: "/test/blueprint.yml",
				OS:        "linux",
			},
		},
	}

	tests := []struct {
		name          string
		rule          parser.Rule
		blueprintFile string
		osName        string
		expected      bool
	}{
		{
			name: "all packages installed for matching blueprint and OS",
			rule: parser.Rule{
				MisePackages: []string{"node@18.0.0", "python@3.11.0"},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux",
			expected:      true,
		},
		{
			name: "single package installed",
			rule: parser.Rule{
				MisePackages: []string{"node@18.0.0"},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux",
			expected:      true,
		},
		{
			name: "package not installed",
			rule: parser.Rule{
				MisePackages: []string{"java@11.0.0"},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux",
			expected:      false,
		},
		{
			name: "package installed but wrong OS",
			rule: parser.Rule{
				MisePackages: []string{"golang@1.21.3"},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux", // golang is installed for darwin, not linux
			expected:      false,
		},
		{
			name: "package installed but wrong blueprint",
			rule: parser.Rule{
				MisePackages: []string{"ruby@3.2.1"},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux", // ruby is in /other/blueprint.yml, not /test/blueprint.yml
			expected:      false,
		},
		{
			name: "partially installed packages",
			rule: parser.Rule{
				MisePackages: []string{"node@18.0.0", "java@11.0.0"},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux",
			expected:      false, // node is installed, but java is not
		},
		{
			name: "empty packages list",
			rule: parser.Rule{
				MisePackages: []string{},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux",
			expected:      true, // empty list should return true
		},
		{
			name: "wrong version",
			rule: parser.Rule{
				MisePackages: []string{"node@19.0.0"},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux",
			expected:      false, // node@18.0.0 is installed, not node@19.0.0
		},
		{
			name: "package without version (matches latest)",
			rule: parser.Rule{
				MisePackages: []string{"terraform"},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux",
			expected:      false, // terraform has version "latest", but rule has empty version
		},
		{
			name: "package with exact latest version",
			rule: parser.Rule{
				MisePackages: []string{"terraform@latest"},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux",
			expected:      true, // terraform@latest matches
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlers.NewMiseHandler(tt.rule, "/test/path")
			result := handler.IsInstalled(status, tt.blueprintFile, tt.osName)
			if result != tt.expected {
				t.Errorf("IsInstalled() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMiseHandler_FindUninstallRules_Pure(t *testing.T) {
	// Create test status with various mise packages
	status := &handlers.Status{
		Mises: []handlers.MiseStatus{
			{
				Tool:      "node",
				Version:   "18.0.0",
				Blueprint: "/test/blueprint.yml",
				OS:        "linux",
			},
			{
				Tool:      "python",
				Version:   "3.11.0",
				Blueprint: "/test/blueprint.yml",
				OS:        "linux",
			},
			{
				Tool:      "ruby",
				Version:   "3.2.1",
				Blueprint: "/other/blueprint.yml", // Different blueprint
				OS:        "linux",
			},
			{
				Tool:      "golang",
				Version:   "1.21.3",
				Blueprint: "/test/blueprint.yml",
				OS:        "darwin", // Different OS
			},
		},
	}

	tests := []struct {
		name          string
		currentRules  []parser.Rule
		blueprintFile string
		osName        string
		expected      []string // Expected packages to uninstall
	}{
		{
			name:          "no current rules - all packages should be uninstalled",
			currentRules:  []parser.Rule{},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux",
			expected:      []string{"node@18.0.0", "python@3.11.0"},
		},
		{
			name: "some packages still in current rules",
			currentRules: []parser.Rule{
				{
					Action:       "mise",
					MisePackages: []string{"node@18.0.0"},
				},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux",
			expected:      []string{"python@3.11.0"}, // node is kept, python should be uninstalled
		},
		{
			name: "all packages still in current rules",
			currentRules: []parser.Rule{
				{
					Action:       "mise",
					MisePackages: []string{"node@18.0.0", "python@3.11.0"},
				},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux",
			expected:      []string{}, // Nothing to uninstall
		},
		{
			name:          "different OS - no packages to uninstall",
			currentRules:  []parser.Rule{},
			blueprintFile: "/test/blueprint.yml",
			osName:        "darwin", // Only golang@1.21.3 for darwin
			expected:      []string{"golang@1.21.3"},
		},
		{
			name:          "different blueprint - no packages to uninstall",
			currentRules:  []parser.Rule{},
			blueprintFile: "/other/blueprint.yml",
			osName:        "linux",
			expected:      []string{"ruby@3.2.1"}, // Only ruby belongs to this blueprint
		},
		{
			name: "mixed scenarios",
			currentRules: []parser.Rule{
				{
					Action:       "mise",
					MisePackages: []string{"node@18.0.0"},
				},
				{
					Action:       "mise",
					MisePackages: []string{"java@11.0.0"}, // New package not in status
				},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux",
			expected:      []string{"python@3.11.0"}, // python should be uninstalled, node is kept
		},
		{
			name: "package without version not in current rules",
			currentRules: []parser.Rule{
				{
					Action:       "mise",
					MisePackages: []string{"node"}, // matches node@18.0.0 in status by tool name
				},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux",
			expected:      []string{"python@3.11.0"}, // node is kept (tool name matches), python is orphan
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a handler (the specific rule doesn't matter for this test)
			rule := parser.Rule{Action: "mise"}
			handler := handlers.NewMiseHandler(rule, "/test/path")

			result := handler.FindUninstallRules(status, tt.currentRules, tt.blueprintFile, tt.osName)

			if len(tt.expected) == 0 {
				if len(result) != 0 {
					t.Errorf("FindUninstallRules() returned %d rules, want 0", len(result))
				}
				return
			}

			if len(result) != 1 {
				t.Errorf("FindUninstallRules() returned %d rules, want 1", len(result))
				return
			}

			uninstallRule := result[0]
			if uninstallRule.Action != "uninstall" {
				t.Errorf("FindUninstallRules() action = %v, want 'uninstall'", uninstallRule.Action)
			}

			if len(uninstallRule.MisePackages) != len(tt.expected) {
				t.Errorf("FindUninstallRules() packages count = %d, want %d", len(uninstallRule.MisePackages), len(tt.expected))
			}

			// Check that all expected packages are in the uninstall rule
			for _, expectedPkg := range tt.expected {
				found := false
				for _, actualPkg := range uninstallRule.MisePackages {
					if actualPkg == expectedPkg {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("FindUninstallRules() missing expected package %v", expectedPkg)
				}
			}
		})
	}
}

// Test helper function for path resolution logic
func TestMiseHandler_IsGlobal_Pure(t *testing.T) {
	// Mock mise command to return consistent path across environments
	handlers.SetMiseCmdFunc(func() string { return "/Users/elpic/.local/bin/mise" })
	defer handlers.ResetMiseCmd()

	tests := []struct {
		name     string
		misePath string
		expected bool
	}{
		{
			name:     "empty mise path is global",
			misePath: "",
			expected: true,
		},
		{
			name:     "specific path is not global",
			misePath: "~/projects/app",
			expected: false,
		},
		{
			name:     "absolute path is not global",
			misePath: "/workspace/myapp",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := parser.Rule{
				Action:       "mise",
				MisePath:     tt.misePath,
				MisePackages: []string{"node@18.0.0"}, // Add a package to get actual commands
			}
			handler := handlers.NewMiseHandler(rule, "/test/path")

			// We can't directly call isGlobal since it's not exported,
			// but we can test its behavior through GetCommand()
			cmd := handler.GetCommand()

			if tt.expected {
				// Global should contain "-g" flag
				if !contains(cmd, "-g") {
					t.Errorf("Expected global command to contain '-g', got: %s", cmd)
				}
			} else {
				// Non-global should contain the path in parentheses
				if !contains(cmd, "(in "+tt.misePath+")") {
					t.Errorf("Expected project-specific command to contain path, got: %s", cmd)
				}
			}
		})
	}
}

// Test constructor and basic properties
func TestMiseHandler_Constructor_Pure(t *testing.T) {
	// Mock mise command to return consistent path across environments
	handlers.SetMiseCmdFunc(func() string { return "/Users/elpic/.local/bin/mise" })
	defer handlers.ResetMiseCmd()

	rule := parser.Rule{
		Action:       "mise",
		MisePackages: []string{"node@18.0.0", "python@3.11.0"},
		MisePath:     "~/projects/app",
	}

	handler := handlers.NewMiseHandler(rule, "/test/base/path")

	// Test that the handler implements the expected interface methods
	if handler.GetCommand() == "" {
		t.Error("GetCommand() should return non-empty string")
	}

	if handler.GetDependencyKey() == "" {
		t.Error("GetDependencyKey() should return non-empty string")
	}

	if handler.GetDisplayDetails(false) == "" {
		t.Error("GetDisplayDetails() should return non-empty string")
	}

	state := handler.GetState(false)
	if len(state) == 0 {
		t.Error("GetState() should return non-empty map")
	}
}

// TestMiseHandler_UpdateStatus_MiseAction covers UpdateStatus for the mise action.
func TestMiseHandler_UpdateStatus_MiseAction(t *testing.T) {
	handlers.SetMiseCmdFunc(func() string { return "/usr/local/bin/mise" })
	defer handlers.ResetMiseCmd()

	rule := parser.Rule{
		Action:       "mise",
		MisePackages: []string{"node@18.0.0"},
	}
	handler := handlers.NewMiseHandler(rule, "/test")
	status := &handlers.Status{}

	cmd := handler.GetCommand()
	records := []handlers.ExecutionRecord{
		{Command: cmd, Status: "success"},
	}

	if err := handler.UpdateStatus(status, records, "/test/bp.yml", "linux"); err != nil {
		t.Fatalf("UpdateStatus() error: %v", err)
	}

	if len(status.Mises) != 1 {
		t.Fatalf("expected 1 mise entry, got %d", len(status.Mises))
	}
	if status.Mises[0].Tool != "node" || status.Mises[0].Version != "18.0.0" {
		t.Errorf("mise entry = %+v, want node@18.0.0", status.Mises[0])
	}
}

// TestMiseHandler_UpdateStatus_PackageWithoutVersion covers UpdateStatus for tool without @version.
func TestMiseHandler_UpdateStatus_PackageWithoutVersion(t *testing.T) {
	handlers.SetMiseCmdFunc(func() string { return "/usr/local/bin/mise" })
	defer handlers.ResetMiseCmd()

	rule := parser.Rule{
		Action:       "mise",
		MisePackages: []string{"terraform"},
	}
	handler := handlers.NewMiseHandler(rule, "/test")
	status := &handlers.Status{}

	cmd := handler.GetCommand()
	records := []handlers.ExecutionRecord{
		{Command: cmd, Status: "success"},
	}

	if err := handler.UpdateStatus(status, records, "/test/bp.yml", "linux"); err != nil {
		t.Fatalf("UpdateStatus() error: %v", err)
	}

	if len(status.Mises) != 1 {
		t.Fatalf("expected 1 mise entry, got %d", len(status.Mises))
	}
	if status.Mises[0].Tool != "terraform" || status.Mises[0].Version != "latest" {
		t.Errorf("mise entry = %+v, want terraform@latest", status.Mises[0])
	}
}

// TestMiseHandler_UpdateStatus_SkipsDuplicates verifies no duplicate is added on second apply.
func TestMiseHandler_UpdateStatus_SkipsDuplicates(t *testing.T) {
	handlers.SetMiseCmdFunc(func() string { return "/usr/local/bin/mise" })
	defer handlers.ResetMiseCmd()

	rule := parser.Rule{
		Action:       "mise",
		MisePackages: []string{"node@18.0.0"},
	}
	handler := handlers.NewMiseHandler(rule, "/test")

	status := &handlers.Status{
		Mises: []handlers.MiseStatus{
			{Tool: "node", Version: "18.0.0", Blueprint: "/test/bp.yml", OS: "linux"},
		},
	}

	cmd := handler.GetCommand()
	records := []handlers.ExecutionRecord{
		{Command: cmd, Status: "success"},
	}

	if err := handler.UpdateStatus(status, records, "/test/bp.yml", "linux"); err != nil {
		t.Fatalf("UpdateStatus() error: %v", err)
	}

	if len(status.Mises) != 1 {
		t.Errorf("expected 1 mise entry (no duplicate added), got %d", len(status.Mises))
	}
}

// TestMiseHandler_IsInstalled_MatchesWithVersion checks IsInstalled when version is present.
func TestMiseHandler_IsInstalled_ExactMatch(t *testing.T) {
	status := &handlers.Status{
		Mises: []handlers.MiseStatus{
			{Tool: "node", Version: "18.0.0", Blueprint: "/test/bp.yml", OS: "linux"},
		},
	}

	rule := parser.Rule{MisePackages: []string{"node@18.0.0"}}
	handler := handlers.NewMiseHandler(rule, "/test")

	if !handler.IsInstalled(status, "/test/bp.yml", "linux") {
		t.Error("IsInstalled() = false, want true (exact match)")
	}
}

// TestMiseHandler_ResolvedMisePath_TildeExpansion exercises resolvedMisePath via GetCommand.
func TestMiseHandler_ResolvedMisePath_TildeExpansion(t *testing.T) {
	handlers.SetMiseCmdFunc(func() string { return "/usr/local/bin/mise" })
	defer handlers.ResetMiseCmd()

	// A rule with a project path containing ~ — GetCommand should include the raw path
	rule := parser.Rule{
		Action:       "mise",
		MisePackages: []string{"node@18.0.0"},
		MisePath:     "~/projects/app",
	}
	handler := handlers.NewMiseHandler(rule, "/test")
	cmd := handler.GetCommand()
	if !contains(cmd, "~/projects/app") {
		t.Errorf("GetCommand() = %q, expected to contain '~/projects/app'", cmd)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s != substr && (len(substr) == 0 || findSubstring(s, substr) >= 0)
}

func findSubstring(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
