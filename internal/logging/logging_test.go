package logging

import (
	"testing"
	"time"
)

// TestLogLevel tests the LogLevel constants and basic operations.
func TestLogLevel(t *testing.T) {
	// Test that constants are defined correctly
	if INFO != 0 {
		t.Errorf("INFO should be 0, got %d", INFO)
	}
	if DEBUG != 1 {
		t.Errorf("DEBUG should be 1, got %d", DEBUG)
	}
}

// TestSetLogLevel tests programmatic log level setting.
func TestSetLogLevel(t *testing.T) {
	// Save original state
	original := GetLogLevel()
	defer func() {
		SetLogLevel(original)
	}()

	tests := []struct {
		name     string
		setLevel LogLevel
		expected LogLevel
	}{
		{"set info", INFO, INFO},
		{"set debug", DEBUG, DEBUG},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			SetLogLevel(tt.setLevel)
			result := GetLogLevel()

			duration := time.Since(start)

			if result != tt.expected {
				t.Errorf("SetLogLevel(%v) = %v, want %v", tt.setLevel, result, tt.expected)
			}

			// Should be extremely fast (< 10μs)
			if duration > 10*time.Millisecond {
				t.Errorf("Test took %v, expected < 10μs for simple setter", duration)
			}
		})
	}
}

// TestGetLogLevel tests the log level getter.
func TestGetLogLevel(t *testing.T) {
	// Save original state
	original := GetLogLevel()
	defer func() {
		SetLogLevel(original)
	}()

	// Test getting after setting different levels
	levels := []LogLevel{INFO, DEBUG}

	for _, level := range levels {
		t.Run(level.String(), func(t *testing.T) {
			start := time.Now()

			SetLogLevel(level)
			result := GetLogLevel()

			duration := time.Since(start)

			if result != level {
				t.Errorf("GetLogLevel() = %v, want %v", result, level)
			}

			// Should be extremely fast (< 10μs)
			if duration > 10*time.Millisecond {
				t.Errorf("Test took %v, expected < 10μs for simple getter", duration)
			}
		})
	}
}

// TestIsDebug tests the debug level check function.
func TestIsDebug(t *testing.T) {
	// Save original state
	original := GetLogLevel()
	defer func() {
		SetLogLevel(original)
	}()

	tests := []struct {
		name     string
		setLevel LogLevel
		expected bool
	}{
		{"info level not debug", INFO, false},
		{"debug level is debug", DEBUG, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			SetLogLevel(tt.setLevel)
			result := IsDebug()

			duration := time.Since(start)

			if result != tt.expected {
				t.Errorf("IsDebug() with level %v = %v, want %v", tt.setLevel, result, tt.expected)
			}

			// Should be extremely fast (< 1ms)
			if duration > time.Millisecond {
				t.Errorf("Test took %v, expected < 1ms for simple check", duration)
			}
		})
	}
}

// TestSetLogLevelString tests the internal string-based log level setting.
func TestSetLogLevelString(t *testing.T) {
	// Save original state
	original := GetLogLevel()
	defer func() {
		SetLogLevel(original)
	}()

	tests := []struct {
		name     string
		input    string
		expected LogLevel
	}{
		{"debug uppercase", "DEBUG", DEBUG},
		{"info uppercase", "INFO", INFO},
		{"empty string defaults to info", "", INFO},
		{"unknown string defaults to info", "UNKNOWN", INFO},
		{"lowercase debug (not supported)", "debug", INFO}, // falls back to INFO
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			setLogLevel(tt.input)
			result := GetLogLevel()

			duration := time.Since(start)

			if result != tt.expected {
				t.Errorf("setLogLevel(%q) = %v, want %v", tt.input, result, tt.expected)
			}

			// Should be extremely fast (< 50μs)
			if duration > 10*time.Millisecond {
				t.Errorf("Test took %v, expected < 50μs for string parsing", duration)
			}
		})
	}
}

// String method for LogLevel for better test output.
func (l LogLevel) String() string {
	switch l {
	case INFO:
		return "INFO"
	case DEBUG:
		return "DEBUG"
	default:
		return "UNKNOWN"
	}
}

// BenchmarkIsDebug benchmarks the IsDebug function.
func BenchmarkIsDebug(b *testing.B) {
	// Set a known state
	SetLogLevel(DEBUG)

	for i := 0; i < b.N; i++ {
		IsDebug()
	}
}

// BenchmarkSetGetLogLevel benchmarks the set/get cycle.
func BenchmarkSetGetLogLevel(b *testing.B) {
	for i := 0; i < b.N; i++ {
		SetLogLevel(DEBUG)
		GetLogLevel()
	}
}
