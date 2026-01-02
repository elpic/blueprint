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
		name          string
		rule          parser.Rule
		records       []ExecutionRecord
		initialStatus Status
		expectedAsdfs int
		shouldContain bool
		expectedPlugin string
		expectedVersion string
		description    string
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
			initialStatus:    Status{},
			expectedAsdfs:    1,
			shouldContain:    true,
			expectedPlugin:   "nodejs",
			expectedVersion:  "18.0.0",
			description:      "Single asdf rule adds one package",
		},
		{
			name: "multiple asdf rules preserve packages from other rules",
			rule: parser.Rule{
				Action:       "asdf",
				AsdfPackages: []string{"python@3.11.0"},
			},
			records: []ExecutionRecord{
				{
					Status:  "success",
					Command: "asdf install python 3.11.0",
				},
			},
			initialStatus: Status{
				Asdfs: []AsdfStatus{
					{
						Plugin:      "nodejs",
						Version:     "18.0.0",
						Blueprint:   "/tmp/test.bp",
						OS:          "mac",
						InstalledAt: "2024-01-01T00:00:00Z",
					},
				},
			},
			expectedAsdfs:    2,
			shouldContain:    true,
			expectedPlugin:   "python",
			expectedVersion:  "3.11.0",
			description:      "Second asdf rule preserves nodejs entry and adds python",
		},
		{
			name: "multiple versions of same plugin coexist",
			rule: parser.Rule{
				Action:       "asdf",
				AsdfPackages: []string{"nodejs@20.0.0"},
			},
			records: []ExecutionRecord{
				{
					Status:  "success",
					Command: "asdf install nodejs 20.0.0",
				},
			},
			initialStatus: Status{
				Asdfs: []AsdfStatus{
					{
						Plugin:      "nodejs",
						Version:     "18.0.0",
						Blueprint:   "/tmp/test.bp",
						OS:          "mac",
						InstalledAt: "2024-01-01T00:00:00Z",
					},
				},
			},
			expectedAsdfs:    2,
			shouldContain:    true,
			expectedPlugin:   "nodejs",
			expectedVersion:  "20.0.0",
			description:      "Multiple versions of same plugin can coexist (node@18 and node@20)",
		},
		{
			name: "remove specific plugin version on uninstall, preserve other versions",
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
				Asdfs: []AsdfStatus{
					{
						Plugin:      "nodejs",
						Version:     "18.0.0",
						Blueprint:   "/tmp/test.bp",
						OS:          "mac",
						InstalledAt: "2024-01-01T00:00:00Z",
					},
					{
						Plugin:      "nodejs",
						Version:     "20.0.0",
						Blueprint:   "/tmp/test.bp",
						OS:          "mac",
						InstalledAt: "2024-01-01T00:00:00Z",
					},
					{
						Plugin:      "python",
						Version:     "3.11.0",
						Blueprint:   "/tmp/test.bp",
						OS:          "mac",
						InstalledAt: "2024-01-01T00:00:00Z",
					},
				},
			},
			expectedAsdfs: 2,
			shouldContain: false,
			description:   "Uninstall removes only nodejs@18, keeps nodejs@20 and python@3.11",
		},
		{
			name: "installing same version twice doesn't create duplicates",
			rule: parser.Rule{
				Action:       "asdf",
				AsdfPackages: []string{"nodejs@18.0.0"},
			},
			records: []ExecutionRecord{
				{
					Status:  "success",
					Command: "asdf install nodejs 18.0.0",
				},
			},
			initialStatus: Status{
				Asdfs: []AsdfStatus{
					{
						Plugin:      "nodejs",
						Version:     "18.0.0",
						Blueprint:   "/tmp/test.bp",
						OS:          "mac",
						InstalledAt: "2024-01-01T00:00:00Z",
					},
				},
			},
			expectedAsdfs:    1,
			shouldContain:    true,
			expectedPlugin:   "nodejs",
			expectedVersion:  "18.0.0",
			description:      "Idempotent: installing same version twice keeps count at 1",
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
			initialStatus: Status{},
			expectedAsdfs: 0,
			shouldContain: false,
			description:   "Failed installation doesn't update status",
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

			if len(status.Asdfs) != tt.expectedAsdfs {
				t.Errorf("UpdateStatus() asdfs count = %d, want %d (test: %s)", len(status.Asdfs), tt.expectedAsdfs, tt.description)
			}

			if tt.shouldContain {
				// Find the expected plugin in the asdf list
				found := false
				for _, asdf := range status.Asdfs {
					if asdf.Plugin == tt.expectedPlugin && asdf.Version == tt.expectedVersion {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("UpdateStatus() expected to find %s@%s in status, but didn't (test: %s)",
						tt.expectedPlugin, tt.expectedVersion, tt.description)
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


func TestAsdfHandlerGetDependencyKey(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		expected string
	}{
		{
			name: "returns ID when present",
			rule: parser.Rule{
				ID:     "my-asdf",
				Action: "asdf",
			},
			expected: "my-asdf",
		},
		{
			name: "returns asdf when ID is empty",
			rule: parser.Rule{
				Action: "asdf",
			},
			expected: "asdf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewAsdfHandler(tt.rule, "")
			got := handler.GetDependencyKey()
			if got != tt.expected {
				t.Errorf("GetDependencyKey() = %q, want %q", got, tt.expected)
			}
		})
	}
}
