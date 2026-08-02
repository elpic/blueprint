package platform

import (
	"runtime"
	"testing"
)

// stubOSDetector provides a controllable OSDetector for command-building tests.
type stubOSDetector struct {
	name string
	root bool
}

func (s stubOSDetector) Name() string { return s.name }
func (s stubOSDetector) Architecture() string {
	return "amd64"
}
func (s stubOSDetector) IsRoot() bool { return s.root }
func (s stubOSDetector) CurrentUser() (UserInfo, error) {
	return UserInfo{}, nil
}

func TestRealPackageManagerProvider_InstallCommand(t *testing.T) {
	mac := &realPackageManagerProvider{osDetector: stubOSDetector{name: "mac"}}
	linux := &realPackageManagerProvider{osDetector: stubOSDetector{name: "linux"}}
	linuxRoot := &realPackageManagerProvider{osDetector: stubOSDetector{name: "linux", root: true}}

	tests := []struct {
		name     string
		p        *realPackageManagerProvider
		packages []string
		manager  string
		want     string
	}{
		{name: "brew", p: mac, packages: []string{"git", "curl"}, manager: "brew", want: "brew install git curl"},
		{name: "brew homebrew alias", p: mac, packages: []string{"git"}, manager: "homebrew", want: "brew install git"},
		{name: "apt with sudo", p: linux, packages: []string{"git", "curl"}, manager: "apt", want: "sudo apt-get install -y git curl"},
		{name: "apt-get with sudo", p: linux, packages: []string{"git"}, manager: "apt-get", want: "sudo apt-get install -y git"},
		{name: "apt as root without sudo", p: linuxRoot, packages: []string{"git"}, manager: "apt", want: "apt-get install -y git"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := tt.p.installCommand(tt.packages, tt.manager)
			if err != nil {
				t.Fatalf("installCommand() error: %v", err)
			}
			if cmd != tt.want {
				t.Errorf("installCommand() = %q, want %q", cmd, tt.want)
			}
		})
	}
}

func TestRealPackageManagerProvider_UninstallCommand(t *testing.T) {
	mac := &realPackageManagerProvider{osDetector: stubOSDetector{name: "mac"}}
	linux := &realPackageManagerProvider{osDetector: stubOSDetector{name: "linux"}}

	tests := []struct {
		name     string
		p        *realPackageManagerProvider
		packages []string
		manager  string
		want     string
	}{
		{name: "brew", p: mac, packages: []string{"git", "curl"}, manager: "brew", want: "brew uninstall -y git curl"},
		{name: "apt with sudo", p: linux, packages: []string{"git"}, manager: "apt", want: "sudo apt-get remove -y git"},
		{name: "apt-get with sudo", p: linux, packages: []string{"git"}, manager: "apt-get", want: "sudo apt-get remove -y git"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := tt.p.uninstallCommand(tt.packages, tt.manager)
			if err != nil {
				t.Fatalf("uninstallCommand() error: %v", err)
			}
			if cmd != tt.want {
				t.Errorf("uninstallCommand() = %q, want %q", cmd, tt.want)
			}
		})
	}
}

func TestRealPackageManagerProvider_CommandErrors(t *testing.T) {
	p := &realPackageManagerProvider{osDetector: stubOSDetector{name: "mac"}}

	if _, err := p.installCommand(nil, "brew"); err == nil {
		t.Error("installCommand(nil, brew) = nil error, want error")
	}
	if _, err := p.installCommand([]string{"git"}, "pacman"); err == nil {
		t.Error("installCommand(git, pacman) = nil error, want error")
	}
	if _, err := p.uninstallCommand(nil, "apt"); err == nil {
		t.Error("uninstallCommand(nil, apt) = nil error, want error")
	}
	if _, err := p.uninstallCommand([]string{"git"}, "pacman"); err == nil {
		t.Error("uninstallCommand(git, pacman) = nil error, want error")
	}
}

func TestRealPackageManagerProvider_ProbeCommand(t *testing.T) {
	p := &realPackageManagerProvider{}

	if got := p.probeCommand("git", "brew"); got != "brew list --versions git" {
		t.Errorf("probeCommand(git, brew) = %q, want %q", got, "brew list --versions git")
	}
	if got := p.probeCommand("git", "apt"); got != "dpkg -s git" {
		t.Errorf("probeCommand(git, apt) = %q, want %q", got, "dpkg -s git")
	}
	if got := p.probeCommand("git", "pacman"); got != "" {
		t.Errorf("probeCommand(git, pacman) = %q, want empty", got)
	}
}

func TestParseBrewVersion(t *testing.T) {
	got, err := parseBrewVersion("git 2.43.0\n")
	if err != nil {
		t.Fatalf("parseBrewVersion error: %v", err)
	}
	if got != "2.43.0" {
		t.Errorf("parseBrewVersion() = %q, want %q", got, "2.43.0")
	}

	if _, err := parseBrewVersion(""); err == nil {
		t.Error("parseBrewVersion(empty) = nil error, want error")
	}
	if _, err := parseBrewVersion("git"); err == nil {
		t.Error("parseBrewVersion(no version) = nil error, want error")
	}
}

func TestParseDpkgVersion(t *testing.T) {
	output := "Package: git\nStatus: install ok installed\nVersion: 2.43.0-1ubuntu1\n"
	got, err := parseDpkgVersion(output)
	if err != nil {
		t.Fatalf("parseDpkgVersion error: %v", err)
	}
	if got != "2.43.0-1ubuntu1" {
		t.Errorf("parseDpkgVersion() = %q, want %q", got, "2.43.0-1ubuntu1")
	}

	if _, err := parseDpkgVersion("Package: git\n"); err == nil {
		t.Error("parseDpkgVersion(no version) = nil error, want error")
	}
}

func TestRealPackageManagerProvider_GetDefaultManager(t *testing.T) {
	p := &realPackageManagerProvider{}

	switch runtime.GOOS {
	case "darwin":
		if got := p.GetDefaultManager(); got != "brew" {
			t.Errorf("GetDefaultManager() = %q, want %q", got, "brew")
		}
	case "linux":
		if got := p.GetDefaultManager(); got != "apt" {
			t.Errorf("GetDefaultManager() = %q, want %q", got, "apt")
		}
	default:
		if got := p.GetDefaultManager(); got != "" {
			t.Errorf("GetDefaultManager() = %q, want empty", got)
		}
	}
}

func TestRealPackageManagerProvider_IsManagerAvailable_Unknown(t *testing.T) {
	p := &realPackageManagerProvider{}
	if p.IsManagerAvailable("pacman") {
		t.Error("IsManagerAvailable(pacman) = true, want false")
	}
}
