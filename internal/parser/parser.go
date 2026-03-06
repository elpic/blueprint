package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/elpic/blueprint/internal/git"
)

type Package struct {
	Name           string
	Version        string
	PackageManager string // e.g., "apt", "snap", defaults to system default
}

type Rule struct {
	ID       string // Unique identifier for this rule
	Action   string // "install", "uninstall", "clone", "mkdir", "decrypt", "asdf", "mise", "homebrew", "ollama", "known_hosts", "gpg-key", "sudoers", or "schedule"
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

	// Mise-specific fields
	MisePackages []string // List of "tool@version" for mise (e.g., "node@20", "python@3.11")
	MisePath     string   // Optional project directory for local (non-global) install

	// Sudoers-specific fields
	SudoersUser string // User to grant passwordless sudo (resolved at runtime if empty)

	// Schedule-specific fields
	SchedulePreset string // "daily", "weekly", "hourly", or ""
	ScheduleCron   string // raw cron expression (overrides preset)
	ScheduleSource string // file path, directory, or repo passed to blueprint apply

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

	// Dotfiles-specific fields
	DotfilesURL    string   // Git repository URL for dotfiles
	DotfilesBranch string   // Optional branch to checkout
	DotfilesPath   string   // Local clone path (auto-derived: ~/.blueprint/dotfiles/<repo-name>)
	DotfilesSkip   []string // Top-level entries to skip (in addition to built-ins)

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

	for lineNum, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "#!/") {
			continue
		}

		// Handle include statements
		if strings.HasPrefix(line, "include ") {
			filePath := strings.TrimPrefix(line, "include ")
			filePath = strings.TrimSpace(filePath)

			// Dispatch git URLs to the remote include handler
			if git.IsGitURL(filePath) {
				if loadedFiles[filePath] {
					fmt.Printf("Warning: Skipping circular include: %s\n", filePath)
					continue
				}
				loadedFiles[filePath] = true
				includedRules, err := loadGitInclude(filePath, loadedFiles)
				if err != nil {
					return nil, fmt.Errorf("failed to include %s: %w", filePath, err)
				}
				rules = append(rules, includedRules...)
				continue
			}

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
			continue
		}

		var (
			rule *Rule
			err  error
		)
		switch {
		case strings.HasPrefix(line, "install "):
			rule, err = parseInstallRule(line)
		case strings.HasPrefix(line, "clone "):
			rule, err = parseCloneRule(line)
		case strings.HasPrefix(line, "mise"):
			rule, err = parseMiseRule(line)
		case strings.HasPrefix(line, "asdf"):
			rule, err = parseAsdfRule(line)
		case strings.HasPrefix(line, "homebrew"):
			rule, err = parseHomebrewRule(line)
		case strings.HasPrefix(line, "ollama"):
			rule, err = parseOllamaRule(line)
		case strings.HasPrefix(line, "decrypt "):
			rule, err = parseDecryptRule(line)
		case strings.HasPrefix(line, "known_hosts "):
			rule, err = parseKnownHostsRule(line)
		case strings.HasPrefix(line, "mkdir "):
			rule, err = parseMkdirRule(line)
		case strings.HasPrefix(line, "gpg-key "):
			rule, err = parseGPGKeyRule(line)
		case strings.HasPrefix(line, "download "):
			rule, err = parseDownloadRule(line)
		case strings.HasPrefix(line, "run-sh "):
			rule, err = parseRunShRule(line)
		case strings.HasPrefix(line, "run "):
			rule, err = parseRunRule(line)
		case strings.HasPrefix(line, "dotfiles "):
			rule, err = parseDotfilesRule(line)
		case strings.HasPrefix(line, "sudoers"):
			rule, err = parseSudoersRule(line)
		case strings.HasPrefix(line, "schedule"):
			rule, err = parseScheduleRule(line)
		default:
			return nil, fmt.Errorf("line %d: unknown directive %q", lineNum+1, line)
		}

		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum+1, err)
		}
		if rule != nil {
			rules = append(rules, *rule)
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

// localPathForGitInclude derives a stable local cache path from a git URL.
// Supports HTTPS, HTTP, git://, and SSH (git@host:org/repo.git) formats.
func localPathForGitInclude(rawURL string) string {
	homeDir, _ := os.UserHomeDir()

	var normalized string
	if strings.HasPrefix(rawURL, "git@") {
		// SSH format: git@github.com:org/repo.git[@branch[:path]]
		// Take everything after "git@", replace the first ":" with "/", drop .git suffix.
		normalized = strings.TrimPrefix(rawURL, "git@")
		// Drop any branch/path specifier (everything from "@" that may follow the repo)
		// SSH repos don't use "@" again in the host:path portion, but be safe.
		if idx := strings.Index(normalized, "@"); idx >= 0 {
			normalized = normalized[:idx]
		}
		normalized = strings.Replace(normalized, ":", "/", 1)
	} else {
		// HTTPS/HTTP/git:// — strip protocol, then drop branch/path specifier (after @)
		normalized = strings.TrimPrefix(rawURL, "https://")
		normalized = strings.TrimPrefix(normalized, "http://")
		normalized = strings.TrimPrefix(normalized, "git://")
		if idx := strings.Index(normalized, "@"); idx >= 0 {
			normalized = normalized[:idx]
		}
	}

	normalized = strings.TrimSuffix(normalized, ".git")
	return filepath.Join(homeDir, ".blueprint", "repos", normalized)
}

// loadGitInclude clones/updates the remote repo and parses the target blueprint file.
func loadGitInclude(rawURL string, loadedFiles map[string]bool) ([]Rule, error) {
	params := git.ParseGitURL(rawURL)
	localPath := localPathForGitInclude(rawURL)

	_, _, _, err := git.CloneOrUpdateRepository(params.URL, localPath, params.Branch)
	if err != nil {
		return nil, fmt.Errorf("failed to clone/update %s: %w", rawURL, err)
	}

	setupFile, err := git.FindSetupFile(localPath, params.Path)
	if err != nil {
		return nil, fmt.Errorf("setup file not found in %s: %w", rawURL, err)
	}

	content, err := os.ReadFile(setupFile) // #nosec G304 -- setupFile is derived from trusted clone path
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", setupFile, err)
	}
	baseDir := filepath.Dir(setupFile)
	return parseContent(string(content), baseDir, loadedFiles)
}

func parseInstallRule(line string) (*Rule, error) {
	f := parseFields(strings.TrimPrefix(line, "install "))
	packageManager := f.word("package-manager:")
	packageNames := f.tokens
	pkgs := make([]Package, len(packageNames))
	for i, pkg := range packageNames {
		pkgs[i] = Package{Name: pkg, Version: "latest", PackageManager: packageManager}
	}
	return &Rule{
		ID:       f.word("id:"),
		Action:   "install",
		Packages: pkgs,
		OSList:   f.osFilter,
		After:    f.list("after:"),
	}, nil
}

func parseCloneRule(line string) (*Rule, error) {
	f := parseFields(strings.TrimPrefix(line, "clone "))
	tokens := f.tokens
	if len(tokens) == 0 {
		return nil, lineError(line, "clone requires a URL")
	}
	cloneURL := tokens[0]
	clonePath := f.word("to:")
	if clonePath == "" {
		return nil, lineError(line, "clone requires to:")
	}
	id := f.word("id:")
	if id == "" {
		id = "clone-" + cloneURL
	}
	return &Rule{
		ID:        id,
		Action:    "clone",
		CloneURL:  cloneURL,
		ClonePath: clonePath,
		Branch:    f.word("branch:"),
		OSList:    f.osFilter,
		After:     f.list("after:"),
	}, nil
}

func parseAsdfRule(line string) (*Rule, error) {
	f := parseFields(strings.TrimPrefix(strings.TrimPrefix(line, "asdf"), " "))
	asdfPackages := f.tokens
	id := f.word("id:")
	if id == "" {
		if len(asdfPackages) > 0 {
			id = fmt.Sprintf("asdf-%s", asdfPackages[0])
		} else {
			id = "asdf"
		}
	}
	return &Rule{
		ID:           id,
		Action:       "asdf",
		OSList:       f.osFilter,
		After:        f.list("after:"),
		AsdfPackages: asdfPackages,
	}, nil
}

func parseHomebrewRule(line string) (*Rule, error) {
	f := parseFields(strings.TrimPrefix(strings.TrimPrefix(line, "homebrew"), " "))
	var homebrewPackages []string
	var homebrewCasks []string
	if cask := f.word("cask:"); cask != "" {
		homebrewCasks = append(homebrewCasks, cask)
	}
	// positional tokens are formulas
	for _, tok := range f.tokens {
		homebrewPackages = append(homebrewPackages, tok)
	}
	id := f.word("id:")
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
		OSList:           f.osFilter,
		After:            f.list("after:"),
		HomebrewPackages: homebrewPackages,
		HomebrewCasks:    homebrewCasks,
	}, nil
}

func parseOllamaRule(line string) (*Rule, error) {
	f := parseFields(strings.TrimPrefix(strings.TrimPrefix(line, "ollama"), " "))
	ollamaModels := f.tokens
	id := f.word("id:")
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
		OSList:       f.osFilter,
		After:        f.list("after:"),
		OllamaModels: ollamaModels,
	}, nil
}

func parseDecryptRule(line string) (*Rule, error) {
	f := parseFields(strings.TrimPrefix(line, "decrypt "))
	tokens := f.tokens
	if len(tokens) == 0 {
		return nil, lineError(line, "decrypt requires a source file")
	}
	encryptedFile := tokens[0]
	decryptPath := f.word("to:")
	if decryptPath == "" {
		return nil, lineError(line, "decrypt requires to:")
	}
	return &Rule{
		ID:                f.word("id:"),
		Action:            "decrypt",
		DecryptFile:       encryptedFile,
		DecryptPath:       decryptPath,
		Group:             f.word("group:"),
		DecryptPasswordID: f.word("password-id:"),
		OSList:            f.osFilter,
		After:             f.list("after:"),
	}, nil
}

func parseKnownHostsRule(line string) (*Rule, error) {
	f := parseFields(strings.TrimPrefix(line, "known_hosts "))
	tokens := f.tokens
	if len(tokens) == 0 {
		return nil, lineError(line, "known_hosts requires a hostname")
	}
	return &Rule{
		ID:            f.word("id:"),
		Action:        "known_hosts",
		KnownHosts:    tokens[0],
		KnownHostsKey: f.word("key:"),
		OSList:        f.osFilter,
		After:         f.list("after:"),
	}, nil
}

func parseMkdirRule(line string) (*Rule, error) {
	f := parseFields(strings.TrimPrefix(line, "mkdir "))
	tokens := f.tokens
	if len(tokens) == 0 {
		return nil, lineError(line, "mkdir requires a path")
	}
	// support both perms: and permissions: keywords
	perms := f.word("perms:")
	if perms == "" {
		perms = f.word("permissions:")
	}
	return &Rule{
		ID:         f.word("id:"),
		Action:     "mkdir",
		Mkdir:      tokens[0],
		MkdirPerms: perms,
		OSList:     f.osFilter,
		After:      f.list("after:"),
	}, nil
}

func parseGPGKeyRule(line string) (*Rule, error) {
	f := parseFields(strings.TrimPrefix(line, "gpg-key "))
	tokens := f.tokens
	if len(tokens) == 0 {
		return nil, lineError(line, "gpg-key requires a URL")
	}
	gpgKeyURL := tokens[0]
	keyring := f.word("keyring:")
	if keyring == "" {
		return nil, lineError(line, "gpg-key requires keyring:")
	}
	debURL := f.word("deb-url:")
	if debURL == "" {
		return nil, lineError(line, "gpg-key requires deb-url:")
	}
	return &Rule{
		ID:         f.word("id:"),
		Action:     "gpg-key",
		GPGKeyURL:  gpgKeyURL,
		GPGKeyring: keyring,
		GPGDebURL:  debURL,
		OSList:     f.osFilter,
		After:      f.list("after:"),
	}, nil
}

func parseDownloadRule(line string) (*Rule, error) {
	f := parseFields(strings.TrimPrefix(line, "download "))
	tokens := f.tokens
	if len(tokens) == 0 {
		return nil, lineError(line, "download requires a URL")
	}
	downloadURL := tokens[0]
	downloadPath := f.word("to:")
	if downloadPath == "" {
		return nil, lineError(line, "download requires to:")
	}
	return &Rule{
		ID:                f.word("id:"),
		Action:            "download",
		DownloadURL:       downloadURL,
		DownloadPath:      downloadPath,
		DownloadOverwrite: f.word("overwrite:") == "true",
		DownloadPerms:     f.word("permissions:"),
		OSList:            f.osFilter,
		After:             f.list("after:"),
	}, nil
}

func parseRunRule(line string) (*Rule, error) {
	f := parseFields(strings.TrimPrefix(line, "run "))
	runCommand := f.rest()
	if runCommand == "" {
		return nil, lineError(line, "run requires a command")
	}
	return &Rule{
		ID:         f.word("id:"),
		Action:     "run",
		RunCommand: runCommand,
		RunUnless:  f.multiword("unless:"),
		RunUndo:    f.multiword("undo:"),
		RunSudo:    f.word("sudo:") == "true",
		OSList:     f.osFilter,
		After:      f.list("after:"),
	}, nil
}

func parseRunShRule(line string) (*Rule, error) {
	f := parseFields(strings.TrimPrefix(line, "run-sh "))
	tokens := f.tokens
	if len(tokens) == 0 {
		return nil, lineError(line, "run-sh requires a URL")
	}
	return &Rule{
		ID:        f.word("id:"),
		Action:    "run-sh",
		RunShURL:  tokens[0],
		RunUnless: f.multiword("unless:"),
		RunUndo:   f.multiword("undo:"),
		RunSudo:   f.word("sudo:") == "true",
		OSList:    f.osFilter,
		After:     f.list("after:"),
	}, nil
}

func parseDotfilesRule(line string) (*Rule, error) {
	f := parseFields(strings.TrimPrefix(line, "dotfiles "))
	tokens := f.tokens
	if len(tokens) == 0 {
		return nil, lineError(line, "dotfiles requires a URL")
	}
	dotfilesURL := tokens[0]

	// Auto-derive repo name from URL (strip .git suffix, take last path segment)
	repoName := strings.TrimSuffix(dotfilesURL, ".git")
	if idx := strings.LastIndex(repoName, "/"); idx >= 0 {
		repoName = repoName[idx+1:]
	}

	id := f.word("id:")
	if id == "" {
		id = fmt.Sprintf("dotfiles-%s", repoName)
	}
	return &Rule{
		ID:             id,
		Action:         "dotfiles",
		DotfilesURL:    dotfilesURL,
		DotfilesBranch: f.word("branch:"),
		DotfilesPath:   fmt.Sprintf("~/.blueprint/dotfiles/%s", repoName),
		DotfilesSkip:   f.skipList(),
		OSList:         f.osFilter,
		After:          f.list("after:"),
	}, nil
}

func parseMiseRule(line string) (*Rule, error) {
	f := parseFields(strings.TrimPrefix(strings.TrimPrefix(line, "mise"), " "))
	misePackages := f.tokens
	id := f.word("id:")
	if id == "" {
		if len(misePackages) > 0 {
			id = fmt.Sprintf("mise-%s", misePackages[0])
		} else {
			id = "mise"
		}
	}
	return &Rule{
		ID:           id,
		Action:       "mise",
		OSList:       f.osFilter,
		After:        f.list("after:"),
		MisePackages: misePackages,
		MisePath:     f.word("path:"),
	}, nil
}

func parseSudoersRule(line string) (*Rule, error) {
	f := parseFields(strings.TrimPrefix(strings.TrimPrefix(line, "sudoers"), " "))
	id := f.word("id:")
	if id == "" {
		id = "sudoers"
	}
	return &Rule{
		ID:          id,
		Action:      "sudoers",
		OSList:      f.osFilter,
		After:       f.list("after:"),
		SudoersUser: f.word("user:"),
	}, nil
}

func parseScheduleRule(line string) (*Rule, error) {
	f := parseFields(strings.TrimPrefix(strings.TrimPrefix(line, "schedule"), " "))

	scheduleCron := f.multiword("cron:")
	scheduleSource := f.word("source:")
	var schedulePreset string
	if scheduleCron == "" {
		// preset is the first positional token
		for _, tok := range f.tokens {
			switch tok {
			case "daily", "weekly", "hourly":
				schedulePreset = tok
			}
			break
		}
	}

	id := f.word("id:")
	if id == "" {
		if schedulePreset != "" {
			id = "schedule-" + schedulePreset
		} else {
			id = "schedule-custom"
		}
	}

	return &Rule{
		ID:             id,
		Action:         "schedule",
		OSList:         f.osFilter,
		After:          f.list("after:"),
		SchedulePreset: schedulePreset,
		ScheduleCron:   scheduleCron,
		ScheduleSource: scheduleSource,
	}, nil
}
