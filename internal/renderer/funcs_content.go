package renderer

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

func init() {
	RegisterTemplateFuncs([]TemplateFuncEntry{
		{Name: "content", Factory: func(d *TemplateData) interface{} { return d.content }},
		{Name: "replace", Factory: func(_ *TemplateData) interface{} { return strings.ReplaceAll }},
		{Name: "regexReplaceAll", Factory: func(_ *TemplateData) interface{} { return regexReplaceAll }},
	})
}

// content reads the current file at OutputPath and returns its content.
// If OutputPath is empty or the file doesn't exist, returns "" (first run).
func (d *TemplateData) content() (string, error) {
	if d.OutputPath == "" {
		return "", nil
	}
	b, err := os.ReadFile(d.OutputPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("content: cannot read %s: %w", d.OutputPath, err)
	}
	return string(b), nil
}

// regexReplaceAll replaces all matches of a regex pattern with a replacement string.
// Returns the modified string. Panics if the pattern is invalid (caught at template
// parse time or execution time as a template error).
//
// Usage in template:
//
//	{{ content | regexReplaceAll "go [0-9]+\\.[0-9]+(\\.[0-9]+)?" "go 1.23.0" }}
func regexReplaceAll(pattern, replacement, input string) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("regexReplaceAll: invalid pattern %q: %w", pattern, err)
	}
	return re.ReplaceAllString(input, replacement), nil
}
