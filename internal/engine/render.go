package engine

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	gitpkg "github.com/elpic/blueprint/internal/git"
	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/renderer"
	"github.com/elpic/blueprint/internal/ui"
)

// Render parses a blueprint and renders tmplPath (file or directory) against it.
// When tmplPath is a directory all *.tmpl files are rendered recursively;
// files whose basename starts with "_" are skipped.
// output is used only for single-file mode ("" → stdout, path → file).
func Render(file, tmplPath, output string, preferSSH bool, cliVars map[string]string) {
	rules := loadRulesForRender(file, preferSSH)
	if err := renderer.RenderWithRules(rules, tmplPath, output, preferSSH, cliVars, true); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// RenderWithRules renders tmplPath using pre-parsed rules. Used by the render
// blueprint action so the rules don't need to be re-parsed from a file.
func RenderWithRules(rules []parser.Rule, tmplPath, output string, preferSSH bool, cliVars map[string]string) error {
	return renderer.RenderWithRules(rules, tmplPath, output, preferSSH, cliVars, true)
}

// Check parses a blueprint, renders tmplPath (file or directory), and compares
// each rendered result against the corresponding committed file.
// against is a file path in single-file mode, or a directory root in directory mode.
// In directory mode with no --against, targets are resolved next to each template.
// Exits 0 when all are identical, 1 when any differ.
func Check(file, tmplPath, against string, preferSSH bool, cliVars map[string]string) {
	rules := loadRulesForRender(file, preferSSH)

	originalTmplPath := tmplPath // preserve for the "Run to fix" hint
	localTmpl, tmplRoot, cleanup, err := renderer.ResolveTemplatePath(tmplPath, preferSSH)
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
		for _, t := range templates {
			pairs = append(pairs, pair{t, renderer.ResolveOutput(t, tmplRoot, against)})
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
		fixOutput := against
		if fixOutput == "" {
			fixOutput = p.target
		}
		fmt.Fprintf(os.Stderr, "Run to fix:\n  %s render %s --template %s --output %s\n\n", ExecutableName, file, originalTmplPath, fixOutput)
	}
	if drifted {
		os.Exit(1)
	}
}

// Get extracts a single value from a blueprint and prints it to stdout.
func Get(file, action, key string, preferSSH bool, cliVars map[string]string) {
	rules := loadRulesForRender(file, preferSSH)
	data := renderer.BuildTemplateData(rules, cliVars)
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

// mustRenderTemplate renders a single template file, exiting on any error.
func mustRenderTemplate(tmplPath string, rules []parser.Rule, cliVars map[string]string) string {
	result, err := renderer.RenderTemplate(tmplPath, rules, cliVars)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	return result
}

// printDiff produces a unified-style diff between old and new strings,
// showing up to 3 lines of context around each changed section.
func printDiff(old, new, label string) string {
	const context = 3

	oldLines := strings.Split(strings.TrimRight(old, "\n"), "\n")
	newLines := strings.Split(strings.TrimRight(new, "\n"), "\n")

	// Build an edit script: for each position record whether the line is
	// equal, removed (-), or added (+). We use a simple LCS-free approach:
	// walk both slices and emit ops greedily, which is sufficient for the
	// typical case of small template drift.
	type op struct {
		kind byte // ' ', '-', '+'
		line string
	}
	var ops []op
	i, j := 0, 0
	for i < len(oldLines) || j < len(newLines) {
		if i < len(oldLines) && j < len(newLines) && oldLines[i] == newLines[j] {
			ops = append(ops, op{' ', oldLines[i]})
			i++
			j++
		} else if i < len(oldLines) {
			ops = append(ops, op{'-', oldLines[i]})
			i++
		} else {
			ops = append(ops, op{'+', newLines[j]})
			j++
		}
	}

	// Collect indices of changed ops so we know which context to show.
	changed := make([]bool, len(ops))
	for k, o := range ops {
		if o.kind != ' ' {
			changed[k] = true
		}
	}

	// Emit hunks: groups of changed lines plus surrounding context.
	var b strings.Builder
	fmt.Fprintf(&b, "--- %s (existing)\n", label)
	fmt.Fprintf(&b, "+++ %s (rendered)\n", label)

	inHunk := false
	for k := range ops {
		// Is this op within context distance of a changed line?
		near := false
		for d := -context; d <= context; d++ {
			idx := k + d
			if idx >= 0 && idx < len(ops) && changed[idx] {
				near = true
				break
			}
		}
		if !near {
			if inHunk {
				fmt.Fprintf(&b, "...\n")
				inHunk = false
			}
			continue
		}
		inHunk = true
		switch ops[k].kind {
		case ' ':
			fmt.Fprintf(&b, " %s\n", ops[k].line)
		case '-':
			fmt.Fprintf(&b, "-%s\n", ops[k].line)
		case '+':
			fmt.Fprintf(&b, "+%s\n", ops[k].line)
		}
	}
	return b.String()
}
