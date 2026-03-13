package handlers

import (
	"strings"
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

// ---------------------------------------------------------------------------
// RunHandler
// ---------------------------------------------------------------------------

func TestRunHandlerGetCommand(t *testing.T) {
	tests := []struct {
		name string
		rule parser.Rule
		want string
	}{
		{
			name: "plain command",
			rule: parser.Rule{Action: "run", RunCommand: "echo hello"},
			want: "echo hello",
		},
		{
			name: "sudo command",
			rule: parser.Rule{Action: "run", RunCommand: "apt install curl", RunSudo: true},
			want: "sudo apt install curl",
		},
		{
			name: "uninstall with undo",
			rule: parser.Rule{Action: "uninstall", RunCommand: "echo hello", RunUndo: "echo bye"},
			want: "echo bye",
		},
		{
			name: "uninstall with sudo undo",
			rule: parser.Rule{Action: "uninstall", RunCommand: "apt install curl", RunUndo: "apt remove curl", RunSudo: true},
			want: "sudo apt remove curl",
		},
		{
			name: "uninstall with no undo",
			rule: parser.Rule{Action: "uninstall", RunCommand: "echo hello"},
			want: "# no undo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewRunHandler(tt.rule, "")
			if got := h.GetCommand(); got != tt.want {
				t.Errorf("GetCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunHandlerGetDependencyKey(t *testing.T) {
	t.Run("uses ID when set", func(t *testing.T) {
		h := NewRunHandler(parser.Rule{Action: "run", RunCommand: "echo hi", ID: "my-id"}, "")
		if got := h.GetDependencyKey(); got != "my-id" {
			t.Errorf("GetDependencyKey() = %q, want %q", got, "my-id")
		}
	})
	t.Run("falls back to command", func(t *testing.T) {
		h := NewRunHandler(parser.Rule{Action: "run", RunCommand: "echo hi"}, "")
		if got := h.GetDependencyKey(); got != "echo hi" {
			t.Errorf("GetDependencyKey() = %q, want %q", got, "echo hi")
		}
	})
}

func TestRunHandlerDown_NoUndo(t *testing.T) {
	h := NewRunHandler(parser.Rule{Action: "run", RunCommand: "echo hi"}, "")
	msg, err := h.Down()
	if err != nil {
		t.Fatalf("Down() error: %v", err)
	}
	if !strings.Contains(msg, "no undo") {
		t.Errorf("Down() = %q, want message containing 'no undo'", msg)
	}
}

// ---------------------------------------------------------------------------
// RunShHandler
// ---------------------------------------------------------------------------

func TestRunShHandlerGetCommand(t *testing.T) {
	tests := []struct {
		name string
		rule parser.Rule
		want string
	}{
		{
			name: "install returns URL",
			rule: parser.Rule{Action: "run-sh", RunShURL: "https://example.com/install.sh"},
			want: "https://example.com/install.sh",
		},
		{
			name: "uninstall with undo",
			rule: parser.Rule{Action: "uninstall", RunShURL: "https://example.com/install.sh", RunUndo: "rm -rf /opt/tool"},
			want: "rm -rf /opt/tool",
		},
		{
			name: "uninstall no undo",
			rule: parser.Rule{Action: "uninstall", RunShURL: "https://example.com/install.sh"},
			want: "# no undo",
		},
		{
			name: "uninstall sudo undo",
			rule: parser.Rule{Action: "uninstall", RunShURL: "https://x.com/s.sh", RunUndo: "rm -rf /opt/tool", RunSudo: true},
			want: "sudo rm -rf /opt/tool",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewRunShHandler(tt.rule, "")
			if got := h.GetCommand(); got != tt.want {
				t.Errorf("GetCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunShHandlerGetDependencyKey(t *testing.T) {
	t.Run("uses ID when set", func(t *testing.T) {
		h := NewRunShHandler(parser.Rule{Action: "run-sh", RunShURL: "https://x.com/s.sh", ID: "my-script"}, "")
		if got := h.GetDependencyKey(); got != "my-script" {
			t.Errorf("GetDependencyKey() = %q, want %q", got, "my-script")
		}
	})
	t.Run("falls back to URL", func(t *testing.T) {
		h := NewRunShHandler(parser.Rule{Action: "run-sh", RunShURL: "https://x.com/s.sh"}, "")
		if got := h.GetDependencyKey(); got != "https://x.com/s.sh" {
			t.Errorf("GetDependencyKey() = %q, want %q", got, "https://x.com/s.sh")
		}
	})
}

func TestRunShHandlerDown_NoUndo(t *testing.T) {
	h := NewRunShHandler(parser.Rule{Action: "run-sh", RunShURL: "https://x.com/s.sh"}, "")
	msg, err := h.Down()
	if err != nil {
		t.Fatalf("Down() error: %v", err)
	}
	if !strings.Contains(msg, "no undo") {
		t.Errorf("Down() = %q, want message containing 'no undo'", msg)
	}
}

func TestRunShHandlerHttpClientHasTimeout(t *testing.T) {
	h := NewRunShHandler(parser.Rule{}, "")
	client := h.httpClient()
	if client.Timeout == 0 {
		t.Error("httpClient() Timeout is 0, want a non-zero timeout")
	}
}
