package ui

import (
	"strings"
	"testing"
	"time"
)

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

			// Verify this is a fast unit test (< 100μs)
			if duration > 100*time.Microsecond {
				t.Errorf("Test took %v, expected < 100μs for pure function test", duration)
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
			if duration > 100*time.Microsecond {
				t.Errorf("Test took %v, expected < 100μs for pure function test", duration)
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
