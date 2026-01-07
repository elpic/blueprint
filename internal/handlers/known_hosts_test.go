package handlers

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

func TestKnownHostsHandlerGetCommand(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		expected string
	}{
		{
			name: "add host to known_hosts",
			rule: parser.Rule{
				Action:     "known_hosts",
				KnownHosts: "github.com",
			},
			expected: "ssh-keyscan -t ed25519 github.com",
		},
		{
			name: "add host with specific key type",
			rule: parser.Rule{
				Action:        "known_hosts",
				KnownHosts:    "example.com",
				KnownHostsKey: "rsa",
			},
			expected: "ssh-keyscan -t rsa example.com",
		},
		{
			name: "uninstall - remove from known_hosts",
			rule: parser.Rule{
				Action:     "uninstall",
				KnownHosts: "github.com",
			},
			expected: `sed -i.bak '/^github\.com[, ]/d' ~/.ssh/known_hosts && rm -f ~/.ssh/known_hosts.bak`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewKnownHostsHandler(tt.rule, "")
			cmd := handler.GetCommand()
			if cmd != tt.expected {
				t.Errorf("GetCommand() = %q, want %q", cmd, tt.expected)
			}
		})
	}
}

func TestKnownHostsHandlerEscapeForSed(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "escape dots",
			input:    "example.com",
			expected: `example\.com`,
		},
		{
			name:     "escape brackets",
			input:    "[example]",
			expected: `\[example\]`,
		},
		{
			name:     "escape asterisks",
			input:    "*.example.com",
			expected: `\*\.example\.com`,
		},
		{
			name:     "escape multiple special chars",
			input:    "192.168.1.1",
			expected: `192\.168\.1\.1`,
		},
		{
			name:     "no special chars",
			input:    "example-host",
			expected: "example-host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeForSed(tt.input)
			if result != tt.expected {
				t.Errorf("escapeForSed(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestKnownHostsHandlerUpdateStatus(t *testing.T) {
	tests := []struct {
		name            string
		rule            parser.Rule
		records         []ExecutionRecord
		initialStatus   Status
		expectedHosts   int
		shouldContain   bool
		expectedKeyType string
	}{
		{
			name: "add host to known_hosts status",
			rule: parser.Rule{
				Action:     "known_hosts",
				KnownHosts: "github.com",
			},
			records: []ExecutionRecord{
				{
					Status:  "success",
					Command: "ssh-keyscan -t ed25519 github.com",
					Output:  "Added github.com to known_hosts (key type: ed25519)",
				},
			},
			initialStatus:   Status{},
			expectedHosts:   1,
			shouldContain:   true,
			expectedKeyType: "",
		},
		{
			name: "remove host from known_hosts status on uninstall",
			rule: parser.Rule{
				Action:     "uninstall",
				KnownHosts: "github.com",
			},
			records: []ExecutionRecord{},
			initialStatus: Status{
				KnownHosts: []KnownHostsStatus{
					{
						Host:      "github.com",
						KeyType:   "ed25519",
						Blueprint: "/tmp/test.bp",
						OS:        "mac",
					},
				},
			},
			expectedHosts: 0,
			shouldContain: false,
		},
		{
			name: "no action if known_hosts command not found",
			rule: parser.Rule{
				Action:     "known_hosts",
				KnownHosts: "github.com",
			},
			records: []ExecutionRecord{
				{
					Status:  "error",
					Command: "ssh-keyscan -t ed25519 github.com",
				},
			},
			initialStatus:   Status{},
			expectedHosts:   0,
			shouldContain:   false,
			expectedKeyType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewKnownHostsHandler(tt.rule, "")
			status := tt.initialStatus

			err := handler.UpdateStatus(&status, tt.records, "/tmp/test.bp", "mac")
			if err != nil {
				t.Errorf("UpdateStatus() error = %v", err)
			}

			if len(status.KnownHosts) != tt.expectedHosts {
				t.Errorf("UpdateStatus() known hosts count = %d, want %d", len(status.KnownHosts), tt.expectedHosts)
			}

			if tt.shouldContain && len(status.KnownHosts) > 0 {
				if status.KnownHosts[0].Host != tt.rule.KnownHosts {
					t.Errorf("UpdateStatus() host = %q, want %q", status.KnownHosts[0].Host, tt.rule.KnownHosts)
				}
				if tt.expectedKeyType != "" && status.KnownHosts[0].KeyType != tt.expectedKeyType {
					t.Errorf("UpdateStatus() key type = %q, want %q", status.KnownHosts[0].KeyType, tt.expectedKeyType)
				}
			}
		})
	}
}

func TestKnownHostsHandlerDisplayInfo(t *testing.T) {
	tests := []struct {
		name             string
		rule             parser.Rule
		expectedContains []string
	}{
		{
			name: "known_hosts action with host only (auto-detect key type)",
			rule: parser.Rule{
				Action:     "known_hosts",
				KnownHosts: "github.com",
			},
			expectedContains: []string{"Host:", "github.com", "Key Type:", "auto-detect"},
		},
		{
			name: "known_hosts action with specific key type",
			rule: parser.Rule{
				Action:        "known_hosts",
				KnownHosts:    "gitlab.com",
				KnownHostsKey: "rsa",
			},
			expectedContains: []string{"Host:", "gitlab.com", "Key Type:", "rsa"},
		},
		{
			name: "known_hosts action with ed25519 key type",
			rule: parser.Rule{
				Action:        "known_hosts",
				KnownHosts:    "bitbucket.org",
				KnownHostsKey: "ed25519",
			},
			expectedContains: []string{"Host:", "bitbucket.org", "Key Type:", "ed25519"},
		},
		{
			name: "uninstall action",
			rule: parser.Rule{
				Action:     "uninstall",
				KnownHosts: "example.com",
			},
			expectedContains: []string{"Host:", "example.com", "Key Type:", "auto-detect"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewKnownHostsHandler(tt.rule, "")

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


func TestKnownHostsHandlerGetDependencyKey(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		expected string
	}{
		{
			name: "returns ID when present",
			rule: parser.Rule{
				ID:         "my-host",
				KnownHosts: "github.com",
			},
			expected: "my-host",
		},
		{
			name: "returns known_hosts when ID is empty",
			rule: parser.Rule{
				KnownHosts: "github.com",
			},
			expected: "github.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewKnownHostsHandler(tt.rule, "")
			got := handler.GetDependencyKey()
			if got != tt.expected {
				t.Errorf("GetDependencyKey() = %q, want %q", got, tt.expected)
			}
		})
	}
}
