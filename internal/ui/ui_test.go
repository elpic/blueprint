package ui

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

// captureOutput captures stdout for testing print functions
func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	return buf.String()
}

// TestFormatFunctions tests all the formatting functions with pure unit tests.
// These tests execute in microseconds as they're pure functions.
func TestFormatFunctions(t *testing.T) {
	tests := []struct {
		name     string
		function func(string) string
		input    string
		contains []string // strings that should be in output
	}{
		{
			name:     "FormatHeader",
			function: FormatHeader,
			input:    "Test Header",
			contains: []string{"Test Header"},
		},
		{
			name:     "FormatSuccess",
			function: FormatSuccess,
			input:    "Success message",
			contains: []string{"✓", "Success message"},
		},
		{
			name:     "FormatError",
			function: FormatError,
			input:    "Error message",
			contains: []string{"✗", "Error message"},
		},
		{
			name:     "FormatInfo",
			function: FormatInfo,
			input:    "Info message",
			contains: []string{"Info message"},
		},
		{
			name:     "FormatHighlight",
			function: FormatHighlight,
			input:    "Highlighted text",
			contains: []string{"Highlighted text"},
		},
		{
			name:     "FormatDim",
			function: FormatDim,
			input:    "Dimmed text",
			contains: []string{"Dimmed text"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			result := tt.function(tt.input)

			duration := time.Since(start)

			// Verify the result contains expected content
			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("%s result %q does not contain expected %q", tt.name, result, expected)
				}
			}

			// Verify this is a fast unit test (< 1ms)
			if duration > 1*time.Millisecond {
				t.Errorf("Test took %v, expected < 1ms for pure function test", duration)
			}
		})
	}
}

// TestFormatFunctionsWithEmptyInput tests edge cases with empty strings.
func TestFormatFunctionsWithEmptyInput(t *testing.T) {
	tests := []struct {
		name     string
		function func(string) string
	}{
		{"FormatHeader", FormatHeader},
		{"FormatSuccess", FormatSuccess},
		{"FormatError", FormatError},
		{"FormatInfo", FormatInfo},
		{"FormatHighlight", FormatHighlight},
		{"FormatDim", FormatDim},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_empty", func(t *testing.T) {
			start := time.Now()

			result := tt.function("")

			duration := time.Since(start)

			// Should handle empty input gracefully (result should not be nil)
			// Some functions might return empty string for empty input, that's ok
			_ = result // Explicitly acknowledge we're checking the function doesn't panic

			// Should be fast
			if duration > 1*time.Millisecond {
				t.Errorf("Test took %v, expected < 1ms for pure function test", duration)
			}
		})
	}
}

// TestFormatSuccess tests the success formatter specifically.
func TestFormatSuccess(t *testing.T) {
	tests := []struct {
		input    string
		expected string // partial match
	}{
		{"test", "✓ test"},
		{"", "✓ "},
		{"multi word message", "✓ multi word message"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := FormatSuccess(tt.input)
			if !strings.Contains(result, tt.expected) {
				t.Errorf("FormatSuccess(%q) = %q, want to contain %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestFormatError tests the error formatter specifically.
func TestFormatError(t *testing.T) {
	tests := []struct {
		input    string
		expected string // partial match
	}{
		{"test", "✗ test"},
		{"", "✗ "},
		{"error message", "✗ error message"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := FormatError(tt.input)
			if !strings.Contains(result, tt.expected) {
				t.Errorf("FormatError(%q) = %q, want to contain %q", tt.input, result, tt.expected)
			}
		})
	}
}

// BenchmarkFormatFunctions benchmarks the formatting functions to ensure they're fast.
func BenchmarkFormatFunctions(b *testing.B) {
	functions := map[string]func(string) string{
		"FormatHeader":    FormatHeader,
		"FormatSuccess":   FormatSuccess,
		"FormatError":     FormatError,
		"FormatInfo":      FormatInfo,
		"FormatHighlight": FormatHighlight,
		"FormatDim":       FormatDim,
	}

	for name, fn := range functions {
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				fn("test message")
			}
		})
	}
}

// TestPrintRuleSummary tests the PrintRuleSummary function
func TestPrintRuleSummary(t *testing.T) {
	tests := []struct {
		name   string
		index  int
		total  int
		action string
		cmd    string
		status string
	}{
		{name: "first rule success", index: 1, total: 3, action: "install", cmd: "apt install vim", status: "success"},
		{name: "middle rule error", index: 2, total: 3, action: "run", cmd: "make build", status: "error"},
		{name: "last rule running", index: 3, total: 3, action: "clone", cmd: "git clone repo", status: "running"},
		{name: "single rule", index: 1, total: 1, action: "mkdir", cmd: "mkdir /tmp/test", status: "success"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(func() {
				PrintRuleSummary(tt.index, tt.total, tt.action, tt.cmd, tt.status)
			})
			if output == "" {
				t.Error("PrintRuleSummary() produced no output")
			}
		})
	}
}

// TestPrintExecutionHeader tests the PrintExecutionHeader function
func TestPrintExecutionHeader(t *testing.T) {
	tests := []struct {
		name             string
		isApply          bool
		os               string
		blueprint        string
		numRules         int
		numAutoUninstall int
		numCleanups      int
	}{
		{name: "apply mode with cleanups", isApply: true, os: "linux", blueprint: "/bp/setup.bp", numRules: 5, numAutoUninstall: 0, numCleanups: 2},
		{name: "apply mode no cleanups", isApply: true, os: "linux", blueprint: "/bp/setup.bp", numRules: 3, numAutoUninstall: 0, numCleanups: 0},
		{name: "plan mode with cleanups", isApply: false, os: "mac", blueprint: "/bp/dotfiles.bp", numRules: 10, numAutoUninstall: 0, numCleanups: 3},
		{name: "plan mode no cleanups", isApply: false, os: "mac", blueprint: "/bp/dotfiles.bp", numRules: 7, numAutoUninstall: 0, numCleanups: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(func() {
				PrintExecutionHeader(tt.isApply, tt.os, tt.blueprint, tt.numRules, tt.numAutoUninstall, tt.numCleanups)
			})
			if output == "" {
				t.Error("PrintExecutionHeader() produced no output")
			}
		})
	}
}

// TestPrintAutoUninstallSection tests the PrintAutoUninstallSection function
func TestPrintAutoUninstallSection(t *testing.T) {
	output := captureOutput(func() {
		PrintAutoUninstallSection()
	})
	if output == "" {
		t.Error("PrintAutoUninstallSection() produced no output")
	}
}

// TestPrintPlanFooter tests the PrintPlanFooter function
func TestPrintPlanFooter(t *testing.T) {
	output := captureOutput(func() {
		PrintPlanFooter()
	})
	if output == "" {
		t.Error("PrintPlanFooter() produced no output")
	}
}

// TestPrintProgressBar tests the PrintProgressBar function
func TestPrintProgressBar(t *testing.T) {
	tests := []struct {
		name    string
		current int
		total   int
	}{
		{name: "zero percent", current: 0, total: 10},
		{name: "half done", current: 5, total: 10},
		{name: "complete", current: 10, total: 10},
		{name: "zero total", current: 5, total: 0},
		{name: "negative total", current: 5, total: -1},
		{name: "large numbers", current: 100, total: 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(func() {
				PrintProgressBar(tt.current, tt.total)
			})
			// Zero or negative total should produce no output
			if tt.total <= 0 && output != "" {
				t.Errorf("PrintProgressBar() should produce no output for total=%d", tt.total)
			}
		})
	}
}

// TestClearProgressBar tests the ClearProgressBar function
func TestClearProgressBar(t *testing.T) {
	output := captureOutput(func() {
		ClearProgressBar()
	})
	if output == "" {
		t.Error("ClearProgressBar() produced no output")
	}
}

// TestFormatFunctionsWithSpecialCharacters tests Format functions with special characters
func TestFormatFunctionsWithSpecialCharacters(t *testing.T) {
	tests := []struct {
		name  string
		fn    func(string) string
		input string
	}{
		{name: "FormatHeader with unicode", fn: FormatHeader, input: "Héllo Wörld"},
		{name: "FormatSuccess with unicode", fn: FormatSuccess, input: "✓ Done ✓"},
		{name: "FormatError with unicode", fn: FormatError, input: "Errör: fäiled"},
		{name: "FormatHighlight with newlines", fn: FormatHighlight, input: "line1\nline2"},
		{name: "FormatDim with tabs", fn: FormatDim, input: "col1\tcol2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fn(tt.input)
			if result == "" {
				t.Errorf("%s() returned empty for input %q", tt.name, tt.input)
			}
		})
	}
}
