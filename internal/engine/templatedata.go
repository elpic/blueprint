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
	rules []parser.Rule
}

// BuildTemplateData constructs a TemplateData from a slice of rules.
func BuildTemplateData(rules []parser.Rule) *TemplateData {
	return &TemplateData{rules: rules}
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
		return d.packages(key), nil
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
	default:
		return "", fmt.Errorf("unknown action %q: supported actions are mise, asdf, packages, homebrew, clone", action)
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
// An optional packageManager argument filters by package manager (e.g. "snap").
// Calling with no argument (or empty string) returns all packages.
func (d *TemplateData) packages(pm ...string) string {
	filter := ""
	if len(pm) > 0 {
		filter = pm[0]
	}
	var names []string
	for _, r := range d.rules {
		if r.Action != "install" {
			continue
		}
		for _, pkg := range r.Packages {
			if filter == "" || strings.EqualFold(pkg.PackageManager, filter) {
				names = append(names, pkg.Name)
			}
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

// splitToolVersion splits "tool@version" into (tool, version, true).
// Returns ("", "", false) if there is no "@".
func splitToolVersion(s string) (string, string, bool) {
	idx := strings.Index(s, "@")
	if idx < 0 {
		return "", "", false
	}
	return s[:idx], s[idx+1:], true
}
