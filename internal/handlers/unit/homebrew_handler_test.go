package unit

import (
	"testing"

	"github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/parser"
)

func TestHomebrewHandler_GetCommand_Pure(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		expected string
	}{
		{
			name: "install single formula",
			rule: parser.Rule{
				Action:           "homebrew",
				HomebrewPackages: []string{"git"},
			},
			expected: "brew install git",
		},
		{
			name: "install multiple formulas",
			rule: parser.Rule{
				Action:           "homebrew",
				HomebrewPackages: []string{"git", "node", "python"},
			},
			expected: "brew install git node python",
		},
		{
			name: "install single cask",
			rule: parser.Rule{
				Action:        "homebrew",
				HomebrewCasks: []string{"docker"},
			},
			expected: "brew install --cask docker",
		},
		{
			name: "install multiple casks",
			rule: parser.Rule{
				Action:        "homebrew",
				HomebrewCasks: []string{"docker", "visual-studio-code"},
			},
			expected: "brew install --cask docker visual-studio-code",
		},
		{
			name: "install both formulas and casks",
			rule: parser.Rule{
				Action:           "homebrew",
				HomebrewPackages: []string{"git", "node"},
				HomebrewCasks:    []string{"docker", "slack"},
			},
			expected: "brew install git node && brew install --cask docker slack",
		},
		{
			name: "install formula with version",
			rule: parser.Rule{
				Action:           "homebrew",
				HomebrewPackages: []string{"node@18", "python@3.11"},
			},
			expected: "brew install node@18 python@3.11",
		},
		{
			name: "empty packages and casks",
			rule: parser.Rule{
				Action:           "homebrew",
				HomebrewPackages: []string{},
				HomebrewCasks:    []string{},
			},
			expected: "",
		},
		{
			name: "uninstall single formula",
			rule: parser.Rule{
				Action:           "uninstall",
				HomebrewPackages: []string{"git"},
			},
			expected: "brew uninstall -y git",
		},
		{
			name: "uninstall multiple formulas",
			rule: parser.Rule{
				Action:           "uninstall",
				HomebrewPackages: []string{"git", "node"},
			},
			expected: "brew uninstall -y git node",
		},
		{
			name: "uninstall single cask",
			rule: parser.Rule{
				Action:        "uninstall",
				HomebrewCasks: []string{"docker"},
			},
			expected: "brew uninstall --cask -y docker",
		},
		{
			name: "uninstall both formulas and casks",
			rule: parser.Rule{
				Action:           "uninstall",
				HomebrewPackages: []string{"git"},
				HomebrewCasks:    []string{"docker"},
			},
			expected: "brew uninstall -y git && brew uninstall --cask -y docker",
		},
		{
			name: "uninstall formula with version (strips version)",
			rule: parser.Rule{
				Action:           "uninstall",
				HomebrewPackages: []string{"node@18", "python@3.11"},
			},
			expected: "brew uninstall -y node python",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlers.NewHomebrewHandler(tt.rule, "/test/path")
			result := handler.GetCommand()
			if result != tt.expected {
				t.Errorf("GetCommand() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestHomebrewHandler_NeedsSudo_Pure(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		expected bool
	}{
		{
			name: "formulas only - no sudo needed",
			rule: parser.Rule{
				HomebrewPackages: []string{"git", "node"},
			},
			expected: false,
		},
		{
			name: "casks present - sudo needed",
			rule: parser.Rule{
				HomebrewCasks: []string{"docker"},
			},
			expected: true,
		},
		{
			name: "both formulas and casks - sudo needed",
			rule: parser.Rule{
				HomebrewPackages: []string{"git"},
				HomebrewCasks:    []string{"docker"},
			},
			expected: true,
		},
		{
			name: "empty packages and casks - no sudo needed",
			rule: parser.Rule{
				HomebrewPackages: []string{},
				HomebrewCasks:    []string{},
			},
			expected: false,
		},
		{
			name: "multiple casks - sudo needed",
			rule: parser.Rule{
				HomebrewCasks: []string{"docker", "visual-studio-code", "slack"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlers.NewHomebrewHandler(tt.rule, "/test/path")
			result := handler.NeedsSudo()
			if result != tt.expected {
				t.Errorf("NeedsSudo() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestHomebrewHandler_GetDependencyKey_Pure(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		expected string
	}{
		{
			name: "uses rule ID when present",
			rule: parser.Rule{
				ID:               "custom-brew-setup",
				HomebrewPackages: []string{"git"},
			},
			expected: "custom-brew-setup",
		},
		{
			name: "falls back to first formula when no ID",
			rule: parser.Rule{
				HomebrewPackages: []string{"git", "node"},
			},
			expected: "git",
		},
		{
			name: "falls back to 'homebrew' when no packages",
			rule: parser.Rule{
				HomebrewPackages: []string{},
			},
			expected: "homebrew",
		},
		{
			name: "prefers first formula over casks",
			rule: parser.Rule{
				HomebrewPackages: []string{"git"},
				HomebrewCasks:    []string{"docker"},
			},
			expected: "git",
		},
		{
			name: "formula with version",
			rule: parser.Rule{
				HomebrewPackages: []string{"node@18"},
			},
			expected: "node@18",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlers.NewHomebrewHandler(tt.rule, "/test/path")
			result := handler.GetDependencyKey()
			if result != tt.expected {
				t.Errorf("GetDependencyKey() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestHomebrewHandler_GetDisplayDetails_Pure(t *testing.T) {
	tests := []struct {
		name        string
		rule        parser.Rule
		isUninstall bool
		expected    string
	}{
		{
			name: "formulas only",
			rule: parser.Rule{
				HomebrewPackages: []string{"git", "node"},
			},
			isUninstall: false,
			expected:    "git, node",
		},
		{
			name: "casks only",
			rule: parser.Rule{
				HomebrewCasks: []string{"docker", "slack"},
			},
			isUninstall: false,
			expected:    "cask: docker, slack",
		},
		{
			name: "both formulas and casks",
			rule: parser.Rule{
				HomebrewPackages: []string{"git", "node"},
				HomebrewCasks:    []string{"docker", "slack"},
			},
			isUninstall: false,
			expected:    "git, node | cask: docker, slack",
		},
		{
			name: "empty packages and casks",
			rule: parser.Rule{
				HomebrewPackages: []string{},
				HomebrewCasks:    []string{},
			},
			isUninstall: false,
			expected:    "",
		},
		{
			name: "single formula",
			rule: parser.Rule{
				HomebrewPackages: []string{"terraform"},
			},
			isUninstall: false,
			expected:    "terraform",
		},
		{
			name: "single cask",
			rule: parser.Rule{
				HomebrewCasks: []string{"visual-studio-code"},
			},
			isUninstall: false,
			expected:    "cask: visual-studio-code",
		},
		{
			name: "formula with version",
			rule: parser.Rule{
				HomebrewPackages: []string{"node@18", "python@3.11"},
			},
			isUninstall: false,
			expected:    "node@18, python@3.11",
		},
		{
			name: "uninstall operation (same as install)",
			rule: parser.Rule{
				HomebrewPackages: []string{"git"},
				HomebrewCasks:    []string{"docker"},
			},
			isUninstall: true,
			expected:    "git | cask: docker",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlers.NewHomebrewHandler(tt.rule, "/test/path")
			result := handler.GetDisplayDetails(tt.isUninstall)
			if result != tt.expected {
				t.Errorf("GetDisplayDetails() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestHomebrewHandler_GetState_Pure(t *testing.T) {
	tests := []struct {
		name        string
		rule        parser.Rule
		isUninstall bool
		expected    map[string]string
	}{
		{
			name: "formulas only",
			rule: parser.Rule{
				HomebrewPackages: []string{"git", "node"},
			},
			isUninstall: false,
			expected: map[string]string{
				"summary":  "git, node",
				"formulas": "git, node",
			},
		},
		{
			name: "casks only",
			rule: parser.Rule{
				HomebrewCasks: []string{"docker", "slack"},
			},
			isUninstall: false,
			expected: map[string]string{
				"summary": "cask: docker, slack",
				"casks":   "docker, slack",
			},
		},
		{
			name: "both formulas and casks",
			rule: parser.Rule{
				HomebrewPackages: []string{"git"},
				HomebrewCasks:    []string{"docker"},
			},
			isUninstall: false,
			expected: map[string]string{
				"summary":  "git | cask: docker",
				"formulas": "git",
				"casks":    "docker",
			},
		},
		{
			name: "empty packages and casks",
			rule: parser.Rule{
				HomebrewPackages: []string{},
				HomebrewCasks:    []string{},
			},
			isUninstall: false,
			expected: map[string]string{
				"summary": "",
			},
		},
		{
			name: "uninstall operation",
			rule: parser.Rule{
				HomebrewPackages: []string{"terraform"},
				HomebrewCasks:    []string{"visual-studio-code"},
			},
			isUninstall: true,
			expected: map[string]string{
				"summary":  "terraform | cask: visual-studio-code",
				"formulas": "terraform",
				"casks":    "visual-studio-code",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlers.NewHomebrewHandler(tt.rule, "/test/path")
			result := handler.GetState(tt.isUninstall)

			if len(result) != len(tt.expected) {
				t.Errorf("GetState() returned %d keys, want %d", len(result), len(tt.expected))
			}

			for key, expectedValue := range tt.expected {
				if actualValue, exists := result[key]; !exists {
					t.Errorf("GetState() missing key %v", key)
				} else if actualValue != expectedValue {
					t.Errorf("GetState()[%v] = %q, want %q", key, actualValue, expectedValue)
				}
			}
		})
	}
}

func TestHomebrewHandler_IsInstalled_Pure(t *testing.T) {
	// Create test status with various homebrew packages
	status := &handlers.Status{
		Brews: []handlers.HomebrewStatus{
			{
				Formula:   "git",
				Version:   "2.41.0",
				Blueprint: "/test/blueprint.yml",
				OS:        "mac",
			},
			{
				Formula:   "node",
				Version:   "18.17.0",
				Blueprint: "/test/blueprint.yml",
				OS:        "mac",
			},
			{
				Formula:   "cask:docker",
				Version:   "cask",
				Blueprint: "/test/blueprint.yml",
				OS:        "mac",
			},
			{
				Formula:   "python",
				Version:   "3.11.4",
				Blueprint: "/other/blueprint.yml", // Different blueprint
				OS:        "mac",
			},
			{
				Formula:   "terraform",
				Version:   "1.5.2",
				Blueprint: "/test/blueprint.yml",
				OS:        "linux", // Different OS
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
			name: "all formulas installed for matching blueprint and OS",
			rule: parser.Rule{
				HomebrewPackages: []string{"git", "node"},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "mac",
			expected:      true,
		},
		{
			name: "single formula installed",
			rule: parser.Rule{
				HomebrewPackages: []string{"git"},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "mac",
			expected:      true,
		},
		{
			name: "cask installed",
			rule: parser.Rule{
				HomebrewCasks: []string{"docker"},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "mac",
			expected:      true,
		},
		{
			name: "formula not installed",
			rule: parser.Rule{
				HomebrewPackages: []string{"vim"},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "mac",
			expected:      false,
		},
		{
			name: "cask not installed",
			rule: parser.Rule{
				HomebrewCasks: []string{"slack"},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "mac",
			expected:      false,
		},
		{
			name: "formula installed but wrong OS",
			rule: parser.Rule{
				HomebrewPackages: []string{"terraform"},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "mac", // terraform is installed for linux, not mac
			expected:      false,
		},
		{
			name: "formula installed but wrong blueprint",
			rule: parser.Rule{
				HomebrewPackages: []string{"python"},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "mac", // python is in /other/blueprint.yml, not /test/blueprint.yml
			expected:      false,
		},
		{
			name: "partially installed packages",
			rule: parser.Rule{
				HomebrewPackages: []string{"git", "vim"},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "mac",
			expected:      false, // git is installed, but vim is not
		},
		{
			name: "mixed formulas and casks - all installed",
			rule: parser.Rule{
				HomebrewPackages: []string{"git"},
				HomebrewCasks:    []string{"docker"},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "mac",
			expected:      true,
		},
		{
			name: "mixed formulas and casks - partially installed",
			rule: parser.Rule{
				HomebrewPackages: []string{"git"},
				HomebrewCasks:    []string{"slack"},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "mac",
			expected:      false, // git is installed, but slack cask is not
		},
		{
			name: "empty packages and casks",
			rule: parser.Rule{
				HomebrewPackages: []string{},
				HomebrewCasks:    []string{},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "mac",
			expected:      true, // empty list should return true
		},
		{
			name: "formula with version (version ignored in status check)",
			rule: parser.Rule{
				HomebrewPackages: []string{"node@18"},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "mac",
			expected:      true, // should match "node" formula in status (version ignored)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlers.NewHomebrewHandler(tt.rule, "/test/path")
			result := handler.IsInstalled(status, tt.blueprintFile, tt.osName)
			if result != tt.expected {
				t.Errorf("IsInstalled() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestHomebrewHandler_FindUninstallRules_Pure(t *testing.T) {
	// Create test status with various homebrew packages
	status := &handlers.Status{
		Brews: []handlers.HomebrewStatus{
			{
				Formula:   "git",
				Version:   "2.41.0",
				Blueprint: "/test/blueprint.yml",
				OS:        "mac",
			},
			{
				Formula:   "node",
				Version:   "18.17.0",
				Blueprint: "/test/blueprint.yml",
				OS:        "mac",
			},
			{
				Formula:   "cask:docker",
				Version:   "cask",
				Blueprint: "/test/blueprint.yml",
				OS:        "mac",
			},
			{
				Formula:   "python",
				Version:   "3.11.4",
				Blueprint: "/other/blueprint.yml", // Different blueprint
				OS:        "mac",
			},
			{
				Formula:   "terraform",
				Version:   "1.5.2",
				Blueprint: "/test/blueprint.yml",
				OS:        "linux", // Different OS
			},
		},
	}

	tests := []struct {
		name             string
		currentRules     []parser.Rule
		blueprintFile    string
		osName           string
		expectedFormulas []string // Expected formulas to uninstall
		expectedCasks    []string // Expected casks to uninstall
	}{
		{
			name:             "no current rules - all packages should be uninstalled",
			currentRules:     []parser.Rule{},
			blueprintFile:    "/test/blueprint.yml",
			osName:           "mac",
			expectedFormulas: []string{"git", "node"},
			expectedCasks:    []string{"docker"},
		},
		{
			name: "some packages still in current rules",
			currentRules: []parser.Rule{
				{
					Action:           "homebrew",
					HomebrewPackages: []string{"git"},
				},
			},
			blueprintFile:    "/test/blueprint.yml",
			osName:           "mac",
			expectedFormulas: []string{"node"},
			expectedCasks:    []string{"docker"},
		},
		{
			name: "all packages still in current rules",
			currentRules: []parser.Rule{
				{
					Action:           "homebrew",
					HomebrewPackages: []string{"git", "node"},
					HomebrewCasks:    []string{"docker"},
				},
			},
			blueprintFile:    "/test/blueprint.yml",
			osName:           "mac",
			expectedFormulas: []string{}, // Nothing to uninstall
			expectedCasks:    []string{}, // Nothing to uninstall
		},
		{
			name:             "different OS - no packages to uninstall from different OS",
			currentRules:     []parser.Rule{},
			blueprintFile:    "/test/blueprint.yml",
			osName:           "linux",
			expectedFormulas: []string{"terraform"}, // Only terraform for linux
			expectedCasks:    []string{},
		},
		{
			name:             "different blueprint - no packages to uninstall from different blueprint",
			currentRules:     []parser.Rule{},
			blueprintFile:    "/other/blueprint.yml",
			osName:           "mac",
			expectedFormulas: []string{"python"}, // Only python belongs to this blueprint
			expectedCasks:    []string{},
		},
		{
			name: "mixed scenarios - keeping some formulas and casks",
			currentRules: []parser.Rule{
				{
					Action:           "homebrew",
					HomebrewPackages: []string{"git"},
					HomebrewCasks:    []string{"docker"},
				},
				{
					Action:           "homebrew",
					HomebrewPackages: []string{"vim"}, // New package not in status
				},
			},
			blueprintFile:    "/test/blueprint.yml",
			osName:           "mac",
			expectedFormulas: []string{"node"}, // node should be uninstalled, git is kept
			expectedCasks:    []string{},       // docker is kept
		},
		{
			name: "formula with version specification",
			currentRules: []parser.Rule{
				{
					Action:           "homebrew",
					HomebrewPackages: []string{"git", "node@20"}, // Different version
				},
			},
			blueprintFile:    "/test/blueprint.yml",
			osName:           "mac",
			expectedFormulas: []string{}, // node@20 should match "node" in status
			expectedCasks:    []string{"docker"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a handler (the specific rule doesn't matter for this test)
			rule := parser.Rule{Action: "homebrew"}
			handler := handlers.NewHomebrewHandler(rule, "/test/path")

			result := handler.FindUninstallRules(status, tt.currentRules, tt.blueprintFile, tt.osName)

			if len(tt.expectedFormulas) == 0 && len(tt.expectedCasks) == 0 {
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

			if len(uninstallRule.HomebrewPackages) != len(tt.expectedFormulas) {
				t.Errorf("FindUninstallRules() formulas count = %d, want %d", len(uninstallRule.HomebrewPackages), len(tt.expectedFormulas))
			}

			if len(uninstallRule.HomebrewCasks) != len(tt.expectedCasks) {
				t.Errorf("FindUninstallRules() casks count = %d, want %d", len(uninstallRule.HomebrewCasks), len(tt.expectedCasks))
			}

			// Check that all expected formulas are in the uninstall rule
			for _, expectedFormula := range tt.expectedFormulas {
				found := false
				for _, actualFormula := range uninstallRule.HomebrewPackages {
					if actualFormula == expectedFormula {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("FindUninstallRules() missing expected formula %v", expectedFormula)
				}
			}

			// Check that all expected casks are in the uninstall rule
			for _, expectedCask := range tt.expectedCasks {
				found := false
				for _, actualCask := range uninstallRule.HomebrewCasks {
					if actualCask == expectedCask {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("FindUninstallRules() missing expected cask %v", expectedCask)
				}
			}
		})
	}
}

// Test constructor and basic properties
func TestHomebrewHandler_Constructor_Pure(t *testing.T) {
	rule := parser.Rule{
		Action:           "homebrew",
		HomebrewPackages: []string{"git", "node"},
		HomebrewCasks:    []string{"docker", "slack"},
	}

	handler := handlers.NewHomebrewHandler(rule, "/test/base/path")

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

	// Test NeedsSudo method (casks require sudo)
	if !handler.NeedsSudo() {
		t.Error("NeedsSudo() should return true when casks are present")
	}
}
