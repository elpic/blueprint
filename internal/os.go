package internal

import (
	"os"
	"runtime"
	"strings"
)

// runtimeOS is a variable for testability, allowing tests to override the OS detection.
var runtimeOS = runtime.GOOS

// osReleaseFile is the file read to detect the Linux distribution. It is a
// variable so tests can point it at a fixture.
var osReleaseFile = "/etc/os-release"

// OSDetector is a port interface for OS detection.
// Adapters can implement this to provide OS information.
type OSDetector interface {
	// Name returns the normalized OS name (mac, linux, windows, or the raw GOOS for unknown).
	Name() string
	// Distro returns the normalized Linux distribution family (e.g. "arch",
	// "debian", "ubuntu", "fedora"), or "" on non-Linux or undetectable systems.
	Distro() string
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

// Distro returns the normalized Linux distribution family by inspecting
// /etc/os-release. It folds Arch derivatives (Manjaro, EndeavourOS, etc.)
// into "arch" via the ID_LIKE field.
func (d *DefaultOSDetector) Distro() string {
	if runtimeOS != "linux" {
		return ""
	}

	data, err := os.ReadFile(osReleaseFile)
	if err != nil {
		return ""
	}

	var id, idLike string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "ID="):
			id = unquoteOSRelease(strings.TrimPrefix(line, "ID="))
		case strings.HasPrefix(line, "ID_LIKE="):
			idLike = unquoteOSRelease(strings.TrimPrefix(line, "ID_LIKE="))
		}
	}

	return normalizeDistro(id, idLike)
}

// unquoteOSRelease strips surrounding double quotes from an os-release value.
func unquoteOSRelease(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		return v[1 : len(v)-1]
	}
	return v
}

// normalizeDistro maps an /etc/os-release ID/ID_LIKE pair to a known
// distribution family, lowercased, or returns the raw ID when unknown.
func normalizeDistro(id, idLike string) string {
	combined := strings.ToLower(id + " " + idLike)
	switch {
	case strings.Contains(combined, "arch"):
		return "arch"
	case strings.Contains(combined, "debian"):
		return "debian"
	case strings.Contains(combined, "ubuntu"):
		return "ubuntu"
	case strings.Contains(combined, "fedora"):
		return "fedora"
	case strings.Contains(combined, "rhel"):
		return "rhel"
	case strings.Contains(combined, "suse"):
		return "suse"
	case strings.Contains(combined, "alpine"):
		return "alpine"
	default:
		return strings.ToLower(id)
	}
}

// NewOSDetector creates a new DefaultOSDetector.
func NewOSDetector() OSDetector {
	return &DefaultOSDetector{}
}
