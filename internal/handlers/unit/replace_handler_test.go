package unit

import (
	"testing"
	"time"

	"github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/platform/testutils"
)

// TestReplaceHandler_GetDependencyKey_Pure tests dependency key generation
// without any I/O operations. Executes in microseconds.
func TestReplaceHandler_GetDependencyKey_Pure(t *testing.T) {
	tests := []struct {
		name         string
		ruleID       string
		replaceFile  string
		replaceMatch string
		expected     string
	}{
		{
			name:         "uses rule ID when present",
			ruleID:       "my-replace",
			replaceFile:  "/tmp/file",
			replaceMatch: "old",
			expected:     "my-replace",
		},
		{
			name:         "falls back to file+match key",
			ruleID:       "",
			replaceFile:  "/home/user/config",
			replaceMatch: "old_text",
			expected:     "/home/user/config\x00old_text",
		},
		{
			name:         "falls back to 'replace' when no file or match",
			ruleID:       "",
			replaceFile:  "",
			replaceMatch: "",
			expected:     "replace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			ruleBuilder := testutils.NewRule().
				WithAction("replace").
				WithID(tt.ruleID)

			rule := ruleBuilder.Build()
			rule.ReplaceFile = tt.replaceFile
			rule.ReplaceMatch = tt.replaceMatch

			handler := handlers.NewReplaceHandlerLegacy(rule, "/test")
			key := handler.GetDependencyKey()

			duration := time.Since(start)

			if key != tt.expected {
				t.Errorf("GetDependencyKey() = %q, want %q", key, tt.expected)
			}

			if duration > 10*time.Millisecond {
				t.Errorf("Test took %v, expected < 10ms for pure logic test", duration)
			}
		})
	}
}

// TestReplaceHandler_GetDisplayDetails_Pure tests display information generation
func TestReplaceHandler_GetDisplayDetails_Pure(t *testing.T) {
	tests := []struct {
		name         string
		replaceFile  string
		replaceMatch string
		replaceWith  string
		expected     string
	}{
		{
			name:         "basic display details",
			replaceFile:  "/tmp/file",
			replaceMatch: "old",
			replaceWith:  "new",
			expected:     "/tmp/file: \"old\" → \"new\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := parser.Rule{
				Action:      "replace",
				ReplaceFile: tt.replaceFile,
				ReplaceMatch: tt.replaceMatch,
				ReplaceWith:  tt.replaceWith,
			}
			handler := handlers.NewReplaceHandlerLegacy(rule, "/test")
			details := handler.GetDisplayDetails(false)
			if details != tt.expected {
				t.Errorf("GetDisplayDetails() = %q, want %q", details, tt.expected)
			}
		})
	}
}

// TestReplaceHandler_GetState_Pure tests state key-value pairs
func TestReplaceHandler_GetState_Pure(t *testing.T) {
	rule := parser.Rule{
		Action:       "replace",
		ReplaceFile:  "/tmp/file",
		ReplaceMatch: "old",
		ReplaceWith:  "new",
	}
	handler := handlers.NewReplaceHandlerLegacy(rule, "/test")
	state := handler.GetState(false)

	if state["summary"] != "/tmp/file: \"old\" → \"new\"" {
		t.Errorf("summary = %q, want %q", state["summary"], "/tmp/file: \"old\" → \"new\"")
	}
	if state["file"] != "/tmp/file" {
		t.Errorf("file = %q, want %q", state["file"], "/tmp/file")
	}
	if state["match"] != "old" {
		t.Errorf("match = %q, want %q", state["match"], "old")
	}
	if state["with"] != "new" {
		t.Errorf("with = %q, want %q", state["with"], "new")
	}
}

// TestReplaceHandler_FindUninstallRules_Pure tests uninstall rule detection
func TestReplaceHandler_FindUninstallRules_Pure(t *testing.T) {
	handler := handlers.NewReplaceHandlerLegacy(parser.Rule{}, "/test")

	status := &handlers.Status{
		Replaces: []handlers.ReplaceStatus{
			{
				File:       "/tmp/file",
				Match:      "old",
				With:       "new",
				Blueprint:  "/home/user/.blueprint",
				OS:         "mac",
				ReplacedAt: "2024-01-01T00:00:00Z",
			},
			{
				File:       "/tmp/other",
				Match:      "foo",
				With:       "bar",
				Blueprint:  "/home/user/.blueprint",
				OS:         "mac",
				ReplacedAt: "2024-01-01T00:00:00Z",
			},
		},
	}

	currentRules := []parser.Rule{
		{
			Action:       "replace",
			ReplaceFile:  "/tmp/file",
			ReplaceMatch: "old",
			ReplaceWith:  "new",
		},
	}

	rules := handler.FindUninstallRules(status, currentRules, "/home/user/.blueprint", "mac")

	if len(rules) != 1 {
		t.Fatalf("expected 1 uninstall rule, got %d", len(rules))
	}
	if rules[0].ReplaceFile != "/tmp/other" {
		t.Errorf("uninstall ReplaceFile = %q, want %q", rules[0].ReplaceFile, "/tmp/other")
	}
	if rules[0].ReplaceMatch != "foo" {
		t.Errorf("uninstall ReplaceMatch = %q, want %q", rules[0].ReplaceMatch, "foo")
	}
	if rules[0].ReplaceWith != "bar" {
		t.Errorf("uninstall ReplaceWith = %q, want %q", rules[0].ReplaceWith, "bar")
	}
}
