package handlers

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	gitpkg "github.com/elpic/blueprint/internal/git"
	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/platform"
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

// MiseStatus tracks installed mise tools/versions
type MiseStatus struct {
	Tool        string `json:"tool"`
	Version     string `json:"version"`
	InstalledAt string `json:"installed_at"`
	Blueprint   string `json:"blueprint"`
	OS          string `json:"os"`
}

// SudoersStatus tracks a sudoers entry added for a user
type SudoersStatus struct {
	User      string `json:"user"`
	AddedAt   string `json:"added_at"`
	Blueprint string `json:"blueprint"`
	OS        string `json:"os"`
}

// ScheduleStatus tracks a crontab schedule entry
type ScheduleStatus struct {
	CronExpr    string `json:"cron_expr"`
	Source      string `json:"source"`
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

// OllamaStatus tracks installed ollama models
type OllamaStatus struct {
	Model       string `json:"model"`
	InstalledAt string `json:"installed_at"`
	Blueprint   string `json:"blueprint"`
	OS          string `json:"os"`
}

// DownloadStatus tracks a downloaded file
type DownloadStatus struct {
	URL          string `json:"url"`
	Path         string `json:"path"`
	DownloadedAt string `json:"downloaded_at"`
	Blueprint    string `json:"blueprint"`
	OS           string `json:"os"`
}

// RunStatus tracks an executed run/run-sh command
type RunStatus struct {
	Action    string `json:"action"`  // "run" or "run-sh"
	Command   string `json:"command"` // The run command or script URL
	UndoCmd   string `json:"undo_cmd,omitempty"`
	Sudo      bool   `json:"sudo,omitempty"` // Whether sudo was used
	RanAt     string `json:"ran_at"`
	Blueprint string `json:"blueprint"`
	OS        string `json:"os"`
}

// AuthorizedKeysStatus tracks an added authorized key
type AuthorizedKeysStatus struct {
	Source    string `json:"source"`
	AddedAt   string `json:"added_at"`
	Blueprint string `json:"blueprint"`
	OS        string `json:"os"`
}

// DotfilesStatus tracks a managed dotfiles repository
type DotfilesStatus struct {
	URL       string   `json:"url"`
	Path      string   `json:"path"`
	Branch    string   `json:"branch,omitempty"`
	SHA       string   `json:"sha"`   // SHA of the cloned repository
	Links     []string `json:"links"` // symlink targets created (e.g. ["/home/user/.zshrc"])
	ClonedAt  string   `json:"cloned_at"`
	Blueprint string   `json:"blueprint"`
	OS        string   `json:"os"`
}

// Status represents the current blueprint state
type Status struct {
	Packages       []PackageStatus        `json:"packages"`
	Clones         []CloneStatus          `json:"clones"`
	Decrypts       []DecryptStatus        `json:"decrypts"`
	Mkdirs         []MkdirStatus          `json:"mkdirs"`
	KnownHosts     []KnownHostsStatus     `json:"known_hosts"`
	GPGKeys        []GPGKeyStatus         `json:"gpg_keys"`
	Asdfs          []AsdfStatus           `json:"asdfs"`
	Mises          []MiseStatus           `json:"mises"`
	Sudoers        []SudoersStatus        `json:"sudoers"`
	Brews          []HomebrewStatus       `json:"brews"`
	Ollamas        []OllamaStatus         `json:"ollamas"`
	Downloads      []DownloadStatus       `json:"downloads"`
	Runs           []RunStatus            `json:"runs"`
	Dotfiles       []DotfilesStatus       `json:"dotfiles"`
	Schedules      []ScheduleStatus       `json:"schedules"`
	Shells         []ShellStatus          `json:"shells"`
	AuthorizedKeys []AuthorizedKeysStatus `json:"authorized_keys"`
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

	// IsInstalled returns true if every resource managed by this rule already has
	// a matching entry in status for the given blueprint file and OS.
	IsInstalled(status *Status, blueprintFile, osName string) bool
}

// RecordAware is an optional interface that handlers can implement to receive
// the execution records accumulated so far in the current run before Up() is called.
type RecordAware interface {
	SetCurrentRecords(records []ExecutionRecord)
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

// StateProvider is an optional interface that handlers can implement
// to expose handler-specific state as key-value pairs.
// The "summary" key is required and used as the display line in blueprint ps.
type StateProvider interface {
	// GetState returns the handler's state as key-value pairs.
	// The "summary" key is required and used for display purposes.
	GetState(isUninstall bool) map[string]string
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
	Rule      parser.Rule
	BasePath  string             // For resolving relative paths
	Container platform.Container // Dependency injection container
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

// RuleKey returns the dependency key for a rule without allocating a handler.
// This mirrors the GetDependencyKey logic of each handler type.
func RuleKey(rule parser.Rule) string {
	if rule.ID != "" {
		return rule.ID
	}
	switch rule.Action {
	case "install", "uninstall":
		if len(rule.Packages) > 0 {
			return rule.Packages[0].Name
		}
		return "install"
	case "clone":
		return rule.ClonePath
	case "decrypt":
		return rule.DecryptPath
	case "download":
		return rule.DownloadPath
	case "known_hosts":
		return rule.KnownHosts
	case "mkdir":
		return rule.Mkdir
	case "run":
		return rule.RunCommand
	case "run-sh":
		return rule.RunShURL
	case "gpg_key":
		return rule.GPGKeyring
	case "homebrew":
		if len(rule.HomebrewPackages) > 0 {
			return rule.HomebrewPackages[0]
		}
		return "homebrew"
	case "asdf":
		return "asdf"
	case "mise":
		return "mise"
	case "ollama":
		if len(rule.OllamaModels) > 0 {
			return rule.OllamaModels[0]
		}
		return "ollama"
	case "schedule":
		if rule.ScheduleSource != "" {
			return "schedule-" + rule.ScheduleSource
		}
		return "schedule"
	case "shell":
		return rule.ShellName
	case "authorized_keys":
		if rule.AuthorizedKeysFile != "" {
			return rule.AuthorizedKeysFile
		}
		return rule.AuthorizedKeysEncrypted
	default:
		return rule.Action
	}
}

// GetFallbackDependencyKey returns the handler-specific fallback key when rule.ID is not present.
// Handlers can override this method to provide their own key logic.
// Default implementation returns the action name as fallback.
func (h *BaseHandler) GetFallbackDependencyKey() string {
	return h.Rule.Action
}

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
	if len(rule.MisePackages) > 0 {
		return "mise"
	}
	if len(rule.HomebrewPackages) > 0 {
		return "homebrew"
	}
	if len(rule.OllamaModels) > 0 {
		return "ollama"
	}
	if rule.KnownHosts != "" {
		return "known_hosts"
	}
	if rule.GPGKeyring != "" {
		return "gpg-key"
	}
	if rule.DownloadURL != "" {
		return "download"
	}
	if rule.RunCommand != "" {
		return "run"
	}
	if rule.RunShURL != "" {
		return "run-sh"
	}
	if rule.DotfilesURL != "" {
		return "dotfiles"
	}
	if rule.SudoersUser != "" {
		return "sudoers"
	}
	if rule.ScheduleSource != "" {
		return "schedule"
	}
	if rule.ShellName != "" {
		return "shell"
	}
	if rule.AuthorizedKeysFile != "" || rule.AuthorizedKeysEncrypted != "" {
		return "authorized_keys"
	}
	return ""
}

// NewHandler creates a handler for the given rule
// Returns nil if the action is not recognized
func NewHandler(rule parser.Rule, basePath string, passwordCache map[string]string) Handler {
	// Create production container
	container := platform.NewContainer()

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
		return NewInstallHandler(rule, basePath, container)
	case "clone":
		return NewCloneHandler(rule, basePath, container)
	case "decrypt":
		return NewDecryptHandler(rule, basePath, passwordCache)
	case "mkdir":
		return NewMkdirHandler(rule, basePath, container)
	case "asdf":
		return NewAsdfHandler(rule, basePath)
	case "mise":
		return NewMiseHandler(rule, basePath)
	case "homebrew":
		return NewHomebrewHandler(rule, basePath)
	case "ollama":
		return NewOllamaHandler(rule, basePath)
	case "known_hosts":
		return NewKnownHostsHandler(rule, basePath)
	case "gpg-key":
		return NewGPGKeyHandlerWithPassword(rule, basePath, passwordCache["sudo"])
	case "download":
		return NewDownloadHandler(rule, basePath)
	case "run":
		return NewRunHandler(rule, basePath)
	case "run-sh":
		return NewRunShHandler(rule, basePath)
	case "dotfiles":
		return NewDotfilesHandler(rule, basePath)
	case "sudoers":
		return NewSudoersHandler(rule, basePath)
	case "schedule":
		return NewScheduleHandler(rule, basePath)
	case "shell":
		return NewShellHandler(rule, basePath)
	case "authorized_keys":
		return NewAuthorizedKeysHandler(rule, basePath, passwordCache)
	default:
		return nil
	}
}

// GetStatusProviderHandlers returns all handler instances that implement StatusProvider
// This is the single place where all handler instantiation happens for status comparisons
// Used by engine for getAutoUninstallRules and other status-related operations
func GetStatusProviderHandlers() []Handler {
	return []Handler{
		NewInstallHandlerLegacy(parser.Rule{}, ""),
		NewCloneHandlerLegacy(parser.Rule{}, ""),
		NewDecryptHandler(parser.Rule{}, "", nil),
		NewAsdfHandler(parser.Rule{}, ""),
		NewMiseHandler(parser.Rule{}, ""),
		NewHomebrewHandler(parser.Rule{}, ""),
		NewOllamaHandler(parser.Rule{}, ""),
		NewMkdirHandlerLegacy(parser.Rule{}, ""),
		NewKnownHostsHandler(parser.Rule{}, ""),
		NewGPGKeyHandler(parser.Rule{}, ""),
		NewDownloadHandler(parser.Rule{}, ""),
		NewRunHandler(parser.Rule{}, ""),
		NewRunShHandler(parser.Rule{}, ""),
		NewDotfilesHandler(parser.Rule{}, ""),
		NewSudoersHandler(parser.Rule{}, ""),
		NewScheduleHandler(parser.Rule{}, ""),
		NewShellHandler(parser.Rule{}, ""),
		NewAuthorizedKeysHandler(parser.Rule{}, "", nil),
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

// NormalizeBlueprint is the exported form of normalizeBlueprint, exposed so
// that external packages (e.g. the engine doctor) can reuse the same logic
// without duplicating it.
func NormalizeBlueprint(input string) string {
	return normalizeBlueprint(input)
}

// normalizeBlueprint normalizes a blueprint identifier for consistent storage
// and comparison. Git URLs are normalized via NormalizeGitURL (SSH/HTTPS → canonical
// lowercase HTTPS form). Local file paths are normalized via normalizePath.
func normalizeBlueprint(input string) string {
	if gitpkg.IsGitURL(input) {
		return gitpkg.NormalizeGitURL(input)
	}
	// Detect mangled git URLs: normalizePath() was previously called on git URL
	// strings, producing absolute paths like "/home/user/https:/github.com/repo.git".
	// Extract and normalize the embedded URL.
	for _, prefix := range []string{"https:/", "http:/", "git@"} {
		if idx := strings.Index(input, prefix); idx > 0 {
			embedded := input[idx:]
			if gitpkg.IsGitURL(embedded) {
				return gitpkg.NormalizeGitURL(embedded)
			}
		}
	}
	return normalizePath(input)
}

// MigrateStatus normalizes all Blueprint fields in a Status struct.
// This is a one-time migration for status files written before URL normalization
// was added, where blueprint values may have been stored with the raw SSH URL or
// with a .git suffix (e.g. "git@github.com:user/repo.git" instead of
// "https://github.com/user/repo").
func MigrateStatus(s *Status) {
	for i := range s.Packages {
		s.Packages[i].Blueprint = normalizeBlueprint(s.Packages[i].Blueprint)
	}
	for i := range s.Clones {
		s.Clones[i].Blueprint = normalizeBlueprint(s.Clones[i].Blueprint)
	}
	for i := range s.Decrypts {
		s.Decrypts[i].Blueprint = normalizeBlueprint(s.Decrypts[i].Blueprint)
	}
	for i := range s.Mkdirs {
		s.Mkdirs[i].Blueprint = normalizeBlueprint(s.Mkdirs[i].Blueprint)
	}
	for i := range s.KnownHosts {
		s.KnownHosts[i].Blueprint = normalizeBlueprint(s.KnownHosts[i].Blueprint)
	}
	for i := range s.GPGKeys {
		s.GPGKeys[i].Blueprint = normalizeBlueprint(s.GPGKeys[i].Blueprint)
	}
	for i := range s.Asdfs {
		s.Asdfs[i].Blueprint = normalizeBlueprint(s.Asdfs[i].Blueprint)
	}
	for i := range s.Mises {
		s.Mises[i].Blueprint = normalizeBlueprint(s.Mises[i].Blueprint)
	}
	for i := range s.Sudoers {
		s.Sudoers[i].Blueprint = normalizeBlueprint(s.Sudoers[i].Blueprint)
	}
	for i := range s.Brews {
		s.Brews[i].Blueprint = normalizeBlueprint(s.Brews[i].Blueprint)
	}
	for i := range s.Ollamas {
		s.Ollamas[i].Blueprint = normalizeBlueprint(s.Ollamas[i].Blueprint)
	}
	for i := range s.Downloads {
		s.Downloads[i].Blueprint = normalizeBlueprint(s.Downloads[i].Blueprint)
	}
	for i := range s.Runs {
		s.Runs[i].Blueprint = normalizeBlueprint(s.Runs[i].Blueprint)
	}
	for i := range s.Dotfiles {
		s.Dotfiles[i].Blueprint = normalizeBlueprint(s.Dotfiles[i].Blueprint)
	}
	for i := range s.Schedules {
		s.Schedules[i].Blueprint = normalizeBlueprint(s.Schedules[i].Blueprint)
	}
	for i := range s.Shells {
		s.Shells[i].Blueprint = normalizeBlueprint(s.Shells[i].Blueprint)
	}
	for i := range s.AuthorizedKeys {
		s.AuthorizedKeys[i].Blueprint = normalizeBlueprint(s.AuthorizedKeys[i].Blueprint)
	}
}

// DeduplicateStatus removes duplicate entries from each status slice.
// An entry is a duplicate when two records have the same resource key, OS, and
// blueprint after normalization — this happens when the same blueprint was applied
// twice using different URL forms (e.g. "https:/host/repo.git" and "https://host/repo").
// The last occurrence (most recent apply) is kept; earlier duplicates are removed.
func DeduplicateStatus(s *Status) {
	s.Packages = dedupPackages(s.Packages)
	s.Clones = dedupClones(s.Clones)
	s.Decrypts = dedupDecrypts(s.Decrypts)
	s.Mkdirs = dedupMkdirs(s.Mkdirs)
	s.KnownHosts = dedupKnownHosts(s.KnownHosts)
	s.GPGKeys = dedupGPGKeys(s.GPGKeys)
	s.Asdfs = dedupAsdfs(s.Asdfs)
	s.Mises = dedupMises(s.Mises)
	s.Sudoers = dedupSudoers(s.Sudoers)
	s.Brews = dedupBrews(s.Brews)
	s.Ollamas = dedupOllamas(s.Ollamas)
	s.Downloads = dedupDownloads(s.Downloads)
	s.Runs = dedupRuns(s.Runs)
	s.Dotfiles = dedupDotfiles(s.Dotfiles)
	s.Schedules = dedupSchedules(s.Schedules)
	s.Shells = dedupShells(s.Shells)
	s.AuthorizedKeys = dedupAuthorizedKeys(s.AuthorizedKeys)
}

func dedupPackages(sl []PackageStatus) []PackageStatus {
	seen := map[string]int{}
	var out []PackageStatus
	for _, v := range sl {
		key := v.Name + "\x00" + v.OS + "\x00" + v.Blueprint
		if idx, ok := seen[key]; ok {
			out[idx] = v // replace with newer
		} else {
			seen[key] = len(out)
			out = append(out, v)
		}
	}
	return out
}

func dedupClones(sl []CloneStatus) []CloneStatus {
	seen := map[string]int{}
	var out []CloneStatus
	for _, v := range sl {
		key := v.Path + "\x00" + v.OS + "\x00" + v.Blueprint
		if idx, ok := seen[key]; ok {
			out[idx] = v
		} else {
			seen[key] = len(out)
			out = append(out, v)
		}
	}
	return out
}

func dedupDecrypts(sl []DecryptStatus) []DecryptStatus {
	seen := map[string]int{}
	var out []DecryptStatus
	for _, v := range sl {
		key := v.DestPath + "\x00" + v.OS + "\x00" + v.Blueprint
		if idx, ok := seen[key]; ok {
			out[idx] = v
		} else {
			seen[key] = len(out)
			out = append(out, v)
		}
	}
	return out
}

func dedupMkdirs(sl []MkdirStatus) []MkdirStatus {
	seen := map[string]int{}
	var out []MkdirStatus
	for _, v := range sl {
		key := v.Path + "\x00" + v.OS + "\x00" + v.Blueprint
		if idx, ok := seen[key]; ok {
			out[idx] = v
		} else {
			seen[key] = len(out)
			out = append(out, v)
		}
	}
	return out
}

func dedupKnownHosts(sl []KnownHostsStatus) []KnownHostsStatus {
	seen := map[string]int{}
	var out []KnownHostsStatus
	for _, v := range sl {
		key := v.Host + "\x00" + v.OS + "\x00" + v.Blueprint
		if idx, ok := seen[key]; ok {
			out[idx] = v
		} else {
			seen[key] = len(out)
			out = append(out, v)
		}
	}
	return out
}

func dedupGPGKeys(sl []GPGKeyStatus) []GPGKeyStatus {
	seen := map[string]int{}
	var out []GPGKeyStatus
	for _, v := range sl {
		key := v.Keyring + "\x00" + v.OS + "\x00" + v.Blueprint
		if idx, ok := seen[key]; ok {
			out[idx] = v
		} else {
			seen[key] = len(out)
			out = append(out, v)
		}
	}
	return out
}

func dedupAsdfs(sl []AsdfStatus) []AsdfStatus {
	seen := map[string]int{}
	var out []AsdfStatus
	for _, v := range sl {
		key := v.Plugin + "\x00" + v.Version + "\x00" + v.OS + "\x00" + v.Blueprint
		if idx, ok := seen[key]; ok {
			out[idx] = v
		} else {
			seen[key] = len(out)
			out = append(out, v)
		}
	}
	return out
}

func dedupMises(sl []MiseStatus) []MiseStatus {
	seen := map[string]int{}
	var out []MiseStatus
	for _, v := range sl {
		key := v.Tool + "\x00" + v.Version + "\x00" + v.OS + "\x00" + v.Blueprint
		if idx, ok := seen[key]; ok {
			out[idx] = v
		} else {
			seen[key] = len(out)
			out = append(out, v)
		}
	}
	return out
}

func dedupSudoers(sl []SudoersStatus) []SudoersStatus {
	seen := map[string]int{}
	var out []SudoersStatus
	for _, v := range sl {
		key := v.User + "\x00" + v.OS + "\x00" + v.Blueprint
		if idx, ok := seen[key]; ok {
			out[idx] = v
		} else {
			seen[key] = len(out)
			out = append(out, v)
		}
	}
	return out
}

func dedupBrews(sl []HomebrewStatus) []HomebrewStatus {
	seen := map[string]int{}
	var out []HomebrewStatus
	for _, v := range sl {
		key := v.Formula + "\x00" + v.OS + "\x00" + v.Blueprint
		if idx, ok := seen[key]; ok {
			out[idx] = v
		} else {
			seen[key] = len(out)
			out = append(out, v)
		}
	}
	return out
}

func dedupOllamas(sl []OllamaStatus) []OllamaStatus {
	seen := map[string]int{}
	var out []OllamaStatus
	for _, v := range sl {
		key := v.Model + "\x00" + v.OS + "\x00" + v.Blueprint
		if idx, ok := seen[key]; ok {
			out[idx] = v
		} else {
			seen[key] = len(out)
			out = append(out, v)
		}
	}
	return out
}

func dedupDownloads(sl []DownloadStatus) []DownloadStatus {
	seen := map[string]int{}
	var out []DownloadStatus
	for _, v := range sl {
		key := v.Path + "\x00" + v.OS + "\x00" + v.Blueprint
		if idx, ok := seen[key]; ok {
			out[idx] = v
		} else {
			seen[key] = len(out)
			out = append(out, v)
		}
	}
	return out
}

func dedupRuns(sl []RunStatus) []RunStatus {
	seen := map[string]int{}
	var out []RunStatus
	for _, v := range sl {
		key := v.Command + "\x00" + v.OS + "\x00" + v.Blueprint
		if idx, ok := seen[key]; ok {
			out[idx] = v
		} else {
			seen[key] = len(out)
			out = append(out, v)
		}
	}
	return out
}

func dedupDotfiles(sl []DotfilesStatus) []DotfilesStatus {
	seen := map[string]int{}
	var out []DotfilesStatus
	for _, v := range sl {
		key := v.URL + "\x00" + v.OS + "\x00" + v.Blueprint
		if idx, ok := seen[key]; ok {
			out[idx] = v
		} else {
			seen[key] = len(out)
			out = append(out, v)
		}
	}
	return out
}

func dedupSchedules(sl []ScheduleStatus) []ScheduleStatus {
	seen := map[string]int{}
	var out []ScheduleStatus
	for _, v := range sl {
		key := v.Source + "\x00" + v.OS + "\x00" + v.Blueprint
		if idx, ok := seen[key]; ok {
			out[idx] = v
		} else {
			seen[key] = len(out)
			out = append(out, v)
		}
	}
	return out
}

func dedupShells(sl []ShellStatus) []ShellStatus {
	seen := map[string]int{}
	var out []ShellStatus
	for _, v := range sl {
		key := v.Shell + "\x00" + v.OS + "\x00" + v.Blueprint
		if idx, ok := seen[key]; ok {
			out[idx] = v
		} else {
			seen[key] = len(out)
			out = append(out, v)
		}
	}
	return out
}

func dedupAuthorizedKeys(sl []AuthorizedKeysStatus) []AuthorizedKeysStatus {
	seen := map[string]int{}
	var out []AuthorizedKeysStatus
	for _, v := range sl {
		key := v.Source + "\x00" + v.OS + "\x00" + v.Blueprint
		if idx, ok := seen[key]; ok {
			out[idx] = v
		} else {
			seen[key] = len(out)
			out = append(out, v)
		}
	}
	return out
}

func commandSuccessfullyExecuted(cmd string, records []ExecutionRecord) (*ExecutionRecord, bool) {
	var resultRecord *ExecutionRecord
	commandExecuted := false

	for i := range records {
		if records[i].Status == "success" && records[i].Command == cmd {
			resultRecord = &records[i]
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
	normalizedBlueprint := normalizeBlueprint(blueprint)
	for _, pkg := range packages {
		normalizedStoredBlueprint := normalizeBlueprint(pkg.Blueprint)
		if pkg.Name != name || normalizedStoredBlueprint != normalizedBlueprint || pkg.OS != osName {
			result = append(result, pkg)
		}
	}
	return result
}

// removeCloneStatus removes a clone from the status clones list
func removeCloneStatus(clones []CloneStatus, path string, blueprint string, osName string) []CloneStatus {
	var result []CloneStatus
	normalizedBlueprint := normalizeBlueprint(blueprint)
	for _, clone := range clones {
		normalizedStoredBlueprint := normalizeBlueprint(clone.Blueprint)
		if clone.Path != path || normalizedStoredBlueprint != normalizedBlueprint || clone.OS != osName {
			result = append(result, clone)
		}
	}
	return result
}

// removeDecryptStatus removes a decrypt from the status decrypts list
func removeDecryptStatus(decrypts []DecryptStatus, destPath string, blueprint string, osName string) []DecryptStatus {
	var result []DecryptStatus
	normalizedBlueprint := normalizeBlueprint(blueprint)
	for _, decrypt := range decrypts {
		normalizedStoredBlueprint := normalizeBlueprint(decrypt.Blueprint)
		if decrypt.DestPath != destPath || normalizedStoredBlueprint != normalizedBlueprint || decrypt.OS != osName {
			result = append(result, decrypt)
		}
	}
	return result
}

// removeKnownHostsStatus removes a known host from the status known_hosts list
func removeKnownHostsStatus(knownHosts []KnownHostsStatus, host string, blueprint string, osName string) []KnownHostsStatus {
	var result []KnownHostsStatus
	normalizedBlueprint := normalizeBlueprint(blueprint)
	for _, kh := range knownHosts {
		normalizedStoredBlueprint := normalizeBlueprint(kh.Blueprint)
		if kh.Host != host || normalizedStoredBlueprint != normalizedBlueprint || kh.OS != osName {
			result = append(result, kh)
		}
	}
	return result
}

// removeGPGKeyStatus removes a GPG key from the status gpg_keys list
func removeGPGKeyStatus(gpgKeys []GPGKeyStatus, keyring string, blueprint string, osName string) []GPGKeyStatus {
	var result []GPGKeyStatus
	normalizedBlueprint := normalizeBlueprint(blueprint)
	for _, gk := range gpgKeys {
		normalizedStoredBlueprint := normalizeBlueprint(gk.Blueprint)
		if gk.Keyring != keyring || normalizedStoredBlueprint != normalizedBlueprint || gk.OS != osName {
			result = append(result, gk)
		}
	}
	return result
}

// removeRunStatus removes a run entry from the status runs list by command key
func removeRunStatus(runs []RunStatus, command string, blueprint string, osName string) []RunStatus {
	var result []RunStatus
	normalizedBlueprint := normalizeBlueprint(blueprint)
	for _, r := range runs {
		normalizedStoredBlueprint := normalizeBlueprint(r.Blueprint)
		if r.Command != command || normalizedStoredBlueprint != normalizedBlueprint || r.OS != osName {
			result = append(result, r)
		}
	}
	return result
}

// removeDotfilesStatus removes a dotfiles entry from the status dotfiles list
func removeDotfilesStatus(dotfiles []DotfilesStatus, url, blueprint, osName string) []DotfilesStatus {
	var result []DotfilesStatus
	normalizedBlueprint := normalizeBlueprint(blueprint)
	for _, d := range dotfiles {
		normalizedStoredBlueprint := normalizeBlueprint(d.Blueprint)
		if d.URL != url || normalizedStoredBlueprint != normalizedBlueprint || d.OS != osName {
			result = append(result, d)
		}
	}
	return result
}

// removeDownloadStatus removes a download from the status downloads list
func removeDownloadStatus(downloads []DownloadStatus, path string, blueprint string, osName string) []DownloadStatus {
	var result []DownloadStatus
	normalizedBlueprint := normalizeBlueprint(blueprint)
	for _, dl := range downloads {
		normalizedStoredBlueprint := normalizeBlueprint(dl.Blueprint)
		if dl.Path != path || normalizedStoredBlueprint != normalizedBlueprint || dl.OS != osName {
			result = append(result, dl)
		}
	}
	return result
}

// removeAuthorizedKeysStatus removes an authorized keys entry from the status list
func removeAuthorizedKeysStatus(authorizedKeys []AuthorizedKeysStatus, source string, blueprint string, osName string) []AuthorizedKeysStatus {
	var result []AuthorizedKeysStatus
	normalizedBlueprint := normalizeBlueprint(blueprint)
	for _, ak := range authorizedKeys {
		normalizedStoredBlueprint := normalizeBlueprint(ak.Blueprint)
		if ak.Source != source || normalizedStoredBlueprint != normalizedBlueprint || ak.OS != osName {
			result = append(result, ak)
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
