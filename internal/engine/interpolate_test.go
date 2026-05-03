package engine

import (
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

func TestExpandVars_Basic(t *testing.T) {
	vars := map[string]string{"WORKSPACE": "/home/user/workspace"}
	got := expandVars("${WORKSPACE}/myapp", vars)
	if got != "/home/user/workspace/myapp" {
		t.Errorf("expected /home/user/workspace/myapp, got %q", got)
	}
}

func TestExpandVars_NoMarker(t *testing.T) {
	vars := map[string]string{"WORKSPACE": "/home/user/workspace"}
	got := expandVars("~/projects/myapp", vars)
	if got != "~/projects/myapp" {
		t.Errorf("expected unchanged, got %q", got)
	}
}

func TestExpandVars_Multiple(t *testing.T) {
	vars := map[string]string{"ORG": "avant", "REPO": "cas"}
	got := expandVars("~/workspace/${ORG}/${REPO}", vars)
	if got != "~/workspace/avant/cas" {
		t.Errorf("expected ~/workspace/avant/cas, got %q", got)
	}
}

func TestExpandVars_Unknown(t *testing.T) {
	vars := map[string]string{"FOO": "bar"}
	got := expandVars("${UNKNOWN}/path", vars)
	// Unknown vars are left as-is
	if got != "${UNKNOWN}/path" {
		t.Errorf("expected ${UNKNOWN}/path unchanged, got %q", got)
	}
}

func TestResolveVarMap_BlueprintDefault(t *testing.T) {
	rules := []parser.Rule{
		{Action: "var", VarName: "WORKSPACE", VarDefault: "~/workspace/avant"},
	}
	vars := resolveVarMap(rules, nil)
	if vars["WORKSPACE"] != "~/workspace/avant" {
		t.Errorf("expected ~/workspace/avant, got %q", vars["WORKSPACE"])
	}
}

func TestResolveVarMap_CLIOverrides(t *testing.T) {
	rules := []parser.Rule{
		{Action: "var", VarName: "WORKSPACE", VarDefault: "~/workspace/avant"},
	}
	vars := resolveVarMap(rules, map[string]string{"WORKSPACE": "/custom/path"})
	if vars["WORKSPACE"] != "/custom/path" {
		t.Errorf("expected /custom/path, got %q", vars["WORKSPACE"])
	}
}

func TestResolveVarMap_CLIOnlyVar(t *testing.T) {
	// CLI var not declared in blueprint still resolves
	vars := resolveVarMap(nil, map[string]string{"EXTRA": "value"})
	if vars["EXTRA"] != "value" {
		t.Errorf("expected value, got %q", vars["EXTRA"])
	}
}

func TestInterpolateRule_ClonePath(t *testing.T) {
	vars := map[string]string{"WORKSPACE": "~/workspace/avant"}
	rule := parser.Rule{
		Action:    "clone",
		CloneURL:  "git@github.com:org/repo.git",
		ClonePath: "${WORKSPACE}/repo",
	}
	got := interpolateRule(rule, vars)
	if got.ClonePath != "~/workspace/avant/repo" {
		t.Errorf("expected ~/workspace/avant/repo, got %q", got.ClonePath)
	}
	// CloneURL without marker is unchanged
	if got.CloneURL != "git@github.com:org/repo.git" {
		t.Errorf("CloneURL should be unchanged, got %q", got.CloneURL)
	}
}

func TestInterpolateRule_MisePath(t *testing.T) {
	vars := map[string]string{"WORKSPACE": "~/workspace/avant"}
	rule := parser.Rule{
		Action:   "mise",
		MisePath: "${WORKSPACE}/card-account-service",
	}
	got := interpolateRule(rule, vars)
	if got.MisePath != "~/workspace/avant/card-account-service" {
		t.Errorf("expected ~/workspace/avant/card-account-service, got %q", got.MisePath)
	}
}

func TestInterpolateRule_RenderOutput(t *testing.T) {
	vars := map[string]string{"WORKSPACE": "~/workspace/avant"}
	rule := parser.Rule{
		Action:         "render",
		RenderTemplate: "@github:elpic/templates@main:containers/python",
		RenderOutput:   "${WORKSPACE}/card-account-service",
	}
	got := interpolateRule(rule, vars)
	if got.RenderOutput != "~/workspace/avant/card-account-service" {
		t.Errorf("expected ~/workspace/avant/card-account-service, got %q", got.RenderOutput)
	}
}

func TestInterpolateRule_NoVars(t *testing.T) {
	rule := parser.Rule{
		Action:    "clone",
		ClonePath: "~/projects/myapp",
	}
	got := interpolateRule(rule, nil)
	if got.ClonePath != "~/projects/myapp" {
		t.Errorf("expected unchanged, got %q", got.ClonePath)
	}
}

func TestInterpolateRule_RunCommand(t *testing.T) {
	vars := map[string]string{"WORKSPACE": "~/workspace/avant"}
	rule := parser.Rule{
		Action:     "run",
		RunCommand: "cd ${WORKSPACE}/myapp && make setup",
	}
	got := interpolateRule(rule, vars)
	if got.RunCommand != "cd ~/workspace/avant/myapp && make setup" {
		t.Errorf("unexpected RunCommand: %q", got.RunCommand)
	}
}
