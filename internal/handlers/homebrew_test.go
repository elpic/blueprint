package handlers

import (
	"strings"
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

func TestHomebrewHandlerGetCommand(t *testing.T) {
	// Mock brew command to return consistent path across environments
	SetBrewCmdFunc(func() string { return "brew" })
	defer ResetBrewCmd()

	brew := brewCmd()
	tests := []struct {
		name string
		rule parser.Rule
		want string
	}{
		{
			name: "single formula",
			rule: parser.Rule{Action: "install", HomebrewPackages: []string{"git"}},
			want: brew + " install git",
		},
		{
			name: "multiple formulas",
			rule: parser.Rule{Action: "install", HomebrewPackages: []string{"git", "curl", "wget"}},
			want: brew + " install git curl wget",
		},
		{
			name: "single cask",
			rule: parser.Rule{Action: "install", HomebrewCasks: []string{"iterm2"}},
			want: brew + " install --cask iterm2",
		},
		{
			name: "formulas and casks",
			rule: parser.Rule{Action: "install", HomebrewPackages: []string{"git"}, HomebrewCasks: []string{"iterm2"}},
			want: brew + " install git && " + brew + " install --cask iterm2",
		},
		{
			name: "uninstall formula",
			rule: parser.Rule{Action: "uninstall", HomebrewPackages: []string{"git"}},
			want: brew + " uninstall -y git",
		},
		{
			name: "uninstall formula with version strips version",
			rule: parser.Rule{Action: "uninstall", HomebrewPackages: []string{"node@18"}},
			want: brew + " uninstall -y node",
		},
		{
			name: "uninstall cask",
			rule: parser.Rule{Action: "uninstall", HomebrewCasks: []string{"iterm2"}},
			want: brew + " uninstall --cask -y iterm2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHomebrewHandler(tt.rule, "")
			if got := h.GetCommand(); got != tt.want {
				t.Errorf("GetCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHomebrewHandlerGetDependencyKey(t *testing.T) {
	t.Run("uses ID when set", func(t *testing.T) {
		h := NewHomebrewHandler(parser.Rule{HomebrewPackages: []string{"git"}, ID: "my-git"}, "")
		if got := h.GetDependencyKey(); got != "my-git" {
			t.Errorf("GetDependencyKey() = %q, want %q", got, "my-git")
		}
	})
	t.Run("falls back to first package", func(t *testing.T) {
		h := NewHomebrewHandler(parser.Rule{HomebrewPackages: []string{"git", "curl"}}, "")
		if got := h.GetDependencyKey(); got != "git" {
			t.Errorf("GetDependencyKey() = %q, want %q", got, "git")
		}
	})
	t.Run("falls back to homebrew when no packages", func(t *testing.T) {
		h := NewHomebrewHandler(parser.Rule{}, "")
		if got := h.GetDependencyKey(); got != "homebrew" {
			t.Errorf("GetDependencyKey() = %q, want %q", got, "homebrew")
		}
	})
}

func TestHomebrewHandlerNeedsSudo(t *testing.T) {
	h := NewHomebrewHandler(parser.Rule{}, "")
	if h.NeedsSudo() {
		t.Error("NeedsSudo() = true, want false")
	}
}

func TestHomebrewHandlerGetDisplayDetails(t *testing.T) {
	h := NewHomebrewHandler(parser.Rule{
		HomebrewPackages: []string{"git", "curl"},
		HomebrewCasks:    []string{"iterm2"},
	}, "")
	details := h.GetDisplayDetails(false)
	if !strings.Contains(details, "git") || !strings.Contains(details, "iterm2") {
		t.Errorf("GetDisplayDetails() = %q, expected to contain package and cask names", details)
	}
}

func TestHomebrewUpSkipsAlreadyInstalled(t *testing.T) {
	origFormula := isBrewFormulaInstalled
	origCask := isBrewCaskInstalled
	origExec := executeCommandWithCache
	defer func() {
		isBrewFormulaInstalled = origFormula
		isBrewCaskInstalled = origCask
		executeCommandWithCache = origExec
	}()

	// All packages already installed
	isBrewFormulaInstalled = func(brew, formula string) bool { return true }
	isBrewCaskInstalled = func(brew, cask string) bool { return true }

	executed := false
	executeCommandWithCache = func(cmd string) (string, error) {
		executed = true
		return "", nil
	}

	rule := parser.Rule{
		Action:           "install",
		HomebrewPackages: []string{"git"},
		HomebrewCasks:    []string{"wezterm"},
	}
	h := NewHomebrewHandler(rule, "")
	out, err := h.Up()
	if err != nil {
		t.Fatalf("Up() error: %v", err)
	}
	if out != "already installed" {
		t.Errorf("expected 'already installed', got %q", out)
	}
	if executed {
		t.Error("executeCommandWithCache should not have been called when all packages already installed")
	}
}

func TestHomebrewUpInstallsMissingOnly(t *testing.T) {
	origFormula := isBrewFormulaInstalled
	origCask := isBrewCaskInstalled
	origExec := executeCommandWithCache
	defer func() {
		isBrewFormulaInstalled = origFormula
		isBrewCaskInstalled = origCask
		executeCommandWithCache = origExec
	}()

	// pkg-a is installed as a formula; pkg-b is not; cask-app is installed as a cask
	// (simulates a package that lives in Caskroom rather than Cellar)
	isBrewFormulaInstalled = func(brew, name string) bool { return name == "pkg-a" }
	isBrewCaskInstalled = func(brew, name string) bool { return name == "cask-app" }

	var ranCmd string
	executeCommandWithCache = func(cmd string) (string, error) {
		ranCmd = cmd
		return "", nil
	}

	rule := parser.Rule{
		Action:           "install",
		HomebrewPackages: []string{"pkg-a", "pkg-b"},
		HomebrewCasks:    []string{"cask-app"},
	}
	h := NewHomebrewHandler(rule, "")
	_, err := h.Up()
	if err != nil {
		t.Fatalf("Up() error: %v", err)
	}
	if !strings.Contains(ranCmd, "pkg-b") {
		t.Errorf("expected command to install pkg-b, got %q", ranCmd)
	}
	if strings.Contains(ranCmd, "pkg-a") {
		t.Errorf("should not reinstall already-installed pkg-a, got %q", ranCmd)
	}
	if strings.Contains(ranCmd, "cask-app") {
		t.Errorf("should not reinstall already-installed cask-app, got %q", ranCmd)
	}
}
