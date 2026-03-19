package handlers

import (
	"strings"
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

func TestMiseHandlerGetCommand(t *testing.T) {
	// Mock mise command to return consistent path across environments
	SetMiseCmdFunc(func() string { return "mise" })
	defer ResetMiseCmd()

	mise := (&MiseHandler{}).miseCmd()
	tests := []struct {
		name     string
		rule     parser.Rule
		contains string
	}{
		{
			name:     "uninstall action",
			rule:     parser.Rule{Action: "uninstall"},
			contains: "mise uninstall",
		},
		{
			name:     "global install single package",
			rule:     parser.Rule{Action: "mise", MisePackages: []string{"node@18"}},
			contains: "node@18",
		},
		{
			name:     "global install uses -g flag",
			rule:     parser.Rule{Action: "mise", MisePackages: []string{"node@18"}},
			contains: mise + " use -g node@18",
		},
		{
			name:     "no packages returns mise-init",
			rule:     parser.Rule{Action: "mise"},
			contains: "mise-init",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewMiseHandler(tt.rule, "")
			got := h.GetCommand()
			if !strings.Contains(got, tt.contains) {
				t.Errorf("GetCommand() = %q, want it to contain %q", got, tt.contains)
			}
		})
	}
}

func TestMiseHandlerGetCommand_LocalPath(t *testing.T) {
	h := NewMiseHandler(parser.Rule{
		Action:       "mise",
		MisePackages: []string{"python@3.11"},
		MisePath:     "~/projects/myapp",
	}, "")
	got := h.GetCommand()
	// Local (non-global) command should include the path prefix and no -g flag
	if strings.Contains(got, " -g ") {
		t.Errorf("GetCommand() = %q, local install should not use -g flag", got)
	}
	if !strings.Contains(got, "myapp") {
		t.Errorf("GetCommand() = %q, expected to contain the project path", got)
	}
}

func TestMiseHandlerGetDependencyKey(t *testing.T) {
	t.Run("uses ID when set", func(t *testing.T) {
		h := NewMiseHandler(parser.Rule{MisePackages: []string{"node@18"}, ID: "node"}, "")
		if got := h.GetDependencyKey(); got != "node" {
			t.Errorf("GetDependencyKey() = %q, want %q", got, "node")
		}
	})
	t.Run("falls back to mise", func(t *testing.T) {
		h := NewMiseHandler(parser.Rule{Action: "mise", MisePackages: []string{"node@18"}}, "")
		if got := h.GetDependencyKey(); got != "mise" {
			t.Errorf("GetDependencyKey() = %q, want %q", got, "mise")
		}
	})
}

func TestMiseHandlerGetDisplayDetails(t *testing.T) {
	h := NewMiseHandler(parser.Rule{MisePackages: []string{"node@18", "python@3.11"}}, "")
	got := h.GetDisplayDetails(false)
	if !strings.Contains(got, "node@18") || !strings.Contains(got, "python@3.11") {
		t.Errorf("GetDisplayDetails() = %q, expected to contain package names", got)
	}
}

func TestMiseUpSkipsAlreadyInstalledVersions(t *testing.T) {
	origVersionInstalled := isMiseVersionInstalled
	defer func() { isMiseVersionInstalled = origVersionInstalled }()
	isMiseVersionInstalled = func(miseBin, tool, version string) bool {
		return tool == "node" && version == "21.4.0"
	}

	origMiseInstalled := miseInstalledCheck
	defer func() { miseInstalledCheck = origMiseInstalled }()
	miseInstalledCheck = func() bool { return true }

	var executedCmds []string
	origExec := executeCommandWithCache
	defer func() { executeCommandWithCache = origExec }()
	executeCommandWithCache = func(cmd string) (string, error) {
		executedCmds = append(executedCmds, cmd)
		return "", nil
	}

	rule := parser.Rule{
		Action:       "mise",
		MisePackages: []string{"node@21.4.0"},
	}
	h := NewMiseHandler(rule, "")
	out, err := h.Up()
	if err != nil {
		t.Fatalf("Up() error: %v", err)
	}
	if !strings.Contains(out, "already installed") {
		t.Errorf("expected 'already installed' message, got %q", out)
	}
	for _, cmd := range executedCmds {
		if strings.Contains(cmd, "mise use") && strings.Contains(cmd, "node@21.4.0") {
			t.Errorf("should not have run mise use for already-installed version, but ran: %q", cmd)
		}
	}
}
