package handlers

import (
	"strings"
	"testing"

	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/platform"
)

// customMockExecutor for test-specific behavior
type customMockExecutor struct {
	executeFunc func(string) (string, error)
}

// Ensure it implements platform.CommandExecutor
var _ platform.CommandExecutor = (*customMockExecutor)(nil)

func (c *customMockExecutor) Execute(cmd string) (string, error) {
	if c.executeFunc != nil {
		return c.executeFunc(cmd)
	}
	return "", nil
}

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
	defer func() {
		isBrewFormulaInstalled = origFormula
		isBrewCaskInstalled = origCask
	}()

	// All packages already installed
	isBrewFormulaInstalled = func(brew, formula string) bool { return true }
	isBrewCaskInstalled = func(brew, cask string) bool { return true }

	// Use dependency injection pattern
	executed := false

	// Create a custom executor that tracks execution
	mockExecutor := &customMockExecutor{
		executeFunc: func(cmd string) (string, error) {
			executed = true
			return "", nil
		},
	}

	originalExecutor := commandExecutor
	defer func() { commandExecutor = originalExecutor }()
	commandExecutor = mockExecutor

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

func TestHomebrewUpdateStatusRecordsBased(t *testing.T) {
	SetBrewCmdFunc(func() string { return "brew" })
	defer ResetBrewCmd()

	t.Run("records formulas on success", func(t *testing.T) {
		h := NewHomebrewHandler(parser.Rule{
			Action:           "homebrew",
			HomebrewPackages: []string{"git", "curl"},
		}, "")
		status := &Status{}
		records := []ExecutionRecord{
			{Command: h.GetCommand(), Status: "success"},
		}
		if err := h.UpdateStatus(status, records, "/tmp/test.bp", "mac"); err != nil {
			t.Fatalf("UpdateStatus() error = %v", err)
		}
		if len(status.Brews) != 2 {
			t.Fatalf("expected 2 brew entries, got %d", len(status.Brews))
		}
		if status.Brews[0].Formula != "git" || status.Brews[1].Formula != "curl" {
			t.Errorf("unexpected formulas: %v", status.Brews)
		}
	})

	t.Run("records casks on success", func(t *testing.T) {
		h := NewHomebrewHandler(parser.Rule{
			Action:        "homebrew",
			HomebrewCasks: []string{"iterm2"},
		}, "")
		status := &Status{}
		records := []ExecutionRecord{
			{Command: h.GetCommand(), Status: "success"},
		}
		if err := h.UpdateStatus(status, records, "/tmp/test.bp", "mac"); err != nil {
			t.Fatalf("UpdateStatus() error = %v", err)
		}
		if len(status.Brews) != 1 {
			t.Fatalf("expected 1 brew entry, got %d", len(status.Brews))
		}
		if status.Brews[0].Formula != "cask:iterm2" {
			t.Errorf("expected cask:iterm2, got %q", status.Brews[0].Formula)
		}
	})

	t.Run("skips status on failed command", func(t *testing.T) {
		h := NewHomebrewHandler(parser.Rule{
			Action:           "homebrew",
			HomebrewPackages: []string{"git"},
		}, "")
		status := &Status{}
		records := []ExecutionRecord{
			{Command: h.GetCommand(), Status: "error"},
		}
		if err := h.UpdateStatus(status, records, "/tmp/test.bp", "mac"); err != nil {
			t.Fatalf("UpdateStatus() error = %v", err)
		}
		if len(status.Brews) != 0 {
			t.Fatalf("expected 0 brew entries after failed command, got %d", len(status.Brews))
		}
	})

	t.Run("records on already installed", func(t *testing.T) {
		h := NewHomebrewHandler(parser.Rule{
			Action:           "homebrew",
			HomebrewPackages: []string{"git"},
		}, "")
		status := &Status{}
		records := []ExecutionRecord{
			{Command: h.GetCommand(), Status: "success", Output: "already installed"},
		}
		if err := h.UpdateStatus(status, records, "/tmp/test.bp", "mac"); err != nil {
			t.Fatalf("UpdateStatus() error = %v", err)
		}
		if len(status.Brews) != 1 {
			t.Fatalf("expected 1 brew entry, got %d", len(status.Brews))
		}
	})

	t.Run("extracts version from package spec", func(t *testing.T) {
		h := NewHomebrewHandler(parser.Rule{
			Action:           "homebrew",
			HomebrewPackages: []string{"node@18"},
		}, "")
		status := &Status{}
		records := []ExecutionRecord{
			{Command: h.GetCommand(), Status: "success"},
		}
		if err := h.UpdateStatus(status, records, "/tmp/test.bp", "mac"); err != nil {
			t.Fatalf("UpdateStatus() error = %v", err)
		}
		if len(status.Brews) != 1 {
			t.Fatalf("expected 1 brew entry, got %d", len(status.Brews))
		}
		if status.Brews[0].Formula != "node" {
			t.Errorf("expected formula 'node', got %q", status.Brews[0].Formula)
		}
		if status.Brews[0].Version != "18" {
			t.Errorf("expected version '18', got %q", status.Brews[0].Version)
		}
	})

	t.Run("uninstall removes from status", func(t *testing.T) {
		bp := normalizeBlueprint("/tmp/test.bp")
		h := NewHomebrewHandler(parser.Rule{
			Action:           "uninstall",
			HomebrewPackages: []string{"git"},
			HomebrewCasks:    []string{"iterm2"},
		}, "")
		status := &Status{
			Brews: []HomebrewStatus{
				{Formula: "git", Blueprint: bp, OS: "mac"},
				{Formula: "cask:iterm2", Blueprint: bp, OS: "mac"},
				{Formula: "curl", Blueprint: bp, OS: "mac"},
			},
		}
		if err := h.UpdateStatus(status, nil, "/tmp/test.bp", "mac"); err != nil {
			t.Fatalf("UpdateStatus() error = %v", err)
		}
		if len(status.Brews) != 1 {
			t.Fatalf("expected 1 remaining brew entry, got %d", len(status.Brews))
		}
		if status.Brews[0].Formula != "curl" {
			t.Errorf("expected remaining formula 'curl', got %q", status.Brews[0].Formula)
		}
	})
}

func TestHomebrewUpInstallsMissingOnly(t *testing.T) {
	origFormula := isBrewFormulaInstalled
	origCask := isBrewCaskInstalled
	defer func() {
		isBrewFormulaInstalled = origFormula
		isBrewCaskInstalled = origCask
	}()

	// pkg-a is installed as a formula; pkg-b is not; cask-app is installed as a cask
	// (simulates a package that lives in Caskroom rather than Cellar)
	isBrewFormulaInstalled = func(brew, name string) bool { return name == "pkg-a" }
	isBrewCaskInstalled = func(brew, name string) bool { return name == "cask-app" }

	// Use dependency injection pattern
	var ranCmd string
	mockExecutor := &customMockExecutor{
		executeFunc: func(cmd string) (string, error) {
			ranCmd = cmd
			return "", nil
		},
	}

	originalExecutor := commandExecutor
	defer func() { commandExecutor = originalExecutor }()
	commandExecutor = mockExecutor

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
