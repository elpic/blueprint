package engine

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/renderer"
	"github.com/elpic/blueprint/internal/ui"
)

// TemplateVar holds information about a variable discovered in a template.
type TemplateVar struct {
	Name       string
	Default    string
	HasDefault bool
}

// templateFuncCall matches Go template function calls that reference variables.
// Groups: funcName, arg1 (var name), arg2 (fallback for default, optional).
// Uses a relaxed unanchored match to handle all pipeline positions:
//   {{ var "NAME" }}                — direct call
//   {{ range toValue "CHECKS" }}    — after range/if/with keyword
//   {{ var "NAME" | upper }}         — before pipe
//   {{ if var "ENABLED" }}           — conditional
//   {{ $x := toValue "DATA" }}       — assignment
// False-positive risk is negligible since var/toValue/default+"quoted" is
// unlikely as literal file content in templates.
var templateFuncCall = regexp.MustCompile(`(?:^|[^A-Za-z])(var|toValue|default)\s+"([^"]+)"(?:\s+"([^"]+)")?`)

// Template prompts for variables and renders a template directory.
// tmplPath can be a local path, @github: shorthand, or git URL.
// output is the destination directory (required).
// cliVars are pre-set values from --var flags; variables in this map skip the prompt.
func Template(tmplPath, output string, preferSSH bool, cliVars map[string]string) {
	// 1. Resolve template source to a local directory
	localTmpl, _, cleanup, err := renderer.ResolveTemplatePath(tmplPath, preferSSH)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()
	tmplPath = localTmpl

	// 2. Load setup.bp if present for default variable values
	rules := loadTemplateBP(tmplPath)

	// 3. Collect all .tmpl files
	templates, err := collectTemplates(tmplPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// 4. Discover variables from template files
	varDefs := discoverTemplateVars(templates, rules)

	// 5. Interactive prompt — merge CLI vars with prompted values
	values := promptForVars(varDefs, cliVars)

	// 6. Render all templates with the collected values
	if err := renderer.RenderWithRules(rules, tmplPath, output, preferSSH, values, true); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// loadTemplateBP loads setup.bp from the template directory if present.
func loadTemplateBP(tmplPath string) []parser.Rule {
	bpPath := filepath.Join(tmplPath, "setup.bp")
	if info, err := os.Stat(bpPath); err != nil || info.IsDir() {
		return nil
	}
	rules, err := parser.ParseFile(bpPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot parse template blueprint: %v\n", err)
		os.Exit(1)
	}
	return rules
}

// discoverTemplateVars scans all template files for variable references and
// merges them with defaults from var rules and template fallbacks.
//
// Detects three patterns:
//   - {{ var "NAME" }}        — hard reference, requires a value
//   - {{ toValue "NAME" }}    — hard reference, requires a value
//   - {{ default "NAME" "v" }} — soft reference, has built-in fallback
//
// Hard references without a var rule default or template fallback are required.
// Soft references (only used via default) always have a fallback to show.
func discoverTemplateVars(templates []string, rules []parser.Rule) []TemplateVar {
	// Build a lookup of var name → default from rules
	rulesDefaults := map[string]string{}
	rulesRequired := map[string]bool{}
	for _, r := range rules {
		if r.Action == "var" {
			if r.VarRequired {
				rulesRequired[r.VarName] = true
			} else {
				rulesDefaults[r.VarName] = r.VarDefault
			}
		}
	}

	// Collected data during template scanning
	type varInfo struct {
		hasHardRef bool   // used via var() or toValue() → will error if not set
		fallback   string // from {{ default "NAME" "fallback" }}
		hasFallback bool
	}
	vars := map[string]*varInfo{}

	for _, tmpl := range templates {
		content, err := os.ReadFile(tmpl)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: cannot read %s: %v\n", tmpl, err)
			continue
		}
		matches := templateFuncCall.FindAllStringSubmatch(string(content), -1)
		for _, m := range matches {
			funcName := m[1]
			name := m[2]

			vi, exists := vars[name]
			if !exists {
				vi = &varInfo{}
				vars[name] = vi
			}

			switch funcName {
			case "var", "toValue":
				vi.hasHardRef = true
			case "default":
				if m[3] != "" {
					vi.fallback = m[3]
					vi.hasFallback = true
				}
			}
		}
	}

	// Merge into ordered TemplateVar slice
	// We iterate the map in a deterministic order
	names := sortedKeys(vars)

	defs := make([]TemplateVar, 0, len(names))
	for _, name := range names {
		vi := vars[name]

		// Priority for defaults: var rule > template fallback
		_, hasRuleDefault := rulesDefaults[name]
		_, isRequired := rulesRequired[name]

		def := TemplateVar{Name: name}

		if hasRuleDefault {
			// Var rule provides the default
			def.Default = rulesDefaults[name]
			def.HasDefault = true
		} else if vi.hasFallback {
			// Template has {{ default "NAME" "fallback" }}
			def.Default = vi.fallback
			def.HasDefault = true
		} else if isRequired || vi.hasHardRef {
			// No default available — required
			def.HasDefault = false
		} else {
			// Soft reference only (default with no fallback, shouldn't happen)
			// or no reference found (shouldn't happen if scanning works)
			def.HasDefault = false
		}

		defs = append(defs, def)
	}
	return defs
}

// sortedKeys returns map keys in alphabetical order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// promptForVars interactively asks the user for each variable value.
// existing contains values already provided via --var flags — these skip the prompt.
// Returns a merged map of all variable values.
func promptForVars(vars []TemplateVar, existing map[string]string) map[string]string {
	if existing == nil {
		existing = map[string]string{}
	}

	// Filter out variables already provided via --var
	var pending []TemplateVar
	for _, v := range vars {
		if _, ok := existing[v.Name]; !ok {
			pending = append(pending, v)
		}
	}

	if len(pending) == 0 {
		return existing
	}

	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, ui.FormatHeader("─── Template Variables ───"))
	fmt.Fprintln(os.Stderr, "")

	reader := bufio.NewReader(os.Stdin)
	values := existing

	for _, v := range pending {
		for {
			var prompt string
			if v.HasDefault {
				prompt = ui.FormatInfo(fmt.Sprintf("%s (default: %s): ", v.Name, v.Default))
			} else {
				prompt = ui.FormatHighlight(fmt.Sprintf("%s (required): ", v.Name))
			}
			fmt.Fprint(os.Stderr, prompt)

			input, err := reader.ReadString('\n')
			if err != nil {
				// EOF or read error — exit gracefully
				fmt.Fprintln(os.Stderr, "")
				os.Exit(1)
			}
			input = strings.TrimRight(input, "\n\r")

			if input == "" && v.HasDefault {
				values[v.Name] = v.Default
				break
			}

			if input == "" && !v.HasDefault {
				fmt.Fprintln(os.Stderr, ui.FormatError("value is required"))
				continue
			}

			values[v.Name] = input
			break
		}
	}
	return values
}
