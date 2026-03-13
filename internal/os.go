package internal

import "runtime"

// OSName returns the normalised OS name used throughout blueprint.
func OSName() string {
	switch runtime.GOOS {
	case "darwin":
		return "mac"
	case "linux":
		return "linux"
	case "windows":
		return "windows"
	default:
		return runtime.GOOS
	}
}
