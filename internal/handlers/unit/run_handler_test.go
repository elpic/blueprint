package unit

import (
	"testing"
	"time"

	"github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/platform/testutils"
)

// TestRunHandler_GetCommand_Pure tests command generation - pure function, no I/O.
func TestRunHandler_GetCommand_Pure(t *testing.T) {
	tests := []struct {
		name      string
		command   string
		undo      string
		sudo      bool
		isInstall bool
		expected  string
	}{
		{
			name:      "simple install command",
			command:   "echo 'hello world'",
			sudo:      false,
			isInstall: true,
			expected:  "echo 'hello world'",
		},
		{
			name:      "install command with sudo",
			command:   "systemctl start docker",
			sudo:      true,
			isInstall: true,
			expected:  "sudo systemctl start docker",
		},
		{
			name:      "uninstall command",
			command:   "start service",
			undo:      "stop service",
			sudo:      false,
			isInstall: false,
			expected:  "stop service",
		},
		{
			name:      "uninstall command with sudo",
			command:   "systemctl start docker",
			undo:      "systemctl stop docker",
			sudo:      true,
			isInstall: false,
			expected:  "sudo systemctl stop docker",
		},
		{
			name:      "uninstall without undo command",
			command:   "some command",
			undo:      "",
			sudo:      false,
			isInstall: false,
			expected:  "# no undo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Create rule
			ruleBuilder := testutils.NewRule().
				WithRun(tt.command)

			// Set additional fields using direct field access
			rule := ruleBuilder.Build()
			rule.RunUndo = tt.undo
			rule.RunSudo = tt.sudo
			if !tt.isInstall {
				rule.Action = "uninstall"
			}

			// Create handler using legacy constructor for pure logic test
			handler := handlers.NewRunHandler(rule, "/test/path")

			// Test command generation (pure function - no I/O)
			cmd := handler.GetCommand()

			duration := time.Since(start)

			// Verify command generation
			if cmd != tt.expected {
				t.Errorf("GetCommand() = %q, want %q", cmd, tt.expected)
			}

			// Verify that this is a fast unit test (< 100μs)
			if duration > 100*time.Microsecond {
				t.Errorf("Test took %v, expected < 100μs for pure unit test", duration)
			}
		})
	}
}

// TestRunHandler_GetDependencyKey_Pure tests dependency key generation
// without any I/O operations. Executes in microseconds.
func TestRunHandler_GetDependencyKey_Pure(t *testing.T) {
	tests := []struct {
		name     string
		ruleID   string
		command  string
		expected string
	}{
		{
			name:     "uses rule ID when present",
			ruleID:   "custom-run-id",
			command:  "echo test",
			expected: "custom-run-id",
		},
		{
			name:     "falls back to command",
			ruleID:   "",
			command:  "systemctl start nginx",
			expected: "systemctl start nginx",
		},
		{
			name:     "empty command",
			ruleID:   "",
			command:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Build rule with test builder
			ruleBuilder := testutils.NewRule().
				WithAction("run").
				WithRun(tt.command)

			if tt.ruleID != "" {
				ruleBuilder = ruleBuilder.WithID(tt.ruleID)
			}

			rule := ruleBuilder.Build()

			// Test dependency key generation using legacy constructor
			handler := handlers.NewRunHandler(rule, "/test")
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

// TestRunHandler_GetDisplayDetails_Pure tests display information generation.
func TestRunHandler_GetDisplayDetails_Pure(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		undo        string
		isUninstall bool
		expected    string
	}{
		{
			name:        "short command install",
			command:     "echo hello",
			isUninstall: false,
			expected:    "echo hello",
		},
		{
			name:        "short command uninstall",
			command:     "start service",
			undo:        "stop service",
			isUninstall: true,
			expected:    "stop service",
		},
		{
			name:        "long command gets truncated",
			command:     "this is a very long command that should be truncated because it exceeds 60 characters",
			isUninstall: false,
			expected:    "this is a very long command that should be truncated because...",
		},
		{
			name:        "empty undo command",
			command:     "start service",
			undo:        "",
			isUninstall: true,
			expected:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			rule := testutils.NewRule().
				WithAction("run").
				WithRun(tt.command).
				Build()
			rule.RunUndo = tt.undo

			handler := handlers.NewRunHandler(rule, "/test")

			details := handler.GetDisplayDetails(tt.isUninstall)
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

// TestRunHandler_IsInstalled_Pure tests IsInstalled for run commands.
func TestRunHandler_IsInstalled_Pure(t *testing.T) {
	status := &handlers.Status{
		Runs: []handlers.RunStatus{
			{Action: "run", Command: "echo hello", Blueprint: "/test/bp.yml", OS: "linux"},
			{Action: "run", Command: "echo world", Blueprint: "/other/bp.yml", OS: "linux"},
		},
	}

	tests := []struct {
		name    string
		command string
		bp      string
		os      string
		want    bool
	}{
		{
			name:    "command exists in status",
			command: "echo hello",
			bp:      "/test/bp.yml",
			os:      "linux",
			want:    true,
		},
		{
			name:    "command not in status",
			command: "echo missing",
			bp:      "/test/bp.yml",
			os:      "linux",
			want:    false,
		},
		{
			name:    "command exists but wrong blueprint",
			command: "echo hello",
			bp:      "/other/bp.yml",
			os:      "linux",
			want:    false,
		},
		{
			name:    "command exists but wrong OS",
			command: "echo hello",
			bp:      "/test/bp.yml",
			os:      "mac",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := testutils.NewRule().WithAction("run").WithRun(tt.command).Build()
			handler := handlers.NewRunHandler(rule, "/test")
			got := handler.IsInstalled(status, tt.bp, tt.os)
			if got != tt.want {
				t.Errorf("IsInstalled() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestRunHandler_UpdateStatus_RunAction covers UpdateStatus for the "run" action.
func TestRunHandler_UpdateStatus_RunAction(t *testing.T) {
	rule := testutils.NewRule().WithAction("run").WithRun("echo hello").Build()
	handler := handlers.NewRunHandler(rule, "/test")
	status := &handlers.Status{}

	cmd := handler.GetCommand()
	records := []handlers.ExecutionRecord{
		{Command: cmd, Status: "success"},
	}

	if err := handler.UpdateStatus(status, records, "/test/bp.yml", "linux"); err != nil {
		t.Fatalf("UpdateStatus() error: %v", err)
	}

	if len(status.Runs) != 1 {
		t.Fatalf("expected 1 run entry, got %d", len(status.Runs))
	}
	if status.Runs[0].Command != "echo hello" || status.Runs[0].Action != "run" {
		t.Errorf("run entry = %+v, unexpected values", status.Runs[0])
	}
}

// TestRunHandler_UpdateStatus_Uninstall covers UpdateStatus for the uninstall action.
func TestRunHandler_UpdateStatus_Uninstall(t *testing.T) {
	rule := testutils.NewRule().WithAction("run").WithRun("echo hello").Build()
	rule.Action = "uninstall"
	handler := handlers.NewRunHandler(rule, "/test")

	status := &handlers.Status{
		Runs: []handlers.RunStatus{
			{Action: "run", Command: "echo hello", Blueprint: "/test/bp.yml", OS: "linux"},
		},
	}

	if err := handler.UpdateStatus(status, nil, "/test/bp.yml", "linux"); err != nil {
		t.Fatalf("UpdateStatus() error: %v", err)
	}
	if len(status.Runs) != 0 {
		t.Errorf("expected run entry removed, got %d", len(status.Runs))
	}
}

// TestRunHandler_FindUninstallRules_Pure covers FindUninstallRules for RunHandler.
func TestRunHandler_FindUninstallRules_Pure(t *testing.T) {
	status := &handlers.Status{
		Runs: []handlers.RunStatus{
			{Action: "run", Command: "echo hello", Blueprint: "/test/bp.yml", OS: "linux"},
			{Action: "run", Command: "echo world", Blueprint: "/test/bp.yml", OS: "linux"},
		},
	}

	// Case 1: "echo hello" still in rules, "echo world" removed
	h1 := handlers.NewRunHandler(
		testutils.NewRule().WithAction("run").WithRun("echo hello").Build(),
		"/test",
	)
	rules1 := h1.FindUninstallRules(status, []parser.Rule{
		{Action: "run", RunCommand: "echo hello"},
	}, "/test/bp.yml", "linux")
	if len(rules1) != 1 {
		t.Errorf("FindUninstallRules() case1 count = %d, want 1", len(rules1))
	} else if rules1[0].RunCommand != "echo world" {
		t.Errorf("FindUninstallRules() case1 command = %q, want 'echo world'", rules1[0].RunCommand)
	}

	// Case 2: both commands still present
	h2 := handlers.NewRunHandler(
		testutils.NewRule().WithAction("run").WithRun("echo hello").Build(),
		"/test",
	)
	rules2 := h2.FindUninstallRules(status, []parser.Rule{
		{Action: "run", RunCommand: "echo hello"},
		{Action: "run", RunCommand: "echo world"},
	}, "/test/bp.yml", "linux")
	if len(rules2) != 0 {
		t.Errorf("FindUninstallRules() case2 count = %d, want 0", len(rules2))
	}
}

// TestRunShHandler_IsInstalled_Pure tests IsInstalled for run-sh commands.
func TestRunShHandler_IsInstalled_Pure(t *testing.T) {
	status := &handlers.Status{
		Runs: []handlers.RunStatus{
			{Action: "run-sh", Command: "https://example.com/install.sh", Blueprint: "/test/bp.yml", OS: "linux"},
		},
	}

	tests := []struct {
		name string
		url  string
		bp   string
		os   string
		want bool
	}{
		{
			name: "URL exists in status",
			url:  "https://example.com/install.sh",
			bp:   "/test/bp.yml",
			os:   "linux",
			want: true,
		},
		{
			name: "URL not in status",
			url:  "https://example.com/other.sh",
			bp:   "/test/bp.yml",
			os:   "linux",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := testutils.NewRule().WithRunSh(tt.url).Build()
			handler := handlers.NewRunShHandler(rule, "/test")
			got := handler.IsInstalled(status, tt.bp, tt.os)
			if got != tt.want {
				t.Errorf("IsInstalled() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestRunShHandler_UpdateStatus_RunShAction covers UpdateStatus for the "run-sh" action.
func TestRunShHandler_UpdateStatus_RunShAction(t *testing.T) {
	rule := testutils.NewRule().WithRunSh("https://example.com/install.sh").Build()
	handler := handlers.NewRunShHandler(rule, "/test")
	status := &handlers.Status{}

	cmd := handler.GetCommand() // returns the URL for run-sh
	records := []handlers.ExecutionRecord{
		{Command: cmd, Status: "success"},
	}

	if err := handler.UpdateStatus(status, records, "/test/bp.yml", "linux"); err != nil {
		t.Fatalf("UpdateStatus() error: %v", err)
	}

	if len(status.Runs) != 1 {
		t.Fatalf("expected 1 run-sh entry, got %d", len(status.Runs))
	}
	if status.Runs[0].Action != "run-sh" || status.Runs[0].Command != "https://example.com/install.sh" {
		t.Errorf("run-sh entry = %+v, unexpected values", status.Runs[0])
	}
}

// TestRunShHandler_UpdateStatus_Uninstall covers UpdateStatus uninstall for run-sh.
func TestRunShHandler_UpdateStatus_Uninstall(t *testing.T) {
	rule := testutils.NewRule().WithRunSh("https://example.com/install.sh").Build()
	rule.Action = "uninstall"
	handler := handlers.NewRunShHandler(rule, "/test")

	status := &handlers.Status{
		Runs: []handlers.RunStatus{
			{Action: "run-sh", Command: "https://example.com/install.sh", Blueprint: "/test/bp.yml", OS: "linux"},
		},
	}

	if err := handler.UpdateStatus(status, nil, "/test/bp.yml", "linux"); err != nil {
		t.Fatalf("UpdateStatus() error: %v", err)
	}
	if len(status.Runs) != 0 {
		t.Errorf("expected run-sh entry removed, got %d", len(status.Runs))
	}
}

// TestRunHandler_GetState_Pure tests state generation for the "blueprint ps" command.
func TestRunHandler_GetState_Pure(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		expected string
	}{
		{
			name:     "short command",
			command:  "echo test",
			expected: "echo test",
		},
		{
			name:     "long command gets truncated",
			command:  "this is a very long command that should be truncated because it exceeds the 60 character limit",
			expected: "this is a very long command that should be truncated because...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			rule := testutils.NewRule().
				WithAction("run").
				WithRun(tt.command).
				Build()

			handler := handlers.NewRunHandler(rule, "/test")
			state := handler.GetState(false)

			duration := time.Since(start)

			// Verify required keys
			if state["summary"] != tt.expected {
				t.Errorf("state[summary] = %q, want %q", state["summary"], tt.expected)
			}

			if duration > 100*time.Microsecond {
				t.Errorf("Test took %v, expected < 100μs for pure logic test", duration)
			}
		})
	}
}
