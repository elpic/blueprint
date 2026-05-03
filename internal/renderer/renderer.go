// Package renderer provides template rendering for blueprint files.
// It is a dependency-free package (no handlers import) so it can be used
// by both the engine package and handler implementations.
package renderer

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"text/template"

	gitpkg "github.com/elpic/blueprint/internal/git"
	"github.com/elpic/blueprint/internal/parser"
)

// TemplateFuncEntry pairs a template function name with a factory that binds
// the function to a specific *TemplateData instance.
type TemplateFuncEntry struct {
	Name    string
	Factory func(d *TemplateData) interface{}
}

var (
	templateFuncMu        sync.RWMutex
	templateFuncProviders []TemplateFuncEntry

	getHandlerMu sync.RWMutex
	getHandlers  = map[string]func(d *TemplateData, key string) (string, error){}
)

// RegisterTemplateFuncs appends entries to the global template-function registry.
// Panics on duplicate names to catch wiring bugs at startup.
func RegisterTemplateFuncs(entries []TemplateFuncEntry) {
	templateFuncMu.Lock()
	defer templateFuncMu.Unlock()
	for _, e := range entries {
		for _, existing := range templateFuncProviders {
			if existing.Name == e.Name {
				panic("renderer: duplicate template func: " + e.Name)
			}
		}
		templateFuncProviders = append(templateFuncProviders, e)
	}
}

// RegisterGetHandler registers a handler for blueprint get queries.
// Panics on duplicate action names.
func RegisterGetHandler(action string, handler func(d *TemplateData, key string) (string, error)) {
	getHandlerMu.Lock()
	defer getHandlerMu.Unlock()
	if _, exists := getHandlers[action]; exists {
		panic("renderer: duplicate get handler: " + action)
	}
	getHandlers[action] = handler
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

// FuncMap returns the template function map built from all registered providers.
func (d *TemplateData) FuncMap() template.FuncMap {
	templateFuncMu.RLock()
	defer templateFuncMu.RUnlock()
	m := make(template.FuncMap, len(templateFuncProviders))
	for _, e := range templateFuncProviders {
		m[e.Name] = e.Factory(d)
	}
	return m
}

// Get returns the value for a (action, key) query. Used by blueprint get.
func (d *TemplateData) Get(action, key string) (string, error) {
	getHandlerMu.RLock()
	handler, ok := getHandlers[action]
	getHandlerMu.RUnlock()

	if !ok {
		getHandlerMu.RLock()
		names := make([]string, 0, len(getHandlers))
		for k := range getHandlers {
			names = append(names, k)
		}
		getHandlerMu.RUnlock()
		sort.Strings(names)
		return "", fmt.Errorf("unknown action %q: supported actions are %s", action, strings.Join(names, ", "))
	}
	return handler(d, key)
}

// RenderWithRules renders tmplPath (file or directory) against the given rules.
// cliVars are KEY=VALUE overrides that take precedence over blueprint var rules.
// output is the destination: "" → stdout (single-file mode), a path → file or directory root.
// preferSSH controls whether git operations prefer SSH over HTTPS.
// verbose controls whether per-file "rendered ..." lines are printed (true for CLI, false for action handler).
// RenderWithRules renders tmplPath (file or directory) against the given rules.
// overrideDirs is an optional list of directories to search for local .tmpl overrides
// before falling back to the remote template. Directories are checked in order;
// the first match wins. The output directory is always checked last.
func RenderWithRules(rules []parser.Rule, tmplPath, output string, preferSSH bool, cliVars map[string]string, verbose bool, overrideDirs ...string) error {
	// Expand leading ~/ so paths like ~/workspace/... resolve correctly.
	if strings.HasPrefix(output, "~/") {
		if homeDir, err := os.UserHomeDir(); err == nil {
			output = filepath.Join(homeDir, output[2:])
		}
	}

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

	// Build the ordered list of directories to search for local overrides.
	// overrideDirs (e.g. blueprint repo dir) take priority over the output dir.
	searchDirs := append(overrideDirs, output)

	findOverride := func(t string) string {
		for _, dir := range searchDirs {
			if ov := LocalOverride(t, tmplRoot, dir); ov != "" {
				return ov
			}
		}
		return ""
	}

	// Directory mode — validate all templates first, then write.
	rendered := make(map[string]string, len(templates))
	for _, t := range templates {
		src := t
		if override := findOverride(t); override != "" {
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
		override := findOverride(t)
		if err := os.MkdirAll(filepath.Dir(out), 0o750); err != nil { // #nosec G301 -- output directories must be group-readable
			return fmt.Errorf("cannot create directory for %s: %w", out, err)
		}
		if err := os.WriteFile(out, []byte(rendered[t]), 0o644); err != nil { // #nosec G306 -- rendered template files must be world-readable
			return fmt.Errorf("cannot write %s: %w", out, err)
		}
		if verbose {
			if override != "" {
				fmt.Printf("rendered  %s (local override)\n", out)
			} else {
				fmt.Printf("rendered  %s\n", out)
			}
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
