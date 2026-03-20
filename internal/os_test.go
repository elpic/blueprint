package internal

import (
	"testing"
)

// MockOSDetector is a test adapter implementing OSDetector.
type MockOSDetector struct {
	osName string
}

func (m *MockOSDetector) Name() string {
	return m.osName
}

func TestOSDetector_Name(t *testing.T) {
	tests := []struct {
		name     string
		goos     string
		expected string
	}{
		{
			name:     "darwin normalizes to mac",
			goos:     "darwin",
			expected: "mac",
		},
		{
			name:     "linux stays as linux",
			goos:     "linux",
			expected: "linux",
		},
		{
			name:     "windows stays as windows",
			goos:     "windows",
			expected: "windows",
		},
		{
			name:     "freebsd returns as-is",
			goos:     "freebsd",
			expected: "freebsd",
		},
		{
			name:     "openbsd returns as-is",
			goos:     "openbsd",
			expected: "openbsd",
		},
		{
			name:     "netbsd returns as-is",
			goos:     "netbsd",
			expected: "netbsd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original value
			original := runtimeOS
			defer func() { runtimeOS = original }()

			// Set test value
			runtimeOS = tt.goos

			// Create detector and test
			detector := NewOSDetector()
			result := detector.Name()

			if result != tt.expected {
				t.Errorf("OSDetector.Name() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestOSDetector_Name_Darwin(t *testing.T) {
	// Test darwin specifically
	original := runtimeOS
	defer func() { runtimeOS = original }()

	runtimeOS = "darwin"
	detector := NewOSDetector()

	if got := detector.Name(); got != "mac" {
		t.Errorf("darwin -> mac: got %q, want %q", got, "mac")
	}
}

func TestOSDetector_Name_Linux(t *testing.T) {
	original := runtimeOS
	defer func() { runtimeOS = original }()

	runtimeOS = "linux"
	detector := NewOSDetector()

	if got := detector.Name(); got != "linux" {
		t.Errorf("linux -> linux: got %q, want %q", got, "linux")
	}
}

func TestOSDetector_Name_Windows(t *testing.T) {
	original := runtimeOS
	defer func() { runtimeOS = original }()

	runtimeOS = "windows"
	detector := NewOSDetector()

	if got := detector.Name(); got != "windows" {
		t.Errorf("windows -> windows: got %q, want %q", got, "windows")
	}
}

func TestOSDetector_Name_Unknown(t *testing.T) {
	original := runtimeOS
	defer func() { runtimeOS = original }()

	runtimeOS = "freebsd"
	detector := NewOSDetector()

	if got := detector.Name(); got != "freebsd" {
		t.Errorf("freebsd -> freebsd: got %q, want %q", got, "freebsd")
	}
}

func TestMockOSDetector(t *testing.T) {
	tests := []struct {
		name     string
		osName   string
		expected string
	}{
		{
			name:     "mock mac",
			osName:   "mac",
			expected: "mac",
		},
		{
			name:     "mock linux",
			osName:   "linux",
			expected: "linux",
		},
		{
			name:     "mock windows",
			osName:   "windows",
			expected: "windows",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &MockOSDetector{osName: tt.osName}
			if got := mock.Name(); got != tt.expected {
				t.Errorf("MockOSDetector.Name() = %q, want %q", got, tt.expected)
			}
		})
	}
}
