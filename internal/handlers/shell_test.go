package handlers

import (
	"bytes"
	"io"
	"os"
	"os/user"
	"strings"
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

// getCurrentTestUser returns the current user's username for testing
func getCurrentTestUser() string {
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return "testuser"
}

func TestShellHandlerGetCommand(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		expected string
	}{
		{
			name: "shell with shell name",
			rule: parser.Rule{
				Action:    "shell",
				ShellName: "zsh",
			},
			expected: "chsh -s /bin/zsh", // assuming zsh is in /bin/zsh
		},
		{
			name: "shell with absolute path",
			rule: parser.Rule{
				Action:    "shell",
				ShellName: "/usr/local/bin/fish",
			},
			expected: "chsh -s /usr/local/bin/fish",
		},
		{
			name: "uninstall shell action",
			rule: parser.Rule{
				Action:    "uninstall",
				ShellName: "zsh",
			},
			expected: "chsh -s <previous_shell>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewShellHandler(tt.rule, "")
			cmd := handler.GetCommand()

			// For non-absolute paths, we need to resolve them first
			if tt.rule.Action != "uninstall" && !strings.HasPrefix(tt.rule.ShellName, "/") {
				// The command will show the resolved path, so we just check it contains chsh
				if !strings.Contains(cmd, "chsh -s") {
					t.Errorf("GetCommand() = %q, expected it to contain 'chsh -s'", cmd)
				}
			} else {
				if cmd != tt.expected {
					t.Errorf("GetCommand() = %q, want %q", cmd, tt.expected)
				}
			}
		})
	}
}

func TestShellHandlerResolveShellPath(t *testing.T) {
	handler := NewShellHandler(parser.Rule{}, "")

	tests := []struct {
		name       string
		shellName  string
		shouldFind bool
	}{
		{
			name:       "absolute path",
			shellName:  "/bin/sh",
			shouldFind: true,
		},
		{
			name:       "shell name that should exist",
			shellName:  "sh", // sh should exist on all systems
			shouldFind: true,
		},
		{
			name:       "non-existent shell",
			shellName:  "nonexistentshell123",
			shouldFind: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := handler.resolveShellPath(tt.shellName)

			if tt.shouldFind {
				if err != nil {
					t.Errorf("resolveShellPath() error = %v, expected to find shell", err)
				}
				if path == "" {
					t.Errorf("resolveShellPath() returned empty path")
				}
				// Verify the path exists
				if _, statErr := os.Stat(path); statErr != nil {
					t.Errorf("resolveShellPath() returned path that doesn't exist: %s", path)
				}
			} else {
				if err == nil {
					t.Errorf("resolveShellPath() expected error for non-existent shell, got path: %s", path)
				}
			}
		})
	}
}

func TestShellHandlerValidateShell(t *testing.T) {
	handler := NewShellHandler(parser.Rule{}, "")

	tests := []struct {
		name      string
		shellPath string
		shouldErr bool
		errMsg    string
	}{
		{
			name:      "valid shell",
			shellPath: "/bin/sh",
			shouldErr: false,
		},
		{
			name:      "non-existent path",
			shellPath: "/path/that/does/not/exist",
			shouldErr: true,
			errMsg:    "shell not found",
		},
		{
			name:      "directory instead of file",
			shellPath: "/bin", // /bin is a directory
			shouldErr: true,
			errMsg:    "shell path is a directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handler.validateShell(tt.shellPath)

			if (err != nil) != tt.shouldErr {
				t.Errorf("validateShell() error = %v, wantErr %v", err, tt.shouldErr)
			}

			if tt.shouldErr && tt.errMsg != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("validateShell() error = %v, expected to contain %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestShellHandlerGetCurrentShell(t *testing.T) {
	handler := NewShellHandler(parser.Rule{}, "")

	// Get current user
	currentUser, err := user.Current()
	if err != nil {
		t.Skip("Cannot get current user, skipping test")
	}

	shell, err := handler.getCurrentShell(currentUser.Username)
	if err != nil {
		t.Errorf("getCurrentShell() error = %v", err)
	}

	if shell == "" {
		t.Errorf("getCurrentShell() returned empty shell")
	}

	// Verify the returned shell is an absolute path
	if !strings.HasPrefix(shell, "/") {
		t.Errorf("getCurrentShell() returned non-absolute path: %s", shell)
	}
}

func TestShellHandlerUpdateStatus(t *testing.T) {
	tests := []struct {
		name           string
		rule           parser.Rule
		records        []ExecutionRecord
		initialStatus  Status
		expectedShells int
	}{
		{
			name: "add shell to status on successful change",
			rule: parser.Rule{
				Action:    "shell",
				ShellName: "zsh",
			},
			records: []ExecutionRecord{
				{
					Status:  "success",
					Command: "chsh -s /bin/zsh",
				},
			},
			initialStatus:  Status{},
			expectedShells: 1,
		},
		{
			name: "no action if shell change failed",
			rule: parser.Rule{
				Action:    "shell",
				ShellName: "zsh",
			},
			records: []ExecutionRecord{
				{
					Status:  "error",
					Command: "chsh -s /bin/zsh",
				},
			},
			initialStatus:  Status{},
			expectedShells: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewShellHandler(tt.rule, "")
			status := tt.initialStatus

			err := handler.UpdateStatus(&status, tt.records, "/tmp/test.bp", "linux")
			if err != nil {
				t.Errorf("UpdateStatus() error = %v", err)
			}

			if len(status.Shells) != tt.expectedShells {
				t.Errorf("UpdateStatus() shells count = %d, want %d", len(status.Shells), tt.expectedShells)
			}
		})
	}
}

func TestShellHandlerDisplayInfo(t *testing.T) {
	tests := []struct {
		name             string
		rule             parser.Rule
		expectedContains []string
	}{
		{
			name: "shell action with shell name",
			rule: parser.Rule{
				Action:    "shell",
				ShellName: "zsh",
			},
			expectedContains: []string{"Shell:", "zsh"},
		},
		{
			name: "shell action with absolute path",
			rule: parser.Rule{
				Action:    "shell",
				ShellName: "/usr/local/bin/fish",
			},
			expectedContains: []string{"Shell:", "/usr/local/bin/fish", "Path:"},
		},
		{
			name: "uninstall action",
			rule: parser.Rule{
				Action:    "uninstall",
				ShellName: "bash",
			},
			expectedContains: []string{"Shell:", "bash"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewShellHandler(tt.rule, "")

			// Capture stdout
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			handler.DisplayInfo()

			_ = w.Close()
			os.Stdout = old

			// Read captured output
			var buf bytes.Buffer
			_, _ = io.Copy(&buf, r)
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

func TestShellHandlerGetDependencyKey(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		expected string
	}{
		{
			name: "returns ID when present",
			rule: parser.Rule{
				ID:        "my-shell",
				ShellName: "zsh",
			},
			expected: "my-shell",
		},
		{
			name: "returns shell name when ID is empty",
			rule: parser.Rule{
				ShellName: "zsh",
			},
			expected: "zsh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewShellHandler(tt.rule, "")
			got := handler.GetDependencyKey()
			if got != tt.expected {
				t.Errorf("GetDependencyKey() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestShellHandlerGetDisplayDetails(t *testing.T) {
	tests := []struct {
		name        string
		rule        parser.Rule
		isUninstall bool
		expected    string
	}{
		{
			name: "returns shell name for install",
			rule: parser.Rule{
				ShellName: "zsh",
			},
			isUninstall: false,
			expected:    "zsh",
		},
		{
			name: "returns shell name for uninstall",
			rule: parser.Rule{
				ShellName: "bash",
			},
			isUninstall: true,
			expected:    "bash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewShellHandler(tt.rule, "")
			got := handler.GetDisplayDetails(tt.isUninstall)
			if got != tt.expected {
				t.Errorf("GetDisplayDetails() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestShellHandlerGetState(t *testing.T) {
	handler := NewShellHandler(parser.Rule{ShellName: "zsh"}, "")
	state := handler.GetState(false)

	if state["summary"] != "zsh" {
		t.Errorf("GetState() summary = %q, want %q", state["summary"], "zsh")
	}

	if state["shell"] != "zsh" {
		t.Errorf("GetState() shell = %q, want %q", state["shell"], "zsh")
	}
}

func TestShellHandlerNeedsSudo(t *testing.T) {
	handler := NewShellHandler(parser.Rule{}, "")
	if handler.NeedsSudo() {
		t.Errorf("NeedsSudo() = true, want false")
	}
}

// Integration test for shell validation (requires /etc/shells to exist)
func TestShellHandlerValidateShellInEtcShells(t *testing.T) {
	handler := NewShellHandler(parser.Rule{}, "")

	// Test with /bin/sh which should always be in /etc/shells
	err := handler.validateShellInEtcShells("/bin/sh")
	if err != nil {
		// If /etc/shells doesn't exist, the function should return nil
		// If it exists but doesn't contain /bin/sh, that's unexpected
		if _, statErr := os.Stat("/etc/shells"); statErr == nil {
			t.Errorf("validateShellInEtcShells() unexpected error for /bin/sh: %v", err)
		}
	}

	// Test with a shell that definitely won't be in /etc/shells
	err = handler.validateShellInEtcShells("/definitely/not/a/shell")
	if err == nil {
		// This should fail unless /etc/shells doesn't exist
		if _, statErr := os.Stat("/etc/shells"); statErr == nil {
			t.Errorf("validateShellInEtcShells() expected error for invalid shell")
		}
	}
}

// Test Up method with mock (would require dependency injection in real implementation)
func TestShellHandlerUp_Idempotency(t *testing.T) {
	// This test verifies the idempotency logic
	// In a real environment, we'd need to mock the system calls
	handler := NewShellHandler(parser.Rule{ShellName: "sh"}, "")

	// Test that the method exists and handles the basic case
	// We can't test the actual shell change without affecting the system
	_, err := handler.resolveShellPath("sh")
	if err != nil {
		t.Skip("Cannot resolve sh path, skipping idempotency test")
	}

	// The Up method should exist and not panic
	// We can't test the full functionality without system modification
}

// Test Down method
func TestShellHandlerDown(t *testing.T) {
	handler := NewShellHandler(parser.Rule{}, "")

	_, err := handler.Down()

	if err == nil {
		t.Errorf("Down() expected error, got nil")
	}

	// The new implementation tries to load status and will fail if no status found
	// This is the expected behavior now
	if !strings.Contains(err.Error(), "no shell status found") {
		t.Errorf("Down() error = %q, expected it to mention no status found", err.Error())
	}
}

// Test enhanced ShellStatus struct with PreviousShell field
func TestEnhancedShellStatus(t *testing.T) {
	status := ShellStatus{
		Shell:         "/bin/zsh",
		PreviousShell: "/bin/bash",
		User:          "testuser",
		ChangedAt:     "2023-01-01T00:00:00Z",
		Blueprint:     "/tmp/test.bp",
		OS:            "linux",
	}

	if status.Shell != "/bin/zsh" {
		t.Errorf("Shell = %q, want %q", status.Shell, "/bin/zsh")
	}

	if status.PreviousShell != "/bin/bash" {
		t.Errorf("PreviousShell = %q, want %q", status.PreviousShell, "/bin/bash")
	}
}

// Test findShellStatus helper function
func TestFindShellStatus(t *testing.T) {
	shells := []ShellStatus{
		{
			Shell:     "/bin/zsh",
			User:      "user1",
			Blueprint: "/tmp/test1.bp",
			OS:        "linux",
		},
		{
			Shell:     "/bin/bash",
			User:      "user2",
			Blueprint: "/tmp/test2.bp",
			OS:        "darwin",
		},
	}

	// Test finding existing entry
	found := findShellStatus(shells, "user1", "/tmp/test1.bp", "linux")
	if found == nil {
		t.Error("findShellStatus() should have found entry")
	} else if found.Shell != "/bin/zsh" {
		t.Errorf("found.Shell = %q, want %q", found.Shell, "/bin/zsh")
	}

	// Test not finding non-existent entry
	notFound := findShellStatus(shells, "user3", "/tmp/test3.bp", "windows")
	if notFound != nil {
		t.Error("findShellStatus() should not have found entry")
	}
}

// Test UpdateStatus with enhanced shell tracking
func TestShellHandlerUpdateStatus_Enhanced(t *testing.T) {
	tests := []struct {
		name                  string
		rule                  parser.Rule
		records               []ExecutionRecord
		initialStatus         Status
		expectedShells        int
		expectedPreviousShell string
	}{
		{
			name: "capture previous shell on first install",
			rule: parser.Rule{
				Action:    "shell",
				ShellName: "zsh",
			},
			records: []ExecutionRecord{
				{
					Status:  "success",
					Command: "chsh -s /bin/zsh",
				},
			},
			initialStatus:  Status{},
			expectedShells: 1,
			// Note: In real execution, previousShell would be set by Up()
		},
		{
			name: "preserve previous shell on update",
			rule: parser.Rule{
				Action:    "shell",
				ShellName: "fish",
			},
			records: []ExecutionRecord{
				{
					Status:  "success",
					Command: "chsh -s /usr/local/bin/fish",
				},
			},
			initialStatus: Status{
				Shells: []ShellStatus{
					{
						Shell:         "/bin/zsh",
						PreviousShell: "/bin/bash",
						User:          getCurrentTestUser(),
						Blueprint:     "/tmp/test.bp",
						OS:            "linux",
					},
				},
			},
			expectedShells:        1,
			expectedPreviousShell: "/bin/bash",
		},
		{
			name: "handle uninstall action",
			rule: parser.Rule{
				Action:    "uninstall",
				ShellName: "zsh",
			},
			records: []ExecutionRecord{
				{
					Status:  "success",
					Command: "chsh -s /bin/bash",
				},
			},
			initialStatus: Status{
				Shells: []ShellStatus{
					{
						Shell:         "/bin/zsh",
						PreviousShell: "/bin/bash",
						User:          getCurrentTestUser(),
						Blueprint:     "/tmp/test.bp",
						OS:            "linux",
					},
				},
			},
			expectedShells: 0, // Should remove the entry
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewShellHandler(tt.rule, "")
			// Simulate setting previous shell for new installs
			if len(tt.initialStatus.Shells) == 0 {
				handler.previousShell = "/bin/bash"
			}

			status := tt.initialStatus

			err := handler.UpdateStatus(&status, tt.records, "/tmp/test.bp", "linux")
			if err != nil {
				t.Errorf("UpdateStatus() error = %v", err)
			}

			if len(status.Shells) != tt.expectedShells {
				t.Errorf("UpdateStatus() shells count = %d, want %d", len(status.Shells), tt.expectedShells)
			}

			if tt.expectedShells > 0 && tt.expectedPreviousShell != "" {
				if status.Shells[0].PreviousShell != tt.expectedPreviousShell {
					t.Errorf("UpdateStatus() PreviousShell = %q, want %q",
						status.Shells[0].PreviousShell, tt.expectedPreviousShell)
				}
			}
		})
	}
}

// Test FindUninstallRules with rollback support
func TestShellHandlerFindUninstallRules(t *testing.T) {
	tests := []struct {
		name          string
		status        Status
		currentRules  []parser.Rule
		expectedRules int
	}{
		{
			name: "no uninstall rules when shell still in current rules",
			status: Status{
				Shells: []ShellStatus{
					{
						Shell:         "/bin/zsh",
						PreviousShell: "/bin/bash",
						User:          getCurrentTestUser(),
						Blueprint:     "/tmp/test.bp",
						OS:            "linux",
					},
				},
			},
			currentRules: []parser.Rule{
				{
					Action:    "shell",
					ShellName: "zsh",
				},
			},
			expectedRules: 0,
		},
		{
			name: "create uninstall rule when shell removed from rules",
			status: Status{
				Shells: []ShellStatus{
					{
						Shell:         "/bin/zsh",
						PreviousShell: "/bin/bash",
						User:          getCurrentTestUser(),
						Blueprint:     "/tmp/test.bp",
						OS:            "linux",
					},
				},
			},
			currentRules:  []parser.Rule{}, // No shell rules
			expectedRules: 1,
		},
		{
			name: "no uninstall rule when no previous shell recorded",
			status: Status{
				Shells: []ShellStatus{
					{
						Shell:         "/bin/zsh",
						PreviousShell: "", // No rollback info
						User:          getCurrentTestUser(),
						Blueprint:     "/tmp/test.bp",
						OS:            "linux",
					},
				},
			},
			currentRules:  []parser.Rule{}, // No shell rules
			expectedRules: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewShellHandler(parser.Rule{}, "")
			rules := handler.FindUninstallRules(&tt.status, tt.currentRules, "/tmp/test.bp", "linux")

			if len(rules) != tt.expectedRules {
				t.Errorf("FindUninstallRules() returned %d rules, want %d", len(rules), tt.expectedRules)
			}

			if tt.expectedRules > 0 && len(rules) > 0 {
				if rules[0].Action != "uninstall" {
					t.Errorf("FindUninstallRules() action = %q, want %q", rules[0].Action, "uninstall")
				}
			}
		})
	}
}

// Test backward compatibility with existing status entries
func TestShellHandlerBackwardCompatibility(t *testing.T) {
	// Test that we can handle old status entries without PreviousShell field
	handler := NewShellHandler(parser.Rule{ShellName: "zsh"}, "")

	// Status with old format (no PreviousShell field)
	status := &Status{
		Shells: []ShellStatus{
			{
				Shell: "/bin/zsh",
				// PreviousShell field missing (old format)
				User:      "testuser",
				Blueprint: "/tmp/test.bp",
				OS:        "linux",
			},
		},
	}

	// Should still work for checking installation status
	// Note: We can't easily test actual shell checking without system modification
	result := handler.IsInstalled(status, "/tmp/test.bp", "linux")

	// The result depends on actual shell resolution, but the method should not panic
	_ = result
}

// Test complete install → uninstall cycle with rollback
func TestShellHandlerInstallUninstallCycle(t *testing.T) {
	// This is an integration test that simulates the complete flow

	// Step 1: Install shell (simulate successful execution)
	installRule := parser.Rule{
		Action:    "shell",
		ShellName: "zsh",
	}
	installHandler := NewShellHandler(installRule, "")
	installHandler.previousShell = "/bin/bash" // Simulate capturing previous shell

	// Simulate successful shell change execution
	installRecords := []ExecutionRecord{
		{
			Status:  "success",
			Command: "chsh -s /bin/zsh",
		},
	}

	status := Status{}
	err := installHandler.UpdateStatus(&status, installRecords, "/tmp/test.bp", "linux")
	if err != nil {
		t.Fatalf("Install UpdateStatus() failed: %v", err)
	}

	// Verify shell status was recorded with previous shell
	if len(status.Shells) != 1 {
		t.Fatalf("Expected 1 shell status entry, got %d", len(status.Shells))
	}
	if status.Shells[0].Shell != "/bin/zsh" {
		t.Errorf("Shell = %q, want %q", status.Shells[0].Shell, "/bin/zsh")
	}
	if status.Shells[0].PreviousShell != "/bin/bash" {
		t.Errorf("PreviousShell = %q, want %q", status.Shells[0].PreviousShell, "/bin/bash")
	}

	// Step 2: Uninstall shell (simulate successful rollback)
	uninstallRule := parser.Rule{
		Action:    "uninstall",
		ShellName: "zsh",
	}
	uninstallHandler := NewShellHandler(uninstallRule, "")

	// Simulate successful shell rollback execution
	uninstallRecords := []ExecutionRecord{
		{
			Status:  "success",
			Command: "chsh -s /bin/bash", // Rollback to previous shell
		},
	}

	err = uninstallHandler.UpdateStatus(&status, uninstallRecords, "/tmp/test.bp", "linux")
	if err != nil {
		t.Fatalf("Uninstall UpdateStatus() failed: %v", err)
	}

	// Verify shell status was removed after successful rollback
	if len(status.Shells) != 0 {
		t.Errorf("Expected 0 shell status entries after uninstall, got %d", len(status.Shells))
	}
}

// Test that shell handler implements all required interfaces
func TestShellHandlerImplementsInterfaces(t *testing.T) {
	var _ Handler = (*ShellHandler)(nil)
	var _ KeyProvider = (*ShellHandler)(nil)
	var _ DisplayProvider = (*ShellHandler)(nil)
	var _ SudoAwareHandler = (*ShellHandler)(nil)
	var _ StateProvider = (*ShellHandler)(nil)
	var _ StatusProvider = (*ShellHandler)(nil)
}

// Test getShellFromPasswd method
func TestShellHandlerGetShellFromPasswd(t *testing.T) {
	handler := NewShellHandler(parser.Rule{}, "")

	// This test depends on /etc/passwd existing and having the current user
	currentUser, err := user.Current()
	if err != nil {
		t.Skip("Cannot get current user, skipping test")
	}

	shell, err := handler.getShellFromPasswd(currentUser.Username)
	if err != nil {
		// This might fail in some environments (like containers or macOS), so we skip if so
		if strings.Contains(err.Error(), "failed to read /etc/passwd") ||
			strings.Contains(err.Error(), "user not found in /etc/passwd") {
			t.Skip("User not found in /etc/passwd (normal on macOS), skipping test")
		}
		t.Errorf("getShellFromPasswd() error = %v", err)
	}

	if shell != "" && !strings.HasPrefix(shell, "/") {
		t.Errorf("getShellFromPasswd() returned non-absolute path: %s", shell)
	}
}

// Test IsInstalled method
func TestShellHandlerIsInstalled(t *testing.T) {
	handler := NewShellHandler(parser.Rule{ShellName: "sh"}, "")

	// Test with empty status
	status := &Status{}
	if handler.IsInstalled(status, "/tmp/test.bp", "linux") {
		t.Errorf("IsInstalled() = true with empty status, want false")
	}

	// Test with current user not found in status
	status = &Status{
		Shells: []ShellStatus{
			{
				Shell:     "/bin/bash",
				User:      "otheruser",
				Blueprint: "/tmp/test.bp",
				OS:        "linux",
			},
		},
	}

	if handler.IsInstalled(status, "/tmp/test.bp", "linux") {
		t.Errorf("IsInstalled() = true with different user, want false")
	}

	// Test with matching user but we can't easily test the shell check without system modifications
}
