package unit

import (
	"testing"

	"github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/parser"
)

func TestAsdfHandler_GetCommand_Pure(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		expected string
	}{
		{
			name: "install with single package",
			rule: parser.Rule{
				Action:       "asdf",
				AsdfPackages: []string{"node@18.0.0"},
			},
			expected: "asdf install node 18.0.0",
		},
		{
			name: "install with multiple packages",
			rule: parser.Rule{
				Action:       "asdf",
				AsdfPackages: []string{"node@18.0.0", "python@3.11.0"},
			},
			expected: "asdf install node 18.0.0 && asdf install python 3.11.0",
		},
		{
			name: "install with complex version numbers",
			rule: parser.Rule{
				Action:       "asdf",
				AsdfPackages: []string{"ruby@3.2.1", "golang@1.21.3"},
			},
			expected: "asdf install ruby 3.2.1 && asdf install golang 1.21.3",
		},
		{
			name: "install with empty packages list",
			rule: parser.Rule{
				Action:       "asdf",
				AsdfPackages: []string{},
			},
			expected: "asdf-init",
		},
		{
			name: "install with malformed package (no @)",
			rule: parser.Rule{
				Action:       "asdf",
				AsdfPackages: []string{"node", "python@3.11"},
			},
			expected: "asdf install python 3.11",
		},
		{
			name: "uninstall action",
			rule: parser.Rule{
				Action:       "uninstall",
				AsdfPackages: []string{"node@18.0.0"},
			},
			expected: "asdf uninstall",
		},
		{
			name: "install with version containing special characters",
			rule: parser.Rule{
				Action:       "asdf",
				AsdfPackages: []string{"java@openjdk-11.0.2", "terraform@1.5.0-beta1"},
			},
			expected: "asdf install java openjdk-11.0.2 && asdf install terraform 1.5.0-beta1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlers.NewAsdfHandler(tt.rule, "/test/path")
			result := handler.GetCommand()
			if result != tt.expected {
				t.Errorf("GetCommand() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestAsdfHandler_GetDependencyKey_Pure(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		expected string
	}{
		{
			name: "uses rule ID when present for install",
			rule: parser.Rule{
				ID:     "custom-asdf-setup",
				Action: "asdf",
			},
			expected: "custom-asdf-setup",
		},
		{
			name: "falls back to 'asdf' for install",
			rule: parser.Rule{
				Action: "asdf",
			},
			expected: "asdf",
		},
		{
			name: "uses rule ID when present for uninstall",
			rule: parser.Rule{
				ID:           "custom-asdf-setup",
				Action:       "uninstall",
				AsdfPackages: []string{"node@18"}, // Makes it detect as asdf type
			},
			expected: "custom-asdf-setup",
		},
		{
			name: "falls back to 'uninstall-asdf' for uninstall",
			rule: parser.Rule{
				Action:       "uninstall",
				AsdfPackages: []string{"node@18"}, // Makes it detect as asdf type
			},
			expected: "uninstall-asdf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlers.NewAsdfHandler(tt.rule, "/test/path")
			result := handler.GetDependencyKey()
			if result != tt.expected {
				t.Errorf("GetDependencyKey() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestAsdfHandler_GetDisplayDetails_Pure(t *testing.T) {
	tests := []struct {
		name        string
		rule        parser.Rule
		isUninstall bool
		expected    string
	}{
		{
			name: "single package",
			rule: parser.Rule{
				AsdfPackages: []string{"node@18.0.0"},
			},
			isUninstall: false,
			expected:    "node@18.0.0",
		},
		{
			name: "multiple packages",
			rule: parser.Rule{
				AsdfPackages: []string{"node@18.0.0", "python@3.11.0", "ruby@3.2.1"},
			},
			isUninstall: false,
			expected:    "node@18.0.0, python@3.11.0, ruby@3.2.1",
		},
		{
			name: "empty packages list",
			rule: parser.Rule{
				AsdfPackages: []string{},
			},
			isUninstall: false,
			expected:    "asdf",
		},
		{
			name: "single package for uninstall",
			rule: parser.Rule{
				AsdfPackages: []string{"golang@1.21.3"},
			},
			isUninstall: true,
			expected:    "golang@1.21.3",
		},
		{
			name: "packages with special characters",
			rule: parser.Rule{
				AsdfPackages: []string{"java@openjdk-11.0.2", "terraform@1.5.0-beta1"},
			},
			isUninstall: false,
			expected:    "java@openjdk-11.0.2, terraform@1.5.0-beta1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlers.NewAsdfHandler(tt.rule, "/test/path")
			result := handler.GetDisplayDetails(tt.isUninstall)
			if result != tt.expected {
				t.Errorf("GetDisplayDetails() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestAsdfHandler_GetState_Pure(t *testing.T) {
	tests := []struct {
		name        string
		rule        parser.Rule
		isUninstall bool
		expected    map[string]string
	}{
		{
			name: "single package",
			rule: parser.Rule{
				AsdfPackages: []string{"node@18.0.0"},
			},
			isUninstall: false,
			expected: map[string]string{
				"summary": "node@18.0.0",
				"plugins": "node@18.0.0",
			},
		},
		{
			name: "multiple packages",
			rule: parser.Rule{
				AsdfPackages: []string{"node@18.0.0", "python@3.11.0"},
			},
			isUninstall: false,
			expected: map[string]string{
				"summary": "node@18.0.0, python@3.11.0",
				"plugins": "node@18.0.0, python@3.11.0",
			},
		},
		{
			name: "empty packages list",
			rule: parser.Rule{
				AsdfPackages: []string{},
			},
			isUninstall: false,
			expected: map[string]string{
				"summary": "asdf",
				"plugins": "",
			},
		},
		{
			name: "uninstall operation",
			rule: parser.Rule{
				AsdfPackages: []string{"ruby@3.2.1", "golang@1.21.3"},
			},
			isUninstall: true,
			expected: map[string]string{
				"summary": "ruby@3.2.1, golang@1.21.3",
				"plugins": "ruby@3.2.1, golang@1.21.3",
			},
		},
		{
			name: "packages with complex versions",
			rule: parser.Rule{
				AsdfPackages: []string{"java@openjdk-11.0.2", "terraform@1.5.0-beta1"},
			},
			isUninstall: false,
			expected: map[string]string{
				"summary": "java@openjdk-11.0.2, terraform@1.5.0-beta1",
				"plugins": "java@openjdk-11.0.2, terraform@1.5.0-beta1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlers.NewAsdfHandler(tt.rule, "/test/path")
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

func TestAsdfHandler_IsInstalled_Pure(t *testing.T) {
	// Create test status with various asdf packages
	status := &handlers.Status{
		Asdfs: []handlers.AsdfStatus{
			{
				Plugin:    "node",
				Version:   "18.0.0",
				Blueprint: "/test/blueprint.yml",
				OS:        "linux",
			},
			{
				Plugin:    "python",
				Version:   "3.11.0",
				Blueprint: "/test/blueprint.yml",
				OS:        "linux",
			},
			{
				Plugin:    "ruby",
				Version:   "3.2.1",
				Blueprint: "/other/blueprint.yml",
				OS:        "linux",
			},
			{
				Plugin:    "golang",
				Version:   "1.21.3",
				Blueprint: "/test/blueprint.yml",
				OS:        "darwin",
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
				AsdfPackages: []string{"node@18.0.0", "python@3.11.0"},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux",
			expected:      true,
		},
		{
			name: "single package installed",
			rule: parser.Rule{
				AsdfPackages: []string{"node@18.0.0"},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux",
			expected:      true,
		},
		{
			name: "package not installed",
			rule: parser.Rule{
				AsdfPackages: []string{"terraform@1.5.0"},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux",
			expected:      false,
		},
		{
			name: "package installed but wrong OS",
			rule: parser.Rule{
				AsdfPackages: []string{"golang@1.21.3"},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux", // golang is installed for darwin, not linux
			expected:      false,
		},
		{
			name: "package installed but wrong blueprint",
			rule: parser.Rule{
				AsdfPackages: []string{"ruby@3.2.1"},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux", // ruby is in /other/blueprint.yml, not /test/blueprint.yml
			expected:      false,
		},
		{
			name: "partially installed packages",
			rule: parser.Rule{
				AsdfPackages: []string{"node@18.0.0", "terraform@1.5.0"},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux",
			expected:      false, // node is installed, but terraform is not
		},
		{
			name: "empty packages list",
			rule: parser.Rule{
				AsdfPackages: []string{},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux",
			expected:      true, // empty list should return true
		},
		{
			name: "wrong version",
			rule: parser.Rule{
				AsdfPackages: []string{"node@19.0.0"},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux",
			expected:      false, // node@18.0.0 is installed, not node@19.0.0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlers.NewAsdfHandler(tt.rule, "/test/path")
			result := handler.IsInstalled(status, tt.blueprintFile, tt.osName)
			if result != tt.expected {
				t.Errorf("IsInstalled() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestAsdfHandler_FindUninstallRules_Pure(t *testing.T) {
	// Create test status with various asdf packages
	status := &handlers.Status{
		Asdfs: []handlers.AsdfStatus{
			{
				Plugin:    "node",
				Version:   "18.0.0",
				Blueprint: "/test/blueprint.yml",
				OS:        "linux",
			},
			{
				Plugin:    "python",
				Version:   "3.11.0",
				Blueprint: "/test/blueprint.yml",
				OS:        "linux",
			},
			{
				Plugin:    "ruby",
				Version:   "3.2.1",
				Blueprint: "/other/blueprint.yml", // Different blueprint
				OS:        "linux",
			},
			{
				Plugin:    "golang",
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
			expected:      []string{"node 18.0.0", "python 3.11.0"},
		},
		{
			name: "some packages still in current rules",
			currentRules: []parser.Rule{
				{
					Action:       "asdf",
					AsdfPackages: []string{"node 18"}, // rule uses "plugin version" format
				},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux",
			expected:      []string{"python 3.11.0"}, // node is kept (by tool name), python should be uninstalled
		},
		{
			name: "all packages still in current rules",
			currentRules: []parser.Rule{
				{
					Action:       "asdf",
					AsdfPackages: []string{"node 18", "python 3.11"},
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
			osName:        "darwin",
			expected:      []string{"golang 1.21.3"},
		},
		{
			name:          "different blueprint - no packages to uninstall",
			currentRules:  []parser.Rule{},
			blueprintFile: "/other/blueprint.yml",
			osName:        "linux",
			expected:      []string{"ruby 3.2.1"}, // Only ruby belongs to this blueprint
		},
		{
			name: "mixed scenarios",
			currentRules: []parser.Rule{
				{
					Action:       "asdf",
					AsdfPackages: []string{"node 18"},
				},
				{
					Action:       "asdf",
					AsdfPackages: []string{"terraform 1.5"}, // New package not in status
				},
			},
			blueprintFile: "/test/blueprint.yml",
			osName:        "linux",
			expected:      []string{"python 3.11.0"}, // python should be uninstalled, node is kept
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a handler (the specific rule doesn't matter for this test)
			rule := parser.Rule{Action: "asdf"}
			handler := handlers.NewAsdfHandler(rule, "/test/path")

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

			if len(uninstallRule.AsdfPackages) != len(tt.expected) {
				t.Errorf("FindUninstallRules() packages count = %d, want %d", len(uninstallRule.AsdfPackages), len(tt.expected))
			}

			// Check that all expected packages are in the uninstall rule
			for _, expectedPkg := range tt.expected {
				found := false
				for _, actualPkg := range uninstallRule.AsdfPackages {
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

// Test constructor and basic properties
func TestAsdfHandler_Constructor_Pure(t *testing.T) {
	rule := parser.Rule{
		Action:       "asdf",
		AsdfPackages: []string{"node@18.0.0", "python@3.11.0"},
	}

	handler := handlers.NewAsdfHandler(rule, "/test/base/path")

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
