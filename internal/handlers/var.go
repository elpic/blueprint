package handlers

import (
	"github.com/elpic/blueprint/internal/parser"
)

func init() {
	RegisterAction(ActionDef{
		Name:   "var",
		Prefix: "var ",
		NewHandler: func(rule parser.Rule, basePath string, passwordCache map[string]string) Handler {
			return &varHandler{BaseHandler: BaseHandler{Rule: rule, BasePath: basePath}}
		},
		RuleKey: func(rule parser.Rule) string {
			return "var:" + rule.VarName
		},
		Detect: func(rule parser.Rule) bool {
			return rule.VarName != ""
		},
		Summary: func(rule parser.Rule) string {
			return rule.VarName
		},
	})
}

// varHandler is a no-op handler. var rules are data-only — they are resolved
// into a map before execution (see engine/interpolate.go) and do not perform
// any system changes themselves.
type varHandler struct {
	BaseHandler
}

func (h *varHandler) Up() (string, error)   { return "", nil }
func (h *varHandler) Down() (string, error) { return "", nil }

func (h *varHandler) GetCommand() string { return "" }

func (h *varHandler) UpdateStatus(status *Status, records []ExecutionRecord, blueprint string, osName string) error {
	return nil
}

func (h *varHandler) DisplayInfo() {}

func (h *varHandler) GetDependencyKey() string {
	return getDependencyKey(h.Rule, "var:"+h.Rule.VarName)
}

func (h *varHandler) GetDisplayDetails(isUninstall bool) string {
	return h.Rule.VarName
}

func (h *varHandler) GetState(isUninstall bool) map[string]string {
	return map[string]string{
		"summary": h.Rule.VarName,
		"name":    h.Rule.VarName,
		"value":   h.Rule.VarDefault,
	}
}

func (h *varHandler) FindUninstallRules(status *Status, currentRules []parser.Rule, blueprintFile, osName string) []parser.Rule {
	return nil
}

func (h *varHandler) IsInstalled(status *Status, blueprintFile, osName string) bool {
	return true // always skip Up() — vars are never "installed"
}

func (h *varHandler) DisplayStatusFromStatus(status *Status) {}
