package handlers

import (
	"strings"
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

// TestValidateShellName tests the validateShellName function for security
func TestValidateShellName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid shell name",
			input:   "zsh",
			wantErr: false,
		},
		{
			name:    "valid shell name with dash",
			input:   "bash-5",
			wantErr: false,
		},
		{
			name:    "valid shell name with underscore",
			input:   "my_shell",
			wantErr: false,
		},
		{
			name:    "command injection attempt with semicolon",
			input:   "zsh; rm -rf /",
			wantErr: true,
			errMsg:  "unsafe characters",
		},
		{
			name:    "command injection attempt with pipe",
			input:   "zsh | cat /etc/passwd",
			wantErr: true,
			errMsg:  "unsafe characters",
		},
		{
			name:    "command injection attempt with backticks",
			input:   "zsh`whoami`",
			wantErr: true,
			errMsg:  "unsafe characters",
		},
		{
			name:    "command injection attempt with dollar",
			input:   "zsh$(id)",
			wantErr: true,
			errMsg:  "unsafe characters",
		},
		{
			name:    "path traversal attempt",
			input:   "../../../bin/sh",
			wantErr: true,
			errMsg:  "unsafe characters", // caught by regex validation first
		},
		{
			name:    "path traversal with dots in shell name",
			input:   "shell..name",
			wantErr: true,
			errMsg:  "unsafe characters", // dots are caught by regex validation first
		},
		{
			name:    "pure path traversal dots",
			input:   "..",
			wantErr: true,
			errMsg:  "unsafe characters", // dots are caught by regex validation first
		},
		{
			name:    "shell metacharacter ampersand",
			input:   "zsh&",
			wantErr: true,
			errMsg:  "unsafe characters",
		},
		{
			name:    "shell metacharacter greater than",
			input:   "zsh>file",
			wantErr: true,
			errMsg:  "unsafe characters",
		},
		{
			name:    "shell metacharacter less than",
			input:   "zsh<file",
			wantErr: true,
			errMsg:  "unsafe characters",
		},
		{
			name:    "shell metacharacter asterisk",
			input:   "zsh*",
			wantErr: true,
			errMsg:  "unsafe characters",
		},
		{
			name:    "shell metacharacter question mark",
			input:   "zsh?",
			wantErr: true,
			errMsg:  "unsafe characters",
		},
		{
			name:    "space character",
			input:   "my shell",
			wantErr: true,
			errMsg:  "unsafe characters",
		},
		{
			name:    "newline character",
			input:   "zsh\nrm -rf /",
			wantErr: true,
			errMsg:  "unsafe characters",
		},
		{
			name:    "carriage return character",
			input:   "zsh\rrm -rf /",
			wantErr: true,
			errMsg:  "unsafe characters",
		},
		{
			name:    "tab character",
			input:   "zsh\trm",
			wantErr: true,
			errMsg:  "unsafe characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateShellName(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("validateShellName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("validateShellName() error = %v, expected to contain %q", err, tt.errMsg)
				}
			}
		})
	}
}

// TestValidateUsername tests the validateUsername function for security
func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid username",
			input:   "user123",
			wantErr: false,
		},
		{
			name:    "valid username with underscore",
			input:   "my_user",
			wantErr: false,
		},
		{
			name:    "valid username with dash",
			input:   "user-name",
			wantErr: false,
		},
		{
			name:    "valid username with dot",
			input:   "user.name",
			wantErr: false,
		},
		{
			name:    "command injection attempt with semicolon",
			input:   "user; rm -rf /",
			wantErr: true,
			errMsg:  "unsafe characters",
		},
		{
			name:    "command injection attempt with pipe",
			input:   "user | cat /etc/passwd",
			wantErr: true,
			errMsg:  "unsafe characters",
		},
		{
			name:    "command injection attempt with backticks",
			input:   "user`whoami`",
			wantErr: true,
			errMsg:  "unsafe characters",
		},
		{
			name:    "command injection attempt with dollar",
			input:   "user$(id)",
			wantErr: true,
			errMsg:  "unsafe characters",
		},
		{
			name:    "path traversal attempt",
			input:   "../../../etc/passwd",
			wantErr: true,
			errMsg:  "unsafe characters", // caught by regex validation first
		},
		{
			name:    "path traversal with dots",
			input:   "user..name",
			wantErr: true,
			errMsg:  "path traversal not allowed",
		},
		{
			name:    "username with space",
			input:   "user name",
			wantErr: true,
			errMsg:  "unsafe characters",
		},
		{
			name:    "username with newline",
			input:   "user\nrm -rf /",
			wantErr: true,
			errMsg:  "unsafe characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUsername(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("validateUsername() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("validateUsername() error = %v, expected to contain %q", err, tt.errMsg)
				}
			}
		})
	}
}

// TestShellHandlerSecurityValidation tests that the shell handler properly validates input
func TestShellHandlerSecurityValidation(t *testing.T) {
	tests := []struct {
		name      string
		shellName string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "malicious shell name fails validation",
			shellName: "zsh; rm -rf /",
			wantErr:   true,
			errMsg:    "shell name validation failed",
		},
		{
			name:      "path traversal attempt fails validation",
			shellName: "../../../bin/sh",
			wantErr:   true,
			errMsg:    "path traversal not allowed",
		},
		{
			name:      "command injection with backticks fails",
			shellName: "zsh`whoami`",
			wantErr:   true,
			errMsg:    "shell name validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := parser.Rule{
				Action:    "shell",
				ShellName: tt.shellName,
			}
			handler := NewShellHandler(rule, "")

			// Test Up method (which contains the validation)
			_, err := handler.Up()

			if (err != nil) != tt.wantErr {
				t.Errorf("ShellHandler.Up() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ShellHandler.Up() error = %v, expected to contain %q", err, tt.errMsg)
				}
			}
		})
	}
}

// TestResolveShellPathSecurity tests that shell path resolution is secure
func TestResolveShellPathSecurity(t *testing.T) {
	handler := NewShellHandler(parser.Rule{}, "")

	tests := []struct {
		name      string
		shellName string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "command injection in shell name",
			shellName: "zsh; echo pwned",
			wantErr:   true,
			errMsg:    "invalid shell name for 'which' command",
		},
		{
			name:      "path traversal in shell name",
			shellName: "../../../bin/bash",
			wantErr:   true,
			errMsg:    "path traversal not allowed",
		},
		{
			name:      "shell metacharacter pipe",
			shellName: "zsh|cat",
			wantErr:   true,
			errMsg:    "invalid shell name for 'which' command",
		},
		{
			name:      "shell metacharacter ampersand",
			shellName: "zsh&whoami",
			wantErr:   true,
			errMsg:    "invalid shell name for 'which' command",
		},
		{
			name:      "valid shell name works",
			shellName: "nonexistentshell123", // will fail to resolve but pass validation
			wantErr:   true,
			errMsg:    "not found in common locations", // fails because shell doesn't exist
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handler.resolveShellPath(tt.shellName)

			if (err != nil) != tt.wantErr {
				t.Errorf("resolveShellPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("resolveShellPath() error = %v, expected to contain %q", err, tt.errMsg)
				}
			}
		})
	}
}

// TestGetCurrentShellSecurity tests that getCurrentShell properly validates usernames
func TestGetCurrentShellSecurity(t *testing.T) {
	handler := NewShellHandler(parser.Rule{}, "")

	tests := []struct {
		name     string
		username string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "command injection in username",
			username: "user; rm -rf /",
			wantErr:  true,
			errMsg:   "username validation failed",
		},
		{
			name:     "path traversal in username",
			username: "../../../etc/passwd",
			wantErr:  true,
			errMsg:   "username validation failed",
		},
		{
			name:     "shell metacharacter pipe in username",
			username: "user|cat",
			wantErr:  true,
			errMsg:   "username validation failed",
		},
		{
			name:     "backtick injection in username",
			username: "user`whoami`",
			wantErr:  true,
			errMsg:   "username validation failed",
		},
		{
			name:     "dollar injection in username",
			username: "user$(id)",
			wantErr:  true,
			errMsg:   "username validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handler.getCurrentShell(tt.username)

			if (err != nil) != tt.wantErr {
				t.Errorf("getCurrentShell() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("getCurrentShell() error = %v, expected to contain %q", err, tt.errMsg)
				}
			}
		})
	}
}

// TestGetShellFromPasswdSecurity tests that getShellFromPasswd properly validates usernames
func TestGetShellFromPasswdSecurity(t *testing.T) {
	handler := NewShellHandler(parser.Rule{}, "")

	tests := []struct {
		name     string
		username string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "command injection attempt",
			username: "user; cat /etc/shadow",
			wantErr:  true,
			errMsg:   "username validation failed",
		},
		{
			name:     "path traversal attempt",
			username: "../../root",
			wantErr:  true,
			errMsg:   "username validation failed",
		},
		{
			name:     "newline injection",
			username: "user\nroot",
			wantErr:  true,
			errMsg:   "username validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handler.getShellFromPasswd(tt.username)

			if (err != nil) != tt.wantErr {
				t.Errorf("getShellFromPasswd() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("getShellFromPasswd() error = %v, expected to contain %q", err, tt.errMsg)
				}
			}
		})
	}
}

// TestPathCleaningInChshCommand tests that shell paths are properly cleaned
func TestPathCleaningInChshCommand(t *testing.T) {
	// Test that the GetCommand method shows cleaned paths
	tests := []struct {
		name      string
		shellPath string
		expected  string
	}{
		{
			name:      "normal path",
			shellPath: "/bin/zsh",
			expected:  "chsh -s /bin/zsh",
		},
		{
			name:      "path with redundant slashes",
			shellPath: "/bin//zsh",
			expected:  "chsh -s /bin/zsh", // filepath.Clean should normalize this
		},
		{
			name:      "path with current directory reference",
			shellPath: "/bin/./zsh",
			expected:  "chsh -s /bin/zsh", // filepath.Clean should normalize this
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := parser.Rule{
				Action:    "shell",
				ShellName: tt.shellPath,
			}
			handler := NewShellHandler(rule, "")
			cmd := handler.GetCommand()

			if cmd != tt.expected {
				t.Errorf("GetCommand() = %q, want %q", cmd, tt.expected)
			}
		})
	}
}
