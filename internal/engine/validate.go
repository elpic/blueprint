package engine

import (
	"fmt"
	"os"

	handlerskg "github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
)

// validOSNames is the set of OS names the engine recognises in os: filters.
var validOSNames = map[string]bool{
	"mac":     true,
	"linux":   true,
	"windows": true,
}

// validateIssue describes a single validation problem.
type validateIssue struct {
	line    int    // 1-based rule index (0 = file-level, not rule-specific)
	summary string // human-readable rule description
	message string
}

func (v validateIssue) String() string {
	if v.line > 0 {
		return fmt.Sprintf("rule %d (%s): %s", v.line, v.summary, v.message)
	}
	return v.message
}

// Validate parses a blueprint file (or git URL) and runs semantic checks.
// It prints all issues found and exits with code 1 if any are found.
func Validate(file string) {
	fmt.Printf("\n%s\n", ui.FormatHighlight("=== Blueprint Validate ==="))
	fmt.Printf("\nParsing %s...\n", file)

	setupPath, _, cleanup, err := resolveBlueprintFile(file, true)
	if err != nil {
		fmt.Printf("  %s\n", ui.FormatError(fmt.Sprintf("Error resolving blueprint: %v", err)))
		fmt.Printf("\n%s\n\n", ui.FormatError("Validation failed."))
		os.Exit(1)
	}
	defer cleanup()

	rules, err := parser.ParseFile(setupPath)
	if err != nil {
		fmt.Printf("  %s\n", ui.FormatError(fmt.Sprintf("Parse error: %v", err)))
		fmt.Printf("\n%s\n\n", ui.FormatError("Validation failed."))
		os.Exit(1)
	}

	fmt.Printf("  %s\n", ui.FormatSuccess(fmt.Sprintf("parsed %d rules", len(rules))))

	issues := semanticCheck(rules)

	if len(issues) == 0 {
		fmt.Printf("\n%s\n\n", ui.FormatSuccess("No issues found."))
		return
	}

	fmt.Printf("\n")
	for _, issue := range issues {
		fmt.Printf("  %s\n", ui.FormatError(issue.String()))
	}

	word := "issue"
	if len(issues) != 1 {
		word = "issues"
	}
	fmt.Printf("\n%s\n\n", ui.FormatError(fmt.Sprintf("%d %s found.", len(issues), word)))
	os.Exit(1)
}

// semanticCheck runs all semantic validations on a parsed rule set.
func semanticCheck(rules []parser.Rule) []validateIssue {
	var issues []validateIssue
	issues = append(issues, checkAfterReferences(rules)...)
	issues = append(issues, checkOSFilters(rules)...)
	return issues
}

// checkAfterReferences flags after: entries that don't resolve to any rule id:
// or primary resource key in the rule set.
func checkAfterReferences(rules []parser.Rule) []validateIssue {
	// Build the set of resolvable keys: all rule IDs and all primary resource keys.
	keys := map[string]bool{}
	for _, r := range rules {
		if r.ID != "" {
			keys[r.ID] = true
		}
		if k := primaryKey(r); k != "" {
			keys[k] = true
		}
	}

	var issues []validateIssue
	for i, r := range rules {
		for _, dep := range r.After {
			if !keys[dep] {
				issues = append(issues, validateIssue{
					line:    i + 1,
					summary: ruleLabel(r),
					message: fmt.Sprintf("after: %q does not match any rule id or resource", dep),
				})
			}
		}
	}
	return issues
}

// checkOSFilters flags os: values that are not recognised OS names.
func checkOSFilters(rules []parser.Rule) []validateIssue {
	var issues []validateIssue
	for i, r := range rules {
		for _, osName := range r.OSList {
			if !validOSNames[osName] {
				issues = append(issues, validateIssue{
					line:    i + 1,
					summary: ruleLabel(r),
					message: fmt.Sprintf("unknown os filter %q (valid: mac, linux, windows)", osName),
				})
			}
		}
	}
	return issues
}

// ruleLabel returns a short human-readable label for a rule, used in issue messages.
func ruleLabel(r parser.Rule) string {
	if r.ID != "" {
		return r.ID
	}
	if k := primaryKey(r); k != "" {
		return r.Action + " " + k
	}
	return r.Action
}

// primaryKey returns the most meaningful single resource identifier for a rule.
// Used for after: resolution and display labels. Delegates to the action registry.
func primaryKey(r parser.Rule) string {
	return handlerskg.RuleKey(r)
}
