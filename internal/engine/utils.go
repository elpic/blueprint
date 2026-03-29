package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/elpic/blueprint/internal"
	gitpkg "github.com/elpic/blueprint/internal/git"
	handlerskg "github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
)

func getOSName() string {
	detector := internal.NewOSDetector()
	return detector.Name()
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

	return deduplicateRules(filtered)
}

// deduplicateRules removes duplicate rules using the same key logic as the
// topological sort, so plan and apply always show the same count.
func deduplicateRules(rules []parser.Rule) []parser.Rule {
	seen := make(map[string]bool)
	var result []parser.Rule
	for _, rule := range rules {
		key := handlerskg.RuleKey(rule)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, rule)
	}
	return result
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

	// Build maps for lookup by ID and other identifiers
	rulesByID := make(map[string]*parser.Rule)
	rulesByKey := make(map[string]*parser.Rule) // Maps handler-provided keys to rules

	for i := range rules {
		if rules[i].ID != "" {
			rulesByID[rules[i].ID] = &rules[i]
		}
		key := handlerskg.RuleKey(rules[i])
		rulesByKey[key] = &rules[i]

		// Also allow install packages to be referenced by package name
		for _, pkg := range rules[i].Packages {
			rulesByKey[pkg.Name] = &rules[i]
		}
	}

	// Track visited and recursion stack for cycle detection
	visited := make(map[string]bool)
	recursionStack := make(map[string]bool)
	var sorted []parser.Rule

	// Helper function for DFS
	var visit func(rule *parser.Rule) error
	visit = func(rule *parser.Rule) error {
		ruleKey := handlerskg.RuleKey(*rule)

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
				// Then try to find by handler-provided key or package name
				depRule = rulesByKey[depName]
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

// isGitURL returns true if the input is a git URL
func isGitURL(input string) bool {
	return gitpkg.IsGitURL(input)
}

// normalizeBlueprint normalizes a blueprint identifier for consistent storage
// and comparison. Git URLs are normalized via NormalizeGitURL (SSH/HTTPS → canonical
// lowercase HTTPS form). Local file paths are normalized via normalizePath.
func normalizeBlueprint(input string) string {
	if isGitURL(input) {
		return gitpkg.NormalizeGitURL(input)
	}
	return normalizePath(input)
}

// resolveBlueprintFile resolves a blueprint file path or git URL to a local path.
// If input is a git URL the repo is cloned/updated to a stable cache directory so
// that subsequent commands (e.g. `blueprint doctor`) can inspect its contents.
// Returns the git SHA of the resolved repo (empty string for local files).
// The caller must call cleanup() when done (no-op for git URLs since the cache is kept).
func resolveBlueprintFile(input string, verbose bool, preferSSH bool) (setupPath string, sha string, cleanup func(), err error) {
	cleanup = func() {}
	if preferSSH {
		input = gitpkg.ExpandShorthandSSH(input)
	} else {
		input = gitpkg.ExpandShorthand(input)
	}

	if gitpkg.IsGitURL(input) {
		params := gitpkg.ParseGitURL(input)
		localPath := blueprintRepoPath(input)

		_, newSHA, _, cloneErr := gitpkg.CloneOrUpdateRepository(params.URL, localPath, params.Branch)
		if cloneErr != nil {
			return "", "", cleanup, fmt.Errorf("error cloning repository: %w", cloneErr)
		}

		setupPath, err = gitpkg.FindSetupFile(localPath, params.Path)
		if err != nil {
			return "", "", cleanup, fmt.Errorf("error finding setup file: %w", err)
		}
		return setupPath, newSHA, cleanup, nil
	}

	return input, "", cleanup, nil
}

// blueprintRepoPath returns the stable local cache path for a blueprint git URL.
// Uses the same human-readable scheme as parser.localPathForGitInclude so that
// `blueprint doctor` can locate the repo without re-cloning it.
func blueprintRepoPath(rawURL string) string {
	homeDir, _ := os.UserHomeDir()
	params := gitpkg.ParseGitURL(rawURL)
	normalized := strings.TrimPrefix(params.URL, "https://")
	normalized = strings.TrimPrefix(normalized, "http://")
	normalized = strings.TrimPrefix(normalized, "git://")
	normalized = strings.TrimSuffix(normalized, ".git")
	return filepath.Join(homeDir, ".blueprint", "repos", normalized)
}

func getBlueprintDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	blueprintDir := filepath.Join(homeDir, ".blueprint")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(blueprintDir, internal.DirectoryPermission); err != nil {
		return "", fmt.Errorf("failed to create .blueprint directory: %w", err)
	}

	return blueprintDir, nil
}
