package renderer

import (
	"fmt"
	"strings"
)

func init() {
	RegisterTemplateFuncs([]TemplateFuncEntry{
		{Name: "mise", Factory: func(d *TemplateData) interface{} { return d.miseVersion }},
	})
	RegisterGetHandler("mise", func(d *TemplateData, key string) (string, error) {
		return d.miseVersion(key)
	})
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
