package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

// ---- helpers ----

func writeTmpl(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readOut(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func miseRule(tool, version string) parser.Rule {
	return parser.Rule{Action: "mise", MisePackages: []string{tool + "@" + version}}
}

func varRule(name, val string) parser.Rule {
	return parser.Rule{Action: "var", VarName: name, VarDefault: val, VarRequired: false}
}

// ---- mustRenderTemplate ----

func TestMustRenderTemplate_BasicSubstitution(t *testing.T) {
	dir := t.TempDir()
	tmpl := filepath.Join(dir, "t.tmpl")
	writeTmpl(t, tmpl, `FROM python:{{ mise "python" }}-slim`)

	rules := []parser.Rule{miseRule("python", "3.13")}
	out := mustRenderTemplate(tmpl, rules, nil)
	if out != "FROM python:3.13-slim" {
		t.Errorf("unexpected: %q", out)
	}
}

func TestMustRenderTemplate_CLIVarOverride(t *testing.T) {
	dir := t.TempDir()
	tmpl := filepath.Join(dir, "t.tmpl")
	writeTmpl(t, tmpl, `APP={{ var "NAME" }}`)

	rules := []parser.Rule{varRule("NAME", "blueprint-default")}
	out := mustRenderTemplate(tmpl, rules, map[string]string{"NAME": "overridden"})
	if out != "APP=overridden" {
		t.Errorf("unexpected: %q", out)
	}
}

func TestMustRenderTemplate_EmptyTemplate(t *testing.T) {
	dir := t.TempDir()
	tmpl := filepath.Join(dir, "empty.tmpl")
	writeTmpl(t, tmpl, "")

	out := mustRenderTemplate(tmpl, nil, nil)
	if out != "" {
		t.Errorf("expected empty output, got %q", out)
	}
}

func TestMustRenderTemplate_ToArgs(t *testing.T) {
	dir := t.TempDir()
	tmpl := filepath.Join(dir, "t.tmpl")
	writeTmpl(t, tmpl, `CMD {{ default "CMD" "python -m app" | toArgs }}`)

	out := mustRenderTemplate(tmpl, nil, nil)
	if !strings.Contains(out, `["python","-m","app"]`) {
		t.Errorf("unexpected toArgs output: %q", out)
	}
}

func TestMustRenderTemplate_AllFunctions(t *testing.T) {
	dir := t.TempDir()
	tmpl := filepath.Join(dir, "t.tmpl")
	writeTmpl(t, tmpl,
		"{{ mise \"python\" }}\n"+
			"{{ packages \"\" \"runtime\" }}\n"+
			"{{ default \"PORT\" \"8080\" }}\n"+
			"{{ var \"APP\" }}\n")

	rules := []parser.Rule{
		miseRule("python", "3.13"),
		{Action: "install", Packages: []parser.Package{{Name: "libpq5", Stage: "runtime"}}},
		varRule("APP", "myapp"),
	}
	out := mustRenderTemplate(tmpl, rules, nil)
	if !strings.Contains(out, "3.13") {
		t.Errorf("missing mise version: %q", out)
	}
	if !strings.Contains(out, "libpq5") {
		t.Errorf("missing package: %q", out)
	}
	if !strings.Contains(out, "8080") {
		t.Errorf("missing default port: %q", out)
	}
	if !strings.Contains(out, "myapp") {
		t.Errorf("missing var: %q", out)
	}
}

// ---- localOverride ----

func TestLocalOverride_Exists(t *testing.T) {
	dir := t.TempDir()
	remoteDir := filepath.Join(dir, "remote")
	outDir := filepath.Join(dir, "out")

	tmplPath := filepath.Join(remoteDir, "foo.tmpl")
	override := filepath.Join(outDir, "foo.tmpl")
	writeTmpl(t, override, `override`)

	result := localOverride(tmplPath, remoteDir, outDir)
	if result != override {
		t.Errorf("expected %q, got %q", override, result)
	}
}

func TestLocalOverride_NotExists(t *testing.T) {
	dir := t.TempDir()
	remoteDir := filepath.Join(dir, "remote")
	tmplPath := filepath.Join(remoteDir, "foo.tmpl")

	result := localOverride(tmplPath, remoteDir, dir)
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestLocalOverride_NestedPath(t *testing.T) {
	dir := t.TempDir()
	remoteDir := filepath.Join(dir, "remote")
	outDir := filepath.Join(dir, "out")

	tmplPath := filepath.Join(remoteDir, "sub", "nested.tmpl")
	override := filepath.Join(outDir, "sub", "nested.tmpl")
	writeTmpl(t, override, `nested override`)

	result := localOverride(tmplPath, remoteDir, outDir)
	if result != override {
		t.Errorf("expected %q, got %q", override, result)
	}
}

func TestLocalOverride_EmptyOutputRoot_DefaultsDot(t *testing.T) {
	dir := t.TempDir()
	remoteDir := filepath.Join(dir, "remote")
	// tmplPath not under remoteDir via rel — just ensure no panic
	tmplPath := filepath.Join(remoteDir, "foo.tmpl")
	result := localOverride(tmplPath, remoteDir, "")
	// Result depends on cwd; just ensure it doesn't panic
	_ = result
}

// ---- resolveOutput ----

func TestResolveOutput_WithOutputRoot(t *testing.T) {
	cases := []struct {
		tmpl, root, output, want string
	}{
		{"/remote/Dockerfile.tmpl", "/remote", "/out", "/out/Dockerfile"},
		{"/remote/sub/app.conf.tmpl", "/remote", "/out", "/out/sub/app.conf"},
	}
	for _, c := range cases {
		got := resolveOutput(c.tmpl, c.root, c.output)
		if got != c.want {
			t.Errorf("resolveOutput(%q, %q, %q) = %q, want %q",
				c.tmpl, c.root, c.output, got, c.want)
		}
	}
}

func TestResolveOutput_NoOutputRoot(t *testing.T) {
	got := resolveOutput("/path/to/Dockerfile.tmpl", "/path/to", "")
	if got != "/path/to/Dockerfile" {
		t.Errorf("unexpected: %q", got)
	}
}

func TestResolveOutput_StripsTmplExtension(t *testing.T) {
	got := resolveOutput("/templates/Makefile.tmpl", "/templates", "/out")
	if got != "/out/Makefile" {
		t.Errorf("unexpected: %q", got)
	}
}

// ---- collectTemplates ----

func TestCollectTemplates_SingleFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "Dockerfile.tmpl")
	writeTmpl(t, f, `content`)

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
	writeTmpl(t, f, `content`)

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
	writeTmpl(t, filepath.Join(dir, "main.tmpl"), `main`)
	writeTmpl(t, filepath.Join(dir, "_partial.tmpl"), `partial`)

	got, err := collectTemplates(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range got {
		if strings.Contains(f, "_partial") {
			t.Errorf("partial should be excluded, got: %v", got)
		}
	}
	if len(got) != 1 {
		t.Errorf("expected 1 template, got %d: %v", len(got), got)
	}
}

func TestCollectTemplates_RecursiveWalk(t *testing.T) {
	dir := t.TempDir()
	writeTmpl(t, filepath.Join(dir, "a.tmpl"), `a`)
	writeTmpl(t, filepath.Join(dir, "sub", "b.tmpl"), `b`)
	writeTmpl(t, filepath.Join(dir, "sub", "deep", "c.tmpl"), `c`)
	writeTmpl(t, filepath.Join(dir, "not-a-template.txt"), `txt`)

	got, err := collectTemplates(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Errorf("expected 3 templates, got %d: %v", len(got), got)
	}
}

// ---- resolveTemplatePath ----

func TestResolveTemplatePath_LocalPath(t *testing.T) {
	dir := t.TempDir()
	local, root, cleanup, err := resolveTemplatePath(dir, false)
	defer cleanup()

	if err != nil {
		t.Fatal(err)
	}
	if local != dir || root != dir {
		t.Errorf("expected local=root=%q, got local=%q root=%q", dir, local, root)
	}
}

func TestResolveTemplatePath_LocalPath_PreferSSH(t *testing.T) {
	dir := t.TempDir()
	local, root, cleanup, err := resolveTemplatePath(dir, true)
	defer cleanup()

	if err != nil {
		t.Fatal(err)
	}
	if local != dir || root != dir {
		t.Errorf("unexpected: local=%q root=%q", local, root)
	}
}

// ---- toArgs ----

func TestToArgs_Basic(t *testing.T) {
	if got := toArgs("python -m myapp"); got != `["python","-m","myapp"]` {
		t.Errorf("unexpected: %q", got)
	}
}

func TestToArgs_SingleWord(t *testing.T) {
	if got := toArgs("python"); got != `["python"]` {
		t.Errorf("unexpected: %q", got)
	}
}

func TestToArgs_EmptyString(t *testing.T) {
	if got := toArgs(""); got != `[]` {
		t.Errorf("unexpected: %q", got)
	}
}

func TestToArgs_ExtraWhitespace(t *testing.T) {
	if got := toArgs("  python   -m   app  "); got != `["python","-m","app"]` {
		t.Errorf("unexpected: %q", got)
	}
}

// ---- printDiff edge cases ----

func TestPrintDiff_Identical(t *testing.T) {
	diff := printDiff("FROM python:3.13", "FROM python:3.13", "Dockerfile")
	for _, l := range strings.Split(diff, "\n")[2:] {
		if strings.HasPrefix(l, "-") || strings.HasPrefix(l, "+") {
			t.Errorf("unexpected diff line for identical content: %q", l)
		}
	}
}

func TestPrintDiff_OnlyAdditions(t *testing.T) {
	diff := printDiff("line1", "line1\nline2\nline3", "f")
	if !strings.Contains(diff, "+line2") || !strings.Contains(diff, "+line3") {
		t.Error("expected added lines")
	}
}

func TestPrintDiff_OnlyDeletions(t *testing.T) {
	diff := printDiff("line1\nline2\nline3", "line1", "f")
	if !strings.Contains(diff, "-line2") || !strings.Contains(diff, "-line3") {
		t.Error("expected removed lines")
	}
}

func TestPrintDiff_EmptyStrings(t *testing.T) {
	diff := printDiff("", "", "f")
	if !strings.Contains(diff, "--- f") {
		t.Error("expected header in diff")
	}
}

func TestPrintDiff_LabelAndMarkers(t *testing.T) {
	diff := printDiff("a", "b", "myfile.txt")
	if !strings.Contains(diff, "myfile.txt") {
		t.Error("label not in diff header")
	}
	if !strings.Contains(diff, "(existing)") || !strings.Contains(diff, "(rendered)") {
		t.Error("expected (existing)/(rendered) markers")
	}
}

// ---- Render directory mode (integration via Render's internal helpers) ----

// TestDirectoryRender_MultipleTemplates exercises the core directory render path
// by calling the internal helpers directly (same package).
func TestDirectoryRender_MultipleTemplates(t *testing.T) {
	dir := t.TempDir()
	tmplDir := filepath.Join(dir, "templates")
	outDir := filepath.Join(dir, "out")

	writeTmpl(t, filepath.Join(tmplDir, "Dockerfile.tmpl"), `FROM python:{{ mise "python" }}-slim`)
	writeTmpl(t, filepath.Join(tmplDir, "app.conf.tmpl"), `port={{ default "PORT" "8000" }}`)

	rules := []parser.Rule{miseRule("python", "3.12")}

	templates, err := collectTemplates(tmplDir)
	if err != nil {
		t.Fatal(err)
	}

	rendered := make(map[string]string, len(templates))
	for _, tmpl := range templates {
		rendered[tmpl] = mustRenderTemplate(tmpl, rules, nil)
	}
	for _, tmpl := range templates {
		out := resolveOutput(tmpl, tmplDir, outDir)
		if err := os.MkdirAll(filepath.Dir(out), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(out, []byte(rendered[tmpl]), 0o644); err != nil { //nolint:gosec
			t.Fatal(err)
		}
	}

	if got := readOut(t, filepath.Join(outDir, "Dockerfile")); got != "FROM python:3.12-slim" {
		t.Errorf("Dockerfile: %q", got)
	}
	if got := readOut(t, filepath.Join(outDir, "app.conf")); got != "port=8000" {
		t.Errorf("app.conf: %q", got)
	}
}

func TestDirectoryRender_LocalOverrideApplied(t *testing.T) {
	dir := t.TempDir()
	remoteDir := filepath.Join(dir, "remote")
	outDir := filepath.Join(dir, "out")

	writeTmpl(t, filepath.Join(remoteDir, "entrypoint.sh.tmpl"), `#!/bin/sh\nexec "$@"`)
	// Local override — service-specific
	writeTmpl(t, filepath.Join(outDir, "entrypoint.sh.tmpl"), `#!/bin/sh\npython manage.py migrate\nexec "$@"`)

	templates, err := collectTemplates(remoteDir)
	if err != nil {
		t.Fatal(err)
	}

	rendered := make(map[string]string, len(templates))
	for _, tmpl := range templates {
		src := tmpl
		if override := localOverride(tmpl, remoteDir, outDir); override != "" {
			src = override
		}
		rendered[tmpl] = mustRenderTemplate(src, nil, nil)
	}
	for _, tmpl := range templates {
		out := resolveOutput(tmpl, remoteDir, outDir)
		if err := os.MkdirAll(filepath.Dir(out), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(out, []byte(rendered[tmpl]), 0o644); err != nil { //nolint:gosec
			t.Fatal(err)
		}
	}

	got := readOut(t, filepath.Join(outDir, "entrypoint.sh"))
	if !strings.Contains(got, "migrate") {
		t.Errorf("local override not used; got: %q", got)
	}
}

func TestDirectoryRender_SubdirectoryStructurePreserved(t *testing.T) {
	dir := t.TempDir()
	tmplDir := filepath.Join(dir, "tmpl")
	outDir := filepath.Join(dir, "out")

	writeTmpl(t, filepath.Join(tmplDir, "a", "b.tmpl"), `hello`)
	writeTmpl(t, filepath.Join(tmplDir, "c.tmpl"), `world`)

	templates, err := collectTemplates(tmplDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, tmpl := range templates {
		out := resolveOutput(tmpl, tmplDir, outDir)
		if err := os.MkdirAll(filepath.Dir(out), 0o750); err != nil {
			t.Fatal(err)
		}
		content := mustRenderTemplate(tmpl, nil, nil)
		if err := os.WriteFile(out, []byte(content), 0o644); err != nil { //nolint:gosec
			t.Fatal(err)
		}
	}

	if readOut(t, filepath.Join(outDir, "a", "b")) != "hello" {
		t.Error("subdirectory file not written correctly")
	}
	if readOut(t, filepath.Join(outDir, "c")) != "world" {
		t.Error("top-level file not written correctly")
	}
}

// ---- full pipeline integration ----

func TestFullRenderPipeline_MultipleVarsAndFunctions(t *testing.T) {
	dir := t.TempDir()
	tmplDir := filepath.Join(dir, "tmpl")
	outDir := filepath.Join(dir, "out")

	writeTmpl(t, filepath.Join(tmplDir, "Dockerfile.tmpl"),
		"FROM python:{{ mise \"python\" }}-slim\n"+
			"LABEL app=\"{{ var \"APP_NAME\" }}\"\n"+
			"EXPOSE {{ default \"PORT\" \"8000\" }}\n"+
			"RUN apt-get install -y {{ packages \"\" \"runtime\" }}\n"+
			"CMD {{ default \"CMD\" \"python -m app\" | toArgs }}\n")

	rules := []parser.Rule{
		miseRule("python", "3.13"),
		varRule("APP_NAME", "myapp"),
		{Action: "install", Packages: []parser.Package{{Name: "libpq5", Stage: "runtime"}}},
	}

	templates, _ := collectTemplates(tmplDir)
	rendered := make(map[string]string, len(templates))
	for _, tmpl := range templates {
		rendered[tmpl] = mustRenderTemplate(tmpl, rules, nil)
	}
	for _, tmpl := range templates {
		out := resolveOutput(tmpl, tmplDir, outDir)
		os.MkdirAll(filepath.Dir(out), 0o750) //nolint:errcheck
		os.WriteFile(out, []byte(rendered[tmpl]), 0o644) //nolint:errcheck
	}

	got := readOut(t, filepath.Join(outDir, "Dockerfile"))
	for _, want := range []string{
		"FROM python:3.13-slim",
		`LABEL app="myapp"`,
		"EXPOSE 8000",
		"libpq5",
		`["python","-m","app"]`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
}
