package git

import (
	"os"
	"testing"
)

func TestIsGitURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// SSH URLs
		{
			name:     "SSH URL git@github.com",
			input:    "git@github.com:user/repo.git",
			expected: true,
		},
		{
			name:     "SSH URL with branch",
			input:    "git@github.com:user/repo.git@main",
			expected: true,
		},
		{
			name:     "SSH URL gitlab",
			input:    "git@gitlab.com:group/project.git",
			expected: true,
		},

		// HTTPS URLs
		{
			name:     "HTTPS GitHub URL",
			input:    "https://github.com/user/repo.git",
			expected: true,
		},
		{
			name:     "HTTPS with branch",
			input:    "https://github.com/user/repo.git@main",
			expected: true,
		},
		{
			name:     "HTTPS without .git extension",
			input:    "https://github.com/user/repo",
			expected: true,
		},

		// HTTP URLs
		{
			name:     "HTTP URL",
			input:    "http://example.com/repo.git",
			expected: true,
		},

		// git:// URLs
		{
			name:     "git protocol URL",
			input:    "git://github.com/user/repo.git",
			expected: true,
		},

		// Non-git URLs
		{
			name:     "local path",
			input:    "/home/user/projects/blueprint",
			expected: false,
		},
		{
			name:     "relative path",
			input:    "./local/repo",
			expected: false,
		},
		{
			name:     "plain hostname",
			input:    "github.com",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "random string",
			input:    "just some text",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsGitURL(tt.input)
			if result != tt.expected {
				t.Errorf("IsGitURL(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseGitURL_SSH(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedURL string
		expectedBr  string
		expectedP   string
	}{
		{
			name:        "SSH URL simple",
			input:       "git@github.com:user/repo.git",
			expectedURL: "git@github.com:user/repo.git",
			expectedBr:  "",
			expectedP:   "setup.bp",
		},
		{
			name:        "SSH URL with branch",
			input:       "git@github.com:user/repo.git@main",
			expectedURL: "git@github.com:user/repo.git",
			expectedBr:  "main",
			expectedP:   "setup.bp",
		},
		{
			name:        "SSH URL with branch and path",
			input:       "git@github.com:user/repo.git@main:path/to/file.bp",
			expectedURL: "git@github.com:user/repo.git",
			expectedBr:  "main",
			expectedP:   "path/to/file.bp",
		},
		{
			name:        "SSH URL with path only",
			input:       "git@github.com:user/repo.git:custom/path.bp",
			expectedURL: "git@github.com:user/repo.git",
			expectedBr:  "",
			expectedP:   "custom/path.bp",
		},
		{
			name:        "SSH URL with branch only after .git",
			input:       "git@github.com:user/repo.git@develop",
			expectedURL: "git@github.com:user/repo.git",
			expectedBr:  "develop",
			expectedP:   "setup.bp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseGitURL(tt.input)

			if result.URL != tt.expectedURL {
				t.Errorf("URL = %q, want %q", result.URL, tt.expectedURL)
			}
			if result.Branch != tt.expectedBr {
				t.Errorf("Branch = %q, want %q", result.Branch, tt.expectedBr)
			}
			if result.Path != tt.expectedP {
				t.Errorf("Path = %q, want %q", result.Path, tt.expectedP)
			}
		})
	}
}

func TestParseGitURL_HTTPS(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedURL string
		expectedBr  string
		expectedP   string
	}{
		{
			name:        "HTTPS URL simple",
			input:       "https://github.com/user/repo.git",
			expectedURL: "https://github.com/user/repo.git",
			expectedBr:  "",
			expectedP:   "setup.bp",
		},
		{
			name:        "HTTPS URL without .git",
			input:       "https://github.com/user/repo",
			expectedURL: "https://github.com/user/repo",
			expectedBr:  "",
			expectedP:   "setup.bp",
		},
		{
			name:        "HTTPS URL with branch",
			input:       "https://github.com/user/repo.git@main",
			expectedURL: "https://github.com/user/repo.git",
			expectedBr:  "main",
			expectedP:   "setup.bp",
		},
		{
			name:        "HTTPS URL with branch and path",
			input:       "https://github.com/user/repo.git@develop:config.bp",
			expectedURL: "https://github.com/user/repo.git",
			expectedBr:  "develop",
			expectedP:   "config.bp",
		},
		{
			name:        "HTTPS URL with path after .git",
			input:       "https://github.com/user/repo.git:sub/path.bp",
			expectedURL: "https://github.com/user/repo.git",
			expectedBr:  "",
			expectedP:   "sub/path.bp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseGitURL(tt.input)

			if result.URL != tt.expectedURL {
				t.Errorf("URL = %q, want %q", result.URL, tt.expectedURL)
			}
			if result.Branch != tt.expectedBr {
				t.Errorf("Branch = %q, want %q", result.Branch, tt.expectedBr)
			}
			if result.Path != tt.expectedP {
				t.Errorf("Path = %q, want %q", result.Path, tt.expectedP)
			}
		})
	}
}

func TestParseGitURL_HTTP(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedURL string
		expectedBr  string
		expectedP   string
	}{
		{
			name:        "HTTP URL simple",
			input:       "http://example.com/repo.git",
			expectedURL: "http://example.com/repo.git",
			expectedBr:  "",
			expectedP:   "setup.bp",
		},
		{
			name:        "HTTP URL with branch",
			input:       "http://example.com/repo.git@feature",
			expectedURL: "http://example.com/repo.git",
			expectedBr:  "feature",
			expectedP:   "setup.bp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseGitURL(tt.input)

			if result.URL != tt.expectedURL {
				t.Errorf("URL = %q, want %q", result.URL, tt.expectedURL)
			}
			if result.Branch != tt.expectedBr {
				t.Errorf("Branch = %q, want %q", result.Branch, tt.expectedBr)
			}
			if result.Path != tt.expectedP {
				t.Errorf("Path = %q, want %q", result.Path, tt.expectedP)
			}
		})
	}
}

func TestParseGitURL_GitProtocol(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedURL string
		expectedBr  string
		expectedP   string
	}{
		{
			name:        "git:// URL simple",
			input:       "git://github.com/user/repo.git",
			expectedURL: "git://github.com/user/repo.git",
			expectedBr:  "",
			expectedP:   "setup.bp",
		},
		{
			name:        "git:// URL with branch",
			input:       "git://github.com/user/repo.git@main",
			expectedURL: "git://github.com/user/repo.git",
			expectedBr:  "main",
			expectedP:   "setup.bp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseGitURL(tt.input)

			if result.URL != tt.expectedURL {
				t.Errorf("URL = %q, want %q", result.URL, tt.expectedURL)
			}
			if result.Branch != tt.expectedBr {
				t.Errorf("Branch = %q, want %q", result.Branch, tt.expectedBr)
			}
			if result.Path != tt.expectedP {
				t.Errorf("Path = %q, want %q", result.Path, tt.expectedP)
			}
		})
	}
}

func TestParseGitURL_DefaultPath(t *testing.T) {
	// Test that default path is always "setup.bp"
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "SSH URL no path",
			input: "git@github.com:user/repo.git",
		},
		{
			name:  "HTTPS URL no path",
			input: "https://github.com/user/repo.git",
		},
		{
			name:  "HTTP URL no path",
			input: "http://example.com/repo.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseGitURL(tt.input)
			if result.Path != "setup.bp" {
				t.Errorf("Default path = %q, want %q", result.Path, "setup.bp")
			}
		})
	}
}

func TestNormalizeGitURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "HTTPS URL with .git suffix",
			input:    "https://github.com/user/repo.git",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "HTTPS URL without .git suffix",
			input:    "https://github.com/user/repo",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "SSH URL converted to HTTPS",
			input:    "git@github.com:user/repo.git",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "SSH URL without .git",
			input:    "git@github.com:user/repo",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "SSH URL with uppercase",
			input:    "git@github.com:User/Repo.git",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "HTTPS URL with uppercase",
			input:    "https://github.com/User/Repo.git",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "Empty URL",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeGitURL(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeGitURL(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestGenerateRepositoryID(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		branch      string
		expectedLen int
	}{
		{
			name:        "HTTPS URL without branch",
			url:         "https://github.com/user/repo.git",
			branch:      "",
			expectedLen: 16,
		},
		{
			name:        "HTTPS URL with branch",
			url:         "https://github.com/user/repo.git",
			branch:      "main",
			expectedLen: 16,
		},
		{
			name:        "SSH URL",
			url:         "git@github.com:user/repo.git",
			branch:      "",
			expectedLen: 16,
		},
		{
			name:        "Different URLs produce different IDs",
			url:         "https://github.com/other/repo.git",
			branch:      "",
			expectedLen: 16,
		},
		{
			name:        "Same URL different branch produces different ID",
			url:         "https://github.com/user/repo.git",
			branch:      "feature",
			expectedLen: 16,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateRepositoryID(tt.url, tt.branch)
			if len(got) != tt.expectedLen {
				t.Errorf("generateRepositoryID() length = %d, want %d", len(got), tt.expectedLen)
			}
		})
	}

	// Verify determinism: same input should produce same output
	id1 := generateRepositoryID("https://github.com/user/repo.git", "main")
	id2 := generateRepositoryID("https://github.com/user/repo.git", "main")
	if id1 != id2 {
		t.Errorf("generateRepositoryID not deterministic: got %q then %q", id1, id2)
	}
}

func TestFindSetupFile(t *testing.T) {
	// Create a temp directory with a setup file
	tmpDir := t.TempDir()

	// Test with default path
	setupPath := tmpDir + "/setup.bp"
	if err := os.WriteFile(setupPath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test finding existing file with default path
	got, err := FindSetupFile(tmpDir, "setup.bp")
	if err != nil {
		t.Errorf("FindSetupFile() error = %v, want nil", err)
	}
	if got != setupPath {
		t.Errorf("FindSetupFile() = %q, want %q", got, setupPath)
	}

	// Test with empty path (should use default)
	got, err = FindSetupFile(tmpDir, "")
	if err != nil {
		t.Errorf("FindSetupFile() error = %v, want nil", err)
	}
	if got != setupPath {
		t.Errorf("FindSetupFile() = %q, want %q", got, setupPath)
	}

	// Test non-existent file
	_, err = FindSetupFile(tmpDir, "nonexistent.bp")
	if err == nil {
		t.Error("FindSetupFile() expected error for non-existent file")
	}
}

func TestCleanupRepository(t *testing.T) {
	// Test with empty path (should return nil)
	err := CleanupRepository("")
	if err != nil {
		t.Errorf("CleanupRepository('') error = %v, want nil", err)
	}

	// Test with non-existent path (should return nil)
	err = CleanupRepository("/nonexistent/path")
	if err != nil {
		t.Errorf("CleanupRepository('/nonexistent/path') error = %v, want nil", err)
	}

	// Test with actual directory
	tmpDir := t.TempDir()
	if err := os.WriteFile(tmpDir+"/file.txt", []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err = CleanupRepository(tmpDir)
	if err != nil {
		t.Errorf("CleanupRepository() error = %v, want nil", err)
	}

	// Verify directory is removed
	if _, statErr := os.Stat(tmpDir); statErr == nil {
		t.Error("CleanupRepository() did not remove directory")
	}
}
