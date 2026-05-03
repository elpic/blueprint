package renderer

import (
	"fmt"
	"strings"
)

func init() {
	RegisterTemplateFuncs([]TemplateFuncEntry{
		{Name: "homebrewFormulas", Factory: func(d *TemplateData) interface{} { return d.homebrewFormulas }},
		{Name: "homebrewCasks", Factory: func(d *TemplateData) interface{} { return d.homebrewCasks }},
	})
	RegisterGetHandler("homebrew", func(d *TemplateData, key string) (string, error) {
		switch key {
		case "formula", "formulas":
			return d.homebrewFormulas(), nil
		case "cask", "casks":
			return d.homebrewCasks(), nil
		default:
			return "", fmt.Errorf("unknown homebrew key %q: use \"formula\" or \"cask\"", key)
		}
	})
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
