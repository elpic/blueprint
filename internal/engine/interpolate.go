package engine

import (
	"strings"

	"github.com/elpic/blueprint/internal/parser"
)

// resolveVarMap builds a map of variable name → value from all var rules.
// CLI vars (passed via --var KEY=VALUE) take precedence over blueprint defaults.
func resolveVarMap(rules []parser.Rule, cliVars map[string]string) map[string]string {
	vars := make(map[string]string)
	for _, r := range rules {
		if r.Action != "var" {
			continue
		}
		if _, overridden := cliVars[r.VarName]; overridden {
			vars[r.VarName] = cliVars[r.VarName]
		} else {
			vars[r.VarName] = r.VarDefault
		}
	}
	// CLI vars always win, even for names not declared in the blueprint.
	for k, v := range cliVars {
		vars[k] = v
	}
	return vars
}

// interpolateRule returns a copy of rule with all ${VAR_NAME} references in
// string fields replaced by their resolved values. Fields that have no
// interpolation markers are returned unchanged.
func interpolateRule(rule parser.Rule, vars map[string]string) parser.Rule {
	if len(vars) == 0 {
		return rule
	}
	expand := func(s string) string { return expandVars(s, vars) }

	rule.CloneURL = expand(rule.CloneURL)
	rule.ClonePath = expand(rule.ClonePath)
	rule.Branch = expand(rule.Branch)
	rule.MisePath = expand(rule.MisePath)
	rule.Mkdir = expand(rule.Mkdir)
	rule.DecryptFile = expand(rule.DecryptFile)
	rule.DecryptPath = expand(rule.DecryptPath)
	rule.DownloadURL = expand(rule.DownloadURL)
	rule.DownloadPath = expand(rule.DownloadPath)
	rule.DotfilesURL = expand(rule.DotfilesURL)
	rule.DotfilesPath = expand(rule.DotfilesPath)
	rule.RunCommand = expand(rule.RunCommand)
	rule.RunUnless = expand(rule.RunUnless)
	rule.RunUndo = expand(rule.RunUndo)
	rule.RunShURL = expand(rule.RunShURL)
	rule.ScheduleSource = expand(rule.ScheduleSource)
	rule.AuthorizedKeysFile = expand(rule.AuthorizedKeysFile)
	rule.RenderTemplate = expand(rule.RenderTemplate)
	rule.RenderOutput = expand(rule.RenderOutput)
	rule.KnownHosts = expand(rule.KnownHosts)

	return rule
}

// expandVars replaces ${VAR_NAME} occurrences in s with values from vars.
func expandVars(s string, vars map[string]string) string {
	if !strings.Contains(s, "${") {
		return s
	}
	for name, value := range vars {
		s = strings.ReplaceAll(s, "${"+name+"}", value)
	}
	return s
}
