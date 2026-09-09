package handlers

import (
	"testing"

	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/platform"
	platformmocks "github.com/elpic/blueprint/internal/platform/mocks"
)

func TestInstallHandler_ArchDefaultManager(t *testing.T) {
	system := platformmocks.NewMockSystemProvider().WithOS("linux").WithDistro("arch")
	container := platform.NewTestContainer().WithSystemProvider(system).Build()
	handler := NewInstallHandler(parser.Rule{
		Action:   "install",
		Packages: []parser.Package{{Name: "git"}},
	}, "", container)

	if got, want := handler.GetCommand(), "sudo pacman -S --noconfirm --needed git"; got != want {
		t.Errorf("GetCommand() = %q, want %q", got, want)
	}
}

func TestInstallHandler_ArchExplicitManagerUninstall(t *testing.T) {
	system := platformmocks.NewMockSystemProvider().WithOS("linux").WithDistro("arch")
	container := platform.NewTestContainer().WithSystemProvider(system).Build()
	handler := NewInstallHandler(parser.Rule{
		Action:   "uninstall",
		Packages: []parser.Package{{Name: "git", PackageManager: "pacman"}},
	}, "", container)

	if got, want := handler.GetCommand(), "sudo pacman -R --noconfirm git"; got != want {
		t.Errorf("GetCommand() = %q, want %q", got, want)
	}
}
