package renderer

import (
	"encoding/json"
	"fmt"
	"strings"
)

func init() {
	RegisterTemplateFuncs([]TemplateFuncEntry{
		{Name: "var", Factory: func(d *TemplateData) interface{} { return d.varValue }},
		{Name: "default", Factory: func(d *TemplateData) interface{} { return d.varDefault }},
		{Name: "toArgs", Factory: func(_ *TemplateData) interface{} { return toArgs }},
	})
	RegisterGetHandler("var", func(d *TemplateData, key string) (string, error) {
		return d.varValue(key)
	})
	RegisterGetHandler("default", func(d *TemplateData, key string) (string, error) {
		parts := strings.SplitN(key, "/", 2)
		fallback := ""
		if len(parts) > 1 {
			fallback = parts[1]
		}
		return d.varDefault(parts[0], fallback), nil
	})
}

func (d *TemplateData) varValue(name string) (string, error) {
	if v, ok := d.cliVars[name]; ok {
		return v, nil
	}
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

func toArgs(cmd string) string {
	parts := strings.Fields(cmd)
	b, _ := json.Marshal(parts)
	return string(b)
}
