package engine

import (
	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/renderer"
)

// TemplateData is a type alias for renderer.TemplateData to preserve the
// engine package's public API for any callers that use engine.BuildTemplateData.
type TemplateData = renderer.TemplateData

// BuildTemplateData constructs a TemplateData from a slice of rules.
// cliVars are values passed via --var KEY=VALUE and take precedence over
// defaults defined in the blueprint.
func BuildTemplateData(rules []parser.Rule, cliVars map[string]string) *TemplateData {
	return renderer.BuildTemplateData(rules, cliVars)
}
