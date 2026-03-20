package internal

import "runtime"

// runtimeOS is a variable for testability, allowing tests to override the OS detection.
var runtimeOS = runtime.GOOS

// OSDetector is a port interface for OS detection.
// Adapters can implement this to provide OS information.
type OSDetector interface {
	// Name returns the normalized OS name (mac, linux, windows, or the raw GOOS for unknown).
	Name() string
}

// DefaultOSDetector is the default adapter that uses runtime.GOOS.
type DefaultOSDetector struct{}

// Name returns the normalised OS name used throughout blueprint.
func (d *DefaultOSDetector) Name() string {
	switch runtimeOS {
	case "darwin":
		return "mac"
	case "linux":
		return "linux"
	case "windows":
		return "windows"
	default:
		return runtimeOS
	}
}

// NewOSDetector creates a new DefaultOSDetector.
func NewOSDetector() OSDetector {
	return &DefaultOSDetector{}
}
