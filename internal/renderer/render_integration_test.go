package renderer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

// ---- helpers ----

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func miseRules(tool, version string) []parser.Rule {
	return []parser.Rule{{
		Action:       "mise",
		MisePackages: []string{tool + "@" + version},
	}}
}

func varRules(name, val string) []parser.Rule {
	return []parser.Rule{{
		Action:      "var",
		VarName:     name,
		VarDefault:  val,
		VarRequired: false,
	}}
}

// ---- RenderTemplate ----

func TestRenderTemplate_BasicSubstitution(t *testing.T) {
	dir := t.TempDir()
	tmpl := filepath.Join(dir, "t.tmpl")
	writeFile(t, tmpl, `FROM python:{{ mise "python" }}-slim`)

	rules := miseRules("python", "3.13")
	out, err := RenderTemplate(tmpl, rules, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != "FROM python:3.13-slim" {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestRenderTemplate_CLIVarOverride(t *testing.T) {
	dir := t.TempDir()
	tmpl := filepath.Join(dir, "t.tmpl")
	writeFile(t, tmpl, `APP={{ var "NAME" }}`)

	rules := varRules("NAME", "default-app")
	out, err := RenderTemplate(tmpl, rules, map[string]string{"NAME": "overridden"})
	if err != nil {
		t.Fatal(err)
	}
	if out != "APP=overridden" {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestRenderTemplate_EmptyTemplate(t *testing.T) {
	dir := t.TempDir()
	tmpl := filepath.Join(dir, "empty.tmpl")
	writeFile(t, tmpl, "")

	out, err := RenderTemplate(tmpl, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Errorf("expected empty output, got %q", out)
	}
}

func TestRenderTemplate_FileNotFound(t *testing.T) {
	_, err := RenderTemplate("/nonexistent/path.tmpl", nil, nil)
	if err == nil {
		t.Error("expected error for missing template file")
	}
	if !strings.Contains(err.Error(), "cannot read template") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRenderTemplate_ParseError(t *testing.T) {
	dir := t.TempDir()
	tmpl := filepath.Join(dir, "bad.tmpl")
	writeFile(t, tmpl, `{{ .Unclosed`)

	_, err := RenderTemplate(tmpl, nil, nil)
	if err == nil {
		t.Error("expected template parse error")
	}
	if !strings.Contains(err.Error(), "template parse error") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRenderTemplate_MissingMiseTool(t *testing.T) {
	dir := t.TempDir()
	tmpl := filepath.Join(dir, "t.tmpl")
	writeFile(t, tmpl, `{{ mise "ruby" }}`)

	_, err := RenderTemplate(tmpl, nil, nil)
	if err == nil {
		t.Error("expected error for missing mise tool")
	}
}

func TestRenderTemplate_RequiredVarMissing(t *testing.T) {
	dir := t.TempDir()
	tmpl := filepath.Join(dir, "t.tmpl")
	writeFile(t, tmpl, `{{ var "REQUIRED" }}`)

	rules := []parser.Rule{{
		Action:      "var",
		VarName:     "REQUIRED",
		VarRequired: true,
	}}
	_, err := RenderTemplate(tmpl, rules, nil)
	if err == nil {
		t.Error("expected error for missing required var")
	}
}

// ---- RenderWithRules — single-file mode ----

func TestRenderWithRules_SingleFileToFile(t *testing.T) {
	dir := t.TempDir()
	tmpl := filepath.Join(dir, "Dockerfile.tmpl")
	out := filepath.Join(dir, "Dockerfile")

	writeFile(t, tmpl, `FROM python:{{ mise "python" }}-slim`)

	rules := miseRules("python", "3.13")
	if err := RenderWithRules(rules, tmpl, out, false, nil); err != nil {
		t.Fatal(err)
	}

	got := readFile(t, out)
	if got != "FROM python:3.13-slim" {
		t.Errorf("unexpected output: %q", got)
	}
}

func TestRenderWithRules_SingleFileToStdout(t *testing.T) {
	dir := t.TempDir()
	tmpl := filepath.Join(dir, "t.tmpl")
	writeFile(t, tmpl, `hello`)

	// output="" → stdout (we just ensure no error)
	if err := RenderWithRules(nil, tmpl, "", false, nil); err != nil {
		t.Fatal(err)
	}
}

// ---- RenderWithRules — directory mode ----

func TestRenderWithRules_DirectoryMode(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "out")

	writeFile(t, filepath.Join(dir, "templates", "Dockerfile.tmpl"), `FROM python:{{ mise "python" }}-slim`)
	writeFile(t, filepath.Join(dir, "templates", "app.conf.tmpl"), `port={{ default "PORT" "8000" }}`)

	rules := miseRules("python", "3.12")
	if err := RenderWithRules(rules, filepath.Join(dir, "templates"), outDir, false, nil); err != nil {
		t.Fatal(err)
	}

	if got := readFile(t, filepath.Join(outDir, "Dockerfile")); got != "FROM python:3.12-slim" {
		t.Errorf("Dockerfile: unexpected %q", got)
	}
	if got := readFile(t, filepath.Join(outDir, "app.conf")); got != "port=8000" {
		t.Errorf("app.conf: unexpected %q", got)
	}
}

func TestRenderWithRules_DirectoryMode_PreservesSubdirs(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "out")

	writeFile(t, filepath.Join(dir, "tmpl", "a", "b.tmpl"), `hello`)
	writeFile(t, filepath.Join(dir, "tmpl", "c.tmpl"), `world`)

	if err := RenderWithRules(nil, filepath.Join(dir, "tmpl"), outDir, false, nil); err != nil {
		t.Fatal(err)
	}

	if readFile(t, filepath.Join(outDir, "a", "b")) != "hello" {
		t.Error("subdirectory file not written correctly")
	}
	if readFile(t, filepath.Join(outDir, "c")) != "world" {
		t.Error("top-level file not written correctly")
	}
}

func TestRenderWithRules_DirectoryMode_PartialsSkipped(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "out")

	writeFile(t, filepath.Join(dir, "tmpl", "main.tmpl"), `hello`)
	writeFile(t, filepath.Join(dir, "tmpl", "_partial.tmpl"), `should not render`)

	if err := RenderWithRules(nil, filepath.Join(dir, "tmpl"), outDir, false, nil); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(outDir, "_partial")); !os.IsNotExist(err) {
		t.Error("partial should have been skipped")
	}
	if readFile(t, filepath.Join(outDir, "main")) != "hello" {
		t.Error("main template not rendered")
	}
}

func TestRenderWithRules_DirectoryMode_LocalOverride(t *testing.T) {
	dir := t.TempDir()
	remoteDir := filepath.Join(dir, "remote")
	outDir := filepath.Join(dir, "out")

	// Remote template
	writeFile(t, filepath.Join(remoteDir, "entrypoint.sh.tmpl"), `#!/bin/sh\nexec "$@"`)
	// Local override in output directory
	writeFile(t, filepath.Join(outDir, "entrypoint.sh.tmpl"), `#!/bin/sh\npython manage.py migrate\nexec "$@"`)

	if err := RenderWithRules(nil, remoteDir, outDir, false, nil); err != nil {
		t.Fatal(err)
	}

	got := readFile(t, filepath.Join(outDir, "entrypoint.sh"))
	if !strings.Contains(got, "migrate") {
		t.Errorf("local override not used; got: %q", got)
	}
}

func TestRenderWithRules_DirectoryMode_TemplateError_NothingWritten(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "out")

	writeFile(t, filepath.Join(dir, "tmpl", "good.tmpl"), `ok`)
	writeFile(t, filepath.Join(dir, "tmpl", "bad.tmpl"), `{{ .Unclosed`)

	err := RenderWithRules(nil, filepath.Join(dir, "tmpl"), outDir, false, nil)
	if err == nil {
		t.Fatal("expected error from bad template")
	}

	// Neither file should have been written (validate-all-first)
	if _, err2 := os.Stat(filepath.Join(outDir, "good")); !os.IsNotExist(err2) {
		t.Error("good.tmpl should not have been written when validation failed")
	}
}

func TestRenderWithRules_DirectoryMode_NoTemplates(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "tmpl"), 0o750); err != nil {
		t.Fatal(err)
	}

	err := RenderWithRules(nil, filepath.Join(dir, "tmpl"), dir, false, nil)
	if err == nil {
		t.Error("expected error for empty template directory")
	}
	if !strings.Contains(err.Error(), "no *.tmpl files") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---- LocalOverride ----

func TestLocalOverride_Exists(t *testing.T) {
	dir := t.TempDir()
	remoteDir := filepath.Join(dir, "remote")
	outDir := filepath.Join(dir, "out")

	tmplPath := filepath.Join(remoteDir, "foo.tmpl")
	override := filepath.Join(outDir, "foo.tmpl")
	writeFile(t, override, `override content`)

	result := LocalOverride(tmplPath, remoteDir, outDir)
	if result != override {
		t.Errorf("expected override path %q, got %q", override, result)
	}
}

func TestLocalOverride_NotExists(t *testing.T) {
	dir := t.TempDir()
	remoteDir := filepath.Join(dir, "remote")
	tmplPath := filepath.Join(remoteDir, "foo.tmpl")

	result := LocalOverride(tmplPath, remoteDir, dir)
	if result != "" {
		t.Errorf("expected empty override, got %q", result)
	}
}

func TestLocalOverride_EmptyOutputRoot(t *testing.T) {
	// With empty outputRoot, checks relative to "."
	// We just ensure it doesn't panic and returns empty when no file exists
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "remote", "foo.tmpl")

	result := LocalOverride(tmplPath, filepath.Join(dir, "remote"), "")
	// Result may or may not be empty depending on cwd; just ensure no panic
	_ = result
}

func TestLocalOverride_NestedPath(t *testing.T) {
	dir := t.TempDir()
	remoteDir := filepath.Join(dir, "remote")
	outDir := filepath.Join(dir, "out")

	tmplPath := filepath.Join(remoteDir, "sub", "nested.tmpl")
	override := filepath.Join(outDir, "sub", "nested.tmpl")
	writeFile(t, override, `nested override`)

	result := LocalOverride(tmplPath, remoteDir, outDir)
	if result != override {
		t.Errorf("expected nested override %q, got %q", override, result)
	}
}

// ---- ResolveOutput ----

func TestResolveOutput_WithOutputRoot(t *testing.T) {
	cases := []struct {
		tmpl    string
		root    string
		output  string
		want    string
	}{
		{
			tmpl:   "/remote/Dockerfile.tmpl",
			root:   "/remote",
			output: "/out",
			want:   "/out/Dockerfile",
		},
		{
			tmpl:   "/remote/sub/app.conf.tmpl",
			root:   "/remote",
			output: "/out",
			want:   "/out/sub/app.conf",
		},
	}
	for _, c := range cases {
		got := ResolveOutput(c.tmpl, c.root, c.output)
		if got != c.want {
			t.Errorf("ResolveOutput(%q, %q, %q) = %q, want %q", c.tmpl, c.root, c.output, got, c.want)
		}
	}
}

func TestResolveOutput_NoOutputRoot(t *testing.T) {
	// Without outputRoot, the output is placed next to the template (extension stripped)
	got := ResolveOutput("/path/to/Dockerfile.tmpl", "/path/to", "")
	if got != "/path/to/Dockerfile" {
		t.Errorf("unexpected: %q", got)
	}
}

func TestResolveOutput_StripsTmplExtension(t *testing.T) {
	got := ResolveOutput("/templates/Makefile.tmpl", "/templates", "/out")
	if got != "/out/Makefile" {
		t.Errorf("unexpected: %q", got)
	}
}

// ---- collectTemplates ----

func TestCollectTemplates_SingleFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "Dockerfile.tmpl")
	writeFile(t, f, `content`)

	got, err := collectTemplates(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != f {
		t.Errorf("expected [%q], got %v", f, got)
	}
}

func TestCollectTemplates_NonTmplFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "Dockerfile")
	writeFile(t, f, `content`)

	_, err := collectTemplates(f)
	if err == nil {
		t.Error("expected error for non-.tmpl file")
	}
	if !strings.Contains(err.Error(), "not a .tmpl file") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCollectTemplates_NonExistentPath(t *testing.T) {
	_, err := collectTemplates("/nonexistent/path")
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}

func TestCollectTemplates_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	_, err := collectTemplates(dir)
	if err == nil {
		t.Error("expected error for empty directory")
	}
	if !strings.Contains(err.Error(), "no *.tmpl files") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCollectTemplates_SkipsPartials(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.tmpl"), `main`)
	writeFile(t, filepath.Join(dir, "_partial.tmpl"), `partial`)

	got, err := collectTemplates(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range got {
		if strings.Contains(f, "_partial") {
			t.Errorf("partial should have been excluded, got: %v", got)
		}
	}
	if len(got) != 1 {
		t.Errorf("expected 1 template, got %d: %v", len(got), got)
	}
}

func TestCollectTemplates_RecursiveWalk(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.tmpl"), `a`)
	writeFile(t, filepath.Join(dir, "sub", "b.tmpl"), `b`)
	writeFile(t, filepath.Join(dir, "sub", "deep", "c.tmpl"), `c`)
	writeFile(t, filepath.Join(dir, "not-a-template.txt"), `txt`)

	got, err := collectTemplates(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Errorf("expected 3 templates, got %d: %v", len(got), got)
	}
}

// ---- ResolveTemplatePath ----

func TestResolveTemplatePath_LocalPath(t *testing.T) {
	dir := t.TempDir()
	local, root, cleanup, err := ResolveTemplatePath(dir, false)
	defer cleanup()

	if err != nil {
		t.Fatal(err)
	}
	if local != dir {
		t.Errorf("expected local = %q, got %q", dir, local)
	}
	if root != dir {
		t.Errorf("expected root = %q, got %q", dir, root)
	}
}

func TestResolveTemplatePath_LocalPath_PreferSSH(t *testing.T) {
	dir := t.TempDir()
	local, root, cleanup, err := ResolveTemplatePath(dir, true)
	defer cleanup()

	if err != nil {
		t.Fatal(err)
	}
	if local != dir || root != dir {
		t.Errorf("unexpected: local=%q root=%q", local, root)
	}
}

// ---- Full render pipeline integration ----

func TestFullPipeline_MultipleVarsAndFunctions(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "out")

	writeFile(t, filepath.Join(dir, "tmpl", "Dockerfile.tmpl"),
		`FROM python:{{ mise "python" }}-slim
LABEL app="{{ var "APP_NAME" }}"
EXPOSE {{ default "PORT" "8000" }}
RUN apt-get install -y {{ packages "" "runtime" }}
CMD {{ default "CMD" "python -m app" | toArgs }}
`)

	rules := []parser.Rule{
		{Action: "mise", MisePackages: []string{"python@3.13"}},
		{Action: "var", VarName: "APP_NAME", VarDefault: "myapp", VarRequired: false},
		{Action: "install", Packages: []parser.Package{
			{Name: "libpq5", Stage: "runtime"},
		}},
	}
	if err := RenderWithRules(rules, filepath.Join(dir, "tmpl"), outDir, false, nil); err != nil {
		t.Fatal(err)
	}

	got := readFile(t, filepath.Join(outDir, "Dockerfile"))
	if !strings.Contains(got, "FROM python:3.13-slim") {
		t.Errorf("missing python version: %s", got)
	}
	if !strings.Contains(got, `LABEL app="myapp"`) {
		t.Errorf("missing app label: %s", got)
	}
	if !strings.Contains(got, "EXPOSE 8000") {
		t.Errorf("missing port: %s", got)
	}
	if !strings.Contains(got, "libpq5") {
		t.Errorf("missing packages: %s", got)
	}
	if !strings.Contains(got, `["python","-m","app"]`) {
		t.Errorf("missing CMD exec-form: %s", got)
	}
}

func TestFullPipeline_CLIVarsOverrideBlueprintVars(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "out")

	writeFile(t, filepath.Join(dir, "tmpl", "label.tmpl"), `{{ var "APP" }}`)

	rules := varRules("APP", "blueprint-default")
	if err := RenderWithRules(rules, filepath.Join(dir, "tmpl"), outDir, false,
		map[string]string{"APP": "cli-override"}); err != nil {
		t.Fatal(err)
	}

	got := readFile(t, filepath.Join(outDir, "label"))
	if got != "cli-override" {
		t.Errorf("expected CLI override, got %q", got)
	}
}
