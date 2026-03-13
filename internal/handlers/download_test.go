package handlers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

func TestDownloadHandlerGetCommand(t *testing.T) {
	tests := []struct {
		name string
		rule parser.Rule
		want string
	}{
		{
			name: "basic download",
			rule: parser.Rule{Action: "download", DownloadURL: "https://example.com/file.tar.gz", DownloadPath: "~/tools/file.tar.gz"},
			want: "curl -fsSL https://example.com/file.tar.gz -o ~/tools/file.tar.gz",
		},
		{
			name: "download with permissions",
			rule: parser.Rule{Action: "download", DownloadURL: "https://example.com/bin", DownloadPath: "~/bin/tool", DownloadPerms: "755"},
			want: "curl -fsSL https://example.com/bin -o ~/bin/tool && chmod 755 ~/bin/tool",
		},
		{
			name: "uninstall",
			rule: parser.Rule{Action: "uninstall", DownloadPath: "~/bin/tool"},
			want: "rm -f ~/bin/tool",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewDownloadHandler(tt.rule, "")
			if got := h.GetCommand(); got != tt.want {
				t.Errorf("GetCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDownloadHandlerGetDependencyKey(t *testing.T) {
	t.Run("uses ID when set", func(t *testing.T) {
		h := NewDownloadHandler(parser.Rule{Action: "download", DownloadPath: "~/bin/tool", ID: "my-tool"}, "")
		if got := h.GetDependencyKey(); got != "my-tool" {
			t.Errorf("GetDependencyKey() = %q, want %q", got, "my-tool")
		}
	})
	t.Run("falls back to path", func(t *testing.T) {
		h := NewDownloadHandler(parser.Rule{Action: "download", DownloadPath: "~/bin/tool"}, "")
		if got := h.GetDependencyKey(); got != "~/bin/tool" {
			t.Errorf("GetDependencyKey() = %q, want %q", got, "~/bin/tool")
		}
	})
}

func TestDownloadHandlerDown_FileNotExist(t *testing.T) {
	h := NewDownloadHandler(parser.Rule{Action: "uninstall", DownloadPath: "/nonexistent/path/file.txt"}, "")
	msg, err := h.Down()
	if err != nil {
		t.Fatalf("Down() error: %v", err)
	}
	if !strings.Contains(msg, "does not exist") {
		t.Errorf("Down() = %q, want message containing 'does not exist'", msg)
	}
}

func TestDownloadHandlerDown_RemovesFile(t *testing.T) {
	// Create a real temp file and verify Down() removes it.
	tmp, err := os.CreateTemp("", "blueprint-download-test-*")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	_ = tmp.Close()
	path := tmp.Name()

	h := NewDownloadHandler(parser.Rule{Action: "uninstall", DownloadPath: path}, "")
	msg, err := h.Down()
	if err != nil {
		t.Fatalf("Down() error: %v", err)
	}
	if !strings.Contains(msg, "Removed") {
		t.Errorf("Down() = %q, want message containing 'Removed'", msg)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("file %s still exists after Down()", path)
	}
}

func TestDownloadHandlerHttpClientHasTimeout(t *testing.T) {
	h := NewDownloadHandler(parser.Rule{}, "")
	if h.httpClient().Timeout == 0 {
		t.Error("httpClient() Timeout is 0, want a non-zero timeout")
	}
}

func TestDownloadExpandPath(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}
	got := expandPath("~/foo/bar")
	want := filepath.Join(homeDir, "foo/bar")
	if got != want {
		t.Errorf("expandPath(~/foo/bar) = %q, want %q", got, want)
	}
}
