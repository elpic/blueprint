package handlers

import (
	"fmt"
	"time"

	gitpkg "github.com/elpic/blueprint/internal/git"
	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/platform"
	"github.com/elpic/blueprint/internal/ui"
)

// localSHA returns the HEAD SHA of a local repository. Var for test stubbing.
var localSHA = func(path string) string {
	return gitpkg.LocalSHA(path)
}

// remoteHeadSHA returns the remote HEAD SHA for a URL+branch. Var for test stubbing.
var remoteHeadSHA = func(url, branch string) string {
	return gitpkg.RemoteHeadSHA(url, branch)
}

// CloneHandler handles git repository cloning and cleanup
type CloneHandler struct {
	BaseHandler
}

// NewCloneHandler creates a new clone handler
func NewCloneHandler(rule parser.Rule, basePath string, container platform.Container) *CloneHandler {
	return &CloneHandler{
		BaseHandler: BaseHandler{
			Rule:      rule,
			BasePath:  basePath,
			Container: container,
		},
	}
}

// NewCloneHandlerLegacy creates a new clone handler without container (for backward compatibility)
func NewCloneHandlerLegacy(rule parser.Rule, basePath string) *CloneHandler {
	return NewCloneHandler(rule, basePath, platform.NewContainer())
}

// Up clones or updates the repository using two-stage approach with dependency injection
func (h *CloneHandler) Up() (string, error) {
	// Use two-stage clone to prevent repository pollution
	oldSHA, newSHA, status, err := gitpkg.CloneOrUpdateRepositoryTwoStage(
		h.Rule.CloneURL,
		h.Rule.ClonePath,
		h.Rule.Branch,
	)
	if err != nil {
		return "", fmt.Errorf("failed to clone/update repository: %w", err)
	}

	// Format output message with SHA tracking
	switch status {
	case "Cloned":
		if newSHA != "" {
			return fmt.Sprintf("Cloned (SHA: %s)", newSHA), nil
		}
		return "Cloned", nil

	case "Updated":
		if oldSHA != "" && newSHA != "" {
			return fmt.Sprintf("Updated (SHA changed: %s → %s) (SHA: %s)",
				oldSHA[:8], newSHA[:8], newSHA), nil
		}
		if newSHA != "" {
			return fmt.Sprintf("Updated (SHA: %s)", newSHA), nil
		}
		return "Updated", nil

	case "Synced":
		if newSHA != "" {
			return fmt.Sprintf("Synced (SHA: %s)", newSHA), nil
		}
		return "Synced", nil

	case "Already up to date":
		if newSHA != "" {
			return fmt.Sprintf("Already up to date (SHA: %s)", newSHA), nil
		}
		return "Already up to date", nil

	default:
		return status, nil
	}
}

// Down removes the cloned repository
func (h *CloneHandler) Down() (string, error) {
	clonePath := h.Container.SystemProvider().Filesystem().ExpandPath(h.Rule.ClonePath)

	// Remove directory if it exists using injected filesystem provider
	if h.Container.SystemProvider().Filesystem().Exists(clonePath) {
		err := h.Container.SystemProvider().Filesystem().RemoveDirectory(clonePath)
		if err != nil {
			return "", fmt.Errorf("failed to remove directory %s: %w", clonePath, err)
		}
		return fmt.Sprintf("Removed cloned repository at %s", clonePath), nil
	}

	return "Repository not found", nil
}

// GetCommand returns the actual command(s) that will be executed
func (h *CloneHandler) GetCommand() string {
	if h.Rule.Action == "uninstall" {
		clonePath := h.Rule.ClonePath
		return fmt.Sprintf("rm -rf %s", clonePath)
	}

	// Clone action - use go-git, so return descriptive command
	if h.Rule.Branch != "" {
		return fmt.Sprintf("git clone -b %s %s %s", h.Rule.Branch, h.Rule.CloneURL, h.Rule.ClonePath)
	}

	return fmt.Sprintf("git clone %s %s", h.Rule.CloneURL, h.Rule.ClonePath)
}

// UpdateStatus updates the status after cloning or removing a repository
func (h *CloneHandler) UpdateStatus(status *Status, records []ExecutionRecord, blueprint string, osName string) error {
	// Normalize blueprint path for consistent storage and comparison
	blueprint = normalizeBlueprint(blueprint)

	if h.Rule.Action == "clone" {
		cloneCmd := h.GetCommand()

		record, commandExecuted := commandSuccessfullyExecuted(cloneCmd, records)

		if commandExecuted {
			cloneSHA := extractSHAFromOutput(record.Output)
			// Remove existing entry if present
			status.Clones = removeCloneStatus(status.Clones, h.Rule.ClonePath, blueprint, osName)
			// Add new entry
			status.Clones = append(status.Clones, CloneStatus{
				URL:       h.Rule.CloneURL,
				Path:      h.Rule.ClonePath,
				SHA:       cloneSHA,
				ClonedAt:  time.Now().Format(time.RFC3339),
				Blueprint: blueprint,
				OS:        osName,
			})
		}
	} else if h.Rule.Action == "uninstall" && DetectRuleType(h.Rule) == "clone" {
		// Check if clone was removed by checking if directory doesn't exist
		expandedPath := h.Container.SystemProvider().Filesystem().ExpandPath(h.Rule.ClonePath)
		if !h.Container.SystemProvider().Filesystem().Exists(expandedPath) {
			// Directory has been removed, update status
			status.Clones = removeCloneStatus(status.Clones, h.Rule.ClonePath, blueprint, osName)
		}
	}

	return nil
}

// DisplayInfo displays handler-specific information
func (h *CloneHandler) DisplayInfo() {
	formatFunc := ui.FormatInfo
	if h.Rule.Action == "uninstall" {
		formatFunc = ui.FormatDim
	}

	fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("URL: %s", h.Rule.CloneURL)))
	fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Path: %s", h.Rule.ClonePath)))
	if h.Rule.Branch != "" {
		fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Branch: %s", h.Rule.Branch)))
	}
}

// DisplayStatus displays cloned repository status information
func (h *CloneHandler) DisplayStatus(clones []CloneStatus) {
	if len(clones) == 0 {
		return
	}

	// Filter out ~/.asdf (handled by AsdfHandler)
	var regularClones []CloneStatus
	for _, clone := range clones {
		if clone.Path != "~/.asdf" {
			regularClones = append(regularClones, clone)
		}
	}

	if len(regularClones) == 0 {
		return
	}

	fmt.Printf("\n%s\n", ui.FormatHighlight("Cloned Repositories:"))
	for _, clone := range regularClones {
		// Parse timestamp for display
		t, err := time.Parse(time.RFC3339, clone.ClonedAt)
		var timeStr string
		if err == nil {
			timeStr = t.Format("2006-01-02 15:04:05")
		} else {
			timeStr = clone.ClonedAt
		}

		fmt.Printf("  %s %s (%s) [%s, %s]\n",
			ui.FormatSuccess("●"),
			ui.FormatInfo(clone.Path),
			ui.FormatDim(timeStr),
			ui.FormatDim(clone.OS),
			ui.FormatDim(abbreviateBlueprintPath(clone.Blueprint)),
		)
		fmt.Printf("     %s %s\n",
			ui.FormatDim("URL:"),
			ui.FormatInfo(clone.URL),
		)
	}
}

// DisplayStatusFromStatus displays clone handler status from Status object
func (h *CloneHandler) DisplayStatusFromStatus(status *Status) {
	if status == nil || status.Clones == nil {
		return
	}
	h.DisplayStatus(status.Clones)
}

// GetDependencyKey returns the unique key for this rule in dependency resolution
func (h *CloneHandler) GetDependencyKey() string {
	return getDependencyKey(h.Rule, h.Rule.ClonePath)
}

// GetDisplayDetails returns the clone path to display during execution
func (h *CloneHandler) GetDisplayDetails(isUninstall bool) string {
	return h.Rule.ClonePath
}

// GetState returns handler-specific state as key-value pairs
func (h *CloneHandler) GetState(isUninstall bool) map[string]string {
	state := map[string]string{
		"summary": h.GetDisplayDetails(isUninstall),
		"url":     h.Rule.CloneURL,
		"path":    h.Rule.ClonePath,
	}
	if h.Rule.Branch != "" {
		state["branch"] = h.Rule.Branch
	}
	return state
}

// FindUninstallRules compares clone status against current rules and returns uninstall rules
func (h *CloneHandler) FindUninstallRules(status *Status, currentRules []parser.Rule, blueprintFile, osName string) []parser.Rule {
	normalizedBlueprint := normalizeBlueprint(blueprintFile)

	// Build set of current clone paths from clone rules (using normalized URLs for comparison)
	currentClonePaths := make(map[string]bool)
	for _, rule := range currentRules {
		if rule.Action == "clone" && rule.ClonePath != "" {
			currentClonePaths[rule.ClonePath] = true
		}
	}

	// Build set of current clone URLs (using normalized URLs for comparison)
	currentCloneURLs := make(map[string]bool)
	for _, rule := range currentRules {
		if rule.Action == "clone" && rule.CloneURL != "" {
			currentCloneURLs[gitpkg.NormalizeGitURL(rule.CloneURL)] = true
		}
	}

	// Find clones to uninstall (in status but not in current rules)
	var rules []parser.Rule
	if status.Clones != nil {
		for _, clone := range status.Clones {
			normalizedStatusBlueprint := normalizeBlueprint(clone.Blueprint)
			normalizedStatusURL := gitpkg.NormalizeGitURL(clone.URL)
			// Match by path OR by normalized URL
			isCurrent := currentClonePaths[clone.Path] || currentCloneURLs[normalizedStatusURL]
			if normalizedStatusBlueprint == normalizedBlueprint && clone.OS == osName && !isCurrent {
				// Don't uninstall asdf which is handled by AsdfHandler
				if clone.Path != "~/.asdf" {
					rules = append(rules, parser.Rule{
						Action:    "uninstall",
						ClonePath: clone.Path,
						CloneURL:  clone.URL,
						OSList:    []string{osName},
					})
				}
			}
		}
	}

	return rules
}

// IsInstalled returns true if the clone path is recorded in status AND the repository
// SHA matches the current remote HEAD. Uses clean repository storage when available,
// falls back to target directory for backward compatibility.
func (h *CloneHandler) IsInstalled(status *Status, blueprintFile, osName string) bool {
	normalizedBlueprint := normalizeBlueprint(blueprintFile)
	for _, clone := range status.Clones {
		if clone.Path != h.Rule.ClonePath || normalizeBlueprint(clone.Blueprint) != normalizedBlueprint || clone.OS != osName {
			continue
		}
		// Found a matching status entry — now check SHA currency
		remoteSHA := remoteHeadSHA(h.Rule.CloneURL, h.Rule.Branch)
		if remoteSHA == "" {
			// Cannot reach remote — trust the status entry as-is.
			return true
		}

		// Try clean repository storage first (prevents pollution issues)
		cleanSHA := gitpkg.GetCleanRepositorySHA(h.Rule.CloneURL, h.Rule.Branch)
		if cleanSHA != "" {
			// Clean storage exists, use it for SHA comparison
			return cleanSHA == remoteSHA
		}

		// Fall back to checking target directory for backward compatibility
		// This handles existing installations that don't have clean storage yet
		localSHAVal := localSHA(h.Container.SystemProvider().Filesystem().ExpandPath(h.Rule.ClonePath))
		return localSHAVal == remoteSHA
	}
	return false
}
