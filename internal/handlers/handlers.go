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

// StatusEntry is implemented by every *Status struct so that cross-cutting
// operations (doctor checks, dedup, migrate) can work generically without
// knowing the concrete type.
type StatusEntry interface {
	GetBlueprint() string
	SetBlueprint(string)
	GetResourceKey() string // the identity used for dedup/orphan checks (name, path, command, etc.)
	GetOS() string
}

// StatusEntry implementations for all status structs.
// Each struct implements GetBlueprint, SetBlueprint, GetOS (identical across all),
// plus GetResourceKey which is the type-specific identity field.

func (v *PackageStatus) GetBlueprint() string   { return v.Blueprint }
func (v *PackageStatus) SetBlueprint(s string)  { v.Blueprint = s }
func (v *PackageStatus) GetResourceKey() string { return v.Name }
func (v *PackageStatus) GetOS() string          { return v.OS }

func (v *CloneStatus) GetBlueprint() string   { return v.Blueprint }
func (v *CloneStatus) SetBlueprint(s string)  { v.Blueprint = s }
func (v *CloneStatus) GetResourceKey() string { return v.Path }
func (v *CloneStatus) GetOS() string          { return v.OS }

func (v *DecryptStatus) GetBlueprint() string   { return v.Blueprint }
func (v *DecryptStatus) SetBlueprint(s string)  { v.Blueprint = s }
func (v *DecryptStatus) GetResourceKey() string { return v.DestPath }
func (v *DecryptStatus) GetOS() string          { return v.OS }

func (v *MkdirStatus) GetBlueprint() string   { return v.Blueprint }
func (v *MkdirStatus) SetBlueprint(s string)  { v.Blueprint = s }
func (v *MkdirStatus) GetResourceKey() string { return v.Path }
func (v *MkdirStatus) GetOS() string          { return v.OS }

func (v *KnownHostsStatus) GetBlueprint() string   { return v.Blueprint }
func (v *KnownHostsStatus) SetBlueprint(s string)  { v.Blueprint = s }
func (v *KnownHostsStatus) GetResourceKey() string { return v.Host }
func (v *KnownHostsStatus) GetOS() string          { return v.OS }

func (v *GPGKeyStatus) GetBlueprint() string   { return v.Blueprint }
func (v *GPGKeyStatus) SetBlueprint(s string)  { v.Blueprint = s }
func (v *GPGKeyStatus) GetResourceKey() string { return v.Keyring }
func (v *GPGKeyStatus) GetOS() string          { return v.OS }

func (v *AsdfStatus) GetBlueprint() string   { return v.Blueprint }
func (v *AsdfStatus) SetBlueprint(s string)  { v.Blueprint = s }
func (v *AsdfStatus) GetResourceKey() string { return v.Plugin + "\x00" + v.Version }
func (v *AsdfStatus) GetOS() string          { return v.OS }

func (v *MiseStatus) GetBlueprint() string   { return v.Blueprint }
func (v *MiseStatus) SetBlueprint(s string)  { v.Blueprint = s }
func (v *MiseStatus) GetResourceKey() string { return v.Tool + "\x00" + v.Version }
func (v *MiseStatus) GetOS() string          { return v.OS }

func (v *SudoersStatus) GetBlueprint() string   { return v.Blueprint }
func (v *SudoersStatus) SetBlueprint(s string)  { v.Blueprint = s }
func (v *SudoersStatus) GetResourceKey() string { return v.User }
func (v *SudoersStatus) GetOS() string          { return v.OS }

func (v *HomebrewStatus) GetBlueprint() string   { return v.Blueprint }
func (v *HomebrewStatus) SetBlueprint(s string)  { v.Blueprint = s }
func (v *HomebrewStatus) GetResourceKey() string { return v.Formula }
func (v *HomebrewStatus) GetOS() string          { return v.OS }

func (v *OllamaStatus) GetBlueprint() string   { return v.Blueprint }
func (v *OllamaStatus) SetBlueprint(s string)  { v.Blueprint = s }
func (v *OllamaStatus) GetResourceKey() string { return v.Model }
func (v *OllamaStatus) GetOS() string          { return v.OS }

func (v *DownloadStatus) GetBlueprint() string   { return v.Blueprint }
func (v *DownloadStatus) SetBlueprint(s string)  { v.Blueprint = s }
func (v *DownloadStatus) GetResourceKey() string { return v.Path }
func (v *DownloadStatus) GetOS() string          { return v.OS }

func (v *RunStatus) GetBlueprint() string   { return v.Blueprint }
func (v *RunStatus) SetBlueprint(s string)  { v.Blueprint = s }
func (v *RunStatus) GetResourceKey() string { return v.Command }
func (v *RunStatus) GetOS() string          { return v.OS }

func (v *DotfilesStatus) GetBlueprint() string   { return v.Blueprint }
func (v *DotfilesStatus) SetBlueprint(s string)  { v.Blueprint = s }
func (v *DotfilesStatus) GetResourceKey() string { return v.URL }
func (v *DotfilesStatus) GetOS() string          { return v.OS }

func (v *ScheduleStatus) GetBlueprint() string   { return v.Blueprint }
func (v *ScheduleStatus) SetBlueprint(s string)  { v.Blueprint = s }
func (v *ScheduleStatus) GetResourceKey() string { return v.Source }
func (v *ScheduleStatus) GetOS() string          { return v.OS }

func (v *AuthorizedKeysStatus) GetBlueprint() string   { return v.Blueprint }
func (v *AuthorizedKeysStatus) SetBlueprint(s string)  { v.Blueprint = s }
func (v *AuthorizedKeysStatus) GetResourceKey() string { return v.Source }
func (v *AuthorizedKeysStatus) GetOS() string          { return v.OS }

// ShellStatus StatusEntry methods are defined here alongside the other 16 types.
// GetResourceKey returns the user field since shell entries are keyed by user.
func (v *ShellStatus) GetBlueprint() string   { return v.Blueprint }
func (v *ShellStatus) SetBlueprint(s string)  { v.Blueprint = s }
func (v *ShellStatus) GetResourceKey() string { return v.User }
func (v *ShellStatus) GetOS() string          { return v.OS }

// Status represents the current blueprint state
type Status struct {
	BlueprintSHA   string                 `json:"blueprint_sha,omitempty"` // git SHA of the blueprint repo at last apply
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

// AllEntries returns all status entries across every typed slice as a flat
// []StatusEntry. Entries are pointer-backed so mutations via SetBlueprint are
// reflected in the original slices.
func (s *Status) AllEntries() []StatusEntry {
	var entries []StatusEntry
	for i := range s.Packages {
		entries = append(entries, &s.Packages[i])
	}
	for i := range s.Clones {
		entries = append(entries, &s.Clones[i])
	}
	for i := range s.Decrypts {
		entries = append(entries, &s.Decrypts[i])
	}
	for i := range s.Mkdirs {
		entries = append(entries, &s.Mkdirs[i])
	}
	for i := range s.KnownHosts {
		entries = append(entries, &s.KnownHosts[i])
	}
	for i := range s.GPGKeys {
		entries = append(entries, &s.GPGKeys[i])
	}
	for i := range s.Asdfs {
		entries = append(entries, &s.Asdfs[i])
	}
	for i := range s.Mises {
		entries = append(entries, &s.Mises[i])
	}
	for i := range s.Sudoers {
		entries = append(entries, &s.Sudoers[i])
	}
	for i := range s.Brews {
		entries = append(entries, &s.Brews[i])
	}
	for i := range s.Ollamas {
		entries = append(entries, &s.Ollamas[i])
	}
	for i := range s.Downloads {
		entries = append(entries, &s.Downloads[i])
	}
	for i := range s.Runs {
		entries = append(entries, &s.Runs[i])
	}
	for i := range s.Dotfiles {
		entries = append(entries, &s.Dotfiles[i])
	}
	for i := range s.Schedules {
		entries = append(entries, &s.Schedules[i])
	}
	for i := range s.Shells {
		entries = append(entries, &s.Shells[i])
	}
	for i := range s.AuthorizedKeys {
		entries = append(entries, &s.AuthorizedKeys[i])
	}
	return entries
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
	def := GetAction(rule.Action)
	if def != nil && def.RuleKey != nil {
		return def.RuleKey(rule)
	}
	return rule.Action
}

// GetFallbackDependencyKey returns the handler-specific fallback key when rule.ID is not present.
// Handlers can override this method to provide their own key logic.
// Default implementation returns the action name as fallback.
func (h *BaseHandler) GetFallbackDependencyKey() string {
	return h.Rule.Action
}

// DetectRuleType determines the actual rule type based on the rule's content.
// This is used for "uninstall" actions where the original action is lost.
func DetectRuleType(rule parser.Rule) string {
	for _, def := range AllActions() {
		if def.Detect != nil && def.Detect(rule) {
			return def.Name
		}
	}
	return ""
}

// NewHandler creates a handler for the given rule.
// Returns nil if the action is not recognized.
func NewHandler(rule parser.Rule, basePath string, passwordCache map[string]string) Handler {
	action := rule.Action

	// For uninstall actions, detect the actual rule type from the rule's content.
	if action == "uninstall" {
		action = DetectRuleType(rule)
		if action == "" {
			return nil
		}
	}

	def := GetAction(action)
	if def == nil || def.NewHandler == nil {
		return nil
	}
	return def.NewHandler(rule, basePath, passwordCache)
}

// GetStatusProviderHandlers returns all handler instances that implement StatusProvider.
// This is used by the engine for getAutoUninstallRules and other status-related operations.
func GetStatusProviderHandlers() []Handler {
	var result []Handler
	for _, def := range AllActions() {
		if def.NewHandler == nil || def.IsAlias {
			continue
		}
		h := def.NewHandler(parser.Rule{Action: def.Name}, "", nil)
		if _, ok := h.(StatusProvider); ok {
			result = append(result, h)
		}
	}
	return result
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
	for _, e := range s.AllEntries() {
		e.SetBlueprint(normalizeBlueprint(e.GetBlueprint()))
	}
}

// filterSlice keeps only elements of sl whose pointer passes keep.
// T must be a value type whose pointer implements StatusEntry.
func filterSlice[T any, PT interface {
	*T
	StatusEntry
}](sl []T, keep func(StatusEntry) bool) []T {
	var out []T
	for i := range sl {
		if keep(PT(&sl[i])) {
			out = append(out, sl[i])
		}
	}
	return out
}

// FilterEntries rebuilds every status slice keeping only entries for which
// keep returns true. It is the single generic filtering point for all
// cross-cutting operations (orphan removal, dedup, etc.) — callers do not
// need to enumerate the concrete slice types.
func (s *Status) FilterEntries(keep func(StatusEntry) bool) {
	s.Packages = filterSlice[PackageStatus, *PackageStatus](s.Packages, keep)
	s.Clones = filterSlice[CloneStatus, *CloneStatus](s.Clones, keep)
	s.Decrypts = filterSlice[DecryptStatus, *DecryptStatus](s.Decrypts, keep)
	s.Mkdirs = filterSlice[MkdirStatus, *MkdirStatus](s.Mkdirs, keep)
	s.KnownHosts = filterSlice[KnownHostsStatus, *KnownHostsStatus](s.KnownHosts, keep)
	s.GPGKeys = filterSlice[GPGKeyStatus, *GPGKeyStatus](s.GPGKeys, keep)
	s.Asdfs = filterSlice[AsdfStatus, *AsdfStatus](s.Asdfs, keep)
	s.Mises = filterSlice[MiseStatus, *MiseStatus](s.Mises, keep)
	s.Sudoers = filterSlice[SudoersStatus, *SudoersStatus](s.Sudoers, keep)
	s.Brews = filterSlice[HomebrewStatus, *HomebrewStatus](s.Brews, keep)
	s.Ollamas = filterSlice[OllamaStatus, *OllamaStatus](s.Ollamas, keep)
	s.Downloads = filterSlice[DownloadStatus, *DownloadStatus](s.Downloads, keep)
	s.Runs = filterSlice[RunStatus, *RunStatus](s.Runs, keep)
	s.Dotfiles = filterSlice[DotfilesStatus, *DotfilesStatus](s.Dotfiles, keep)
	s.Schedules = filterSlice[ScheduleStatus, *ScheduleStatus](s.Schedules, keep)
	s.Shells = filterSlice[ShellStatus, *ShellStatus](s.Shells, keep)
	s.AuthorizedKeys = filterSlice[AuthorizedKeysStatus, *AuthorizedKeysStatus](s.AuthorizedKeys, keep)
}

// DeduplicateStatus removes duplicate entries from each status slice.
// An entry is a duplicate when two records have the same resource key, OS, and
// blueprint after normalization — this happens when the same blueprint was applied
// twice using different URL forms (e.g. "https:/host/repo.git" and "https://host/repo").
// The last occurrence (most recent apply) is kept; earlier duplicates are removed.
func DeduplicateStatus(s *Status) {
	// Build a set of pointers for the last occurrence of each (resource, os, blueprint) key.
	lastSeen := map[string]StatusEntry{}
	for _, e := range s.AllEntries() {
		key := e.GetResourceKey() + "\x00" + e.GetOS() + "\x00" + normalizeBlueprint(e.GetBlueprint())
		lastSeen[key] = e
	}
	keepSet := map[StatusEntry]bool{}
	for _, e := range lastSeen {
		keepSet[e] = true
	}
	s.FilterEntries(func(e StatusEntry) bool { return keepSet[e] })
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

// removeStatusEntry removes all entries from sl whose resource key, blueprint
// (after normalization), and OS match the given values. T must be a value type
// whose pointer implements StatusEntry — this single generic replaces the
// per-type removeXxxStatus functions that all had identical logic.
func removeStatusEntry[T any, PT interface {
	*T
	StatusEntry
}](sl []T, resourceKey, blueprint, osName string) []T {
	normalizedBlueprint := normalizeBlueprint(blueprint)
	var result []T
	for i := range sl {
		e := PT(&sl[i])
		if e.GetResourceKey() != resourceKey ||
			normalizeBlueprint(e.GetBlueprint()) != normalizedBlueprint ||
			e.GetOS() != osName {
			result = append(result, sl[i])
		}
	}
	return result
}

// Typed wrappers so call sites stay readable without repeating the type parameters.
func removePackageStatus(sl []PackageStatus, key, bp, os string) []PackageStatus {
	return removeStatusEntry[PackageStatus, *PackageStatus](sl, key, bp, os)
}
func removeCloneStatus(sl []CloneStatus, key, bp, os string) []CloneStatus {
	return removeStatusEntry[CloneStatus, *CloneStatus](sl, key, bp, os)
}
func removeDecryptStatus(sl []DecryptStatus, key, bp, os string) []DecryptStatus {
	return removeStatusEntry[DecryptStatus, *DecryptStatus](sl, key, bp, os)
}
func removeKnownHostsStatus(sl []KnownHostsStatus, key, bp, os string) []KnownHostsStatus {
	return removeStatusEntry[KnownHostsStatus, *KnownHostsStatus](sl, key, bp, os)
}
func removeGPGKeyStatus(sl []GPGKeyStatus, key, bp, os string) []GPGKeyStatus {
	return removeStatusEntry[GPGKeyStatus, *GPGKeyStatus](sl, key, bp, os)
}
func removeRunStatus(sl []RunStatus, key, bp, os string) []RunStatus {
	return removeStatusEntry[RunStatus, *RunStatus](sl, key, bp, os)
}
func removeDotfilesStatus(sl []DotfilesStatus, key, bp, os string) []DotfilesStatus {
	return removeStatusEntry[DotfilesStatus, *DotfilesStatus](sl, key, bp, os)
}
func removeDownloadStatus(sl []DownloadStatus, key, bp, os string) []DownloadStatus {
	return removeStatusEntry[DownloadStatus, *DownloadStatus](sl, key, bp, os)
}
func removeAuthorizedKeysStatus(sl []AuthorizedKeysStatus, key, bp, os string) []AuthorizedKeysStatus {
	return removeStatusEntry[AuthorizedKeysStatus, *AuthorizedKeysStatus](sl, key, bp, os)
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
