package renderer

import "strings"

func init() {
	RegisterTemplateFuncs([]TemplateFuncEntry{
		{Name: "packages", Factory: func(d *TemplateData) interface{} { return d.packages }},
	})
	RegisterGetHandler("packages", func(d *TemplateData, key string) (string, error) {
		parts := strings.SplitN(key, "/", 2)
		pm := parts[0]
		stage := ""
		if len(parts) > 1 {
			stage = parts[1]
		}
		return d.packages(pm, stage), nil
	})
}

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
