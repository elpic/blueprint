package engine

import (
	"bytes"
	"strings"
	"testing"
	"text/template"

	"github.com/elpic/blueprint/internal/parser"
)

func testRules() []parser.Rule {
	return []parser.Rule{
		{
			Action:       "mise",
			MisePackages: []string{"ruby@3.3.0", "node@20.11.0"},
		},
		{
			Action:       "asdf",
			AsdfPackages: []string{"nodejs@21.4.0", "python@3.11.0"},
		},
		{
			Action: "install",
			Packages: []parser.Package{
				{Name: "git"},
				{Name: "curl"},
				{Name: "code", PackageManager: "snap"},
			},
		},
		{
			Action:           "homebrew",
			HomebrewPackages: []string{"wget", "jq"},
			HomebrewCasks:    []string{"docker", "rectangle"},
		},
		{
			Action:    "clone",
			CloneURL:  "https://github.com/user/myapp",
			ClonePath: "~/projects/myapp",
		},
	}
}

func TestMiseVersion(t *testing.T) {
	d := BuildTemplateData(testRules(), nil)
	v, err := d.Get("mise", "ruby")
	if err != nil {
		t.Fatal(err)
	}
	if v != "3.3.0" {
		t.Errorf("expected 3.3.0, got %q", v)
	}
}

func TestMiseVersion_CaseInsensitive(t *testing.T) {
	d := BuildTemplateData(testRules(), nil)
	v, err := d.Get("mise", "Ruby")
	if err != nil {
		t.Fatal(err)
	}
	if v != "3.3.0" {
		t.Errorf("expected 3.3.0, got %q", v)
	}
}

func TestMiseVersion_NotFound(t *testing.T) {
	d := BuildTemplateData(testRules(), nil)
	_, err := d.Get("mise", "python")
	if err == nil {
		t.Error("expected error for missing mise tool")
	}
}

func TestAsdfVersion(t *testing.T) {
	d := BuildTemplateData(testRules(), nil)
	v, err := d.Get("asdf", "nodejs")
	if err != nil {
		t.Fatal(err)
	}
	if v != "21.4.0" {
		t.Errorf("expected 21.4.0, got %q", v)
	}
}

func TestPackages_All(t *testing.T) {
	d := BuildTemplateData(testRules(), nil)
	v, err := d.Get("packages", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(v, "git") || !strings.Contains(v, "curl") || !strings.Contains(v, "code") {
		t.Errorf("expected all packages, got %q", v)
	}
}

func TestPackages_FilteredBySnap(t *testing.T) {
	d := BuildTemplateData(testRules(), nil)
	// packages filter uses Get("packages", "snap") but packages() is variadic
	// so we call via FuncMap
	fm := d.FuncMap()
	pkgFn := fm["packages"].(func(...string) string)
	v := pkgFn("snap")
	if v != "code" {
		t.Errorf("expected only snap package 'code', got %q", v)
	}
}

func TestHomebrewFormulas(t *testing.T) {
	d := BuildTemplateData(testRules(), nil)
	v, err := d.Get("homebrew", "formula")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(v, "wget") || !strings.Contains(v, "jq") {
		t.Errorf("expected homebrew formulas, got %q", v)
	}
}

func TestHomebrewCasks(t *testing.T) {
	d := BuildTemplateData(testRules(), nil)
	v, err := d.Get("homebrew", "cask")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(v, "docker") || !strings.Contains(v, "rectangle") {
		t.Errorf("expected homebrew casks, got %q", v)
	}
}

func TestCloneURL(t *testing.T) {
	d := BuildTemplateData(testRules(), nil)
	v, err := d.Get("clone", "myapp")
	if err != nil {
		t.Fatal(err)
	}
	if v != "https://github.com/user/myapp" {
		t.Errorf("expected clone URL, got %q", v)
	}
}

func TestCloneURL_NotFound(t *testing.T) {
	d := BuildTemplateData(testRules(), nil)
	_, err := d.Get("clone", "nonexistent")
	if err == nil {
		t.Error("expected error for missing clone rule")
	}
}

func TestUnknownAction(t *testing.T) {
	d := BuildTemplateData(testRules(), nil)
	_, err := d.Get("bogus", "key")
	if err == nil {
		t.Error("expected error for unknown action")
	}
}

func TestFuncMap_TemplateRender(t *testing.T) {
	d := BuildTemplateData(testRules(), nil)
	src := `FROM ruby:{{ mise "ruby" }}-slim
RUN apt-get install -y {{ packages }}`

	tmpl, err := template.New("t").Funcs(d.FuncMap()).Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nil); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "ruby:3.3.0-slim") {
		t.Errorf("expected ruby version in FROM line, got:\n%s", out)
	}
	if !strings.Contains(out, "git") {
		t.Errorf("expected packages in RUN line, got:\n%s", out)
	}
}

func TestFuncMap_MissingKeyFails(t *testing.T) {
	d := BuildTemplateData(testRules(), nil)
	src := `{{ mise "nonexistent" }}`
	tmpl, err := template.New("t").Option("missingkey=error").Funcs(d.FuncMap()).Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, nil)
	if err == nil {
		t.Error("expected error for missing mise tool in template")
	}
}

func TestVar_Default(t *testing.T) {
	rules := append(testRules(), parser.Rule{
		Action:      "var",
		VarName:     "PORT",
		VarDefault:  "8000",
		VarRequired: false,
	})
	d := BuildTemplateData(rules, nil)
	v, err := d.Get("var", "PORT")
	if err != nil {
		t.Fatal(err)
	}
	if v != "8000" {
		t.Errorf("expected default 8000, got %q", v)
	}
}

func TestVar_CLIOverridesDefault(t *testing.T) {
	rules := append(testRules(), parser.Rule{
		Action:      "var",
		VarName:     "PORT",
		VarDefault:  "8000",
		VarRequired: false,
	})
	d := BuildTemplateData(rules, map[string]string{"PORT": "9000"})
	v, err := d.Get("var", "PORT")
	if err != nil {
		t.Fatal(err)
	}
	if v != "9000" {
		t.Errorf("expected CLI override 9000, got %q", v)
	}
}

func TestVar_RequiredWithoutCLI_Fails(t *testing.T) {
	rules := append(testRules(), parser.Rule{
		Action:      "var",
		VarName:     "APP_NAME",
		VarRequired: true,
	})
	d := BuildTemplateData(rules, nil)
	_, err := d.Get("var", "APP_NAME")
	if err == nil {
		t.Error("expected error for required var with no value")
	}
}

func TestVar_RequiredSatisfiedByCLI(t *testing.T) {
	rules := append(testRules(), parser.Rule{
		Action:      "var",
		VarName:     "APP_NAME",
		VarRequired: true,
	})
	d := BuildTemplateData(rules, map[string]string{"APP_NAME": "myapp"})
	v, err := d.Get("var", "APP_NAME")
	if err != nil {
		t.Fatal(err)
	}
	if v != "myapp" {
		t.Errorf("expected myapp, got %q", v)
	}
}

func TestVar_Undefined_Fails(t *testing.T) {
	d := BuildTemplateData(testRules(), nil)
	_, err := d.Get("var", "UNDEFINED")
	if err == nil {
		t.Error("expected error for undefined var")
	}
}

func TestVar_InTemplate(t *testing.T) {
	rules := append(testRules(), parser.Rule{
		Action:      "var",
		VarName:     "APP_NAME",
		VarDefault:  "myapp",
		VarRequired: false,
	})
	d := BuildTemplateData(rules, nil)
	src := `CMD ["uv", "run", "python", "-m", "{{ var "APP_NAME" }}"]`
	tmpl, err := template.New("t").Funcs(d.FuncMap()).Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "myapp") {
		t.Errorf("expected APP_NAME in output, got: %s", buf.String())
	}
}

func testRulesWithStages() []parser.Rule {
	return []parser.Rule{
		{
			Action: "install",
			Packages: []parser.Package{
				{Name: "build-essential", Stage: "build"},
				{Name: "libpq-dev", Stage: "build"},
				{Name: "libpq5", Stage: "runtime"},
				{Name: "ca-certificates", Stage: "runtime"},
				{Name: "git"}, // no stage — returned by packages() with no filter
			},
		},
	}
}

func TestPackages_StageFilter_Build(t *testing.T) {
	d := BuildTemplateData(testRulesWithStages(), nil)
	fm := d.FuncMap()
	pkgFn := fm["packages"].(func(...string) string)
	v := pkgFn("", "build")
	if !strings.Contains(v, "build-essential") || !strings.Contains(v, "libpq-dev") {
		t.Errorf("expected build packages, got %q", v)
	}
	if strings.Contains(v, "libpq5") || strings.Contains(v, "ca-certificates") {
		t.Errorf("should not include runtime packages, got %q", v)
	}
}

func TestPackages_StageFilter_Runtime(t *testing.T) {
	d := BuildTemplateData(testRulesWithStages(), nil)
	fm := d.FuncMap()
	pkgFn := fm["packages"].(func(...string) string)
	v := pkgFn("", "runtime")
	if !strings.Contains(v, "libpq5") || !strings.Contains(v, "ca-certificates") {
		t.Errorf("expected runtime packages, got %q", v)
	}
	if strings.Contains(v, "build-essential") {
		t.Errorf("should not include build packages, got %q", v)
	}
}

func TestPackages_NoStageFilter_ReturnsAll(t *testing.T) {
	d := BuildTemplateData(testRulesWithStages(), nil)
	fm := d.FuncMap()
	pkgFn := fm["packages"].(func(...string) string)
	v := pkgFn()
	if !strings.Contains(v, "build-essential") || !strings.Contains(v, "libpq5") || !strings.Contains(v, "git") {
		t.Errorf("expected all packages, got %q", v)
	}
}

func TestVarDefault_UsesFallback(t *testing.T) {
	d := BuildTemplateData(testRules(), nil)
	v := d.varDefault("REGISTRY", "docker.io")
	if v != "docker.io" {
		t.Errorf("expected fallback docker.io, got %q", v)
	}
}

func TestVarDefault_UsesBlueprintDefault(t *testing.T) {
	rules := append(testRules(), parser.Rule{
		Action:      "var",
		VarName:     "REGISTRY",
		VarDefault:  "ghcr.io",
		VarRequired: false,
	})
	d := BuildTemplateData(rules, nil)
	v := d.varDefault("REGISTRY", "docker.io")
	if v != "ghcr.io" {
		t.Errorf("expected blueprint default ghcr.io, got %q", v)
	}
}

func TestVarDefault_CLIWins(t *testing.T) {
	rules := append(testRules(), parser.Rule{
		Action:      "var",
		VarName:     "REGISTRY",
		VarDefault:  "ghcr.io",
		VarRequired: false,
	})
	d := BuildTemplateData(rules, map[string]string{"REGISTRY": "myorg.azurecr.io"})
	v := d.varDefault("REGISTRY", "docker.io")
	if v != "myorg.azurecr.io" {
		t.Errorf("expected CLI value myorg.azurecr.io, got %q", v)
	}
}

func TestVarDefault_InTemplate(t *testing.T) {
	d := BuildTemplateData(testRules(), nil)
	src := `FROM {{ default "REGISTRY" "docker.io" }}/python:3.12-slim`
	tmpl, err := template.New("t").Funcs(d.FuncMap()).Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nil); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "FROM docker.io/python:3.12-slim" {
		t.Errorf("unexpected output: %s", buf.String())
	}
}

func TestParseVarRule_WithDefault(t *testing.T) {
	// Ensure the parser correctly populates var rules
	rules := []parser.Rule{
		{Action: "var", VarName: "PORT", VarDefault: "8000", VarRequired: false},
	}
	d := BuildTemplateData(rules, nil)
	v, err := d.varValue("PORT")
	if err != nil || v != "8000" {
		t.Errorf("expected 8000, got %q, err=%v", v, err)
	}
}

func TestPrintDiff(t *testing.T) {
	old := "FROM ruby:3.2-slim\nRUN apt-get install git"
	new := "FROM ruby:3.3-slim\nRUN apt-get install git"
	diff := printDiff(old, new, "Dockerfile")
	if !strings.Contains(diff, "-FROM ruby:3.2-slim") {
		t.Error("expected removal line in diff")
	}
	if !strings.Contains(diff, "+FROM ruby:3.3-slim") {
		t.Error("expected addition line in diff")
	}
}

func TestSplitToolVersion(t *testing.T) {
	tests := []struct {
		input   string
		name    string
		version string
		ok      bool
	}{
		{"ruby@3.3.0", "ruby", "3.3.0", true},
		{"node@20.11.0", "node", "20.11.0", true},
		{"nover", "", "", false},
	}
	for _, tt := range tests {
		name, ver, ok := splitToolVersion(tt.input)
		if ok != tt.ok || name != tt.name || ver != tt.version {
			t.Errorf("splitToolVersion(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tt.input, name, ver, ok, tt.name, tt.version, tt.ok)
		}
	}
}
