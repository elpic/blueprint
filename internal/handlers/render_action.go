package handlers

import (
	"fmt"
	"strings"

	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/renderer"
	"github.com/elpic/blueprint/internal/ui"
)

func init() {
	RegisterAction(ActionDef{
		Name:   "render",
		Prefix: "render ",
		NewHandler: func(rule parser.Rule, basePath string, passwordCache map[string]string) Handler {
			return NewRenderActionHandler(rule, basePath)
		},
		RuleKey: func(rule parser.Rule) string {
			return rule.RenderTemplate
		},
		Detect: func(rule parser.Rule) bool {
			return rule.RenderTemplate != ""
		},
		Summary: func(rule parser.Rule) string {
			out := rule.RenderTemplate
			if rule.RenderOutput != "" && rule.RenderOutput != "." {
				out += " → " + rule.RenderOutput
			}
			return out
		},
		OrphanIndex: nil,
		ShellExport: nil, // render is not exportable to shell scripts
		// AlwaysRunUp: render is idempotent — re-running just overwrites with the
		// same content. We always run it so templates stay in sync on every apply.
		AlwaysRunUp: true,
	})
}

// RenderActionHandler handles the render action inside a blueprint.
type RenderActionHandler struct {
	BaseHandler
}

// NewRenderActionHandler creates a new render action handler.
func NewRenderActionHandler(rule parser.Rule, basePath string) *RenderActionHandler {
	return &RenderActionHandler{
		BaseHandler: BaseHandler{
			Rule:     rule,
			BasePath: basePath,
		},
	}
}

// Up renders the template(s) using the current blueprint as the data source.
func (h *RenderActionHandler) Up() (string, error) {
	rules, err := parser.ParseFile(h.BasePath)
	if err != nil {
		return "", fmt.Errorf("render: failed to parse blueprint %s: %w", h.BasePath, err)
	}

	cliVars := parseVarsSlice(h.Rule.RenderVars)
	output := h.Rule.RenderOutput
	if output == "" {
		output = "."
	}

	if err := renderer.RenderWithRules(rules, h.Rule.RenderTemplate, output, false, cliVars); err != nil {
		return "", fmt.Errorf("render: %w", err)
	}

	return fmt.Sprintf("rendered templates from %s to %s", h.Rule.RenderTemplate, output), nil
}

// Down is a no-op — rendered files are not removed on cleanup.
func (h *RenderActionHandler) Down() (string, error) {
	return "render: nothing to undo", nil
}

// GetCommand returns a human-readable description of the render operation.
func (h *RenderActionHandler) GetCommand() string {
	parts := []string{"blueprint render", h.BasePath, "--template", h.Rule.RenderTemplate}
	if h.Rule.RenderOutput != "" && h.Rule.RenderOutput != "." {
		parts = append(parts, "--output", h.Rule.RenderOutput)
	}
	for _, v := range h.Rule.RenderVars {
		parts = append(parts, "--var", v)
	}
	return strings.Join(parts, " ")
}

// UpdateStatus is a no-op — render does not track status entries.
func (h *RenderActionHandler) UpdateStatus(status *Status, records []ExecutionRecord, blueprint string, osName string) error {
	return nil
}

// DisplayInfo prints the template and output paths.
func (h *RenderActionHandler) DisplayInfo() {
	fmt.Printf("  %s\n", ui.FormatInfo(fmt.Sprintf("Template: %s", h.Rule.RenderTemplate)))
	out := h.Rule.RenderOutput
	if out == "" {
		out = "."
	}
	fmt.Printf("  %s\n", ui.FormatInfo(fmt.Sprintf("Output:   %s", out)))
	for _, v := range h.Rule.RenderVars {
		fmt.Printf("  %s\n", ui.FormatInfo(fmt.Sprintf("Var:      %s", v)))
	}
}

// GetDependencyKey returns the unique key for dependency resolution.
func (h *RenderActionHandler) GetDependencyKey() string {
	return getDependencyKey(h.Rule, h.Rule.RenderTemplate)
}

// GetDisplayDetails returns the template path for display during execution.
func (h *RenderActionHandler) GetDisplayDetails(isUninstall bool) string {
	return h.Rule.RenderTemplate
}

// GetState returns handler-specific state as key-value pairs.
func (h *RenderActionHandler) GetState(isUninstall bool) map[string]string {
	return map[string]string{
		"summary":  h.Rule.RenderTemplate,
		"template": h.Rule.RenderTemplate,
		"output":   h.Rule.RenderOutput,
	}
}

// FindUninstallRules returns nothing — render has no orphan cleanup.
func (h *RenderActionHandler) FindUninstallRules(status *Status, currentRules []parser.Rule, blueprintFile, osName string) []parser.Rule {
	return nil
}

// IsInstalled always returns false so Up() always runs (AlwaysRunUp handles this,
// but IsInstalled must still satisfy the Handler interface).
func (h *RenderActionHandler) IsInstalled(status *Status, blueprintFile, osName string) bool {
	return false
}

// DisplayStatusFromStatus is a no-op — render has no status entries.
func (h *RenderActionHandler) DisplayStatusFromStatus(status *Status) {}

// parseVarsSlice converts a []string of "KEY=VALUE" entries into a map.
func parseVarsSlice(vars []string) map[string]string {
	m := make(map[string]string, len(vars))
	for _, v := range vars {
		k, val, ok := strings.Cut(v, "=")
		if ok {
			m[k] = val
		}
	}
	return m
}
