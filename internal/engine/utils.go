package engine

import (
	"fmt"
	handlerskg "github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func getOSName() string {
	switch runtime.GOOS {
	case "darwin":
		return "mac"
	case "linux":
		return "linux"
	case "windows":
		return "windows"
	default:
		return runtime.GOOS
	}
}


func filterRulesByOS(rules []parser.Rule) []parser.Rule {
	currentOS := getOSName()
	var filtered []parser.Rule

	for _, rule := range rules {
		// If no OS is specified, rule applies to all systems
		if len(rule.OSList) == 0 {
			filtered = append(filtered, rule)
			continue
		}

		// Check if rule applies to current OS
		for _, os := range rule.OSList {
			if strings.TrimSpace(os) == currentOS {
				filtered = append(filtered, rule)
				break
			}
		}
	}

	return filtered
}

func displayRules(rules []parser.Rule) {
	for i, rule := range rules {
		fmt.Printf("Rule #%s:\n", ui.FormatHighlight(fmt.Sprint(i+1)))
		fmt.Printf("  Action: %s\n", ui.FormatHighlight(rule.Action))

		if rule.ID != "" {
			fmt.Printf("  ID: %s\n", ui.FormatDim(rule.ID))
		}

		// Display rule-specific information using handler
		handler := handlerskg.NewHandler(rule, "", make(map[string]string))
		if handler != nil {
			handler.DisplayInfo()
		}

		if len(rule.After) > 0 {
			fmt.Print("  After: ")
			for j, dep := range rule.After {
				if j > 0 {
					fmt.Print(", ")
				}
				fmt.Print(ui.FormatHighlight(dep))
			}
			fmt.Println()
		}

		if len(rule.OSList) > 0 {
			fmt.Print("  On: ")
			for j, os := range rule.OSList {
				if j > 0 {
					fmt.Print(", ")
				}
				fmt.Print(ui.FormatDim(os))
			}
			fmt.Println()
		}

		// Display command that will be executed - use handler's GetCommand method
		if handler != nil {
			cmd := handler.GetCommand()
			if cmd != "" {
				fmt.Printf("  Command: %s\n", ui.FormatDim(cmd))
			}
		}
		fmt.Println()
	}
}

func normalizePath(filePath string) string {
	// Try to get absolute path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		// If conversion fails, just return the normalized version of the input
		return filepath.Clean(filePath)
	}
	return filepath.Clean(absPath)
}

func resolveDependencies(rules []parser.Rule) ([]parser.Rule, error) {
	if len(rules) == 0 {
		return rules, nil
	}

	// Build maps for lookup by ID and package name
	rulesByID := make(map[string]*parser.Rule)
	rulesByPackage := make(map[string]*parser.Rule)

	for i := range rules {
		if rules[i].ID != "" {
			rulesByID[rules[i].ID] = &rules[i]
		}
		for _, pkg := range rules[i].Packages {
			rulesByPackage[pkg.Name] = &rules[i]
		}
		// Also allow clone rules to be referenced by their path
		if rules[i].Action == "clone" && rules[i].ClonePath != "" {
			rulesByPackage[rules[i].ClonePath] = &rules[i]
		}
	}

	// Track visited and recursion stack for cycle detection
	visited := make(map[string]bool)
	recursionStack := make(map[string]bool)
	var sorted []parser.Rule

	// Helper function for DFS
	var visit func(rule *parser.Rule) error
	visit = func(rule *parser.Rule) error {
		ruleKey := rule.ID
		if ruleKey == "" {
			// Use handler's KeyProvider interface to get the key
			handler := handlerskg.NewHandler(*rule, "", nil)
			if handler != nil {
				if keyProvider, ok := handler.(handlerskg.KeyProvider); ok {
					ruleKey = keyProvider.GetDependencyKey()
				} else {
					// Fallback: use action as key if no KeyProvider implemented
					ruleKey = rule.Action
				}
			} else {
				// Fallback: use action as key if no handler found
				ruleKey = rule.Action
			}
		}

		if recursionStack[ruleKey] {
			return fmt.Errorf("circular dependency detected involving rule %s", ruleKey)
		}

		if visited[ruleKey] {
			return nil
		}

		recursionStack[ruleKey] = true

		// Visit dependencies first
		for _, depName := range rule.After {
			var depRule *parser.Rule

			// First try to find by ID
			if depRule = rulesByID[depName]; depRule == nil {
				// Then try to find by package name
				depRule = rulesByPackage[depName]
			}

			if depRule != nil {
				if err := visit(depRule); err != nil {
					return err
				}
			}
		}

		recursionStack[ruleKey] = false
		visited[ruleKey] = true
		sorted = append(sorted, *rule)

		return nil
	}

	// Visit all rules
	for i := range rules {
		if err := visit(&rules[i]); err != nil {
			return nil, err
		}
	}

	return sorted, nil
}


func getBlueprintDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	blueprintDir := filepath.Join(homeDir, ".blueprint")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(blueprintDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create .blueprint directory: %w", err)
	}

	return blueprintDir, nil
}
