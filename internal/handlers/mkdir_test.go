package handlers

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

func TestMkdirHandlerGetCommand(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		expected string
	}{
		{
			name: "mkdir without permissions",
			rule: parser.Rule{
				Action: "mkdir",
				Mkdir:  "/tmp/test-dir",
			},
			expected: "mkdir -p /tmp/test-dir",
		},
		{
			name: "mkdir with permissions",
			rule: parser.Rule{
				Action:       "mkdir",
				Mkdir:        "/tmp/test-dir",
				MkdirPerms:   "755",
			},
			expected: "mkdir -p /tmp/test-dir && chmod 755 /tmp/test-dir",
		},
		{
			name: "mkdir with path containing spaces",
			rule: parser.Rule{
				Action: "mkdir",
				Mkdir:  "/tmp/my test dir",
			},
			expected: "mkdir -p '/tmp/my test dir'",
		},
		{
			name: "uninstall - remove directory",
			rule: parser.Rule{
				Action: "uninstall",
				Mkdir:  "/tmp/test-dir",
			},
			expected: "rm -rf /tmp/test-dir",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewMkdirHandler(tt.rule, "")
			cmd := handler.GetCommand()
			if cmd != tt.expected {
				t.Errorf("GetCommand() = %q, want %q", cmd, tt.expected)
			}
		})
	}
}

func TestMkdirHandlerUp(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "test-mkdir")

	tests := []struct {
		name      string
		rule      parser.Rule
		shouldErr bool
		checkFunc func(string) bool // Check if dir was created properly
	}{
		{
			name: "create directory successfully",
			rule: parser.Rule{
				Action: "mkdir",
				Mkdir:  testDir,
			},
			shouldErr: false,
			checkFunc: func(path string) bool {
				info, err := os.Stat(path)
				return err == nil && info.IsDir()
			},
		},
		{
			name: "create directory with permissions",
			rule: parser.Rule{
				Action:     "mkdir",
				Mkdir:      filepath.Join(tmpDir, "test-perms"),
				MkdirPerms: "700",
			},
			shouldErr: false,
			checkFunc: func(path string) bool {
				info, err := os.Stat(path)
				if err != nil {
					return false
				}
				// Check permissions are restrictive (700)
				return info.Mode().Perm() == 0700
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewMkdirHandler(tt.rule, "")
			output, err := handler.Up()

			if (err != nil) != tt.shouldErr {
				t.Errorf("Up() error = %v, wantErr %v", err, tt.shouldErr)
			}

			if !tt.shouldErr && output == "" {
				t.Errorf("Up() returned empty output")
			}

			if tt.checkFunc != nil && !tt.checkFunc(tt.rule.Mkdir) {
				t.Errorf("Up() did not create directory properly")
			}
		})
	}
}

func TestMkdirHandlerDown(t *testing.T) {
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "test-remove")

	// Create a directory first
	os.MkdirAll(testDir, 0755)

	tests := []struct {
		name      string
		rule      parser.Rule
		shouldErr bool
		checkFunc func(string) bool // Check if dir was removed
	}{
		{
			name: "remove existing directory",
			rule: parser.Rule{
				Action: "mkdir",
				Mkdir:  testDir,
			},
			shouldErr: false,
			checkFunc: func(path string) bool {
				_, err := os.Stat(path)
				return os.IsNotExist(err)
			},
		},
		{
			name: "remove non-existent directory",
			rule: parser.Rule{
				Action: "mkdir",
				Mkdir:  "/tmp/non-existent-dir-xyz",
			},
			shouldErr: false,
			checkFunc: func(path string) bool {
				return true // Should succeed with message
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Recreate dir for each test
			if tt.name == "remove existing directory" {
				os.MkdirAll(testDir, 0755)
			}

			handler := NewMkdirHandler(tt.rule, "")
			output, err := handler.Down()

			if (err != nil) != tt.shouldErr {
				t.Errorf("Down() error = %v, wantErr %v", err, tt.shouldErr)
			}

			if output == "" {
				t.Errorf("Down() returned empty output")
			}

			if tt.checkFunc != nil && !tt.checkFunc(tt.rule.Mkdir) {
				t.Errorf("Down() verification failed")
			}
		})
	}
}

func TestMkdirHandlerUpdateStatus(t *testing.T) {
	tests := []struct {
		name            string
		rule            parser.Rule
		records         []ExecutionRecord
		initialStatus   Status
		expectedMkdirs  int
		shouldContainMk bool
	}{
		{
			name: "add mkdir to status",
			rule: parser.Rule{
				Action: "mkdir",
				Mkdir:  "/tmp/test-dir",
			},
			records: []ExecutionRecord{
				{
					Status:  "success",
					Command: "mkdir -p /tmp/test-dir",
				},
			},
			initialStatus:   Status{},
			expectedMkdirs:  1,
			shouldContainMk: true,
		},
		{
			name: "remove mkdir from status on uninstall",
			rule: parser.Rule{
				Action: "uninstall",
				Mkdir:  "/tmp/test-dir",
			},
			records:         []ExecutionRecord{},
			initialStatus:   Status{},
			expectedMkdirs:  0,
			shouldContainMk: false,
		},
		{
			name: "no action if mkdir failed",
			rule: parser.Rule{
				Action: "mkdir",
				Mkdir:  "/tmp/test-dir",
			},
			records: []ExecutionRecord{
				{
					Status:  "error",
					Command: "mkdir -p /tmp/test-dir",
				},
			},
			initialStatus:   Status{},
			expectedMkdirs:  0,
			shouldContainMk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewMkdirHandler(tt.rule, "")
			status := tt.initialStatus

			err := handler.UpdateStatus(&status, tt.records, "/tmp/test.bp", "mac")
			if err != nil {
				t.Errorf("UpdateStatus() error = %v", err)
			}

			if len(status.Mkdirs) != tt.expectedMkdirs {
				t.Errorf("UpdateStatus() Mkdirs count = %d, want %d", len(status.Mkdirs), tt.expectedMkdirs)
			}

			if tt.shouldContainMk && len(status.Mkdirs) > 0 {
				if status.Mkdirs[0].Path != tt.rule.Mkdir {
					t.Errorf("UpdateStatus() mkdir path = %q, want %q", status.Mkdirs[0].Path, tt.rule.Mkdir)
				}
			}
		})
	}
}

func TestMkdirHandlerDisplayInfo(t *testing.T) {
	tests := []struct {
		name             string
		rule             parser.Rule
		expectedContains []string
	}{
		{
			name: "mkdir action with path only",
			rule: parser.Rule{
				Action: "mkdir",
				Mkdir:  "/tmp/test-dir",
			},
			expectedContains: []string{"Path:", "/tmp/test-dir"},
		},
		{
			name: "mkdir action with permissions",
			rule: parser.Rule{
				Action:     "mkdir",
				Mkdir:      "~/.config/myapp",
				MkdirPerms: "700",
			},
			expectedContains: []string{"Path:", "~/.config/myapp", "Permissions:", "700"},
		},
		{
			name: "mkdir action with different permissions",
			rule: parser.Rule{
				Action:     "mkdir",
				Mkdir:      "/var/log/myapp",
				MkdirPerms: "755",
			},
			expectedContains: []string{"Path:", "/var/log/myapp", "Permissions:", "755"},
		},
		{
			name: "uninstall action",
			rule: parser.Rule{
				Action: "uninstall",
				Mkdir:  "/tmp/remove-me",
			},
			expectedContains: []string{"Path:", "/tmp/remove-me"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewMkdirHandler(tt.rule, "")

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
