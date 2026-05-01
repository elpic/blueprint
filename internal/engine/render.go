package engine

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	gitpkg "github.com/elpic/blueprint/internal/git"
	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
)

// Render parses a blueprint and renders tmplPath (file or directory) against it.
// When tmplPath is a directory all *.tmpl files are rendered recursively;
// files whose basename starts with "_" are skipped.
// output is used only for single-file mode ("" → stdout, path → file).
func Render(file, tmplPath, output string, preferSSH bool, cliVars map[string]string) {
	rules := loadRulesForRender(file, preferSSH)

	localTmpl, tmplRoot, cleanup, err := resolveTemplatePath(tmplPath, preferSSH)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()
	tmplPath = localTmpl

	templates, err := collectTemplates(tmplPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if len(templates) == 1 && !isDir(tmplPath) {
		// Single-file mode — respect --output / stdout
		result := mustRenderTemplate(templates[0], rules, cliVars)
		writeOutput(result, output)
		return
	}

	// Directory (or explicit multi) mode — validate all templates first,
	// then write; fail before touching any file if any template errors.
	rendered := make(map[string]string, len(templates))
	for _, t := range templates {
		rendered[t] = mustRenderTemplate(t, rules, cliVars)
	}
	for _, t := range templates {
		out := resolveOutput(t, tmplRoot, output)
		if err := os.MkdirAll(filepath.Dir(out), 0o750); err != nil { // #nosec G301 -- output directories must be group-readable
			fmt.Fprintf(os.Stderr, "error: cannot create directory for %s: %v\n", out, err)
			os.Exit(1)
		}
		if err := os.WriteFile(out, []byte(rendered[t]), 0o644); err != nil { // #nosec G306 -- rendered template files must be world-readable
			fmt.Fprintf(os.Stderr, "error: cannot write %s: %v\n", out, err)
			os.Exit(1)
		}
		fmt.Printf("rendered  %s\n", out)
	}
}

// resolveOutput computes the output path for a template in directory mode.
// When outputRoot is set, the template's path relative to tmplRoot is preserved
// and rooted at outputRoot. Without outputRoot, output is written next to the template.
func resolveOutput(tmplPath, tmplRoot, outputRoot string) string {
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

// Check parses a blueprint, renders tmplPath (file or directory), and compares
// each rendered result against the corresponding committed file.
// against is a file path in single-file mode, or a directory root in directory mode.
// In directory mode with no --against, targets are resolved next to each template.
// Exits 0 when all are identical, 1 when any differ.
func Check(file, tmplPath, against string, preferSSH bool, cliVars map[string]string) {
	rules := loadRulesForRender(file, preferSSH)

	localTmpl, tmplRoot, cleanup, err := resolveTemplatePath(tmplPath, preferSSH)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()
	tmplPath = localTmpl

	templates, err := collectTemplates(tmplPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Build the list of (template → target) pairs.
	type pair struct{ tmpl, target string }
	var pairs []pair

	if !isDir(tmplPath) {
		// Single-file mode — --against is required.
		if against == "" {
			fmt.Fprintln(os.Stderr, "error: --against <file> is required when --template is a single file")
			os.Exit(1)
		}
		pairs = []pair{{templates[0], against}}
	} else {
		// Directory mode — when --against is a directory, use it as the output
		// root (same logic as render --output). Without --against, fall back to
		// the file next to each template.
		for _, t := range templates {
			pairs = append(pairs, pair{t, resolveOutput(t, tmplRoot, against)})
		}
	}

	drifted := false
	for _, p := range pairs {
		rendered := mustRenderTemplate(p.tmpl, rules, cliVars)
		existing, err := os.ReadFile(p.target) // #nosec G304 -- user-supplied path, intentional
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: cannot read %s: %v\n", p.target, err)
			os.Exit(1)
		}
		if bytes.Equal([]byte(rendered), existing) {
			fmt.Printf("ok        %s\n", p.target)
			continue
		}
		drifted = true
		fmt.Fprintf(os.Stderr, "%s\n", ui.FormatError(fmt.Sprintf("%s is out of date.", p.target)))
		fmt.Fprintln(os.Stderr, printDiff(string(existing), rendered, p.target))
		fmt.Fprintf(os.Stderr, "Run to fix:\n  blueprint render %s --template %s --output %s\n\n", file, p.tmpl, p.target)
	}
	if drifted {
		os.Exit(1)
	}
}

// Get extracts a single value from a blueprint and prints it to stdout.
func Get(file, action, key string, preferSSH bool, cliVars map[string]string) {
	rules := loadRulesForRender(file, preferSSH)
	data := BuildTemplateData(rules, cliVars)
	val, err := data.Get(action, key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(val)
}

// collectTemplates returns all *.tmpl files under path.
// If path is a file it returns [path].
// If path is a directory it walks recursively, skipping files whose
// basename starts with "_".
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

// isDir reports whether path is a directory.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// loadRulesForRender resolves and parses a blueprint, exiting on any error.
func loadRulesForRender(file string, preferSSH bool) []parser.Rule {
	if preferSSH {
		file = gitpkg.ExpandShorthandSSH(file)
	} else {
		file = gitpkg.ExpandShorthand(file)
	}

	setupPath, _, cleanup, err := resolveBlueprintFile(file, false, preferSSH)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", ui.FormatError(fmt.Sprintf("Error: %v", err)))
		os.Exit(1)
	}
	defer cleanup()

	rules, err := parser.ParseFile(setupPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", ui.FormatError(fmt.Sprintf("Parse error: %v", err)))
		os.Exit(1)
	}
	return rules
}

// resolveTemplatePath resolves --template to a local filesystem path.
// If tmplPath is a git URL or @provider: shorthand, the repo is cloned/updated
// to the blueprint cache and the path within the repo is returned.
// Returns (localPath, rootForRelativeCalc, cleanup, error).
// For local paths: localPath == tmplPath, rootForRelativeCalc == tmplPath.
// For remote paths: localPath is the directory inside the clone, rootForRelativeCalc == localPath.
func resolveTemplatePath(tmplPath string, preferSSH bool) (local, root string, cleanup func(), err error) {
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

	// params.Path defaults to "setup.bp" for blueprint files, but for templates
	// there is no default — an empty path means the repo root.
	tmplDir := localRepo
	if params.Path != "" && params.Path != "setup.bp" {
		tmplDir = filepath.Join(localRepo, filepath.FromSlash(params.Path))
	}

	if _, statErr := os.Stat(tmplDir); statErr != nil {
		return "", "", cleanup, fmt.Errorf("template path %q not found in repository", params.Path)
	}

	return tmplDir, tmplDir, cleanup, nil
}

// mustRenderTemplate renders a single template file, exiting on any error.
func mustRenderTemplate(tmplPath string, rules []parser.Rule, cliVars map[string]string) string {
	tmplContent, err := os.ReadFile(tmplPath) // #nosec G304 -- user-supplied path, intentional
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot read template %s: %v\n", tmplPath, err)
		os.Exit(1)
	}

	data := BuildTemplateData(rules, cliVars)

	tmpl, err := template.New(filepath.Base(tmplPath)).
		Option("missingkey=error").
		Funcs(data.FuncMap()).
		Parse(string(tmplContent))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: template parse error in %s: %v\n", tmplPath, err)
		os.Exit(1)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nil); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s: %v\n", tmplPath, err)
		os.Exit(1)
	}
	return buf.String()
}

// writeOutput writes content to a file or stdout.
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

// printDiff produces a simple unified-style diff between old and new strings.
func printDiff(old, new, label string) string {
	oldLines := strings.Split(old, "\n")
	newLines := strings.Split(new, "\n")

	var b strings.Builder
	fmt.Fprintf(&b, "--- %s (existing)\n", label)
	fmt.Fprintf(&b, "+++ %s (rendered)\n", label)

	maxLen := len(oldLines)
	if len(newLines) > maxLen {
		maxLen = len(newLines)
	}

	for i := 0; i < maxLen; i++ {
		oldLine := ""
		newLine := ""
		if i < len(oldLines) {
			oldLine = oldLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}
		if oldLine != newLine {
			if i < len(oldLines) {
				fmt.Fprintf(&b, "-%s\n", oldLine)
			}
			if i < len(newLines) {
				fmt.Fprintf(&b, "+%s\n", newLine)
			}
		}
	}
	return b.String()
}
