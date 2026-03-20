package engine

import (
	"testing"
	"time"

	"github.com/elpic/blueprint/internal/parser"
)

// ---------------------------------------------------------------------------
// filterRulesByOS
// ---------------------------------------------------------------------------

func TestFilterRulesByOS(t *testing.T) {
	current := getOSName() // darwin → "mac", linux → "linux", etc.

	rules := []parser.Rule{
		{Action: "install", ID: "no-os"}, // no OSList → always included
		{Action: "install", ID: "current", OSList: []string{current}},
		{Action: "install", ID: "other", OSList: []string{"other-os"}},
		{Action: "install", ID: "multi", OSList: []string{"other-os", current}},
	}

	got := filterRulesByOS(rules)

	wantIDs := map[string]bool{"no-os": true, "current": true, "multi": true}
	if len(got) != len(wantIDs) {
		t.Fatalf("filterRulesByOS() returned %d rules, want %d", len(got), len(wantIDs))
	}
	for _, r := range got {
		if !wantIDs[r.ID] {
			t.Errorf("unexpected rule %q in result", r.ID)
		}
	}
}

func TestFilterRulesByOSEmptyList(t *testing.T) {
	got := filterRulesByOS(nil)
	if got != nil {
		t.Errorf("expected nil for empty input, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// deduplicateRules
// ---------------------------------------------------------------------------

func TestDeduplicateRulesByID(t *testing.T) {
	rules := []parser.Rule{
		{ID: "a", Action: "install"},
		{ID: "b", Action: "install"},
		{ID: "a", Action: "install"}, // duplicate
	}
	got := deduplicateRules(rules)
	if len(got) != 2 {
		t.Errorf("deduplicateRules() = %d rules, want 2", len(got))
	}
}

func TestDeduplicateRulesPreservesOrder(t *testing.T) {
	rules := []parser.Rule{
		{ID: "z", Action: "install"},
		{ID: "a", Action: "install"},
		{ID: "m", Action: "install"},
	}
	got := deduplicateRules(rules)
	if len(got) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(got))
	}
	if got[0].ID != "z" || got[1].ID != "a" || got[2].ID != "m" {
		t.Errorf("order not preserved: %v", got)
	}
}

func TestDeduplicateRulesEmpty(t *testing.T) {
	got := deduplicateRules(nil)
	if got != nil {
		t.Errorf("expected nil for nil input")
	}
}

// ---------------------------------------------------------------------------
// resolveDependencies
// ---------------------------------------------------------------------------

func TestResolveDependenciesNoDeps(t *testing.T) {
	rules := []parser.Rule{
		{ID: "a", Action: "run", RunCommand: "echo a"},
		{ID: "b", Action: "run", RunCommand: "echo b"},
	}
	got, err := resolveDependencies(rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 rules, got %d", len(got))
	}
}

func TestResolveDependenciesOrdering(t *testing.T) {
	// b depends on a → a must come first
	rules := []parser.Rule{
		{ID: "b", Action: "run", RunCommand: "echo b", After: []string{"a"}},
		{ID: "a", Action: "run", RunCommand: "echo a"},
	}
	got, err := resolveDependencies(rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(got))
	}
	if got[0].ID != "a" || got[1].ID != "b" {
		t.Errorf("wrong order: got [%s, %s], want [a, b]", got[0].ID, got[1].ID)
	}
}

func TestResolveDependenciesCircularDetected(t *testing.T) {
	rules := []parser.Rule{
		{ID: "a", Action: "run", RunCommand: "echo a", After: []string{"b"}},
		{ID: "b", Action: "run", RunCommand: "echo b", After: []string{"a"}},
	}
	_, err := resolveDependencies(rules)
	if err == nil {
		t.Error("expected circular dependency error, got nil")
	}
}

func TestResolveDependenciesEmpty(t *testing.T) {
	got, err := resolveDependencies(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for nil input")
	}
}

func TestResolveDependenciesMissingDepIgnored(t *testing.T) {
	// A rule referencing a non-existent dep should not error — the dep is just absent
	rules := []parser.Rule{
		{ID: "a", Action: "run", RunCommand: "echo a", After: []string{"nonexistent"}},
	}
	got, err := resolveDependencies(rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 rule, got %d", len(got))
	}
}

func TestResolveDependenciesChain(t *testing.T) {
	// c → b → a : result must be [a, b, c]
	rules := []parser.Rule{
		{ID: "c", Action: "run", RunCommand: "echo c", After: []string{"b"}},
		{ID: "b", Action: "run", RunCommand: "echo b", After: []string{"a"}},
		{ID: "a", Action: "run", RunCommand: "echo a"},
	}
	got, err := resolveDependencies(rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	order := []string{got[0].ID, got[1].ID, got[2].ID}
	want := []string{"a", "b", "c"}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("position %d: got %q, want %q", i, order[i], want[i])
		}
	}
}

// ---------------------------------------------------------------------------
// formatDuration
// ---------------------------------------------------------------------------

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "zero",
			input:    "0s",
			expected: "0s",
		},
		{
			name:     "seconds only",
			input:    "30s",
			expected: "30s",
		},
		{
			name:     "just under a minute",
			input:    "59s",
			expected: "59s",
		},
		{
			name:     "one minute",
			input:    "1m0s",
			expected: "1m 0s",
		},
		{
			name:     "minutes and seconds",
			input:    "2m30s",
			expected: "2m 30s",
		},
		{
			name:     "just under an hour",
			input:    "59m59s",
			expected: "59m 59s",
		},
		{
			name:     "one hour",
			input:    "1h0m0s",
			expected: "1h 0m 0s",
		},
		{
			name:     "hours and minutes",
			input:    "2h30m0s",
			expected: "2h 30m 0s",
		},
		{
			name:     "hours minutes and seconds",
			input:    "2h30m45s",
			expected: "2h 30m 45s",
		},
		{
			name:     "large duration",
			input:    "29h30m15s",
			expected: "29h 30m 15s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse duration from test format
			d, err := parseDuration(tt.input)
			if err != nil {
				t.Fatalf("invalid test duration %q: %v", tt.input, err)
			}
			got := formatDuration(d)
			if got != tt.expected {
				t.Errorf("formatDuration(%s) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// parseDuration is a helper to parse duration strings for testing
func parseDuration(s string) (time.Duration, error) {
	// Parse strings like "1h30m15s"
	return time.ParseDuration(s)
}

// ---------------------------------------------------------------------------
// normalizePath
// ---------------------------------------------------------------------------

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "absolute path unchanged",
			input:    "/Users/test/file.txt",
			expected: "/Users/test/file.txt",
		},
		{
			name:     "path with dots normalized",
			input:    "/path/./to/../file.txt",
			expected: "/path/file.txt",
		},
		{
			name:     "path with trailing slash",
			input:    "/path/to/dir//",
			expected: "/path/to/dir",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizePath(tt.input)
			if got != tt.expected {
				t.Errorf("normalizePath(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
