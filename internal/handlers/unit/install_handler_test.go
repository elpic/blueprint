package unit

import (
	"testing"
	"time"

	"github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/platform/testutils"
)

// TestInstallHandler_BuildCommand_Pure demonstrates fast, pure unit testing
// using mocks for dependency injection. This test executes in <1ms.
func TestInstallHandler_BuildCommand_Pure(t *testing.T) {
	tests := []struct {
		name           string
		packages       []string
		osName         string
		expectedCmd    string
		shouldHaveSudo bool
	}{
		{
			name:           "single package on mac uses brew",
			packages:       []string{"curl"},
			osName:         "mac",
			expectedCmd:    "brew install curl",
			shouldHaveSudo: true,
		},
		{
			name:           "single package on linux uses apt with sudo",
			packages:       []string{"curl"},
			osName:         "linux",
			expectedCmd:    "sudo apt-get install -y curl",
			shouldHaveSudo: true,
		},
		{
			name:           "multiple packages on mac",
			packages:       []string{"git", "curl", "wget"},
			osName:         "mac",
			expectedCmd:    "brew install git curl wget",
			shouldHaveSudo: true,
		},
		{
			name:           "multiple packages on linux",
			packages:       []string{"git", "curl"},
			osName:         "linux",
			expectedCmd:    "sudo apt-get install -y git curl",
			shouldHaveSudo: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Create a rule using the test builder
			rule := testutils.NewRule().
				WithAction("install").
				WithPackages(tt.packages...).
				Build()

			// Create mock container with OS detection and command mocking
			containerBuilder := testutils.NewMockContainer().
				WithOS(tt.osName).
				WithUser("testuser", "1001", "1001", "/home/testuser")

			// For Mac tests, mock the sysctl command to avoid I/O
			if tt.osName == "mac" {
				// Mock sysctl command to return non-Rosetta (Intel) result
				sysctlResult := testutils.NewExecuteResult().
					WithStdout("0\n"). // Not running under Rosetta
					AsSuccess().
					Build()
				containerBuilder = containerBuilder.WithCommandResult("sysctl -n sysctl.proc_translated", sysctlResult)
			}

			container := containerBuilder.Build()

			// Create handler with dependency injection
			handler := handlers.NewInstallHandler(rule, "/test/path", container)

			// Test command generation (pure function - no I/O)
			cmd := handler.GetCommand()

			duration := time.Since(start)

			// Verify command generation
			if cmd != tt.expectedCmd {
				t.Errorf("GetCommand() = %q, want %q", cmd, tt.expectedCmd)
			}

			// Verify that this is a fast unit test (< 1ms)
			if duration > time.Millisecond {
				t.Errorf("Test took %v, expected < 1ms for pure unit test", duration)
			}

			// Test NeedsSudo method (also pure)
			needsSudo := handler.NeedsSudo()
			if needsSudo != tt.shouldHaveSudo {
				t.Errorf("NeedsSudo() = %v, want %v", needsSudo, tt.shouldHaveSudo)
			}
		})
	}
}

// TestInstallHandler_GetDependencyKey_Pure tests dependency key generation
// without any I/O operations. Executes in microseconds.
func TestInstallHandler_GetDependencyKey_Pure(t *testing.T) {
	tests := []struct {
		name     string
		ruleID   string
		packages []string
		expected string
	}{
		{
			name:     "uses rule ID when present",
			ruleID:   "custom-install-id",
			packages: []string{"git"},
			expected: "custom-install-id",
		},
		{
			name:     "falls back to first package name",
			ruleID:   "",
			packages: []string{"git", "curl"},
			expected: "git",
		},
		{
			name:     "falls back to 'install' when no packages",
			ruleID:   "",
			packages: []string{},
			expected: "install",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Build rule with test builder
			ruleBuilder := testutils.NewRule().
				WithAction("install").
				WithPackages(tt.packages...)

			if tt.ruleID != "" {
				ruleBuilder = ruleBuilder.WithID(tt.ruleID)
			}

			rule := ruleBuilder.Build()

			// Create minimal container (no I/O needed for this test)
			container := testutils.NewMockContainer().
				WithOS("linux").
				Build()

			// Test dependency key generation
			handler := handlers.NewInstallHandler(rule, "/test", container)
			key := handler.GetDependencyKey()

			duration := time.Since(start)

			if key != tt.expected {
				t.Errorf("GetDependencyKey() = %q, want %q", key, tt.expected)
			}

			// This should be extremely fast (microseconds)
			if duration > 100*time.Microsecond {
				t.Errorf("Test took %v, expected < 100μs for pure logic test", duration)
			}
		})
	}
}

// TestInstallHandler_GetDisplayDetails_Pure tests display information generation.
func TestInstallHandler_GetDisplayDetails_Pure(t *testing.T) {
	tests := []struct {
		name     string
		packages []string
		expected string
	}{
		{
			name:     "single package",
			packages: []string{"git"},
			expected: "git",
		},
		{
			name:     "multiple packages",
			packages: []string{"git", "curl", "wget"},
			expected: "git, curl, wget",
		},
		{
			name:     "no packages",
			packages: []string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			rule := testutils.NewRule().
				WithAction("install").
				WithPackages(tt.packages...).
				Build()

				// Create minimal container - using Linux so no brew command mocking needed
			// Create minimal container - using Linux so no brew command mocking needed
			container := testutils.NewMockContainer().WithOS("linux").Build()
			handler := handlers.NewInstallHandler(rule, "/test", container)

			details := handler.GetDisplayDetails(false)
			duration := time.Since(start)

			if details != tt.expected {
				t.Errorf("GetDisplayDetails() = %q, want %q", details, tt.expected)
			}

			if duration > 100*time.Microsecond {
				t.Errorf("Test took %v, expected < 100μs for pure logic test", duration)
			}
		})
	}
}

// TestInstallHandler_GetState_Pure tests state generation for the "blueprint ps" command.
func TestInstallHandler_GetState_Pure(t *testing.T) {
	packages := []string{"git", "curl"}

	rule := testutils.NewRule().
		WithAction("install").
		WithPackages(packages...).
		Build()

	container := testutils.NewMockContainer().WithOS("linux").Build()
	handler := handlers.NewInstallHandler(rule, "/test", container)

	start := time.Now()
	state := handler.GetState(false)
	duration := time.Since(start)

	// Verify required keys
	if state["summary"] != "git, curl" {
		t.Errorf("state[summary] = %q, want %q", state["summary"], "git, curl")
	}
	if state["packages"] != "git, curl" {
		t.Errorf("state[packages] = %q, want %q", state["packages"], "git, curl")
	}

	if duration > 100*time.Microsecond {
		t.Errorf("Test took %v, expected < 100μs for pure logic test", duration)
	}
}

func TestInstallHandler_NeedsSudo_Pure(t *testing.T) {
	tests := []struct {
		name         string
		packages     []string
		osName       string
		expectedSudo bool
	}{
		{
			name:         "macOS with packages always needs sudo (brew may need it for casks)",
			packages:     []string{"curl"},
			osName:       "mac",
			expectedSudo: true,
		},
		{
			name:         "macOS with no packages doesn't need sudo",
			packages:     []string{},
			osName:       "mac",
			expectedSudo: false,
		},
		{
			name:         "Linux with packages needs sudo (for apt-get)",
			packages:     []string{"curl"},
			osName:       "linux",
			expectedSudo: true,
		},
		{
			name:         "Linux with no packages doesn't need sudo",
			packages:     []string{},
			osName:       "linux",
			expectedSudo: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Create mock container with specified OS
			mockContainer := testutils.NewMockContainer().
				WithOS(tt.osName).
				Build()

			// Build rule with packages (manually for now)
			var packages []parser.Package
			for _, pkg := range tt.packages {
				packages = append(packages, parser.Package{Name: pkg})
			}
			rule := parser.Rule{
				Action:   "install",
				Packages: packages,
			}

			// Create handler with mocked dependencies
			handler := handlers.NewInstallHandler(rule, "/test", mockContainer)

			// Test NeedsSudo method (pure function)
			needsSudo := handler.NeedsSudo()

			duration := time.Since(start)

			if needsSudo != tt.expectedSudo {
				t.Errorf("NeedsSudo() = %v, want %v", needsSudo, tt.expectedSudo)
			}

			if duration > 100*time.Microsecond {
				t.Errorf("Test took %v, expected < 100μs for pure logic test", duration)
			}
		})
	}
}
