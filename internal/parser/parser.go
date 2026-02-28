package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Package struct {
	Name           string
	Version        string
	PackageManager string // e.g., "apt", "snap", defaults to system default
}

type Rule struct {
	ID       string // Unique identifier for this rule
	Action   string // "install", "uninstall", "clone", "mkdir", "decrypt", "asdf", "homebrew", "ollama", "known_hosts", or "gpg-key"
	Packages []Package
	OSList   []string
	After    []string // List of IDs or package names this rule depends on
	Group    string

	// Clone-specific fields
	CloneURL  string // Git repository URL
	ClonePath string // Destination path for cloned repository
	Branch    string // Branch to clone (optional, defaults to repo default)

	// ASDF-specific fields
	AsdfPackages []string // List of "plugin@version" for asdf (e.g., "nodejs@21.4.0")

	// Decrypt-specific fields
	DecryptFile       string // Source encrypted file
	DecryptPath       string // Destination path for decrypted file
	DecryptPasswordID string // Password ID to use for decryption

	// KnownHosts-specific fields
	KnownHosts    string // SSH host to add to known_hosts (hostname or IP)
	KnownHostsKey string // Key type for ssh-keyscan (ed25519, ecdsa, rsa, etc.) - optional

	// Mkdir-specific fields
	Mkdir      string // Directory path to create
	MkdirPerms string // Octal permissions (e.g., "755", "700") - optional

	// GPG Key-specific fields
	GPGKeyURL  string // URL to the GPG key file
	GPGKeyring string // Name of the keyring (without path or .gpg extension)
	GPGDebURL  string // Debian repository URL

	// Homebrew-specific fields
	HomebrewPackages []string // List of "formula[@version]" for homebrew (e.g., "node@20", "git")
	HomebrewCasks    []string // List of cask names for brew install --cask (e.g., "visual-studio-code")

	// Ollama-specific fields
	OllamaModels []string // List of model names for ollama (e.g., "llama3", "codellama")

	// Download-specific fields
	DownloadURL       string // Source URL
	DownloadPath      string // Destination path
	DownloadOverwrite bool   // If true, always re-download
	DownloadPerms     string // Optional octal permissions (e.g. "0755")

	// Run-specific fields
	RunCommand string // Shell command to execute
	RunUnless  string // Skip if this command exits 0 (idempotency check)
	RunUndo    string // Execute when rule is removed from blueprint
	RunSudo    bool   // If true, prepend sudo to the command

	// Run-sh-specific fields
	RunShURL string // URL to the script to download and execute
}

// Parse parses content without include support
func Parse(content string) ([]Rule, error) {
	return parseContent(content, "", make(map[string]bool))
}

// ParseFile parses a file with include support
func ParseFile(filePath string) ([]Rule, error) {
	// Convert to absolute path first to ensure relative includes work correctly
	// regardless of the current working directory
	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve blueprint path: %w", err)
	}

	content, err := os.ReadFile(absFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// baseDir is now absolute, so all relative includes will be resolved correctly
	baseDir := filepath.Dir(absFilePath)
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
		} else if strings.HasPrefix(line, "homebrew") {
			// Parse format: homebrew [formula[@version] ...] [id: <id>] [after: <deps>] on: [<platforms>]
			rule := parseHomebrewRule(line)
			if rule != nil {
				rules = append(rules, *rule)
			}
		} else if strings.HasPrefix(line, "ollama") {
			// Parse format: ollama [model ...] [id: <id>] [after: <deps>] on: [<platforms>]
			rule := parseOllamaRule(line)
			if rule != nil {
				rules = append(rules, *rule)
			}
		} else if strings.HasPrefix(line, "decrypt ") {
			// Parse format: decrypt <file> to: <path> [group: <group>] [password-id: <id>] [id: <id>] [after: <deps>] on: [<platforms>]
			rule := parseDecryptRule(line)
			if rule != nil {
				rules = append(rules, *rule)
			}
		} else if strings.HasPrefix(line, "known_hosts ") {
			// Parse format: known_hosts <host> [id: <id>] [after: <deps>] on: [<platforms>]
			rule := parseKnownHostsRule(line)
			if rule != nil {
				rules = append(rules, *rule)
			}
		} else if strings.HasPrefix(line, "mkdir ") {
			// Parse format: mkdir <path> [permissions: <octal>] [id: <id>] [after: <deps>] on: [<platforms>]
			rule := parseMkdirRule(line)
			if rule != nil {
				rules = append(rules, *rule)
			}
		} else if strings.HasPrefix(line, "gpg-key ") {
			// Parse format: gpg-key <url> keyring: <name> deb-url: <url> [id: <id>] [after: <deps>] on: [<platforms>]
			rule := parseGPGKeyRule(line)
			if rule != nil {
				rules = append(rules, *rule)
			}
		} else if strings.HasPrefix(line, "download ") {
			// Parse format: download <url> to: <path> [overwrite: <true|false>] [permissions: <octal>] [id: <id>] [after: <deps>] on: [<platforms>]
			rule := parseDownloadRule(line)
			if rule != nil {
				rules = append(rules, *rule)
			}
		} else if strings.HasPrefix(line, "run-sh ") {
			// Parse format: run-sh <url> [unless: <check>] [sudo: <true|false>] [undo: <cmd>] [id: <id>] [after: <deps>] on: [<platforms>]
			rule := parseRunShRule(line)
			if rule != nil {
				rules = append(rules, *rule)
			}
		} else if strings.HasPrefix(line, "run ") {
			// Parse format: run <cmd> [unless: <check>] [sudo: <true|false>] [undo: <cmd>] [id: <id>] [after: <deps>] on: [<platforms>]
			rule := parseRunRule(line)
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
	if _, err := os.Stat(filePath); err != nil { // #nosec G703 -- filePath is a user-supplied blueprint path
		return nil, fmt.Errorf("file not found: %s", filePath)
	}

	// Mark as loaded
	loadedFiles[filePath] = true

	// Read file
	content, err := os.ReadFile(filePath) // #nosec G703 -- filePath is a user-supplied blueprint path
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

	// Parse rule part: extract id:, after:, and package-manager: clauses
	var id string
	var dependencies []string
	var packageManager string

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

	// Extract package-manager: value
	if strings.Contains(rulePart, "package-manager:") {
		pkgMgrParts := strings.Split(rulePart, "package-manager:")
		if len(pkgMgrParts) >= 2 {
			pkgMgrValue := strings.TrimSpace(pkgMgrParts[1])
			// Get the package manager (first word after package-manager:)
			pkgMgrFields := strings.Fields(pkgMgrValue)
			if len(pkgMgrFields) > 0 {
				packageManager = pkgMgrFields[0]
				// Reconstruct rulePart without the package-manager: part
				rulePart = pkgMgrParts[0] + " " + strings.Join(pkgMgrFields[1:], " ")
			}
		}
	}

	// Extract package names (remaining words in rulePart)
	packageNames := strings.Fields(rulePart)
	pkgs := make([]Package, len(packageNames))
	for i, pkg := range packageNames {
		pkgs[i] = Package{Name: pkg, Version: "latest", PackageManager: packageManager}
	}

	return &Rule{
		ID:       id,
		Action:   "install",
		Packages: pkgs,
		OSList:   osList,
		After:    dependencies,
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

	// If no ID is provided, generate a unique ID based on the first package
	if id == "" {
		if len(asdfPackages) > 0 {
			// Use the full package (plugin@version) to ensure uniqueness
			// e.g., "node@18" becomes "asdf-node@18"
			firstPkg := asdfPackages[0]
			id = fmt.Sprintf("asdf-%s", firstPkg)
		} else {
			// Fallback if no packages (shouldn't happen)
			id = "asdf"
		}
	}

	return &Rule{
		ID:           id,
		Action:       "asdf",
		OSList:       osList,
		After:        dependencies,
		AsdfPackages: asdfPackages,
	}
}

func parseHomebrewRule(line string) *Rule {
	// Remove "homebrew" prefix
	line = strings.TrimPrefix(line, "homebrew")
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

	// Parse rule part: extract formulas, casks, id:, and after: clauses
	var id string
	var dependencies []string
	var homebrewPackages []string
	var homebrewCasks []string

	// Extract id: value first
	if strings.Contains(rulePart, "id:") {
		idParts := strings.Split(rulePart, "id:")
		if len(idParts) >= 2 {
			idValue := strings.TrimSpace(idParts[1])
			idFields := strings.Fields(idValue)
			if len(idFields) > 0 {
				id = idFields[0]
				rulePart = idParts[0] + " " + strings.Join(idFields[1:], " ")
			}
		}
	}

	// Extract after: value
	if strings.Contains(rulePart, "after:") {
		afterParts := strings.Split(rulePart, "after:")
		if len(afterParts) >= 2 {
			afterValue := strings.TrimSpace(afterParts[1])
			deps := strings.Split(afterValue, ",")
			for _, dep := range deps {
				dep = strings.TrimSpace(dep)
				if dep != "" {
					dependencies = append(dependencies, dep)
				}
			}
			rulePart = afterParts[0]
		}
	}

	// Extract cask: — the single cask name immediately following the keyword
	if strings.Contains(rulePart, "cask:") {
		caskParts := strings.SplitN(rulePart, "cask:", 2)
		caskValue := strings.TrimSpace(caskParts[1])
		rulePart = strings.TrimSpace(caskParts[0])
		fields := strings.Fields(caskValue)
		if len(fields) > 0 {
			homebrewCasks = append(homebrewCasks, fields[0])
		}
	}

	// Remaining first token is the formula (formula or formula@version); one per rule
	rulePart = strings.TrimSpace(rulePart)
	if rulePart != "" {
		fields := strings.Fields(rulePart)
		if len(fields) > 0 && !strings.Contains(fields[0], ":") {
			homebrewPackages = append(homebrewPackages, fields[0])
		}
	}

	// If no ID is provided, generate one based on the formula or cask
	if id == "" {
		if len(homebrewPackages) > 0 {
			id = fmt.Sprintf("homebrew-%s", homebrewPackages[0])
		} else if len(homebrewCasks) > 0 {
			id = fmt.Sprintf("homebrew-cask-%s", homebrewCasks[0])
		} else {
			id = "homebrew"
		}
	}

	return &Rule{
		ID:               id,
		Action:           "homebrew",
		OSList:           osList,
		After:            dependencies,
		HomebrewPackages: homebrewPackages,
		HomebrewCasks:    homebrewCasks,
	}
}

func parseOllamaRule(line string) *Rule {
	// Remove "ollama" prefix
	line = strings.TrimPrefix(line, "ollama")
	line = strings.TrimSpace(line)

	// Split by "on:" to get OS list (on: is optional)
	parts := strings.Split(line, "on:")
	var osListStr string
	var rulePart string

	if len(parts) == 2 {
		osListStr = strings.TrimSpace(parts[1])
		rulePart = strings.TrimSpace(parts[0])
	} else if len(parts) == 1 {
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

	var id string
	var dependencies []string
	var ollamaModels []string

	// Extract id: value first
	if strings.Contains(rulePart, "id:") {
		idParts := strings.Split(rulePart, "id:")
		if len(idParts) >= 2 {
			idValue := strings.TrimSpace(idParts[1])
			idFields := strings.Fields(idValue)
			if len(idFields) > 0 {
				id = idFields[0]
				rulePart = idParts[0] + " " + strings.Join(idFields[1:], " ")
			}
		}
	}

	// Extract after: value
	if strings.Contains(rulePart, "after:") {
		afterParts := strings.Split(rulePart, "after:")
		if len(afterParts) >= 2 {
			afterValue := strings.TrimSpace(afterParts[1])
			deps := strings.Split(afterValue, ",")
			for _, dep := range deps {
				dep = strings.TrimSpace(dep)
				if dep != "" {
					dependencies = append(dependencies, dep)
				}
			}
			rulePart = afterParts[0]
		}
	}

	// Extract model names (remaining tokens)
	rulePart = strings.TrimSpace(rulePart)
	if rulePart != "" {
		fields := strings.Fields(rulePart)
		for _, field := range fields {
			if !strings.Contains(field, ":") {
				ollamaModels = append(ollamaModels, field)
			}
		}
	}

	// Auto-generate ID based on first model
	if id == "" {
		if len(ollamaModels) > 0 {
			id = fmt.Sprintf("ollama-%s", ollamaModels[0])
		} else {
			id = "ollama"
		}
	}

	return &Rule{
		ID:           id,
		Action:       "ollama",
		OSList:       osList,
		After:        dependencies,
		OllamaModels: ollamaModels,
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
	}
}

func parseKnownHostsRule(line string) *Rule {
	// Remove "known_hosts " prefix
	line = strings.TrimPrefix(line, "known_hosts ")

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

	// Parse rule part: extract host, key-type:, id:, and after: clauses
	var host string
	var keyType string
	var id string
	var dependencies []string

	// Extract host (first token)
	fields := strings.Fields(rulePart)
	if len(fields) == 0 {
		return nil
	}
	host = fields[0]
	rulePart = strings.Join(fields[1:], " ")

	// Extract key-type: value
	if strings.Contains(rulePart, "key-type:") {
		keyParts := strings.Split(rulePart, "key-type:")
		if len(keyParts) >= 2 {
			keyValue := strings.TrimSpace(keyParts[1])
			// Get the key type (first word after key-type:)
			keyFields := strings.Fields(keyValue)
			if len(keyFields) > 0 {
				keyType = keyFields[0]
				// Reconstruct rulePart without the key-type: part
				rulePart = keyParts[0] + " " + strings.Join(keyFields[1:], " ")
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

	if host == "" {
		return nil
	}

	return &Rule{
		ID:            id,
		Action:        "known_hosts",
		KnownHosts:    host,
		KnownHostsKey: keyType, // Will be empty if not specified
		OSList:        osList,
		After:         dependencies,
	}
}

func parseMkdirRule(line string) *Rule {
	// Remove "mkdir " prefix
	line = strings.TrimPrefix(line, "mkdir ")

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

	// Parse rule part: extract path, permissions:, id:, and after: clauses
	var path string
	var permissions string
	var id string
	var dependencies []string

	// Extract path (first token)
	fields := strings.Fields(rulePart)
	if len(fields) == 0 {
		return nil
	}
	path = fields[0]
	rulePart = strings.Join(fields[1:], " ")

	// Extract permissions: value
	if strings.Contains(rulePart, "permissions:") {
		permParts := strings.Split(rulePart, "permissions:")
		if len(permParts) >= 2 {
			permValue := strings.TrimSpace(permParts[1])
			// Get the permissions (first word after permissions:)
			permFields := strings.Fields(permValue)
			if len(permFields) > 0 {
				permissions = permFields[0]
				// Reconstruct rulePart without the permissions: part
				rulePart = permParts[0] + " " + strings.Join(permFields[1:], " ")
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

	if path == "" {
		return nil
	}

	return &Rule{
		ID:         id,
		Action:     "mkdir",
		Mkdir:      path,
		MkdirPerms: permissions, // Will be empty if not specified
		OSList:     osList,
		After:      dependencies,
	}
}

func parseGPGKeyRule(line string) *Rule {
	// Remove "gpg-key " prefix
	line = strings.TrimPrefix(line, "gpg-key ")

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

	// Parse rule part: extract URL, keyring:, and deb-url:
	var gpgKeyURL string
	var keyring string
	var debURL string
	var id string
	var dependencies []string

	// Extract URL (first token)
	fields := strings.Fields(rulePart)
	if len(fields) == 0 {
		return nil
	}
	gpgKeyURL = fields[0]
	rulePart = strings.Join(fields[1:], " ")

	// Extract keyring: value
	if strings.Contains(rulePart, "keyring:") {
		krParts := strings.Split(rulePart, "keyring:")
		if len(krParts) >= 2 {
			krValue := strings.TrimSpace(krParts[1])
			// Get the keyring name (first word after keyring:)
			krFields := strings.Fields(krValue)
			if len(krFields) > 0 {
				keyring = krFields[0]
				// Reconstruct rulePart without the keyring: part
				rulePart = krParts[0] + " " + strings.Join(krFields[1:], " ")
			}
		}
	}

	// Extract deb-url: value
	if strings.Contains(rulePart, "deb-url:") {
		debParts := strings.Split(rulePart, "deb-url:")
		if len(debParts) >= 2 {
			debValue := strings.TrimSpace(debParts[1])
			// Get the deb-url (we need to extract the quoted URL or single word)
			// For now, let's handle it as a single URL up to the next space or end of string
			debFields := strings.Fields(debValue)
			if len(debFields) > 0 {
				debURL = debFields[0]
				// Reconstruct rulePart without the deb-url: part
				rulePart = debParts[0] + " " + strings.Join(debFields[1:], " ")
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

	// Validate required fields
	if gpgKeyURL == "" || keyring == "" || debURL == "" {
		return nil
	}

	return &Rule{
		ID:         id,
		Action:     "gpg-key",
		GPGKeyURL:  gpgKeyURL,
		GPGKeyring: keyring,
		GPGDebURL:  debURL,
		OSList:     osList,
		After:      dependencies,
	}
}

func parseDownloadRule(line string) *Rule {
	// Remove "download " prefix
	line = strings.TrimPrefix(line, "download ")

	// Split by "on:" to get OS list (on: is optional)
	parts := strings.Split(line, "on:")
	var osListStr string
	var rulePart string

	if len(parts) == 2 {
		osListStr = strings.TrimSpace(parts[1])
		rulePart = strings.TrimSpace(parts[0])
	} else if len(parts) == 1 {
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

	// Parse rule part: extract URL, to:, overwrite:, permissions:, id:, and after: clauses
	var downloadURL string
	var downloadPath string
	var overwrite bool
	var permissions string
	var id string
	var dependencies []string

	// Extract URL (first token)
	fields := strings.Fields(rulePart)
	if len(fields) == 0 {
		return nil
	}
	downloadURL = fields[0]
	rulePart = strings.Join(fields[1:], " ")

	// Extract to: value
	if strings.Contains(rulePart, "to:") {
		toParts := strings.Split(rulePart, "to:")
		if len(toParts) >= 2 {
			toValue := strings.TrimSpace(toParts[1])
			toFields := strings.Fields(toValue)
			if len(toFields) > 0 {
				downloadPath = toFields[0]
				rulePart = toParts[0] + " " + strings.Join(toFields[1:], " ")
			}
		}
	}

	// Extract overwrite: value
	if strings.Contains(rulePart, "overwrite:") {
		owParts := strings.Split(rulePart, "overwrite:")
		if len(owParts) >= 2 {
			owValue := strings.TrimSpace(owParts[1])
			owFields := strings.Fields(owValue)
			if len(owFields) > 0 {
				overwrite = owFields[0] == "true"
				rulePart = owParts[0] + " " + strings.Join(owFields[1:], " ")
			}
		}
	}

	// Extract permissions: value
	if strings.Contains(rulePart, "permissions:") {
		permParts := strings.Split(rulePart, "permissions:")
		if len(permParts) >= 2 {
			permValue := strings.TrimSpace(permParts[1])
			permFields := strings.Fields(permValue)
			if len(permFields) > 0 {
				permissions = permFields[0]
				rulePart = permParts[0] + " " + strings.Join(permFields[1:], " ")
			}
		}
	}

	// Extract id: value
	if strings.Contains(rulePart, "id:") {
		idParts := strings.Split(rulePart, "id:")
		if len(idParts) >= 2 {
			idValue := strings.TrimSpace(idParts[1])
			idFields := strings.Fields(idValue)
			if len(idFields) > 0 {
				id = idFields[0]
				rulePart = idParts[0] + " " + strings.Join(idFields[1:], " ")
			}
		}
	}

	// Extract after: value
	if strings.Contains(rulePart, "after:") {
		afterParts := strings.Split(rulePart, "after:")
		if len(afterParts) >= 2 {
			afterValue := strings.TrimSpace(afterParts[1])
			deps := strings.Split(afterValue, ",")
			for _, dep := range deps {
				dep = strings.TrimSpace(dep)
				if dep != "" {
					dependencies = append(dependencies, dep)
				}
			}
		}
	}

	if downloadURL == "" || downloadPath == "" {
		return nil
	}

	return &Rule{
		ID:                id,
		Action:            "download",
		DownloadURL:       downloadURL,
		DownloadPath:      downloadPath,
		DownloadOverwrite: overwrite,
		DownloadPerms:     permissions,
		OSList:            osList,
		After:             dependencies,
	}
}

// extractRunKeywords extracts keyword-delimited fields from a run rule line.
// Keywords: unless:, undo:, sudo:, id:, after:
// Returns the remaining text (the command/URL) after extraction.
func extractRunKeywords(rulePart string) (runUnless, runUndo string, runSudo bool, id string, dependencies []string, remaining string) {
	// Known keyword list — order matters for multi-word values
	keywords := []string{"unless:", "undo:", "sudo:", "id:", "after:"}

	// Extract each keyword value by finding its position and taking text until the next keyword
	extractValue := func(text, keyword string) (value, rest string) {
		idx := strings.Index(text, keyword)
		if idx == -1 {
			return "", text
		}
		// Text before the keyword
		before := text[:idx]
		// Text after keyword prefix
		after := strings.TrimSpace(text[idx+len(keyword):])

		// Find where the value ends (at the next keyword)
		endIdx := len(after)
		for _, kw := range keywords {
			if kw == keyword {
				continue
			}
			if i := strings.Index(after, kw); i != -1 && i < endIdx {
				endIdx = i
			}
		}
		value = strings.TrimSpace(after[:endIdx])
		// Reassemble remaining text (before + what comes after this keyword's value)
		rest = before + " " + after[endIdx:]
		return value, rest
	}

	remaining = rulePart

	if strings.Contains(remaining, "unless:") {
		runUnless, remaining = extractValue(remaining, "unless:")
	}
	if strings.Contains(remaining, "undo:") {
		runUndo, remaining = extractValue(remaining, "undo:")
	}
	if strings.Contains(remaining, "sudo:") {
		var sudoStr string
		sudoStr, remaining = extractValue(remaining, "sudo:")
		runSudo = sudoStr == "true"
	}
	if strings.Contains(remaining, "id:") {
		var idRaw string
		idRaw, remaining = extractValue(remaining, "id:")
		// id is a single word
		fields := strings.Fields(idRaw)
		if len(fields) > 0 {
			id = fields[0]
		}
	}
	if strings.Contains(remaining, "after:") {
		var afterRaw string
		afterRaw, remaining = extractValue(remaining, "after:")
		deps := strings.Split(afterRaw, ",")
		for _, dep := range deps {
			dep = strings.TrimSpace(dep)
			if dep != "" {
				dependencies = append(dependencies, dep)
			}
		}
	}

	remaining = strings.TrimSpace(remaining)
	return
}

func parseRunRule(line string) *Rule {
	// Remove "run " prefix
	line = strings.TrimPrefix(line, "run ")

	// Split by "on:" first (rightmost split to handle URLs with on: in them)
	parts := strings.SplitN(line, " on:", 2)
	var osListStr, rulePart string
	if len(parts) == 2 {
		osListStr = strings.TrimSpace(parts[1])
		rulePart = strings.TrimSpace(parts[0])
	} else {
		rulePart = strings.TrimSpace(parts[0])
	}

	// Parse OS list
	var osList []string
	if osListStr != "" {
		osListStr = strings.Trim(osListStr, "[]")
		for _, os := range strings.Split(osListStr, ",") {
			if s := strings.TrimSpace(os); s != "" {
				osList = append(osList, s)
			}
		}
	}

	runUnless, runUndo, runSudo, id, dependencies, runCommand := extractRunKeywords(rulePart)

	if runCommand == "" {
		return nil
	}

	return &Rule{
		ID:         id,
		Action:     "run",
		RunCommand: runCommand,
		RunUnless:  runUnless,
		RunUndo:    runUndo,
		RunSudo:    runSudo,
		OSList:     osList,
		After:      dependencies,
	}
}

func parseRunShRule(line string) *Rule {
	// Remove "run-sh " prefix
	line = strings.TrimPrefix(line, "run-sh ")

	// Split by "on:" first
	parts := strings.SplitN(line, " on:", 2)
	var osListStr, rulePart string
	if len(parts) == 2 {
		osListStr = strings.TrimSpace(parts[1])
		rulePart = strings.TrimSpace(parts[0])
	} else {
		rulePart = strings.TrimSpace(parts[0])
	}

	// Parse OS list
	var osList []string
	if osListStr != "" {
		osListStr = strings.Trim(osListStr, "[]")
		for _, os := range strings.Split(osListStr, ",") {
			if s := strings.TrimSpace(os); s != "" {
				osList = append(osList, s)
			}
		}
	}

	runUnless, runUndo, runSudo, id, dependencies, remaining := extractRunKeywords(rulePart)

	// First token of remaining is the URL
	fields := strings.Fields(remaining)
	if len(fields) == 0 {
		return nil
	}
	scriptURL := fields[0]

	return &Rule{
		ID:        id,
		Action:    "run-sh",
		RunShURL:  scriptURL,
		RunUnless: runUnless,
		RunUndo:   runUndo,
		RunSudo:   runSudo,
		OSList:    osList,
		After:     dependencies,
	}
}
