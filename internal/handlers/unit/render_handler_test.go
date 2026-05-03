package unit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/parser"
)

// buildRenderRule constructs a minimal render rule for testing.
func buildRenderRule(tmpl, output string, vars ...string) parser.Rule {
	return parser.Rule{
		Action:         "render",
		RenderTemplate: tmpl,
		RenderOutput:   output,
		RenderVars:     vars,
	}
}

func TestRenderHandler_Down_NoOp(t *testing.T) {
	h := handlers.NewRenderActionHandler(buildRenderRule("tmpl", "."), "setup.bp")
	msg, err := h.Down()
	if err != nil {
		t.Fatalf("Down() returned error: %v", err)
	}
	if !strings.Contains(msg, "nothing to undo") {
		t.Errorf("unexpected Down message: %q", msg)
	}
}

func TestRenderHandler_IsInstalled_AlwaysFalse(t *testing.T) {
	h := handlers.NewRenderActionHandler(buildRenderRule("tmpl", "."), "setup.bp")
	if h.IsInstalled(nil, "setup.bp", "mac") {
		t.Error("IsInstalled should always return false")
	}
}

func TestRenderHandler_UpdateStatus_NoOp(t *testing.T) {
	h := handlers.NewRenderActionHandler(buildRenderRule("tmpl", "."), "setup.bp")
	err := h.UpdateStatus(nil, nil, "setup.bp", "mac")
	if err != nil {
		t.Errorf("UpdateStatus returned error: %v", err)
	}
}

func TestRenderHandler_FindUninstallRules_ReturnsNil(t *testing.T) {
	h := handlers.NewRenderActionHandler(buildRenderRule("tmpl", "."), "setup.bp")
	rules := h.FindUninstallRules(nil, nil, "setup.bp", "mac")
	if rules != nil {
		t.Errorf("expected nil, got %v", rules)
	}
}

func TestRenderHandler_GetDependencyKey(t *testing.T) {
	h := handlers.NewRenderActionHandler(buildRenderRule("./templates", "."), "setup.bp")
	key := h.GetDependencyKey()
	if !strings.Contains(key, "templates") {
		t.Errorf("dependency key should contain template path, got %q", key)
	}
}

func TestRenderHandler_GetDisplayDetails(t *testing.T) {
	h := handlers.NewRenderActionHandler(buildRenderRule("./templates", "."), "setup.bp")
	got := h.GetDisplayDetails(false)
	if got != "./templates" {
		t.Errorf("expected ./templates, got %q", got)
	}
	// Same regardless of isUninstall
	got2 := h.GetDisplayDetails(true)
	if got2 != "./templates" {
		t.Errorf("expected ./templates, got %q", got2)
	}
}

func TestRenderHandler_GetState(t *testing.T) {
	h := handlers.NewRenderActionHandler(buildRenderRule("./tmpl", "/out"), "setup.bp")
	state := h.GetState(false)
	if state["template"] != "./tmpl" {
		t.Errorf("state[template] = %q", state["template"])
	}
	if state["output"] != "/out" {
		t.Errorf("state[output] = %q", state["output"])
	}
	if state["summary"] != "./tmpl" {
		t.Errorf("state[summary] = %q", state["summary"])
	}
}

func TestRenderHandler_GetCommand_Basic(t *testing.T) {
	h := handlers.NewRenderActionHandler(buildRenderRule("./tmpl", "."), "setup.bp")
	cmd := h.GetCommand()
	if !strings.Contains(cmd, "--template") || !strings.Contains(cmd, "./tmpl") {
		t.Errorf("GetCommand missing template flag: %q", cmd)
	}
	// output "." is the default — should not appear
	if strings.Contains(cmd, "--output") {
		t.Errorf("GetCommand should not include --output for '.': %q", cmd)
	}
}

func TestRenderHandler_GetCommand_WithOutputDir(t *testing.T) {
	h := handlers.NewRenderActionHandler(buildRenderRule("./tmpl", "/my/output"), "setup.bp")
	cmd := h.GetCommand()
	if !strings.Contains(cmd, "--output") || !strings.Contains(cmd, "/my/output") {
		t.Errorf("GetCommand should include --output for non-default output: %q", cmd)
	}
}

func TestRenderHandler_GetCommand_WithVars(t *testing.T) {
	h := handlers.NewRenderActionHandler(
		buildRenderRule("./tmpl", ".", "APP_NAME=myapp", "PORT=9000"), "setup.bp")
	cmd := h.GetCommand()
	if !strings.Contains(cmd, "--var APP_NAME=myapp") {
		t.Errorf("GetCommand missing first var: %q", cmd)
	}
	if !strings.Contains(cmd, "--var PORT=9000") {
		t.Errorf("GetCommand missing second var: %q", cmd)
	}
}

func TestRenderHandler_GetCommand_EmptyOutput(t *testing.T) {
	// Output="" should also not append --output (treated like ".")
	h := handlers.NewRenderActionHandler(buildRenderRule("tmpl", ""), "setup.bp")
	cmd := h.GetCommand()
	if strings.Contains(cmd, "--output") {
		t.Errorf("GetCommand should not include --output for empty string: %q", cmd)
	}
}

// TestRenderHandler_Up_MissingBlueprint verifies Up() returns an error when the
// blueprint file doesn't exist.
func TestRenderHandler_Up_MissingBlueprint(t *testing.T) {
	h := handlers.NewRenderActionHandler(
		buildRenderRule("./tmpl", "."), "/nonexistent/setup.bp")
	_, err := h.Up()
	if err == nil {
		t.Error("expected error when blueprint file is missing")
	}
}

// TestRenderHandler_Up_Success renders a single template from a real blueprint file.
func TestRenderHandler_Up_Success(t *testing.T) {
	dir := t.TempDir()

	// Write a minimal blueprint file
	bpContent := "mise python@3.13\nvar APP_NAME myapp\n"
	bpFile := filepath.Join(dir, "setup.bp")
	if err := os.WriteFile(bpFile, []byte(bpContent), 0o600); err != nil {
		t.Fatalf("write blueprint: %v", err)
	}

	// Write a template directory
	tmplDir := filepath.Join(dir, "templates")
	if err := os.MkdirAll(tmplDir, 0o750); err != nil {
		t.Fatal(err)
	}
	tmplFile := filepath.Join(tmplDir, "label.tmpl")
	if err := os.WriteFile(tmplFile, []byte(`{{ mise "python" }}-{{ var "APP_NAME" }}`), 0o600); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(dir, "out")
	rule := buildRenderRule(tmplDir, outDir)
	h := handlers.NewRenderActionHandler(rule, bpFile)

	msg, err := h.Up()
	if err != nil {
		t.Fatalf("Up() failed: %v", err)
	}
	if !strings.Contains(msg, "rendered templates") {
		t.Errorf("unexpected success message: %q", msg)
	}

	// Verify the file was actually written
	got, err := os.ReadFile(filepath.Join(outDir, "label"))
	if err != nil {
		t.Fatalf("output file not written: %v", err)
	}
	if string(got) != "3.13-myapp" {
		t.Errorf("unexpected output: %q", string(got))
	}
}

// TestRenderHandler_Up_VarsOverride verifies --var overrides blueprint defaults.
func TestRenderHandler_Up_VarsOverride(t *testing.T) {
	dir := t.TempDir()

	bpFile := filepath.Join(dir, "setup.bp")
	if err := os.WriteFile(bpFile, []byte("var APP blueprint-default\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	tmplDir := filepath.Join(dir, "templates")
	if err := os.MkdirAll(tmplDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmplDir, "t.tmpl"), []byte(`{{ var "APP" }}`), 0o600); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(dir, "out")
	rule := buildRenderRule(tmplDir, outDir, "APP=cli-override")
	h := handlers.NewRenderActionHandler(rule, bpFile)

	if _, err := h.Up(); err != nil {
		t.Fatalf("Up() failed: %v", err)
	}

	got, _ := os.ReadFile(filepath.Join(outDir, "t"))
	if string(got) != "cli-override" {
		t.Errorf("expected CLI override, got %q", string(got))
	}
}

// TestParseVarsSlice verifies KEY=VALUE parsing for render vars.
func TestParseVarsSlice(t *testing.T) {
	// We exercise parseVarsSlice indirectly through the handler's Up() method
	// by checking that vars with "=" in the value are handled correctly.
	dir := t.TempDir()

	bpFile := filepath.Join(dir, "setup.bp")
	if err := os.WriteFile(bpFile, []byte("var X default\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	tmplDir := filepath.Join(dir, "templates")
	if err := os.MkdirAll(tmplDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmplDir, "t.tmpl"), []byte(`{{ var "X" }}`), 0o600); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(dir, "out")
	// Value contains "=" — should split on first "=" only
	rule := buildRenderRule(tmplDir, outDir, "X=val=with=equals")
	h := handlers.NewRenderActionHandler(rule, bpFile)

	if _, err := h.Up(); err != nil {
		t.Fatalf("Up() failed: %v", err)
	}

	got, _ := os.ReadFile(filepath.Join(outDir, "t"))
	if string(got) != "val=with=equals" {
		t.Errorf("expected 'val=with=equals', got %q", string(got))
	}
}

// TestRenderActionDef verifies the ActionDef is correctly registered.
func TestRenderActionDef(t *testing.T) {
	def := handlers.GetAction("render")
	if def == nil {
		t.Fatal("render action not registered")
	}
	if !def.AlwaysRunUp {
		t.Error("render action should have AlwaysRunUp=true")
	}

	rule := parser.Rule{
		Action:         "render",
		RenderTemplate: "@github:org/templates@main:python",
		RenderOutput:   "/custom/output",
	}

	summary := def.Summary(rule)
	if !strings.Contains(summary, "@github:org/templates") {
		t.Errorf("summary missing template: %q", summary)
	}
	if !strings.Contains(summary, "/custom/output") {
		t.Errorf("summary missing output: %q", summary)
	}

	if !def.Detect(rule) {
		t.Error("Detect should return true when RenderTemplate is set")
	}
	if def.Detect(parser.Rule{}) {
		t.Error("Detect should return false when RenderTemplate is empty")
	}
	if def.RuleKey(rule) != rule.RenderTemplate {
		t.Errorf("RuleKey should return RenderTemplate")
	}
}

// TestRenderActionDef_SummaryDefaultOutput verifies summary omits output when it's ".".
func TestRenderActionDef_SummaryDefaultOutput(t *testing.T) {
	def := handlers.GetAction("render")
	rule := parser.Rule{
		Action:         "render",
		RenderTemplate: "./templates",
		RenderOutput:   ".",
	}
	summary := def.Summary(rule)
	if strings.Contains(summary, "→") {
		t.Errorf("summary should not include arrow for default output '.': %q", summary)
	}
}
