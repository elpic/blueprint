package git

import (
	"os"
	"os/exec"
	"path/filepath"
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
		{
			name:     "single-slash HTTPS with .git (legacy malformed)",
			input:    "https:/github.com/user/repo.git",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "single-slash HTTPS without .git (legacy malformed)",
			input:    "https:/github.com/user/repo",
			expected: "https://github.com/user/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeGitURL(tt.input)
			if got != tt.expected {
				t.Errorf("NormalizeGitURL(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// TestNormalizeGitURLEquality verifies that SSH and HTTPS URLs for the same
// repository return identical normalized values.
func TestNormalizeGitURLEquality(t *testing.T) {
	testCases := []struct {
		name          string
		httpsURL      string
		sshURL        string
		expectedValue string
	}{
		{
			name:          "GitHub",
			httpsURL:      "https://github.com/user/repo.git",
			sshURL:        "git@github.com:user/repo.git",
			expectedValue: "https://github.com/user/repo",
		},
		{
			name:          "GitLab",
			httpsURL:      "https://gitlab.com/user/repo.git",
			sshURL:        "git@gitlab.com:user/repo.git",
			expectedValue: "https://gitlab.com/user/repo",
		},
		{
			name:          "Bitbucket",
			httpsURL:      "https://bitbucket.org/user/repo.git",
			sshURL:        "git@bitbucket.org:user/repo.git",
			expectedValue: "https://bitbucket.org/user/repo",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			httpsNormalized := NormalizeGitURL(tc.httpsURL)
			sshNormalized := NormalizeGitURL(tc.sshURL)

			if httpsNormalized != tc.expectedValue {
				t.Errorf("HTTPS URL %s normalized to %q, want %q", tc.httpsURL, httpsNormalized, tc.expectedValue)
			}

			if sshNormalized != tc.expectedValue {
				t.Errorf("SSH URL %s normalized to %q, want %q", tc.sshURL, sshNormalized, tc.expectedValue)
			}

			if httpsNormalized != sshNormalized {
				t.Errorf("SSH and HTTPS URLs should normalize to the same value:\n  SSH:   %s → %s\n  HTTPS: %s → %s",
					tc.sshURL, sshNormalized, tc.httpsURL, httpsNormalized)
			}
		})
	}
}

func TestStripBranch(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://github.com/user/repo", "https://github.com/user/repo"},
		{"https://github.com/user/repo@main", "https://github.com/user/repo"},
		{"https://github.com/user/repo@feat/test", "https://github.com/user/repo"},
		{"https://github.com/user/repo@feat/test:setup.bp", "https://github.com/user/repo"},
		{"git@github.com:user/repo.git", "git@github.com:user/repo.git"},
		{"git@github.com:user/repo.git@main", "git@github.com:user/repo.git"},
		{"git@github.com:user/repo.git@feat/test:setup.bp", "git@github.com:user/repo.git"},
		{"/local/path/setup.bp", "/local/path/setup.bp"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := StripBranch(tt.input)
			if got != tt.want {
				t.Errorf("StripBranch(%q) = %q, want %q", tt.input, got, tt.want)
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

func TestExpandShorthand(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// GitHub
		{"@github:user/repo", "https://github.com/user/repo"},
		{"@github:user/repo@main", "https://github.com/user/repo@main"},
		{"@github:user/repo@main:infra/setup.bp", "https://github.com/user/repo@main:infra/setup.bp"},

		// GitLab
		{"@gitlab:user/repo", "https://gitlab.com/user/repo"},
		{"@gitlab:group/subgroup/repo", "https://gitlab.com/group/subgroup/repo"},
		{"@gitlab:user/repo@develop", "https://gitlab.com/user/repo@develop"},

		// Bitbucket
		{"@bitbucket:user/repo", "https://bitbucket.org/user/repo"},
		{"@bitbucket:user/repo@main:setup.bp", "https://bitbucket.org/user/repo@main:setup.bp"},

		// Codeberg
		{"@codeberg:user/repo", "https://codeberg.org/user/repo"},

		// Non-shorthand — pass through unchanged
		{"https://github.com/user/repo", "https://github.com/user/repo"},
		{"git@github.com:user/repo.git", "git@github.com:user/repo.git"},
		{"./local.bp", "./local.bp"},
		{"@unknown:user/repo", "@unknown:user/repo"},
		{"@noslash", "@noslash"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ExpandShorthand(tt.input)
			if got != tt.want {
				t.Errorf("ExpandShorthand(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsGitURL_Shorthand(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"@github:user/repo", true},
		{"@gitlab:user/repo", true},
		{"@bitbucket:user/repo", true},
		{"@codeberg:user/repo", true},
		{"@unknown:user/repo", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsGitURL(tt.input)
			if got != tt.want {
				t.Errorf("IsGitURL(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseGitURL_Shorthand(t *testing.T) {
	tests := []struct {
		input      string
		wantURL    string
		wantBranch string
		wantPath   string
	}{
		{
			input:    "@github:user/repo",
			wantURL:  "https://github.com/user/repo",
			wantPath: "setup.bp",
		},
		{
			input:      "@gitlab:user/repo@main",
			wantURL:    "https://gitlab.com/user/repo",
			wantBranch: "main",
			wantPath:   "setup.bp",
		},
		{
			input:      "@github:user/repo@main:infra/setup.bp",
			wantURL:    "https://github.com/user/repo",
			wantBranch: "main",
			wantPath:   "infra/setup.bp",
		},
		{
			input:    "@bitbucket:user/repo",
			wantURL:  "https://bitbucket.org/user/repo",
			wantPath: "setup.bp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseGitURL(tt.input)
			if got.URL != tt.wantURL {
				t.Errorf("URL = %q, want %q", got.URL, tt.wantURL)
			}
			if got.Branch != tt.wantBranch {
				t.Errorf("Branch = %q, want %q", got.Branch, tt.wantBranch)
			}
			if got.Path != tt.wantPath {
				t.Errorf("Path = %q, want %q", got.Path, tt.wantPath)
			}
		})
	}
}

func TestExpandShorthandSSH(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// GitHub
		{"@github:user/repo", "git@github.com:user/repo"},
		{"@github:user/repo@main", "git@github.com:user/repo@main"},
		{"@github:user/repo@main:infra/setup.bp", "git@github.com:user/repo@main:infra/setup.bp"},

		// GitLab
		{"@gitlab:user/repo", "git@gitlab.com:user/repo"},
		{"@gitlab:group/subgroup/repo", "git@gitlab.com:group/subgroup/repo"},

		// Bitbucket
		{"@bitbucket:user/repo", "git@bitbucket.org:user/repo"},

		// Codeberg
		{"@codeberg:user/repo", "git@codeberg.org:user/repo"},

		// Non-shorthand — pass through unchanged
		{"https://github.com/user/repo", "https://github.com/user/repo"},
		{"git@github.com:user/repo.git", "git@github.com:user/repo.git"},
		{"./local.bp", "./local.bp"},
		{"@unknown:user/repo", "@unknown:user/repo"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ExpandShorthandSSH(tt.input)
			if got != tt.want {
				t.Errorf("ExpandShorthandSSH(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestHttpsAuth(t *testing.T) {
	tests := []struct {
		name         string
		envToken     string
		envUser      string
		wantNil      bool
		wantUsername string
		wantPassword string
	}{
		{
			name:    "no env vars returns nil",
			wantNil: true,
		},
		{
			name:         "token only uses x-access-token as username",
			envToken:     "ghp_token123",
			wantUsername: "x-access-token",
			wantPassword: "ghp_token123",
		},
		{
			name:         "token and user uses provided username",
			envToken:     "ghp_token123",
			envUser:      "myuser",
			wantUsername: "myuser",
			wantPassword: "ghp_token123",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GITHUB_TOKEN", tt.envToken)
			t.Setenv("GITHUB_USER", tt.envUser)
			auth := httpsAuth()
			if tt.wantNil {
				if auth != nil {
					t.Errorf("expected nil, got %+v", auth)
				}
				return
			}
			if auth == nil {
				t.Fatal("expected non-nil auth")
			}
			if auth.Username != tt.wantUsername {
				t.Errorf("Username = %q, want %q", auth.Username, tt.wantUsername)
			}
			if auth.Password != tt.wantPassword {
				t.Errorf("Password = %q, want %q", auth.Password, tt.wantPassword)
			}
		})
	}
}

func TestLocalSHAWithError(t *testing.T) {
	t.Run("nonexistent path returns error", func(t *testing.T) {
		sha, err := LocalSHAWithError("/nonexistent/path/that/does/not/exist")
		if err == nil {
			t.Error("expected error, got nil")
		}
		if sha != "" {
			t.Errorf("expected empty SHA, got %q", sha)
		}
	})

	t.Run("valid repo returns SHA", func(t *testing.T) {
		dir := t.TempDir()
		// Init a real git repo in the temp dir
		cmd := exec.Command("git", "init", dir)
		if err := cmd.Run(); err != nil {
			t.Fatalf("git init: %v", err)
		}
		// Need at least one commit for HEAD to exist
		f := filepath.Join(dir, "f.txt")
		if err := os.WriteFile(f, []byte("hi"), 0o600); err != nil {
			t.Fatalf("write file: %v", err)
		}
		for _, args := range [][]string{
			{"git", "-C", dir, "config", "user.email", "test@test.com"},
			{"git", "-C", dir, "config", "user.name", "Test"},
			{"git", "-C", dir, "add", "."},
			{"git", "-C", dir, "commit", "-m", "init"},
		} {
			if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
				t.Fatalf("cmd %v: %v", args, err)
			}
		}
		sha, err := LocalSHAWithError(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(sha) != 40 {
			t.Errorf("expected 40-char SHA, got %q", sha)
		}
	})
}

func TestRemoteHeadSHAWithError(t *testing.T) {
	t.Run("invalid URL returns error", func(t *testing.T) {
		sha, err := RemoteHeadSHAWithError("https://invalid.example.invalid/nonexistent/repo.git", "")
		if err == nil {
			t.Error("expected error for unreachable URL, got nil")
		}
		if sha != "" {
			t.Errorf("expected empty SHA, got %q", sha)
		}
	})
}

func TestGitTimeout(t *testing.T) {
	t.Run("default timeout is 120s", func(t *testing.T) {
		_ = os.Unsetenv("BLUEPRINT_GIT_TIMEOUT")
		if got := gitTimeout(); got != 120*1e9 {
			t.Errorf("expected 120s, got %v", got)
		}
	})

	t.Run("custom timeout from env", func(t *testing.T) {
		t.Setenv("BLUEPRINT_GIT_TIMEOUT", "30")
		if got := gitTimeout(); got != 30*1e9 {
			t.Errorf("expected 30s, got %v", got)
		}
	})

	t.Run("invalid env falls back to default", func(t *testing.T) {
		t.Setenv("BLUEPRINT_GIT_TIMEOUT", "notanumber")
		if got := gitTimeout(); got != 120*1e9 {
			t.Errorf("expected 120s, got %v", got)
		}
	})

	t.Run("zero env falls back to default", func(t *testing.T) {
		t.Setenv("BLUEPRINT_GIT_TIMEOUT", "0")
		if got := gitTimeout(); got != 120*1e9 {
			t.Errorf("expected 120s, got %v", got)
		}
	})
}
