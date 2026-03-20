package handlers

import (
	"strings"
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

func TestDotfilesHandlerGetCommand(t *testing.T) {
	tests := []struct {
		name string
		rule parser.Rule
		want string
	}{
		{
			name: "basic clone",
			rule: parser.Rule{
				Action:       "dotfiles",
				DotfilesURL:  "https://github.com/user/dotfiles",
				DotfilesPath: "~/.dotfiles",
			},
			want: "git clone https://github.com/user/dotfiles ~/.dotfiles",
		},
		{
			name: "clone with branch",
			rule: parser.Rule{
				Action:         "dotfiles",
				DotfilesURL:    "https://github.com/user/dotfiles",
				DotfilesPath:   "~/.dotfiles",
				DotfilesBranch: "main",
			},
			want: "git clone -b main https://github.com/user/dotfiles ~/.dotfiles",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewDotfilesHandler(tt.rule, "")
			if got := h.GetCommand(); got != tt.want {
				t.Errorf("GetCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDotfilesHandlerGetDependencyKey(t *testing.T) {
	t.Run("uses ID when set", func(t *testing.T) {
		h := NewDotfilesHandler(parser.Rule{
			DotfilesURL: "https://github.com/user/dotfiles",
			ID:          "my-dots",
		}, "")
		if got := h.GetDependencyKey(); got != "my-dots" {
			t.Errorf("GetDependencyKey() = %q, want %q", got, "my-dots")
		}
	})
	t.Run("falls back to URL", func(t *testing.T) {
		h := NewDotfilesHandler(parser.Rule{
			DotfilesURL: "https://github.com/user/dotfiles",
		}, "")
		if got := h.GetDependencyKey(); got != "https://github.com/user/dotfiles" {
			t.Errorf("GetDependencyKey() = %q, want %q", got, "https://github.com/user/dotfiles")
		}
	})
}

func TestDotfilesHandlerDown_PathNotExist(t *testing.T) {
	h := NewDotfilesHandler(parser.Rule{
		Action:       "dotfiles",
		DotfilesURL:  "https://github.com/user/dotfiles",
		DotfilesPath: "/nonexistent/path/.dotfiles",
	}, "")
	msg, err := h.Down()
	if err != nil {
		t.Fatalf("Down() unexpected error: %v", err)
	}
	// Should report already removed or not found — not crash
	if msg == "" {
		t.Error("Down() returned empty message for non-existent path")
	}
}

func TestDotfilesHandlerGetDisplayDetails(t *testing.T) {
	h := NewDotfilesHandler(parser.Rule{
		DotfilesURL:  "https://github.com/user/dotfiles",
		DotfilesPath: "~/.dotfiles",
	}, "")
	got := h.GetDisplayDetails(false)
	if !strings.Contains(got, "dotfiles") {
		t.Errorf("GetDisplayDetails() = %q, expected to contain path info", got)
	}
}

func TestDotfilesHandlerIsInstalled(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		status   Status
		expected bool
	}{
		{
			name: "not installed - no status entry",
			rule: parser.Rule{
				Action:       "dotfiles",
				DotfilesURL:  "https://github.com/user/dotfiles",
				DotfilesPath: "~/.dotfiles",
			},
			status:   Status{},
			expected: false,
		},
		{
			name: "not installed - different URL",
			rule: parser.Rule{
				Action:       "dotfiles",
				DotfilesURL:  "https://github.com/user/dotfiles",
				DotfilesPath: "~/.dotfiles",
			},
			status: Status{
				Dotfiles: []DotfilesStatus{
					{
						URL:       "https://github.com/other/dotfiles",
						Path:      "~/.dotfiles",
						Blueprint: "/tmp/test.bp",
						OS:        "linux",
					},
				},
			},
			expected: false,
		},
		{
			name: "not installed - different blueprint",
			rule: parser.Rule{
				Action:       "dotfiles",
				DotfilesURL:  "https://github.com/user/dotfiles",
				DotfilesPath: "~/.dotfiles",
			},
			status: Status{
				Dotfiles: []DotfilesStatus{
					{
						URL:       "https://github.com/user/dotfiles",
						Path:      "~/.dotfiles",
						Blueprint: "/tmp/other.bp",
						OS:        "linux",
					},
				},
			},
			expected: false,
		},
		{
			name: "not installed - different OS",
			rule: parser.Rule{
				Action:       "dotfiles",
				DotfilesURL:  "https://github.com/user/dotfiles",
				DotfilesPath: "~/.dotfiles",
			},
			status: Status{
				Dotfiles: []DotfilesStatus{
					{
						URL:       "https://github.com/user/dotfiles",
						Path:      "~/.dotfiles",
						Blueprint: "/tmp/test.bp",
						OS:        "mac",
					},
				},
			},
			expected: false,
		},
		{
			name: "installed - exact match without SHA",
			rule: parser.Rule{
				Action:       "dotfiles",
				DotfilesURL:  "https://github.com/user/dotfiles",
				DotfilesPath: "~/.dotfiles",
			},
			status: Status{
				Dotfiles: []DotfilesStatus{
					{
						URL:       "https://github.com/user/dotfiles",
						Path:      "~/.dotfiles",
						Blueprint: "/tmp/test.bp",
						OS:        "linux",
						SHA:       "", // Empty SHA - will return true
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewDotfilesHandler(tt.rule, "/tmp")
			got := h.IsInstalled(&tt.status, "/tmp/test.bp", "linux")
			if got != tt.expected {
				t.Errorf("IsInstalled() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestShouldSkipEntry(t *testing.T) {
	tests := []struct {
		name       string
		entryName  string
		userSkip   []string
		shouldSkip bool
	}{
		{
			name:       "git directory",
			entryName:  ".git",
			userSkip:   nil,
			shouldSkip: true,
		},
		{
			name:       "github directory",
			entryName:  ".github",
			userSkip:   nil,
			shouldSkip: true,
		},
		{
			name:       "readme file case insensitive",
			entryName:  "README.md",
			userSkip:   nil,
			shouldSkip: true,
		},
		{
			name:       "readme lowercase",
			entryName:  "readme.txt",
			userSkip:   nil,
			shouldSkip: true,
		},
		{
			name:       "normal file",
			entryName:  ".bashrc",
			userSkip:   nil,
			shouldSkip: false,
		},
		{
			name:       "normal directory",
			entryName:  "nvim",
			userSkip:   nil,
			shouldSkip: false,
		},
		{
			name:       "user skip exact match",
			entryName:  "skipme",
			userSkip:   []string{"skipme"},
			shouldSkip: true,
		},
		{
			name:       "user skip partial match",
			entryName:  "something",
			userSkip:   []string{"skipme"},
			shouldSkip: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldSkipEntry(tt.entryName, tt.userSkip)
			if got != tt.shouldSkip {
				t.Errorf("shouldSkipEntry(%q, %v) = %v, want %v", tt.entryName, tt.userSkip, got, tt.shouldSkip)
			}
		})
	}
}

// TestDotfilesFindUninstallRulesNormalizesURLs tests that FindUninstallRules
// uses normalized URLs for comparison so SSH and HTTPS URLs for the same repo are treated as equal.
func TestDotfilesFindUninstallRulesNormalizesURLs(t *testing.T) {
	status := Status{
		Dotfiles: []DotfilesStatus{
			{
				URL:       "git@github.com:user/dotfiles.git",
				Path:      "~/.dotfiles",
				Blueprint: "/bp/setup.bp",
				OS:        "linux",
				SHA:       "abc123",
				Links:     []string{"~/.bashrc"},
			},
		},
	}

	// Current rules use HTTPS URL for the same repo
	currentRules := []parser.Rule{
		{
			Action:       "dotfiles",
			DotfilesURL:  "https://github.com/user/dotfiles.git",
			DotfilesPath: "~/.dotfiles",
		},
	}

	handler := NewDotfilesHandler(currentRules[0], "/bp/setup.bp")
	rules := handler.FindUninstallRules(&status, currentRules, "/bp/setup.bp", "linux")

	// Since the URLs are normalized to the same value, the repo should NOT be marked for uninstall
	if len(rules) > 0 {
		t.Errorf("FindUninstallRules should not return uninstall rules when SSH and HTTPS URLs refer to the same repo, got %d rules", len(rules))
	}
}

// TestDotfilesIsInstalledNormalizesURLs tests that IsInstalled uses normalized URLs
// for comparison so SSH and HTTPS URLs for the same repo are treated as equal.
func TestDotfilesIsInstalledNormalizesURLs(t *testing.T) {
	// Status has SSH URL - no SHA or Links to avoid additional checks
	// that would fail in a unit test (LocalSHA lookup, symlink existence)
	status := Status{
		Dotfiles: []DotfilesStatus{
			{
				URL:       "git@github.com:user/dotfiles.git",
				Path:      "~/.dotfiles",
				Blueprint: "/bp/setup.bp",
				OS:        "linux",
			},
		},
	}

	// Rule uses HTTPS URL for the same repo
	rule := parser.Rule{
		Action:       "dotfiles",
		DotfilesURL:  "https://github.com/user/dotfiles.git",
		DotfilesPath: "~/.dotfiles",
	}

	handler := NewDotfilesHandler(rule, "/bp/setup.bp")

	// Should find the installed dotfiles because URLs are normalized for comparison
	if !handler.IsInstalled(&status, "/bp/setup.bp", "linux") {
		t.Error("IsInstalled should return true when SSH and HTTPS URLs refer to the same repo")
	}
}
