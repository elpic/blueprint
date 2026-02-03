package handlers

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/elpic/blueprint/internal/parser"
)

// ExecutionRecord represents a single command execution
type ExecutionRecord struct {
	Timestamp string
	Blueprint string
	OS        string
	Command   string
	Output    string
	Status    string
	Error     string
}

// PackageStatus tracks an installed package
type PackageStatus struct {
	Name        string `json:"name"`
	InstalledAt string `json:"installed_at"`
	Blueprint   string `json:"blueprint"`
	OS          string `json:"os"`
}

// CloneStatus tracks a cloned repository
type CloneStatus struct {
	URL       string `json:"url"`
	Path      string `json:"path"`
	SHA       string `json:"sha"`
	ClonedAt  string `json:"cloned_at"`
	Blueprint string `json:"blueprint"`
	OS        string `json:"os"`
}

// DecryptStatus tracks a decrypted file
type DecryptStatus struct {
	SourceFile  string `json:"source_file"`
	DestPath    string `json:"dest_path"`
	DecryptedAt string `json:"decrypted_at"`
	Blueprint   string `json:"blueprint"`
	OS          string `json:"os"`
}

// MkdirStatus tracks a created directory
type MkdirStatus struct {
	Path      string `json:"path"`
	CreatedAt string `json:"created_at"`
	Blueprint string `json:"blueprint"`
	OS        string `json:"os"`
}

// KnownHostsStatus tracks an SSH known host entry
type KnownHostsStatus struct {
	Host      string `json:"host"`
	KeyType   string `json:"key_type"`
	AddedAt   string `json:"added_at"`
	Blueprint string `json:"blueprint"`
	OS        string `json:"os"`
}

// GPGKeyStatus tracks an added GPG key and repository
type GPGKeyStatus struct {
	Keyring   string `json:"keyring"`
	URL       string `json:"url"`
	DebURL    string `json:"deb_url"`
	AddedAt   string `json:"added_at"`
	Blueprint string `json:"blueprint"`
	OS        string `json:"os"`
}

// AsdfStatus tracks installed asdf plugins/versions
type AsdfStatus struct {
	Plugin      string `json:"plugin"`
	Version     string `json:"version"`
	InstalledAt string `json:"installed_at"`
	Blueprint   string `json:"blueprint"`
	OS          string `json:"os"`
}

// HomebrewStatus tracks installed homebrew formulas
type HomebrewStatus struct {
	Formula     string `json:"formula"`
	Version     string `json:"version"`
	InstalledAt string `json:"installed_at"`
	Blueprint   string `json:"blueprint"`
	OS          string `json:"os"`
}

// Status represents the current blueprint state
type Status struct {
	Packages   []PackageStatus    `json:"packages"`
	Clones     []CloneStatus      `json:"clones"`
	Decrypts   []DecryptStatus    `json:"decrypts"`
	Mkdirs     []MkdirStatus      `json:"mkdirs"`
	KnownHosts []KnownHostsStatus `json:"known_hosts"`
	GPGKeys    []GPGKeyStatus     `json:"gpg_keys"`
	Asdfs      []AsdfStatus       `json:"asdfs"`
	Brews      []HomebrewStatus   `json:"brews"`
}

// Handler is the interface that all command handlers must implement
type Handler interface {
	// Up executes the action (install, clone, decrypt, etc.)
	// Returns the output message and any error
	Up() (string, error)

	// Down removes/uninstalls the resource
	// Returns the output message and any error
	Down() (string, error)

	// UpdateStatus updates the status with the result of executing this handler
	// Takes the current status, execution records, blueprint path, and OS name
	UpdateStatus(status *Status, records []ExecutionRecord, blueprint string, osName string) error

	// GetCommand returns the actual command(s) that will be executed
	// Used for display purposes (in DEBUG mode)
	GetCommand() string

	// DisplayInfo displays handler-specific information
	// Used by the engine to show rule details in plan mode
	DisplayInfo()
}

// SudoAwareHandler is an optional interface that handlers can implement
// to specify their own sudo requirements. If a handler implements this,
// the engine will use this method instead of the global needsSudo function.
// This allows handlers to override sudo detection based on their specific needs.
type SudoAwareHandler interface {
	// NeedsSudo returns true if this handler requires sudo for its operations
	NeedsSudo() bool
}

// KeyProvider is an optional interface that handlers can implement
// to specify how they should be identified in dependency resolution.
// If a handler implements this, the engine will use GetDependencyKey()
// instead of hardcoded action type checks. This makes dependency
// resolution fully extensible without modifying the engine.
type KeyProvider interface {
	// GetDependencyKey returns the unique key for this rule when no ID is present.
	// This is used for resolving dependencies in topological sort.
	// Examples: clone path, decrypt destination, mkdir path, etc.
	GetDependencyKey() string
}

// DisplayProvider is an optional interface that handlers can implement
// to specify what details should be displayed during execution.
// This eliminates hardcoded action type checks in the engine by allowing
// each handler to provide its own display information (path, packages, hostname, etc.)
type DisplayProvider interface {
	// GetDisplayDetails returns the detail to display for this rule during execution
	// and whether it should be formatted as an error/uninstall action.
	// Examples: "~/path/to/repo", "package1, package2", "github.com", "~/.ssh/config"
	// isError should be true for uninstall actions or errors
	GetDisplayDetails(isUninstall bool) string
}

// StatusProvider is an optional interface that handlers can implement
// to specify how to manage status records for auto-uninstall.
// This eliminates ALL hardcoded action type checks by allowing each handler
// to completely own the logic of comparing its status against current rules.
type StatusProvider interface {
	// FindUninstallRules compares status records against current rules and returns
	// uninstall rules for any resources that are no longer in the blueprint.
	// The handler encapsulates ALL logic for status comparison - the engine has
	// no knowledge of specific status types or field names.
	//
	// Parameters:
	//   status - The current blueprint status with all installed resources
	//   currentRules - The rules currently in the blueprint being applied
	//   blueprintFile - The blueprint file being applied (for filtering records)
	//   osName - The OS being targeted (for filtering records)
	//
	// Returns:
	//   A slice of uninstall rules for resources no longer in the blueprint
	FindUninstallRules(status *Status, currentRules []parser.Rule, blueprintFile, osName string) []parser.Rule
}

// BaseHandler contains common fields for all handlers
type BaseHandler struct {
	Rule     parser.Rule
	BasePath string // For resolving relative paths
}

// getDependencyKey is a helper function that centralizes the ID check logic.
// Handlers should call this instead of duplicating the ID check.
// If rule.ID is present, it's returned; otherwise fallbackKey is returned.
func getDependencyKey(rule parser.Rule, fallbackKey string) string {
	if rule.ID != "" {
		return rule.ID
	}
	return fallbackKey
}

// GetFallbackDependencyKey returns the handler-specific fallback key when rule.ID is not present.
// Handlers can override this method to provide their own key logic.
// Default implementation returns the action name as fallback.
func (h *BaseHandler) GetFallbackDependencyKey() string {
	return h.Rule.Action
}

// HandlerFactory creates a handler for a given rule
// passwordCache is optional and only used by DecryptHandler
type HandlerFactory func(rule parser.Rule, basePath string, passwordCache map[string]string) Handler

// DetectRuleType determines the actual rule type based on the rule's content
// This is used for "uninstall" actions where the original action is lost
func DetectRuleType(rule parser.Rule) string {
	if len(rule.Packages) > 0 {
		return "install"
	}
	if rule.CloneURL != "" {
		return "clone"
	}
	if rule.DecryptFile != "" {
		return "decrypt"
	}
	if rule.Mkdir != "" {
		return "mkdir"
	}
	if len(rule.AsdfPackages) > 0 {
		return "asdf"
	}
	if len(rule.HomebrewPackages) > 0 {
		return "homebrew"
	}
	if rule.KnownHosts != "" {
		return "known_hosts"
	}
	if rule.GPGKeyring != "" {
		return "gpg-key"
	}
	return ""
}

// NewHandler creates a handler for the given rule
// Returns nil if the action is not recognized
func NewHandler(rule parser.Rule, basePath string, passwordCache map[string]string) Handler {
	action := rule.Action

	// For uninstall actions, detect the actual rule type from the rule's content
	if action == "uninstall" {
		action = DetectRuleType(rule)
		if action == "" {
			return nil
		}
	}

	switch action {
	case "install":
		return NewInstallHandler(rule, basePath)
	case "clone":
		return NewCloneHandler(rule, basePath)
	case "decrypt":
		return NewDecryptHandler(rule, basePath, passwordCache)
	case "mkdir":
		return NewMkdirHandler(rule, basePath)
	case "asdf":
		return NewAsdfHandler(rule, basePath)
	case "homebrew":
		return NewHomebrewHandler(rule, basePath)
	case "known_hosts":
		return NewKnownHostsHandler(rule, basePath)
	case "gpg-key":
		return NewGPGKeyHandler(rule, basePath)
	default:
		return nil
	}
}

// GetHandlerFactory returns a handler factory function for the given action
// If no factory is found for the action, returns nil
func GetHandlerFactory(action string) HandlerFactory {
	factories := map[string]HandlerFactory{
		"install": func(rule parser.Rule, basePath string, _ map[string]string) Handler {
			return NewInstallHandler(rule, basePath)
		},
		"uninstall": func(rule parser.Rule, basePath string, _ map[string]string) Handler {
			return NewInstallHandler(rule, basePath)
		},
		"clone": func(rule parser.Rule, basePath string, _ map[string]string) Handler {
			return NewCloneHandler(rule, basePath)
		},
		"decrypt": func(rule parser.Rule, basePath string, passwordCache map[string]string) Handler {
			return NewDecryptHandler(rule, basePath, passwordCache)
		},
		"mkdir": func(rule parser.Rule, basePath string, _ map[string]string) Handler {
			return NewMkdirHandler(rule, basePath)
		},
		"asdf": func(rule parser.Rule, basePath string, _ map[string]string) Handler {
			return NewAsdfHandler(rule, basePath)
		},
		"homebrew": func(rule parser.Rule, basePath string, _ map[string]string) Handler {
			return NewHomebrewHandler(rule, basePath)
		},
		"known_hosts": func(rule parser.Rule, basePath string, _ map[string]string) Handler {
			return NewKnownHostsHandler(rule, basePath)
		},
		"gpg-key": func(rule parser.Rule, basePath string, _ map[string]string) Handler {
			return NewGPGKeyHandler(rule, basePath)
		},
	}

	return factories[action]
}

// GetStatusProviderHandlers returns all handler instances that implement StatusProvider
// This is the single place where all handler instantiation happens for status comparisons
// Used by engine for getAutoUninstallRules and other status-related operations
func GetStatusProviderHandlers() []Handler {
	return []Handler{
		NewInstallHandler(parser.Rule{}, ""),
		NewCloneHandler(parser.Rule{}, ""),
		NewDecryptHandler(parser.Rule{}, "", nil),
		NewAsdfHandler(parser.Rule{}, ""),
		NewHomebrewHandler(parser.Rule{}, ""),
		NewMkdirHandler(parser.Rule{}, ""),
		NewKnownHostsHandler(parser.Rule{}, ""),
		NewGPGKeyHandler(parser.Rule{}, ""),
	}
}

// Shared utility functions for status management

// normalizePath normalizes a file path to an absolute path for consistent comparison
func normalizePath(filePath string) string {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return filepath.Clean(filePath)
	}
	return filepath.Clean(absPath)
}

func commandSuccessfullyExecuted(cmd string, records []ExecutionRecord) (*ExecutionRecord, bool) {
	var resultRecord *ExecutionRecord
	commandExecuted := false

	for _, record := range records {
		if record.Status == "success" && record.Command == cmd {
			resultRecord = &record
			commandExecuted = true
			break
		}
	}

	return resultRecord, commandExecuted
}

// extractSHAFromOutput extracts the SHA from clone operation output using regex
func extractSHAFromOutput(output string) string {
	re := regexp.MustCompile(`\(SHA:\s*([a-fA-F0-9]+)\)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// removePackageStatus removes a package from the status packages list
func removePackageStatus(packages []PackageStatus, name string, blueprint string, osName string) []PackageStatus {
	var result []PackageStatus
	normalizedBlueprint := normalizePath(blueprint)
	for _, pkg := range packages {
		normalizedStoredBlueprint := normalizePath(pkg.Blueprint)
		if pkg.Name != name || normalizedStoredBlueprint != normalizedBlueprint || pkg.OS != osName {
			result = append(result, pkg)
		}
	}
	return result
}

// removeCloneStatus removes a clone from the status clones list
func removeCloneStatus(clones []CloneStatus, path string, blueprint string, osName string) []CloneStatus {
	var result []CloneStatus
	normalizedBlueprint := normalizePath(blueprint)
	for _, clone := range clones {
		normalizedStoredBlueprint := normalizePath(clone.Blueprint)
		if clone.Path != path || normalizedStoredBlueprint != normalizedBlueprint || clone.OS != osName {
			result = append(result, clone)
		}
	}
	return result
}

// removeDecryptStatus removes a decrypt from the status decrypts list
func removeDecryptStatus(decrypts []DecryptStatus, destPath string, blueprint string, osName string) []DecryptStatus {
	var result []DecryptStatus
	normalizedBlueprint := normalizePath(blueprint)
	for _, decrypt := range decrypts {
		normalizedStoredBlueprint := normalizePath(decrypt.Blueprint)
		if decrypt.DestPath != destPath || normalizedStoredBlueprint != normalizedBlueprint || decrypt.OS != osName {
			result = append(result, decrypt)
		}
	}
	return result
}

// removeKnownHostsStatus removes a known host from the status known_hosts list
func removeKnownHostsStatus(knownHosts []KnownHostsStatus, host string, blueprint string, osName string) []KnownHostsStatus {
	var result []KnownHostsStatus
	normalizedBlueprint := normalizePath(blueprint)
	for _, kh := range knownHosts {
		normalizedStoredBlueprint := normalizePath(kh.Blueprint)
		if kh.Host != host || normalizedStoredBlueprint != normalizedBlueprint || kh.OS != osName {
			result = append(result, kh)
		}
	}
	return result
}

// removeGPGKeyStatus removes a GPG key from the status gpg_keys list
func removeGPGKeyStatus(gpgKeys []GPGKeyStatus, keyring string, blueprint string, osName string) []GPGKeyStatus {
	var result []GPGKeyStatus
	normalizedBlueprint := normalizePath(blueprint)
	for _, gk := range gpgKeys {
		normalizedStoredBlueprint := normalizePath(gk.Blueprint)
		if gk.Keyring != keyring || normalizedStoredBlueprint != normalizedBlueprint || gk.OS != osName {
			result = append(result, gk)
		}
	}
	return result
}

// removeHomebrewStatus removes a homebrew formula from the status brews list
func removeHomebrewStatus(brews []HomebrewStatus, formula string, blueprint string, osName string) []HomebrewStatus {
	var result []HomebrewStatus
	normalizedBlueprint := normalizePath(blueprint)
	for _, brew := range brews {
		normalizedStoredBlueprint := normalizePath(brew.Blueprint)
		if brew.Formula != formula || normalizedStoredBlueprint != normalizedBlueprint || brew.OS != osName {
			result = append(result, brew)
		}
	}
	return result
}

// abbreviateBlueprintPath shortens blueprint paths for display
// Shows relative paths for blueprints in the repo, full paths for external ones
func abbreviateBlueprintPath(path string) string {
	// Try to get the current working directory
	cwd, err := os.Getwd()
	if err == nil && strings.HasPrefix(path, cwd) {
		// Path is within the repo, show relative path
		relPath, err := filepath.Rel(cwd, path)
		if err == nil {
			return relPath
		}
	}
	// Path is outside the repo or error getting cwd, show full path
	return path
}
