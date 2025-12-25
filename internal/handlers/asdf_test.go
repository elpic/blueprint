package handlers

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

func TestAsdfHandlerGetCommand(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		expected string
	}{
		{
			name: "install asdf with single package",
			rule: parser.Rule{
				Action:       "asdf",
				AsdfPackages: []string{"nodejs@18.0.0"},
			},
			expected: "asdf install nodejs 18.0.0",
		},
		{
			name: "install asdf with multiple packages",
			rule: parser.Rule{
				Action: "asdf",
				AsdfPackages: []string{
					"nodejs@18.0.0",
					"ruby@3.1.0",
				},
			},
			expected: "asdf install nodejs 18.0.0 && asdf install ruby 3.1.0",
		},
		{
			name: "asdf without packages",
			rule: parser.Rule{
				Action:       "asdf",
				AsdfPackages: []string{},
			},
			expected: "asdf-init",
		},
		{
			name: "uninstall asdf",
			rule: parser.Rule{
				Action: "uninstall",
			},
			expected: "asdf uninstall",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewAsdfHandler(tt.rule, "")
			cmd := handler.GetCommand()
			if cmd != tt.expected {
				t.Errorf("GetCommand() = %q, want %q", cmd, tt.expected)
			}
		})
	}
}

func TestAsdfHandlerUpdateStatus(t *testing.T) {
	tests := []struct {
		name           string
		rule           parser.Rule
		records        []ExecutionRecord
		initialStatus  Status
		expectedClones int
		shouldContain  bool
		expectedPath   string
	}{
		{
			name: "add asdf to status on successful install",
			rule: parser.Rule{
				Action:       "asdf",
				AsdfPackages: []string{"nodejs@18.0.0"},
			},
			records: []ExecutionRecord{
				{
					Status:  "success",
					Command: "asdf install nodejs 18.0.0",
					Output:  "Installed asdf (SHA: abc123def456)",
				},
			},
			initialStatus:  Status{},
			expectedClones: 1,
			shouldContain:  true,
			expectedPath:   "~/.asdf",
		},
		{
			name: "remove asdf from status on uninstall",
			rule: parser.Rule{
				Action:       "uninstall",
				AsdfPackages: []string{"nodejs@18.0.0"},
			},
			records: []ExecutionRecord{
				{
					Status:  "success",
					Command: "asdf uninstall",
				},
			},
			initialStatus: Status{
				Clones: []CloneStatus{
					{
						URL:       "https://github.com/asdf-vm/asdf.git",
						Path:      "~/.asdf",
						Blueprint: "/tmp/test.bp",
						OS:        "mac",
					},
				},
			},
			expectedClones: 0,
			shouldContain:  false,
		},
		{
			name: "no action if asdf install failed",
			rule: parser.Rule{
				Action:       "asdf",
				AsdfPackages: []string{"nodejs@18.0.0"},
			},
			records: []ExecutionRecord{
				{
					Status:  "error",
					Command: "asdf install nodejs 18.0.0",
					Error:   "Installation failed",
				},
			},
			initialStatus:  Status{},
			expectedClones: 0,
			shouldContain:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewAsdfHandler(tt.rule, "")
			status := tt.initialStatus

			err := handler.UpdateStatus(&status, tt.records, "/tmp/test.bp", "mac")
			if err != nil {
				t.Errorf("UpdateStatus() error = %v", err)
			}

			if len(status.Clones) != tt.expectedClones {
				t.Errorf("UpdateStatus() clones count = %d, want %d", len(status.Clones), tt.expectedClones)
			}

			if tt.shouldContain && len(status.Clones) > 0 {
				if status.Clones[0].Path != tt.expectedPath {
					t.Errorf("UpdateStatus() path = %q, want %q", status.Clones[0].Path, tt.expectedPath)
				}
				if status.Clones[0].URL != "https://github.com/asdf-vm/asdf.git" {
					t.Errorf("UpdateStatus() URL = %q, want %q", status.Clones[0].URL, "https://github.com/asdf-vm/asdf.git")
				}
			}
		})
	}
}

func TestAsdfPackageFormatting(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected struct {
			plugin  string
			version string
		}
	}{
		{
			name:  "valid format",
			input: "nodejs@18.0.0",
			expected: struct {
				plugin  string
				version string
			}{"nodejs", "18.0.0"},
		},
		{
			name:  "ruby version",
			input: "ruby@3.1.0",
			expected: struct {
				plugin  string
				version string
			}{"ruby", "3.1.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that packages are correctly formatted
			rule := parser.Rule{
				Action:       "asdf",
				AsdfPackages: []string{tt.input},
			}
			handler := NewAsdfHandler(rule, "")

			// Just verify that GetCommand can be called without error
			cmd := handler.GetCommand()
			if cmd == "" {
				t.Errorf("GetCommand() returned empty string for package %q", tt.input)
			}
		})
	}
}

func TestAsdfHandlerDisplayInfo(t *testing.T) {
	tests := []struct {
		name             string
		rule             parser.Rule
		expectedContains []string
	}{
		{
			name: "asdf action with single plugin",
			rule: parser.Rule{
				Action:       "asdf",
				AsdfPackages: []string{"nodejs@18.0.0"},
			},
			expectedContains: []string{"Plugins:", "nodejs@18.0.0"},
		},
		{
			name: "asdf action with multiple plugins",
			rule: parser.Rule{
				Action: "asdf",
				AsdfPackages: []string{
					"nodejs@18.0.0",
					"ruby@3.1.0",
					"python@3.10.0",
				},
			},
			expectedContains: []string{"Plugins:", "nodejs@18.0.0", "ruby@3.1.0", "python@3.10.0"},
		},
		{
			name: "asdf action without packages (init only)",
			rule: parser.Rule{
				Action:       "asdf",
				AsdfPackages: []string{},
			},
			expectedContains: []string{"Description:", "Installs asdf version manager"},
		},
		{
			name: "uninstall action with plugins",
			rule: parser.Rule{
				Action: "uninstall",
				AsdfPackages: []string{
					"nodejs@18.0.0",
					"ruby@3.1.0",
				},
			},
			expectedContains: []string{"Plugins:", "nodejs@18.0.0", "ruby@3.1.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewAsdfHandler(tt.rule, "")

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
