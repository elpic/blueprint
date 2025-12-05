package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Package struct {
	Name    string
	Version string
}

type Rule struct {
	Action   string
	Packages []Package
	OSList   []string
	Tool     string
	After    []string
	Group    string
}

// Parse parses content without include support
func Parse(content string) ([]Rule, error) {
	return parseContent(content, "", make(map[string]bool))
}

// ParseFile parses a file with include support
func ParseFile(filePath string) ([]Rule, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	baseDir := filepath.Dir(filePath)
	return parseContent(string(content), baseDir, make(map[string]bool))
}

// parseContent parses content with optional include file support
func parseContent(content string, baseDir string, loadedFiles map[string]bool) ([]Rule, error) {
	lines := strings.Split(content, "\n")
	var rules []Rule

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle include statements
		if strings.HasPrefix(line, "include ") {
			filePath := strings.TrimPrefix(line, "include ")
			filePath = strings.TrimSpace(filePath)

			// Resolve relative paths
			if !filepath.IsAbs(filePath) && baseDir != "" {
				filePath = filepath.Join(baseDir, filePath)
			}

			// Prevent circular includes
			absPath, err := filepath.Abs(filePath)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve include path: %w", err)
			}

			if loadedFiles[absPath] {
				fmt.Printf("Warning: Skipping circular include: %s\n", filePath)
				continue
			}

			// Load included file
			includedRules, err := loadInclude(absPath, loadedFiles)
			if err != nil {
				return nil, fmt.Errorf("failed to include %s: %w", filePath, err)
			}
			rules = append(rules, includedRules...)

		} else if strings.HasPrefix(line, "install ") {
			// Parse format: install <packages> on: [<platforms>]
			rule := parseInstallRule(line)
			if rule != nil {
				rules = append(rules, *rule)
			}
		}
	}

	return rules, nil
}

// loadInclude loads and parses an included file
func loadInclude(filePath string, loadedFiles map[string]bool) ([]Rule, error) {
	// Check if file exists
	if _, err := os.Stat(filePath); err != nil {
		return nil, fmt.Errorf("file not found: %s", filePath)
	}

	// Mark as loaded
	loadedFiles[filePath] = true

	// Read file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse with base directory for nested includes
	baseDir := filepath.Dir(filePath)
	return parseContent(string(content), baseDir, loadedFiles)
}

func parseInstallRule(line string) *Rule {
	// Remove "install " prefix
	line = strings.TrimPrefix(line, "install ")

	// Split by "on:"
	parts := strings.Split(line, "on:")
	if len(parts) != 2 {
		return nil
	}

	packages := strings.Fields(strings.TrimSpace(parts[0]))
	osListStr := strings.TrimSpace(parts[1])

	// Parse OS list [linux, mac, windows]
	osListStr = strings.Trim(osListStr, "[]")
	osList := strings.Split(osListStr, ",")
	for i := range osList {
		osList[i] = strings.TrimSpace(osList[i])
	}

	// Create packages
	pkgs := make([]Package, len(packages))
	for i, pkg := range packages {
		pkgs[i] = Package{Name: pkg, Version: "latest"}
	}

	return &Rule{
		Action:   "install",
		Packages: pkgs,
		OSList:   osList,
		Tool:     "package-manager",
	}
}

