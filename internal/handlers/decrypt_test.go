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

func TestDecryptHandlerGetCommand(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		expected string
	}{
		{
			name: "decrypt file",
			rule: parser.Rule{
				Action:      "decrypt",
				DecryptFile: "/path/to/encrypted.enc",
				DecryptPath: "~/.ssh/id_rsa",
			},
			expected: "Decrypt file: /path/to/encrypted.enc → ~/.ssh/id_rsa",
		},
		{
			name: "uninstall - remove decrypted file",
			rule: parser.Rule{
				Action:      "uninstall",
				DecryptPath: "~/.ssh/id_rsa",
			},
			expected: "rm -f ~/.ssh/id_rsa",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			passwordCache := make(map[string]string)
			handler := NewDecryptHandler(tt.rule, "", passwordCache)
			cmd := handler.GetCommand()
			if cmd != tt.expected {
				t.Errorf("GetCommand() = %q, want %q", cmd, tt.expected)
			}
		})
	}
}

func TestDecryptHandlerDown(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "decrypted-file")

	// Create a test file
	_ = os.WriteFile(testFile, []byte("secret"), 0600)

	tests := []struct {
		name      string
		rule      parser.Rule
		shouldErr bool
		checkFunc func(string) bool
	}{
		{
			name: "remove existing decrypted file",
			rule: parser.Rule{
				Action:      "decrypt",
				DecryptPath: testFile,
			},
			shouldErr: false,
			checkFunc: func(path string) bool {
				_, err := os.Stat(path)
				return os.IsNotExist(err)
			},
		},
		{
			name: "remove non-existent file",
			rule: parser.Rule{
				Action:      "decrypt",
				DecryptPath: "/tmp/non-existent-file-xyz",
			},
			shouldErr: false,
			checkFunc: func(path string) bool {
				return true // Should succeed with message
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Recreate file for test
			if tt.name == "remove existing decrypted file" {
				_ = os.WriteFile(testFile, []byte("secret"), 0600)
			}

			passwordCache := make(map[string]string)
			handler := NewDecryptHandler(tt.rule, "", passwordCache)
			output, err := handler.Down()

			if (err != nil) != tt.shouldErr {
				t.Errorf("Down() error = %v, wantErr %v", err, tt.shouldErr)
			}

			if output == "" {
				t.Errorf("Down() returned empty output")
			}

			if tt.checkFunc != nil && !tt.checkFunc(tt.rule.DecryptPath) {
				t.Errorf("Down() verification failed")
			}
		})
	}
}

func TestDecryptHandlerUpdateStatus(t *testing.T) {
	tests := []struct {
		name             string
		rule             parser.Rule
		records          []ExecutionRecord
		initialStatus    Status
		expectedDecrypts int
		shouldContain    bool
	}{
		{
			name: "add decrypted file to status",
			rule: parser.Rule{
				Action:      "decrypt",
				DecryptFile: "/path/to/encrypted.enc",
				DecryptPath: "~/.ssh/id_rsa",
			},
			records: []ExecutionRecord{
				{
					Status:  "success",
					Command: "Decrypt file: /path/to/encrypted.enc → ~/.ssh/id_rsa",
				},
			},
			initialStatus:    Status{},
			expectedDecrypts: 1,
			shouldContain:    true,
		},
		{
			name: "remove decrypted file from status on uninstall",
			rule: parser.Rule{
				Action:      "uninstall",
				DecryptFile: "/path/to/encrypted.enc",
				DecryptPath: "~/.nonexistent/test_secret.key",
			},
			records: []ExecutionRecord{},
			initialStatus: Status{
				Decrypts: []DecryptStatus{
					{
						SourceFile: "/path/to/encrypted.enc",
						DestPath:   "~/.nonexistent/test_secret.key",
						Blueprint:  "/tmp/test.bp",
						OS:         "mac",
					},
				},
			},
			expectedDecrypts: 0,
			shouldContain:    false,
		},
		{
			name: "no action if decrypt command not found",
			rule: parser.Rule{
				Action:      "decrypt",
				DecryptFile: "/path/to/encrypted.enc",
				DecryptPath: "~/.ssh/id_rsa",
			},
			records: []ExecutionRecord{
				{
					Status:  "error",
					Command: "decrypt /path/to/encrypted.enc to ~/.ssh/id_rsa",
				},
			},
			initialStatus:    Status{},
			expectedDecrypts: 0,
			shouldContain:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			passwordCache := make(map[string]string)
			handler := NewDecryptHandler(tt.rule, "", passwordCache)
			status := tt.initialStatus

			err := handler.UpdateStatus(&status, tt.records, "/tmp/test.bp", "mac")
			if err != nil {
				t.Errorf("UpdateStatus() error = %v", err)
			}

			if len(status.Decrypts) != tt.expectedDecrypts {
				t.Errorf("UpdateStatus() decrypts count = %d, want %d", len(status.Decrypts), tt.expectedDecrypts)
			}

			if tt.shouldContain && len(status.Decrypts) > 0 {
				if status.Decrypts[0].SourceFile != tt.rule.DecryptFile {
					t.Errorf("UpdateStatus() source file = %q, want %q", status.Decrypts[0].SourceFile, tt.rule.DecryptFile)
				}
			}
		})
	}
}

func TestDecryptHandlerDisplayInfo(t *testing.T) {
	tests := []struct {
		name             string
		rule             parser.Rule
		expectedContains []string
	}{
		{
			name: "decrypt action with file and path",
			rule: parser.Rule{
				Action:      "decrypt",
				DecryptFile: "/path/to/encrypted.enc",
				DecryptPath: "~/.ssh/id_rsa",
			},
			expectedContains: []string{"File:", "/path/to/encrypted.enc", "Path:", "~/.ssh/id_rsa"},
		},
		{
			name: "decrypt action with password ID",
			rule: parser.Rule{
				Action:            "decrypt",
				DecryptFile:       "/secrets/api-key.enc",
				DecryptPath:       "~/.config/api-key",
				DecryptPasswordID: "production",
			},
			expectedContains: []string{"File:", "/secrets/api-key.enc", "Path:", "~/.config/api-key", "Password ID:", "production"},
		},
		{
			name: "decrypt action with group",
			rule: parser.Rule{
				Action:      "decrypt",
				DecryptFile: "/config/secret.enc",
				DecryptPath: "~/.secret",
				Group:       "credentials",
			},
			expectedContains: []string{"File:", "/config/secret.enc", "Path:", "~/.secret", "Group:", "credentials"},
		},
		{
			name: "uninstall action",
			rule: parser.Rule{
				Action:      "uninstall",
				DecryptFile: "/path/to/encrypted.enc",
				DecryptPath: "~/.ssh/id_rsa",
			},
			expectedContains: []string{"File:", "/path/to/encrypted.enc", "Path:", "~/.ssh/id_rsa"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			passwordCache := make(map[string]string)
			handler := NewDecryptHandler(tt.rule, "", passwordCache)

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


func TestDecryptHandlerGetDependencyKey(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		expected string
	}{
		{
			name: "returns ID when present",
			rule: parser.Rule{
				ID:          "my-decrypt",
				DecryptPath: "~/.ssh/config",
			},
			expected: "my-decrypt",
		},
		{
			name: "returns decrypt path when ID is empty",
			rule: parser.Rule{
				DecryptPath: "~/.ssh/config",
			},
			expected: "~/.ssh/config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewDecryptHandler(tt.rule, "", nil)
			got := handler.GetDependencyKey()
			if got != tt.expected {
				t.Errorf("GetDependencyKey() = %q, want %q", got, tt.expected)
			}
		})
	}
}
