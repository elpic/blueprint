package renderer

import (
	"fmt"
	"strings"
)

func init() {
	RegisterTemplateFuncs([]TemplateFuncEntry{
		{Name: "asdf", Factory: func(d *TemplateData) interface{} { return d.asdfVersion }},
	})
	RegisterGetHandler("asdf", func(d *TemplateData, key string) (string, error) {
		return d.asdfVersion(key)
	})
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
