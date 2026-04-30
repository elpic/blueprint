package engine

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"

	gitpkg "github.com/elpic/blueprint/internal/git"
	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
)

// Render parses a blueprint, renders tmplPath against it, and writes the
// result to output (stdout when output is "").
func Render(file, tmplPath, output string, preferSSH bool) {
	rules := loadRulesForRender(file, preferSSH)
	result := renderTemplate(tmplPath, rules)
	writeOutput(result, output)
}

// Check parses a blueprint, renders tmplPath, and compares the result with the
// content of againstPath. Exits 0 when identical, 1 when different.
func Check(file, tmplPath, againstPath string, preferSSH bool) {
	rules := loadRulesForRender(file, preferSSH)
	rendered := renderTemplate(tmplPath, rules)

	existing, err := os.ReadFile(againstPath) // #nosec G304 -- user-supplied path, intentional
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot read %s: %v\n", againstPath, err)
		os.Exit(1)
	}

	if bytes.Equal([]byte(rendered), existing) {
		fmt.Printf("%s is up to date.\n", againstPath)
		os.Exit(0)
	}

	fmt.Fprintf(os.Stderr, "%s\n", ui.FormatError(fmt.Sprintf("%s is out of date.", againstPath)))
	fmt.Fprintln(os.Stderr, printDiff(string(existing), rendered, againstPath))
	fmt.Fprintf(os.Stderr, "\nRun to fix:\n  blueprint render %s --template %s --output %s\n", file, tmplPath, againstPath)
	os.Exit(1)
}

// Get extracts a single value from a blueprint and prints it to stdout.
// action is e.g. "mise", key is e.g. "ruby".
func Get(file, action, key string, preferSSH bool) {
	rules := loadRulesForRender(file, preferSSH)
	data := BuildTemplateData(rules)
	val, err := data.Get(action, key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(val)
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

// renderTemplate renders tmplPath using data from rules, exiting on error.
func renderTemplate(tmplPath string, rules []parser.Rule) string {
	tmplContent, err := os.ReadFile(tmplPath) // #nosec G304 -- user-supplied path, intentional
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot read template %s: %v\n", tmplPath, err)
		os.Exit(1)
	}

	data := BuildTemplateData(rules)

	// option: missingkey=error makes unknown template calls fail loudly
	tmpl, err := template.New("blueprint").
		Option("missingkey=error").
		Funcs(data.FuncMap()).
		Parse(string(tmplContent))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: template parse error in %s: %v\n", tmplPath, err)
		os.Exit(1)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, nil); err != nil {
		fmt.Fprintf(os.Stderr, "error: template render error: %v\n", err)
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
	if err := os.WriteFile(output, []byte(content), 0o644); err != nil {
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
