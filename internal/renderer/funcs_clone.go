package renderer

import (
	"fmt"
	"strings"
)

func init() {
	RegisterTemplateFuncs([]TemplateFuncEntry{
		{Name: "cloneURL", Factory: func(d *TemplateData) interface{} { return d.cloneURL }},
	})
	RegisterGetHandler("clone", func(d *TemplateData, key string) (string, error) {
		return d.cloneURL(key)
	})
}

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
