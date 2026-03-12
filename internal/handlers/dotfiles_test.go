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
