package handlers

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

func TestCloneHandlerGetCommand(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		expected string
	}{
		{
			name: "clone repository without branch",
			rule: parser.Rule{
				Action:    "clone",
				CloneURL:  "https://github.com/user/repo.git",
				ClonePath: "~/projects/repo",
			},
			expected: "git clone https://github.com/user/repo.git ~/projects/repo",
		},
		{
			name: "clone with specific branch",
			rule: parser.Rule{
				Action:    "clone",
				CloneURL:  "https://github.com/user/repo.git",
				ClonePath: "~/projects/repo",
				Branch:    "develop",
			},
			expected: "git clone -b develop https://github.com/user/repo.git ~/projects/repo",
		},
		{
			name: "uninstall - remove clone",
			rule: parser.Rule{
				Action:    "uninstall",
				ClonePath: "~/projects/repo",
			},
			expected: "rm -rf ~/projects/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewCloneHandler(tt.rule, "")
			cmd := handler.GetCommand()
			if cmd != tt.expected {
				t.Errorf("GetCommand() = %q, want %q", cmd, tt.expected)
			}
		})
	}
}

func TestCloneHandlerDown(t *testing.T) {
	tmpDir := t.TempDir()
	testRepo := filepath.Join(tmpDir, "test-repo")

	// Create a test directory to represent cloned repo
	_ = os.MkdirAll(testRepo, 0755)

	tests := []struct {
		name      string
		rule      parser.Rule
		shouldErr bool
		checkFunc func(string) bool
	}{
		{
			name: "remove existing cloned repository",
			rule: parser.Rule{
				Action:    "clone",
				ClonePath: testRepo,
			},
			shouldErr: false,
			checkFunc: func(path string) bool {
				_, err := os.Stat(path)
				return os.IsNotExist(err)
			},
		},
		{
			name: "remove non-existent repository",
			rule: parser.Rule{
				Action:    "clone",
				ClonePath: "/tmp/non-existent-repo-xyz",
			},
			shouldErr: false,
			checkFunc: func(path string) bool {
				return true // Should succeed with message
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Recreate repo for test
			if tt.name == "remove existing cloned repository" {
				_ = os.MkdirAll(testRepo, 0755)
			}

			handler := NewCloneHandler(tt.rule, "")
			output, err := handler.Down()

			if (err != nil) != tt.shouldErr {
				t.Errorf("Down() error = %v, wantErr %v", err, tt.shouldErr)
			}

			if output == "" {
				t.Errorf("Down() returned empty output")
			}

			if tt.checkFunc != nil && !tt.checkFunc(tt.rule.ClonePath) {
				t.Errorf("Down() verification failed")
			}
		})
	}
}

func TestCloneHandlerUpdateStatus(t *testing.T) {
	tests := []struct {
		name           string
		rule           parser.Rule
		records        []ExecutionRecord
		initialStatus  Status
		expectedClones int
		shouldContain  bool
	}{
		{
			name: "add cloned repository to status",
			rule: parser.Rule{
				Action:    "clone",
				CloneURL:  "https://github.com/user/repo.git",
				ClonePath: "~/projects/repo",
			},
			records: []ExecutionRecord{
				{
					Status:  "success",
					Command: "git clone https://github.com/user/repo.git ~/projects/repo",
					Output:  "Cloned (SHA: abc123def456)",
				},
			},
			initialStatus:  Status{},
			expectedClones: 1,
			shouldContain:  true,
		},
		{
			name: "remove cloned repository from status on uninstall",
			rule: parser.Rule{
				Action:    "uninstall",
				CloneURL:  "https://github.com/user/repo.git",
				ClonePath: "~/projects/repo",
			},
			records: []ExecutionRecord{},
			initialStatus: Status{
				Clones: []CloneStatus{
					{
						URL:       "https://github.com/user/repo.git",
						Path:      "~/projects/repo",
						Blueprint: "/tmp/test.bp",
						OS:        "mac",
					},
				},
			},
			expectedClones: 0,
			shouldContain:  false,
		},
		{
			name: "no action if clone command not found",
			rule: parser.Rule{
				Action:    "clone",
				CloneURL:  "https://github.com/user/repo.git",
				ClonePath: "~/projects/repo",
			},
			records: []ExecutionRecord{
				{
					Status:  "error",
					Command: "git clone https://github.com/user/repo.git ~/projects/repo",
				},
			},
			initialStatus:  Status{},
			expectedClones: 0,
			shouldContain:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewCloneHandler(tt.rule, "")
			status := tt.initialStatus

			err := handler.UpdateStatus(&status, tt.records, "/tmp/test.bp", "mac")
			if err != nil {
				t.Errorf("UpdateStatus() error = %v", err)
			}

			if len(status.Clones) != tt.expectedClones {
				t.Errorf("UpdateStatus() clones count = %d, want %d", len(status.Clones), tt.expectedClones)
			}

			if tt.shouldContain && len(status.Clones) > 0 {
				if status.Clones[0].URL != tt.rule.CloneURL {
					t.Errorf("UpdateStatus() clone URL = %q, want %q", status.Clones[0].URL, tt.rule.CloneURL)
				}
			}
		})
	}
}

func TestCloneHandlerDisplayInfo(t *testing.T) {
	tests := []struct {
		name             string
		rule             parser.Rule
		expectedContains []string
	}{
		{
			name: "clone action with URL and path",
			rule: parser.Rule{
				Action:    "clone",
				CloneURL:  "https://github.com/user/repo.git",
				ClonePath: "~/projects/repo",
			},
			expectedContains: []string{"URL:", "https://github.com/user/repo.git", "Path:", "~/projects/repo"},
		},
		{
			name: "clone action with branch",
			rule: parser.Rule{
				Action:    "clone",
				CloneURL:  "https://github.com/user/dotfiles.git",
				ClonePath: "~/.dotfiles",
				Branch:    "main",
			},
			expectedContains: []string{"URL:", "https://github.com/user/dotfiles.git", "Path:", "~/.dotfiles", "Branch:", "main"},
		},
		{
			name: "uninstall action",
			rule: parser.Rule{
				Action:    "uninstall",
				CloneURL:  "https://github.com/user/repo.git",
				ClonePath: "~/projects/repo",
			},
			expectedContains: []string{"URL:", "https://github.com/user/repo.git", "Path:", "~/projects/repo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewCloneHandler(tt.rule, "")

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

func TestCloneHandlerDisplayStatus(t *testing.T) {
	tests := []struct {
		name         string
		clones       []CloneStatus
		expectOutput []string
		notExpect    []string
	}{
		{
			name:         "empty clones list",
			clones:       []CloneStatus{},
			expectOutput: []string{},
			notExpect:    []string{"Cloned Repositories"},
		},
		{
			name: "single clone with https URL",
			clones: []CloneStatus{
				{
					URL:       "https://github.com/golang/go.git",
					Path:      "/tmp/test-go-repo",
					SHA:       "abc123def456",
					ClonedAt:  "2025-12-26T17:00:00Z",
					Blueprint: "/tmp/test.bp",
					OS:        "mac",
				},
			},
			expectOutput: []string{
				"Cloned Repositories",
				"/tmp/test-go-repo",
				"https://github.com/golang/go.git",
				"URL:",
			},
			notExpect: []string{"https:/github.com"}, // Ensure not truncated to single slash
		},
		{
			name: "multiple clones with different https URLs",
			clones: []CloneStatus{
				{
					URL:       "https://github.com/golang/go.git",
					Path:      "/tmp/go-repo",
					ClonedAt:  "2025-12-26T17:00:00Z",
					Blueprint: "/tmp/test1.bp",
					OS:        "mac",
				},
				{
					URL:       "https://github.com/charmbracelet/bubbletea.git",
					Path:      "/tmp/bubbletea-repo",
					ClonedAt:  "2025-12-26T17:01:00Z",
					Blueprint: "/tmp/test2.bp",
					OS:        "linux",
				},
			},
			expectOutput: []string{
				"Cloned Repositories",
				"/tmp/go-repo",
				"/tmp/bubbletea-repo",
				"https://github.com/golang/go.git",
				"https://github.com/charmbracelet/bubbletea.git",
				"URL:",
			},
			notExpect: []string{},
		},
		{
			name: "clone with long https URL path",
			clones: []CloneStatus{
				{
					URL:       "https://github.com/user/very-long-repository-name-with-many-characters.git",
					Path:      "/tmp/repo",
					ClonedAt:  "2025-12-26T17:00:00Z",
					Blueprint: "/tmp/test.bp",
					OS:        "mac",
				},
			},
			expectOutput: []string{
				"https://github.com/user/very-long-repository-name-with-many-characters.git",
				"URL:",
			},
			notExpect: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &CloneHandler{}

			// Capture stdout
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			handler.DisplayStatus(tt.clones)

			_ = w.Close()
			os.Stdout = old

			// Read captured output
			var buf bytes.Buffer
			_, _ = io.Copy(&buf, r)
			output := buf.String()

			// Verify expected content is present
			for _, expected := range tt.expectOutput {
				if !strings.Contains(output, expected) {
					t.Errorf("DisplayStatus() output missing expected content %q\nGot:\n%s", expected, output)
				}
			}

			// Verify undesired content is not present
			for _, notExpected := range tt.notExpect {
				if strings.Contains(output, notExpected) {
					t.Errorf("DisplayStatus() output contains unexpected content %q\nGot:\n%s", notExpected, output)
				}
			}
		})
	}
}

func TestCloneHandlerDisplayStatusURLPreservation(t *testing.T) {
	// Test specifically for https:// preservation (not truncated to https:/)
	httpURLs := []string{
		"https://github.com/golang/go.git",
		"https://github.com/user/repo.git",
		"https://gitlab.com/group/project.git",
		"https://bitbucket.org/user/repo.git",
		"https://git.example.com/path/to/repo.git",
	}

	for _, url := range httpURLs {
		t.Run(fmt.Sprintf("URL_preservation_%s", url), func(t *testing.T) {
			clones := []CloneStatus{
				{
					URL:       url,
					Path:      "/tmp/test-repo",
					ClonedAt:  "2025-12-26T17:00:00Z",
					Blueprint: "/tmp/test.bp",
					OS:        "mac",
				},
			}

			handler := &CloneHandler{}

			// Capture stdout
			old := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			handler.DisplayStatus(clones)

			_ = w.Close()
			os.Stdout = old

			// Read captured output
			var buf bytes.Buffer
			_, _ = io.Copy(&buf, r)
			output := buf.String()

			// Check that full URL with https:// is present
			if !strings.Contains(output, url) {
				t.Errorf("DisplayStatus() output missing full URL %q\nGot:\n%s", url, output)
			}

			// Check that the URL is NOT truncated to single slash
			truncatedURL := strings.Replace(url, "https://", "https:/", 1)
			if strings.Contains(output, truncatedURL) && !strings.Contains(output, url) {
				t.Errorf("DisplayStatus() output shows truncated URL %q (missing slash)\nGot:\n%s", truncatedURL, output)
			}
		})
	}
}


func TestCloneHandlerGetDependencyKey(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		expected string
	}{
		{
			name: "returns ID when present",
			rule: parser.Rule{
				ID:        "my-clone",
				ClonePath: "~/projects/repo",
			},
			expected: "my-clone",
		},
		{
			name: "returns clone path when ID is empty",
			rule: parser.Rule{
				ClonePath: "~/projects/repo",
			},
			expected: "~/projects/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewCloneHandler(tt.rule, "")
			got := handler.GetDependencyKey()
			if got != tt.expected {
				t.Errorf("GetDependencyKey() = %q, want %q", got, tt.expected)
			}
		})
	}
}
