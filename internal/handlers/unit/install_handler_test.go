package unit

import (
	"strings"
	"testing"
	"time"

	"github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/platform/testutils"
)

// TestInstallHandler_GroupPackagesByManager covers groupPackagesByManager.
func TestInstallHandler_GroupPackagesByManager(t *testing.T) {
	tests := []struct {
		name     string
		packages []parser.Package
		wantKeys []string
	}{
		{
			name: "no manager goes to default",
			packages: []parser.Package{
				{Name: "curl"},
				{Name: "git"},
			},
			wantKeys: []string{"default"},
		},
		{
			name: "explicit manager is grouped",
			packages: []parser.Package{
				{Name: "vlc", PackageManager: "snap"},
			},
			wantKeys: []string{"snap"},
		},
		{
			name: "mixed managers",
			packages: []parser.Package{
				{Name: "curl"},
				{Name: "vlc", PackageManager: "snap"},
				{Name: "htop", PackageManager: "brew"},
			},
			wantKeys: []string{"default", "snap", "brew"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := parser.Rule{Action: "install", Packages: tt.packages}
			container := testutils.NewMockContainer().WithOS("linux").Build()
			handler := handlers.NewInstallHandler(rule, "/test", container)

			cmd := handler.GetCommand()
			// Verify a command is built (non-empty) for all cases except truly empty packages
			if len(tt.packages) > 0 && cmd == "" {
				t.Errorf("GetCommand() returned empty string for packages %v", tt.packages)
			}
		})
	}
}

// TestInstallHandler_BuildInstallCommandForManager_Snap covers snap install cases.
func TestInstallHandler_BuildInstallCommandForManager_Snap(t *testing.T) {
	tests := []struct {
		name     string
		packages []parser.Package
		wantPart string
	}{
		{
			name: "snap single package on linux with sudo",
			packages: []parser.Package{
				{Name: "vlc", PackageManager: "snap"},
			},
			wantPart: "snap install vlc",
		},
		{
			name: "snap multiple packages on linux",
			packages: []parser.Package{
				{Name: "vlc", PackageManager: "snap"},
				{Name: "discord", PackageManager: "snap"},
			},
			wantPart: "snap install vlc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := parser.Rule{Action: "install", Packages: tt.packages}
			container := testutils.NewMockContainer().
				WithOS("linux").
				WithUser("user", "1001", "1001", "/home/user").
				Build()
			handler := handlers.NewInstallHandler(rule, "/test", container)
			cmd := handler.GetCommand()
			if !strings.Contains(cmd, tt.wantPart) {
				t.Errorf("GetCommand() = %q, want to contain %q", cmd, tt.wantPart)
			}
		})
	}
}

// TestInstallHandler_BuildInstallCommandForManager_SnapOnMac verifies snap returns empty on mac.
func TestInstallHandler_BuildInstallCommandForManager_SnapOnMac(t *testing.T) {
	sysctlResult := testutils.NewExecuteResult().WithStdout("0\n").AsSuccess().Build()
	container := testutils.NewMockContainer().
		WithOS("mac").
		WithCommandResult("sysctl -n sysctl.proc_translated", sysctlResult).
		Build()

	rule := parser.Rule{
		Action: "install",
		Packages: []parser.Package{
			{Name: "vlc", PackageManager: "snap"},
		},
	}
	handler := handlers.NewInstallHandler(rule, "/test", container)
	cmd := handler.GetCommand()
	// snap is not supported on mac, so the command should be empty
	if cmd != "" {
		t.Errorf("GetCommand() = %q, want empty (snap unsupported on mac)", cmd)
	}
}

// TestInstallHandler_BuildInstallCommandForManager_Brew tests brew manager.
func TestInstallHandler_BuildInstallCommandForManager_Brew(t *testing.T) {
	sysctlResult := testutils.NewExecuteResult().WithStdout("0\n").AsSuccess().Build()
	container := testutils.NewMockContainer().
		WithOS("mac").
		WithCommandResult("sysctl -n sysctl.proc_translated", sysctlResult).
		Build()

	tests := []struct {
		name     string
		manager  string
		wantPart string
	}{
		{
			name:     "homebrew manager",
			manager:  "homebrew",
			wantPart: "brew install git",
		},
		{
			name:     "brew manager",
			manager:  "brew",
			wantPart: "brew install git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := parser.Rule{
				Action: "install",
				Packages: []parser.Package{
					{Name: "git", PackageManager: tt.manager},
				},
			}
			handler := handlers.NewInstallHandler(rule, "/test", container)
			cmd := handler.GetCommand()
			if !strings.Contains(cmd, tt.wantPart) {
				t.Errorf("GetCommand() = %q, want to contain %q", cmd, tt.wantPart)
			}
		})
	}
}

// TestInstallHandler_BuildInstallCommandForManager_AptOnMac verifies apt falls back to brew on mac.
func TestInstallHandler_BuildInstallCommandForManager_AptOnMac(t *testing.T) {
	sysctlResult := testutils.NewExecuteResult().WithStdout("0\n").AsSuccess().Build()
	container := testutils.NewMockContainer().
		WithOS("mac").
		WithCommandResult("sysctl -n sysctl.proc_translated", sysctlResult).
		Build()

	rule := parser.Rule{
		Action: "install",
		Packages: []parser.Package{
			{Name: "curl", PackageManager: "apt"},
		},
	}
	handler := handlers.NewInstallHandler(rule, "/test", container)
	cmd := handler.GetCommand()
	if !strings.Contains(cmd, "brew install curl") {
		t.Errorf("GetCommand() = %q, want brew install curl (apt falls back to brew on mac)", cmd)
	}
}

// TestInstallHandler_BuildInstallCommandForManager_UnknownManager tests unknown manager fallback.
func TestInstallHandler_BuildInstallCommandForManager_UnknownManager(t *testing.T) {
	container := testutils.NewMockContainer().
		WithOS("linux").
		WithUser("user", "1001", "1001", "/home/user").
		Build()

	rule := parser.Rule{
		Action: "install",
		Packages: []parser.Package{
			{Name: "mytool", PackageManager: "custompkg"},
		},
	}
	handler := handlers.NewInstallHandler(rule, "/test", container)
	cmd := handler.GetCommand()
	if !strings.Contains(cmd, "custompkg install mytool") {
		t.Errorf("GetCommand() = %q, want to contain 'custompkg install mytool'", cmd)
	}
}

// TestInstallHandler_BuildUninstallCommandForManager_AllCases covers uninstall command building.
func TestInstallHandler_BuildUninstallCommandForManager_AllCases(t *testing.T) {
	tests := []struct {
		name     string
		osName   string
		packages []parser.Package
		wantPart string
	}{
		{
			name:   "apt uninstall on linux",
			osName: "linux",
			packages: []parser.Package{
				{Name: "curl"},
			},
			wantPart: "apt-get remove -y curl",
		},
		{
			name:   "brew uninstall on mac",
			osName: "mac",
			packages: []parser.Package{
				{Name: "git", PackageManager: "brew"},
			},
			wantPart: "brew uninstall -y git",
		},
		{
			name:   "apt uninstall falls back to brew on mac",
			osName: "mac",
			packages: []parser.Package{
				{Name: "curl", PackageManager: "apt"},
			},
			wantPart: "brew uninstall -y curl",
		},
		{
			name:   "snap uninstall on linux",
			osName: "linux",
			packages: []parser.Package{
				{Name: "vlc", PackageManager: "snap"},
			},
			wantPart: "snap remove vlc",
		},
		{
			name:   "unknown manager uninstall",
			osName: "linux",
			packages: []parser.Package{
				{Name: "mytool", PackageManager: "custompkg"},
			},
			wantPart: "custompkg remove mytool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := parser.Rule{
				Action:   "uninstall",
				Packages: tt.packages,
			}

			containerBuilder := testutils.NewMockContainer().WithOS(tt.osName)
			if tt.osName == "linux" {
				containerBuilder = containerBuilder.WithUser("user", "1001", "1001", "/home/user")
			} else if tt.osName == "mac" {
				sysctlResult := testutils.NewExecuteResult().WithStdout("0\n").AsSuccess().Build()
				containerBuilder = containerBuilder.WithCommandResult("sysctl -n sysctl.proc_translated", sysctlResult)
			}
			container := containerBuilder.Build()
			handler := handlers.NewInstallHandler(rule, "/test", container)
			cmd := handler.GetCommand()
			if !strings.Contains(cmd, tt.wantPart) {
				t.Errorf("GetCommand() = %q, want to contain %q", cmd, tt.wantPart)
			}
		})
	}
}

// TestInstallHandler_NeedsSudo_Function tests the package-level needsSudo function indirectly
// through NeedsSudo on the default (unknown OS) branch.
func TestInstallHandler_NeedsSudo_UnknownOS(t *testing.T) {
	// On an unknown OS NeedsSudo falls back to checking if the generated command contains "sudo"
	// For an unknown OS with default (apt) package manager, the generated command won't include sudo
	container := testutils.NewMockContainer().WithOS("windows").Build()

	rule := parser.Rule{
		Action:   "install",
		Packages: []parser.Package{{Name: "curl"}},
	}
	handler := handlers.NewInstallHandler(rule, "/test", container)
	// On windows, the command is "windows install curl" (unknown manager branch)
	// which does not contain "sudo", so NeedsSudo should be false
	if handler.NeedsSudo() {
		t.Errorf("NeedsSudo() = true on windows, want false (no sudo in generated command)")
	}
}

// TestInstallHandler_IsInstalled_AllCases tests IsInstalled with matching and non-matching entries.
func TestInstallHandler_IsInstalled_AllCases(t *testing.T) {
	status := &handlers.Status{
		Packages: []handlers.PackageStatus{
			{Name: "curl", Blueprint: "/test/bp.yml", OS: "linux"},
			{Name: "git", Blueprint: "/test/bp.yml", OS: "mac"},
		},
	}

	container := testutils.NewMockContainer().WithOS("linux").Build()

	tests := []struct {
		name     string
		packages []string
		bp       string
		osName   string
		want     bool
	}{
		{
			name:     "installed package matches",
			packages: []string{"curl"},
			bp:       "/test/bp.yml",
			osName:   "linux",
			want:     true,
		},
		{
			name:     "package not in status",
			packages: []string{"wget"},
			bp:       "/test/bp.yml",
			osName:   "linux",
			want:     false,
		},
		{
			name:     "right package wrong OS",
			packages: []string{"curl"},
			bp:       "/test/bp.yml",
			osName:   "mac",
			want:     false,
		},
		{
			name:     "right package wrong blueprint",
			packages: []string{"curl"},
			bp:       "/other/bp.yml",
			osName:   "linux",
			want:     false,
		},
		{
			name:     "all packages installed",
			packages: []string{"curl"},
			bp:       "/test/bp.yml",
			osName:   "linux",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := testutils.NewRule().WithAction("install").WithPackages(tt.packages...).Build()
			handler := handlers.NewInstallHandler(rule, "/test", container)
			got := handler.IsInstalled(status, tt.bp, tt.osName)
			if got != tt.want {
				t.Errorf("IsInstalled() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestInstallHandler_UpdateStatus_Install covers UpdateStatus for the install action.
func TestInstallHandler_UpdateStatus_Install(t *testing.T) {
	container := testutils.NewMockContainer().WithOS("linux").Build()
	rule := parser.Rule{
		Action:   "install",
		Packages: []parser.Package{{Name: "curl"}},
	}
	handler := handlers.NewInstallHandler(rule, "/test", container)
	status := &handlers.Status{}

	// Build record that matches the install command
	cmd := handler.GetCommand()
	records := []handlers.ExecutionRecord{
		{Command: cmd, Status: "success"},
	}
	if err := handler.UpdateStatus(status, records, "/test/bp.yml", "linux"); err != nil {
		t.Fatalf("UpdateStatus() error: %v", err)
	}
	if len(status.Packages) != 1 || status.Packages[0].Name != "curl" {
		t.Errorf("expected curl in Packages after UpdateStatus, got %v", status.Packages)
	}
}

// TestInstallHandler_UpdateStatus_Uninstall covers UpdateStatus for the uninstall action.
func TestInstallHandler_UpdateStatus_Uninstall(t *testing.T) {
	container := testutils.NewMockContainer().WithOS("linux").Build()
	rule := parser.Rule{
		Action:   "uninstall",
		Packages: []parser.Package{{Name: "curl"}},
	}
	handler := handlers.NewInstallHandler(rule, "/test", container)

	status := &handlers.Status{
		Packages: []handlers.PackageStatus{
			{Name: "curl", Blueprint: "/test/bp.yml", OS: "linux"},
		},
	}

	// Uninstall always removes from status regardless of records
	if err := handler.UpdateStatus(status, nil, "/test/bp.yml", "linux"); err != nil {
		t.Fatalf("UpdateStatus() error: %v", err)
	}
	if len(status.Packages) != 0 {
		t.Errorf("expected curl removed from Packages, got %v", status.Packages)
	}
}

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
			shouldHaveSudo: false,
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
			shouldHaveSudo: false,
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

			// Test that the command matches the expected sudo usage
			hasSudo := strings.Contains(cmd, "sudo")
			if hasSudo != tt.shouldHaveSudo {
				t.Errorf("Command has sudo = %v, want %v. Command: %q", hasSudo, tt.shouldHaveSudo, cmd)
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
			name:         "macOS with packages doesn't need sudo (brew handles it internally)",
			packages:     []string{"curl"},
			osName:       "mac",
			expectedSudo: false,
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
		{
			name:         "Windows with packages matches command generation (currently no sudo)",
			packages:     []string{"curl"},
			osName:       "windows",
			expectedSudo: false, // Matches current command generation behavior
		},
		{
			name:         "Windows with no packages doesn't need sudo",
			packages:     []string{},
			osName:       "windows",
			expectedSudo: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Create mock container with specified OS and non-root user for Linux
			containerBuilder := testutils.NewMockContainer().
				WithOS(tt.osName)

			// Set up non-root user for Linux to test sudo requirement logic
			if tt.osName == "linux" {
				containerBuilder = containerBuilder.WithUser("testuser", "1001", "1001", "/home/testuser")
			}

			mockContainer := containerBuilder.Build()

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
