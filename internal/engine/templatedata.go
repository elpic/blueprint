package engine

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/elpic/blueprint/internal/parser"
)

// TemplateData holds all values extractable from a parsed blueprint for use
// in templates and blueprint get queries.
type TemplateData struct {
	rules   []parser.Rule
	cliVars map[string]string // --var overrides from the command line
}

// BuildTemplateData constructs a TemplateData from a slice of rules.
// cliVars are values passed via --var KEY=VALUE and take precedence over
// defaults defined in the blueprint.
func BuildTemplateData(rules []parser.Rule, cliVars map[string]string) *TemplateData {
	if cliVars == nil {
		cliVars = map[string]string{}
	}
	return &TemplateData{rules: rules, cliVars: cliVars}
}

// FuncMap returns a text/template FuncMap with all blueprint template functions.
// Each function fails loudly when the requested key is not found.
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
	}
}

// Get returns the value for a query of the form (action, key).
// Used by blueprint get.
func (d *TemplateData) Get(action, key string) (string, error) {
	switch action {
	case "mise":
		return d.miseVersion(key)
	case "asdf":
		return d.asdfVersion(key)
	case "packages":
		// key may be "pm/stage" or just "pm" or ""
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
	default:
		return "", fmt.Errorf("unknown action %q: supported actions are mise, asdf, packages, homebrew, clone, var, default", action)
	}
}

// miseVersion returns the pinned version for a mise-managed tool.
// The tool name is matched against the "tool@version" entries, e.g. "ruby@3.3.0".
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

// asdfVersion returns the pinned version for an asdf-managed tool.
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

// packages returns a space-separated list of package names.
// Accepts up to two optional arguments:
//
//	packages()              → all packages
//	packages("snap")        → filtered by package manager
//	packages("", "build")   → filtered by stage
//	packages("snap", "build") → filtered by both
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

// homebrewFormulas returns a space-separated list of homebrew formula names.
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

// homebrewCasks returns a space-separated list of homebrew cask names.
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

// cloneURL returns the clone URL for a repo whose clone path contains the
// given name (matched as a suffix or substring).
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

// varDefault resolves a variable by name, returning fallback if not set.
// Precedence: CLI --var > blueprint var rule > fallback (never errors).
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

// varValue resolves a variable by name. Precedence:
//  1. CLI --var override
//  2. Default defined in the blueprint via "var NAME default"
//  3. Missing with no default → error
func (d *TemplateData) varValue(name string) (string, error) {
	// CLI overrides win
	if v, ok := d.cliVars[name]; ok {
		return v, nil
	}
	// Look for a var rule in the blueprint
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

// splitToolVersion splits "tool@version" into (tool, version, true).
// Returns ("", "", false) if there is no "@".
func splitToolVersion(s string) (string, string, bool) {
	idx := strings.Index(s, "@")
	if idx < 0 {
		return "", "", false
	}
	return s[:idx], s[idx+1:], true
}
