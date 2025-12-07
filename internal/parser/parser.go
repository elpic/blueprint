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
	ID        string     // Unique identifier for this rule
	Action    string     // "install", "uninstall", or "clone"
	Packages  []Package
	OSList    []string
	Tool      string
	After     []string   // List of IDs or package names this rule depends on
	Group     string

	// Clone-specific fields
	CloneURL  string     // Git repository URL
	ClonePath string     // Destination path for cloned repository
	Branch    string     // Branch to clone (optional, defaults to repo default)

	// ASDF-specific fields
	AsdfPackages []string // List of "plugin@version" for asdf (e.g., "nodejs@21.4.0")

	// Decrypt-specific fields
	DecryptFile     string // Source encrypted file
	DecryptPath     string // Destination path for decrypted file
	DecryptPasswordID string // Password ID to use for decryption
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
		} else if strings.HasPrefix(line, "clone ") {
			// Parse format: clone <url> to: <path> [branch: <branch>] [id: <id>] on: [<platforms>]
			rule := parseCloneRule(line)
			if rule != nil {
				rules = append(rules, *rule)
			}
		} else if strings.HasPrefix(line, "asdf") {
			// Parse format: asdf [id: <id>] [after: <deps>] on: [<platforms>]
			rule := parseAsdfRule(line)
			if rule != nil {
				rules = append(rules, *rule)
			}
		} else if strings.HasPrefix(line, "decrypt ") {
			// Parse format: decrypt <file> to: <path> [group: <group>] [password-id: <id>] [id: <id>] [after: <deps>] on: [<platforms>]
			rule := parseDecryptRule(line)
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

	// Split by "on:" to get OS list (on: is optional)
	parts := strings.Split(line, "on:")
	var osListStr string
	var rulePart string

	if len(parts) == 2 {
		// "on:" clause is present
		osListStr = strings.TrimSpace(parts[1])
		rulePart = strings.TrimSpace(parts[0])
	} else if len(parts) == 1 {
		// No "on:" clause - rule applies to all systems
		rulePart = strings.TrimSpace(parts[0])
		osListStr = ""
	} else {
		return nil
	}

	// Parse OS list [linux, mac, windows]
	var osList []string
	if osListStr != "" {
		osListStr = strings.Trim(osListStr, "[]")
		osList = strings.Split(osListStr, ",")
		for i := range osList {
			osList[i] = strings.TrimSpace(osList[i])
		}
	}

	// Parse rule part: extract id: and after: clauses
	var id string
	var dependencies []string

	// Extract id: value
	if strings.Contains(rulePart, "id:") {
		idParts := strings.Split(rulePart, "id:")
		if len(idParts) >= 2 {
			idValue := strings.TrimSpace(idParts[1])
			// Get the ID (first word after id:)
			idFields := strings.Fields(idValue)
			if len(idFields) > 0 {
				id = idFields[0]
				// Reconstruct rulePart without the id: part
				rulePart = idParts[0] + " " + strings.Join(idFields[1:], " ")
			}
		}
	}

	// Extract after: value
	if strings.Contains(rulePart, "after:") {
		afterParts := strings.Split(rulePart, "after:")
		if len(afterParts) >= 2 {
			afterValue := strings.TrimSpace(afterParts[1])
			// Parse comma-separated dependencies
			deps := strings.Split(afterValue, ",")
			for _, dep := range deps {
				dep = strings.TrimSpace(dep)
				if dep != "" {
					dependencies = append(dependencies, dep)
				}
			}
			// Reconstruct rulePart without the after: part
			rulePart = afterParts[0]
		}
	}

	// Extract package names (remaining words in rulePart)
	packageNames := strings.Fields(rulePart)
	pkgs := make([]Package, len(packageNames))
	for i, pkg := range packageNames {
		pkgs[i] = Package{Name: pkg, Version: "latest"}
	}

	return &Rule{
		ID:       id,
		Action:   "install",
		Packages: pkgs,
		OSList:   osList,
		After:    dependencies,
		Tool:     "package-manager",
	}
}

func parseCloneRule(line string) *Rule {
	// Remove "clone " prefix
	line = strings.TrimPrefix(line, "clone ")

	// Split by "on:" to get OS list (on: is optional)
	parts := strings.Split(line, "on:")
	var osListStr string
	var rulePart string

	if len(parts) == 2 {
		// "on:" clause is present
		osListStr = strings.TrimSpace(parts[1])
		rulePart = strings.TrimSpace(parts[0])
	} else if len(parts) == 1 {
		// No "on:" clause - rule applies to all systems
		rulePart = strings.TrimSpace(parts[0])
		osListStr = ""
	} else {
		return nil
	}

	// Parse OS list [linux, mac, windows]
	var osList []string
	if osListStr != "" {
		osListStr = strings.Trim(osListStr, "[]")
		osList = strings.Split(osListStr, ",")
		for i := range osList {
			osList[i] = strings.TrimSpace(osList[i])
		}
	}

	// Parse rule part: extract url, to:, branch:, id:, and after: clauses
	var cloneURL string
	var clonePath string
	var branch string
	var id string
	var dependencies []string

	// Extract clone URL (first token)
	fields := strings.Fields(rulePart)
	if len(fields) == 0 {
		return nil
	}
	cloneURL = fields[0]
	rulePart = strings.Join(fields[1:], " ")

	// Extract to: value
	if strings.Contains(rulePart, "to:") {
		toParts := strings.Split(rulePart, "to:")
		if len(toParts) >= 2 {
			toValue := strings.TrimSpace(toParts[1])
			// Get the path (first word after to:)
			toFields := strings.Fields(toValue)
			if len(toFields) > 0 {
				clonePath = toFields[0]
				// Reconstruct rulePart without the to: part
				rulePart = toParts[0] + " " + strings.Join(toFields[1:], " ")
			}
		}
	}

	// Extract branch: value
	if strings.Contains(rulePart, "branch:") {
		branchParts := strings.Split(rulePart, "branch:")
		if len(branchParts) >= 2 {
			branchValue := strings.TrimSpace(branchParts[1])
			// Get the branch name (first word after branch:)
			branchFields := strings.Fields(branchValue)
			if len(branchFields) > 0 {
				branch = branchFields[0]
				// Reconstruct rulePart without the branch: part
				rulePart = branchParts[0] + " " + strings.Join(branchFields[1:], " ")
			}
		}
	}

	// Extract id: value
	if strings.Contains(rulePart, "id:") {
		idParts := strings.Split(rulePart, "id:")
		if len(idParts) >= 2 {
			idValue := strings.TrimSpace(idParts[1])
			// Get the ID (first word after id:)
			idFields := strings.Fields(idValue)
			if len(idFields) > 0 {
				id = idFields[0]
				// Reconstruct rulePart without the id: part
				rulePart = idParts[0] + " " + strings.Join(idFields[1:], " ")
			}
		}
	}

	// Extract after: value
	if strings.Contains(rulePart, "after:") {
		afterParts := strings.Split(rulePart, "after:")
		if len(afterParts) >= 2 {
			afterValue := strings.TrimSpace(afterParts[1])
			// Parse comma-separated dependencies
			deps := strings.Split(afterValue, ",")
			for _, dep := range deps {
				dep = strings.TrimSpace(dep)
				if dep != "" {
					dependencies = append(dependencies, dep)
				}
			}
		}
	}

	if cloneURL == "" || clonePath == "" {
		return nil
	}

	return &Rule{
		ID:        id,
		Action:    "clone",
		CloneURL:  cloneURL,
		ClonePath: clonePath,
		Branch:    branch,
		OSList:    osList,
		After:     dependencies,
		Tool:      "git",
	}
}

func parseAsdfRule(line string) *Rule {
	// Remove "asdf" prefix
	line = strings.TrimPrefix(line, "asdf")
	line = strings.TrimSpace(line)

	// Split by "on:" to get OS list (on: is optional)
	parts := strings.Split(line, "on:")
	var osListStr string
	var rulePart string

	if len(parts) == 2 {
		// "on:" clause is present
		osListStr = strings.TrimSpace(parts[1])
		rulePart = strings.TrimSpace(parts[0])
	} else if len(parts) == 1 {
		// No "on:" clause - rule applies to all systems
		rulePart = strings.TrimSpace(parts[0])
		osListStr = ""
	} else {
		return nil
	}

	// Parse OS list [linux, mac, windows]
	var osList []string
	if osListStr != "" {
		osListStr = strings.Trim(osListStr, "[]")
		osList = strings.Split(osListStr, ",")
		for i := range osList {
			osList[i] = strings.TrimSpace(osList[i])
		}
	}

	// Parse rule part: extract packages first, then id: and after: clauses
	var id string
	var dependencies []string
	var asdfPackages []string

	// Extract id: value first
	if strings.Contains(rulePart, "id:") {
		idParts := strings.Split(rulePart, "id:")
		if len(idParts) >= 2 {
			idValue := strings.TrimSpace(idParts[1])
			// Get the ID (first word after id:)
			idFields := strings.Fields(idValue)
			if len(idFields) > 0 {
				id = idFields[0]
				// Reconstruct rulePart without the id: part
				rulePart = idParts[0] + " " + strings.Join(idFields[1:], " ")
			}
		}
	}

	// Extract after: value
	if strings.Contains(rulePart, "after:") {
		afterParts := strings.Split(rulePart, "after:")
		if len(afterParts) >= 2 {
			afterValue := strings.TrimSpace(afterParts[1])
			// Parse comma-separated dependencies
			deps := strings.Split(afterValue, ",")
			for _, dep := range deps {
				dep = strings.TrimSpace(dep)
				if dep != "" {
					dependencies = append(dependencies, dep)
				}
			}
			// Reconstruct rulePart without the after: part
			rulePart = afterParts[0]
		}
	}

	// Extract asdf packages (plugin@version format)
	// Remaining text in rulePart should be space-separated plugin@version pairs
	rulePart = strings.TrimSpace(rulePart)
	if rulePart != "" {
		// Split by spaces and keep only valid plugin@version pairs
		fields := strings.Fields(rulePart)
		for _, field := range fields {
			// Valid asdf package format: name@version or just name
			if strings.Contains(field, "@") || !strings.Contains(field, ":") {
				asdfPackages = append(asdfPackages, field)
			}
		}
	}

	// If no ID is provided, use "asdf" as the ID
	if id == "" {
		id = "asdf"
	}

	return &Rule{
		ID:           id,
		Action:       "asdf",
		OSList:       osList,
		After:        dependencies,
		Tool:         "asdf-vm",
		AsdfPackages: asdfPackages,
	}
}

func parseDecryptRule(line string) *Rule {
	// Remove "decrypt " prefix
	line = strings.TrimPrefix(line, "decrypt ")

	// Split by "on:" to get OS list (on: is optional)
	parts := strings.Split(line, "on:")
	var osListStr string
	var rulePart string

	if len(parts) == 2 {
		// "on:" clause is present
		osListStr = strings.TrimSpace(parts[1])
		rulePart = strings.TrimSpace(parts[0])
	} else if len(parts) == 1 {
		// No "on:" clause - rule applies to all systems
		rulePart = strings.TrimSpace(parts[0])
		osListStr = ""
	} else {
		return nil
	}

	// Parse OS list [linux, mac, windows]
	var osList []string
	if osListStr != "" {
		osListStr = strings.Trim(osListStr, "[]")
		osList = strings.Split(osListStr, ",")
		for i := range osList {
			osList[i] = strings.TrimSpace(osList[i])
		}
	}

	// Parse rule part: extract encrypted file, to:, group:, password-id:, id:, and after: clauses
	var encryptedFile string
	var decryptPath string
	var group string
	var passwordID string
	var id string
	var dependencies []string

	// Extract encrypted file (first token)
	fields := strings.Fields(rulePart)
	if len(fields) == 0 {
		return nil
	}
	encryptedFile = fields[0]
	rulePart = strings.Join(fields[1:], " ")

	// Extract to: value
	if strings.Contains(rulePart, "to:") {
		toParts := strings.Split(rulePart, "to:")
		if len(toParts) >= 2 {
			toValue := strings.TrimSpace(toParts[1])
			// Get the path (first word after to:)
			toFields := strings.Fields(toValue)
			if len(toFields) > 0 {
				decryptPath = toFields[0]
				// Reconstruct rulePart without the to: part
				rulePart = toParts[0] + " " + strings.Join(toFields[1:], " ")
			}
		}
	}

	// Extract group: value
	if strings.Contains(rulePart, "group:") {
		groupParts := strings.Split(rulePart, "group:")
		if len(groupParts) >= 2 {
			groupValue := strings.TrimSpace(groupParts[1])
			// Get the group name (first word after group:)
			groupFields := strings.Fields(groupValue)
			if len(groupFields) > 0 {
				group = groupFields[0]
				// Reconstruct rulePart without the group: part
				rulePart = groupParts[0] + " " + strings.Join(groupFields[1:], " ")
			}
		}
	}

	// Extract password-id: value
	if strings.Contains(rulePart, "password-id:") {
		passwordParts := strings.Split(rulePart, "password-id:")
		if len(passwordParts) >= 2 {
			passwordValue := strings.TrimSpace(passwordParts[1])
			// Get the password ID (first word after password-id:)
			passwordFields := strings.Fields(passwordValue)
			if len(passwordFields) > 0 {
				passwordID = passwordFields[0]
				// Reconstruct rulePart without the password-id: part
				rulePart = passwordParts[0] + " " + strings.Join(passwordFields[1:], " ")
			}
		}
	}

	// Extract id: value
	if strings.Contains(rulePart, "id:") {
		idParts := strings.Split(rulePart, "id:")
		if len(idParts) >= 2 {
			idValue := strings.TrimSpace(idParts[1])
			// Get the ID (first word after id:)
			idFields := strings.Fields(idValue)
			if len(idFields) > 0 {
				id = idFields[0]
				// Reconstruct rulePart without the id: part
				rulePart = idParts[0] + " " + strings.Join(idFields[1:], " ")
			}
		}
	}

	// Extract after: value
	if strings.Contains(rulePart, "after:") {
		afterParts := strings.Split(rulePart, "after:")
		if len(afterParts) >= 2 {
			afterValue := strings.TrimSpace(afterParts[1])
			// Parse comma-separated dependencies
			deps := strings.Split(afterValue, ",")
			for _, dep := range deps {
				dep = strings.TrimSpace(dep)
				if dep != "" {
					dependencies = append(dependencies, dep)
				}
			}
		}
	}

	if encryptedFile == "" || decryptPath == "" {
		return nil
	}

	return &Rule{
		ID:                id,
		Action:            "decrypt",
		DecryptFile:       encryptedFile,
		DecryptPath:       decryptPath,
		Group:             group,
		DecryptPasswordID: passwordID,
		OSList:            osList,
		After:             dependencies,
		Tool:              "decrypt",
	}
}

