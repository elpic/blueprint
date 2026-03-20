package handlers

import (
	"errors"
	"testing"
)

// mockHomeDirProvider is a test adapter for homeDirProvider.
type mockHomeDirProvider struct {
	homeDir string
	err     error
}

func (m *mockHomeDirProvider) UserHomeDir() (string, error) {
	return m.homeDir, m.err
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "regular path returns unchanged",
			input:    "/usr/local/bin",
			expected: "/usr/local/bin",
		},
		{
			name:     "relative path returns unchanged",
			input:    "./relative/path",
			expected: "./relative/path",
		},
		{
			name:     "tilde at start expands to home directory",
			input:    "~/some/path",
			expected: "/home/user/some/path",
		},
		{
			name:     "tilde with file returns home + file",
			input:    "~/.bashrc",
			expected: "/home/user/.bashrc",
		},
		{
			name:     "tilde only returns home directory",
			input:    "~",
			expected: "/home/user",
		},
		{
			name:     "tilde with multiple segments",
			input:    "~/path/to/file",
			expected: "/home/user/path/to/file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original provider
			original := homeDir
			defer func() { homeDir = original }()

			// Set mock provider
			homeDir = &mockHomeDirProvider{homeDir: "/home/user"}

			result := expandPath(tt.input)
			if result != tt.expected {
				t.Errorf("expandPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExpandPath_ErrorGettingHomeDir(t *testing.T) {
	// Save original provider
	original := homeDir
	defer func() { homeDir = original }()

	// Set mock that returns error
	homeDir = &mockHomeDirProvider{
		homeDir: "",
		err:     errors.New("cannot determine home directory"),
	}

	// When home dir fails, should return the original path unchanged
	input := "~/some/path"
	result := expandPath(input)
	if result != input {
		t.Errorf("expandPath(%q) = %q on error, want %q (unchanged)", input, result, input)
	}
}

func TestExpandPath_NonTilde(t *testing.T) {
	// Save original provider
	original := homeDir
	defer func() { homeDir = original }()

	// Set mock (should not be used)
	homeDir = &mockHomeDirProvider{homeDir: "/home/user"}

	// Regular paths should return unchanged (not use the mock)
	paths := []string{
		"/absolute/path",
		"/home/user/file",
		"./relative/path",
		"../parent/path",
		"relative/path/../file",
	}

	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			result := expandPath(p)
			if result != p {
				t.Errorf("expandPath(%q) = %q, want %q (unchanged)", p, result, p)
			}
		})
	}
}

func TestExpandPath_EmptyPath(t *testing.T) {
	// Save original provider
	original := homeDir
	defer func() { homeDir = original }()

	homeDir = &mockHomeDirProvider{homeDir: "/home/user"}

	result := expandPath("")
	if result != "" {
		t.Errorf("expandPath(%q) = %q, want %q", "", result, "")
	}
}
