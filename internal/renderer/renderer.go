// Package renderer provides template rendering for blueprint files.
// It is a dependency-free package (no handlers import) so it can be used
// by both the engine package and handler implementations.
package renderer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	gitpkg "github.com/elpic/blueprint/internal/git"
	"github.com/elpic/blueprint/internal/parser"
)

// RenderWithRules renders tmplPath (file or directory) against the given rules.
// cliVars are KEY=VALUE overrides that take precedence over blueprint var rules.
// output is the destination: "" → stdout (single-file mode), a path → file or directory root.
// preferSSH controls whether git operations prefer SSH over HTTPS.
func RenderWithRules(rules []parser.Rule, tmplPath, output string, preferSSH bool, cliVars map[string]string) error {
	localTmpl, tmplRoot, cleanup, err := ResolveTemplatePath(tmplPath, preferSSH)
	if err != nil {
		return err
	}
	defer cleanup()
	tmplPath = localTmpl

	templates, err := collectTemplates(tmplPath)
	if err != nil {
		return err
	}

	if len(templates) == 1 && !isDir(tmplPath) {
		// Single-file mode — respect output / stdout
		result, err := RenderTemplate(templates[0], rules, cliVars)
		if err != nil {
			return err
		}
		writeOutput(result, output)
		return nil
	}

	// Directory mode — validate all templates first, then write.
	// A local override (.tmpl file next to output) shadows the remote template.
	rendered := make(map[string]string, len(templates))
	for _, t := range templates {
		src := t
		if override := LocalOverride(t, tmplRoot, output); override != "" {
			src = override
		}
		result, err := RenderTemplate(src, rules, cliVars)
		if err != nil {
			return fmt.Errorf("%s: %w", t, err)
		}
		rendered[t] = result
	}
	for _, t := range templates {
		out := ResolveOutput(t, tmplRoot, output)
		override := LocalOverride(t, tmplRoot, output)
		if err := os.MkdirAll(filepath.Dir(out), 0o750); err != nil { // #nosec G301 -- output directories must be group-readable
			return fmt.Errorf("cannot create directory for %s: %w", out, err)
		}
		if err := os.WriteFile(out, []byte(rendered[t]), 0o644); err != nil { // #nosec G306 -- rendered template files must be world-readable
			return fmt.Errorf("cannot write %s: %w", out, err)
		}
		if override != "" {
			fmt.Printf("rendered  %s (local override)\n", out)
		} else {
			fmt.Printf("rendered  %s\n", out)
		}
	}
	return nil
}

// RenderTemplate renders a single .tmpl file against rules and cliVars.
// Returns the rendered string or an error.
func RenderTemplate(tmplPath string, rules []parser.Rule, cliVars map[string]string) (string, error) {
	tmplContent, err := os.ReadFile(tmplPath) // #nosec G304 -- user-supplied path, intentional
	if err != nil {
		return "", fmt.Errorf("cannot read template %s: %w", tmplPath, err)
	}

	data := BuildTemplateData(rules, cliVars)

	tmpl, err := template.New(filepath.Base(tmplPath)).
		Option("missingkey=error").
		Funcs(data.FuncMap()).
		Parse(string(tmplContent))
	if err != nil {
		return "", fmt.Errorf("template parse error in %s: %w", tmplPath, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nil); err != nil {
		return "", fmt.Errorf("%s: %w", tmplPath, err)
	}
	return buf.String(), nil
}

// ResolveTemplatePath resolves a template path (local or remote @github: shorthand / git URL)
// to a local filesystem path. Returns (localPath, rootForRelativeCalc, cleanup, error).
func ResolveTemplatePath(tmplPath string, preferSSH bool) (local, root string, cleanup func(), err error) {
	cleanup = func() {}

	var expanded string
	if preferSSH {
		expanded = gitpkg.ExpandShorthandSSH(tmplPath)
	} else {
		expanded = gitpkg.ExpandShorthand(tmplPath)
	}

	if !gitpkg.IsGitURL(expanded) {
		return tmplPath, tmplPath, cleanup, nil
	}

	params := gitpkg.ParseGitURL(expanded)
	localRepo := blueprintRepoPath(expanded)

	if _, _, _, cloneErr := gitpkg.CloneOrUpdateRepository(params.URL, localRepo, params.Branch); cloneErr != nil {
		return "", "", cleanup, fmt.Errorf("error cloning template repository: %w", cloneErr)
	}

	tmplDir := localRepo
	if params.Path != "" && params.Path != "setup.bp" {
		tmplDir = filepath.Join(localRepo, filepath.FromSlash(params.Path))
	}

	if _, statErr := os.Stat(tmplDir); statErr != nil {
		return "", "", cleanup, fmt.Errorf("template path %q not found in repository", params.Path)
	}

	return tmplDir, tmplDir, cleanup, nil
}

// LocalOverride returns the path to a local .tmpl file that overrides the given
// remote template, or "" if no override exists.
func LocalOverride(tmplPath, tmplRoot, outputRoot string) string {
	rel, err := filepath.Rel(tmplRoot, tmplPath)
	if err != nil {
		return ""
	}
	base := outputRoot
	if base == "" {
		base = "."
	}
	candidate := filepath.Join(base, rel)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return ""
}

// ResolveOutput computes the output path for a template in directory mode.
func ResolveOutput(tmplPath, tmplRoot, outputRoot string) string {
	name := strings.TrimSuffix(tmplPath, ".tmpl")
	if outputRoot == "" {
		return name
	}
	rel, err := filepath.Rel(tmplRoot, name)
	if err != nil {
		return name
	}
	return filepath.Join(outputRoot, rel)
}

// collectTemplates returns all *.tmpl files under path.
func collectTemplates(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("cannot access %s: %w", path, err)
	}
	if !info.IsDir() {
		if !strings.HasSuffix(path, ".tmpl") {
			return nil, fmt.Errorf("%s is not a .tmpl file", path)
		}
		return []string{path}, nil
	}

	var templates []string
	err = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		base := filepath.Base(p)
		if strings.HasPrefix(base, "_") {
			return nil // skip partials
		}
		if strings.HasSuffix(base, ".tmpl") {
			templates = append(templates, p)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking %s: %w", path, err)
	}
	if len(templates) == 0 {
		return nil, fmt.Errorf("no *.tmpl files found in %s", path)
	}
	return templates, nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func writeOutput(content, output string) {
	if output == "" {
		fmt.Print(content)
		return
	}
	if err := os.WriteFile(output, []byte(content), 0o644); err != nil { // #nosec G306 -- rendered template files must be world-readable
		fmt.Fprintf(os.Stderr, "error: cannot write %s: %v\n", output, err)
		os.Exit(1)
	}
	fmt.Printf("Written to %s\n", output)
}

// blueprintRepoPath returns the stable local cache path for a blueprint git URL.
func blueprintRepoPath(rawURL string) string {
	homeDir, _ := os.UserHomeDir()
	params := gitpkg.ParseGitURL(rawURL)
	normalized := strings.TrimPrefix(params.URL, "https://")
	normalized = strings.TrimPrefix(normalized, "http://")
	normalized = strings.TrimPrefix(normalized, "git://")
	normalized = strings.TrimSuffix(normalized, ".git")
	return filepath.Join(homeDir, ".blueprint", "repos", normalized)
}

// TemplateData holds all values extractable from a parsed blueprint for use
// in templates and blueprint get queries.
type TemplateData struct {
	rules   []parser.Rule
	cliVars map[string]string
}

// BuildTemplateData constructs a TemplateData from a slice of rules.
func BuildTemplateData(rules []parser.Rule, cliVars map[string]string) *TemplateData {
	if cliVars == nil {
		cliVars = map[string]string{}
	}
	return &TemplateData{rules: rules, cliVars: cliVars}
}

// FuncMap returns the template function map.
func (d *TemplateData) FuncMap() template.FuncMap {
	return template.FuncMap{
		"mise":             d.miseVersion,
		"asdf":             d.asdfVersion,
		"packages":         d.packages,
		"homebrewFormulas": d.homebrewFormulas,
		"homebrewCasks":    d.homebrewCasks,
		"cloneURL":         d.cloneURL,
		"var":              d.varValue,
		"default":          d.varDefault,
		"toArgs":           toArgs,
	}
}

// Get returns the value for a (action, key) query. Used by blueprint get.
func (d *TemplateData) Get(action, key string) (string, error) {
	switch action {
	case "mise":
		return d.miseVersion(key)
	case "asdf":
		return d.asdfVersion(key)
	case "packages":
		parts := strings.SplitN(key, "/", 2)
		pm := parts[0]
		stage := ""
		if len(parts) > 1 {
			stage = parts[1]
		}
		return d.packages(pm, stage), nil
	case "homebrew":
		switch key {
		case "formula", "formulas":
			return d.homebrewFormulas(), nil
		case "cask", "casks":
			return d.homebrewCasks(), nil
		default:
			return "", fmt.Errorf("unknown homebrew key %q: use \"formula\" or \"cask\"", key)
		}
	case "clone":
		return d.cloneURL(key)
	case "var":
		return d.varValue(key)
	case "default":
		parts := strings.SplitN(key, "/", 2)
		fallback := ""
		if len(parts) > 1 {
			fallback = parts[1]
		}
		return d.varDefault(parts[0], fallback), nil
	default:
		return "", fmt.Errorf("unknown action %q: supported actions are mise, asdf, packages, homebrew, clone, var, default", action)
	}
}

func (d *TemplateData) miseVersion(tool string) (string, error) {
	for _, r := range d.rules {
		if r.Action != "mise" {
			continue
		}
		for _, pkg := range r.MisePackages {
			name, version, ok := splitToolVersion(pkg)
			if !ok {
				continue
			}
			if strings.EqualFold(name, tool) {
				return version, nil
			}
		}
	}
	return "", fmt.Errorf("mise tool %q not found in blueprint", tool)
}

func (d *TemplateData) asdfVersion(tool string) (string, error) {
	for _, r := range d.rules {
		if r.Action != "asdf" {
			continue
		}
		for _, pkg := range r.AsdfPackages {
			name, version, ok := splitToolVersion(pkg)
			if !ok {
				continue
			}
			if strings.EqualFold(name, tool) {
				return version, nil
			}
		}
	}
	return "", fmt.Errorf("asdf tool %q not found in blueprint", tool)
}

func (d *TemplateData) packages(filters ...string) string {
	pmFilter := ""
	stageFilter := ""
	if len(filters) > 0 {
		pmFilter = filters[0]
	}
	if len(filters) > 1 {
		stageFilter = filters[1]
	}
	var names []string
	for _, r := range d.rules {
		if r.Action != "install" {
			continue
		}
		for _, pkg := range r.Packages {
			if pmFilter != "" && !strings.EqualFold(pkg.PackageManager, pmFilter) {
				continue
			}
			if stageFilter != "" && !strings.EqualFold(pkg.Stage, stageFilter) {
				continue
			}
			names = append(names, pkg.Name)
		}
	}
	return strings.Join(names, " ")
}

func (d *TemplateData) homebrewFormulas() string {
	var names []string
	for _, r := range d.rules {
		if r.Action != "homebrew" {
			continue
		}
		names = append(names, r.HomebrewPackages...)
	}
	return strings.Join(names, " ")
}

func (d *TemplateData) homebrewCasks() string {
	var names []string
	for _, r := range d.rules {
		if r.Action != "homebrew" {
			continue
		}
		names = append(names, r.HomebrewCasks...)
	}
	return strings.Join(names, " ")
}

func (d *TemplateData) cloneURL(name string) (string, error) {
	for _, r := range d.rules {
		if r.Action != "clone" {
			continue
		}
		if strings.Contains(r.ClonePath, name) || strings.Contains(r.CloneURL, name) {
			return r.CloneURL, nil
		}
	}
	return "", fmt.Errorf("clone rule matching %q not found in blueprint", name)
}

func (d *TemplateData) varDefault(name, fallback string) string {
	if v, ok := d.cliVars[name]; ok {
		return v
	}
	for _, r := range d.rules {
		if r.Action != "var" || r.VarName != name {
			continue
		}
		if !r.VarRequired {
			return r.VarDefault
		}
	}
	return fallback
}

func (d *TemplateData) varValue(name string) (string, error) {
	if v, ok := d.cliVars[name]; ok {
		return v, nil
	}
	for _, r := range d.rules {
		if r.Action != "var" || r.VarName != name {
			continue
		}
		if r.VarRequired {
			return "", fmt.Errorf("variable %q is required but was not set\nhint: pass it with --var %s=<value>", name, name)
		}
		return r.VarDefault, nil
	}
	return "", fmt.Errorf("variable %q is not defined in the blueprint\nhint: add \"var %s <default>\" to your blueprint or pass --var %s=<value>", name, name, name)
}

func toArgs(cmd string) string {
	parts := strings.Fields(cmd)
	b, _ := json.Marshal(parts)
	return string(b)
}

func splitToolVersion(s string) (string, string, bool) {
	idx := strings.Index(s, "@")
	if idx < 0 {
		return "", "", false
	}
	return s[:idx], s[idx+1:], true
}
