package handlers

import (
	"fmt"
	"path/filepath"
	"time"

	giturl "github.com/elpic/blueprint/internal/giturl"
	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/platform"
	"github.com/elpic/blueprint/internal/ui"
)

func init() {
	RegisterAction(ActionDef{
		Name:   "clone",
		Prefix: "clone ",
		NewHandler: func(rule parser.Rule, basePath string, passwordCache map[string]string) Handler {
			return NewCloneHandler(rule, basePath, platform.NewContainer())
		},
		RuleKey: func(rule parser.Rule) string {
			return rule.ClonePath
		},
		Detect: func(rule parser.Rule) bool {
			return rule.CloneURL != ""
		},
		Summary: func(rule parser.Rule) string {
			return rule.CloneURL + " → " + rule.ClonePath
		},
		OrphanIndex: func(rule parser.Rule, index func(string)) {
			index(rule.ClonePath)
		},
		ShellExport: func(rule parser.Rule, _, _ string) []string {
			// Bare layout: <path>/.git plus a worktree per branch.
			if rule.CloneBare {
				gitDir := shellHome(rule.ClonePath) + "/.git"
				branchFlag := ""
				branch := `"$(git -C "$GIT_DIR" symbolic-ref --short HEAD 2>/dev/null)"`
				if rule.Branch != "" {
					branchFlag = " -b " + shellQ(rule.Branch)
					branch = shellQ(rule.Branch)
				}
				return []string{
					fmt.Sprintf(`GIT_DIR=%s`, gitDir),
					`if [ ! -d "$GIT_DIR" ]; then`,
					fmt.Sprintf("  git clone --bare%s %s \"$GIT_DIR\" -q", branchFlag, shellQ(rule.CloneURL)),
					`fi`,
					`git -C "$GIT_DIR" config remote.origin.fetch '+refs/heads/*:refs/remotes/origin/*'`,
					`git -C "$GIT_DIR" fetch --prune -q origin`,
					`git -C "$GIT_DIR" worktree prune`,
					fmt.Sprintf(`BRANCH=%s`, branch),
					`WORKTREE="$(dirname "$GIT_DIR")/$BRANCH"`,
					`if [ ! -e "$WORKTREE/.git" ]; then`,
					`  git -C "$GIT_DIR" worktree add "$WORKTREE" "$BRANCH" 2>/dev/null || true`,
					`fi`,
				}
			}

			path := shellHome(rule.ClonePath)
			branchFlag := ""
			if rule.Branch != "" {
				branchFlag = " -b " + shellQ(rule.Branch)
			}
			cacheKey := shellQ(rule.CloneURL)
			if rule.Branch != "" {
				cacheKey = shellQ(rule.CloneURL + "@" + rule.Branch)
			}
			resetRef := "origin/HEAD"
			if rule.Branch != "" {
				resetRef = "origin/" + rule.Branch
			}
			return []string{
				fmt.Sprintf(`CLONE_CACHE="$HOME/.blueprint/repos/$(echo -n %s | shasum -a 256 | cut -c1-16)"`, cacheKey),
				`if [ -d "$CLONE_CACHE/.git" ]; then`,
				`  git -C "$CLONE_CACHE" fetch -q origin`,
				fmt.Sprintf(`  git -C "$CLONE_CACHE" reset --hard %s -q 2>/dev/null || git -C "$CLONE_CACHE" reset --hard FETCH_HEAD -q`, resetRef),
				`else`,
				`  rm -rf "$CLONE_CACHE"`,
				fmt.Sprintf("  git clone%s %s \"$CLONE_CACHE\" -q", branchFlag, shellQ(rule.CloneURL)),
				`fi`,
				`CLONE_SHA="$(git -C "$CLONE_CACHE" rev-parse HEAD)"`,
				fmt.Sprintf(`CLONE_SHA_FILE=%s.blueprint-sha`, path),
				`OLD_SHA=""`,
				`[ -f "$CLONE_SHA_FILE" ] && OLD_SHA="$(cat "$CLONE_SHA_FILE")"`,
				`if [ "$CLONE_SHA" != "$OLD_SHA" ]; then`,
				fmt.Sprintf("  mkdir -p %s", path),
				fmt.Sprintf(`  rsync -a --delete --exclude='.git' "$CLONE_CACHE/" %s/`, path),
				`  echo "$CLONE_SHA" > "$CLONE_SHA_FILE"`,
				`fi`,
			}
		},
	})
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

// Up clones or updates the repository.
// When CloneBare is true a bare clone is used — <path>/.git plus a worktree per
// branch — the layout git worktree managers such as worktrunk expect.
// When CloneWorkdir is true a direct git clone is used so the .git directory is
// preserved — the target becomes a fully functional working copy.
// Otherwise the default two-stage approach is used: clone to clean storage then
// copy files without .git, preventing accidental pollution of the target.
func (h *CloneHandler) Up() (string, error) {
	// Interpret the rule into a resolved clone spec; the mode selection stays
	// above the seam, the clone mechanics below it.
	mode := platform.ModeTwoStage
	switch {
	case h.Rule.CloneBare:
		mode = platform.ModeBare
	case h.Rule.CloneWorkdir:
		mode = platform.ModeDirect
	}

	result, err := h.Container.GitProvider().Clone(platform.CloneSpec{
		URL:    h.Rule.CloneURL,
		Path:   h.Rule.ClonePath,
		Branch: h.Rule.Branch,
		Mode:   mode,
	})
	if err != nil {
		return "", fmt.Errorf("failed to clone/update repository: %w", err)
	}

	oldSHA, newSHA := string(result.OldSHA), string(result.NewSHA)

	// Format output message with SHA tracking
	var msg string
	switch result.Status {
	case platform.StatusCloned:
		if newSHA != "" {
			msg = fmt.Sprintf("Cloned (SHA: %s)", newSHA)
		} else {
			msg = "Cloned"
		}

	case platform.StatusUpdated:
		if oldSHA != "" && newSHA != "" {
			msg = fmt.Sprintf("Updated (SHA changed: %s → %s) (SHA: %s)",
				oldSHA[:8], newSHA[:8], newSHA)
		} else if newSHA != "" {
			msg = fmt.Sprintf("Updated (SHA: %s)", newSHA)
		} else {
			msg = "Updated"
		}

	case platform.StatusSynced:
		if newSHA != "" {
			msg = fmt.Sprintf("Synced (SHA: %s)", newSHA)
		} else {
			msg = "Synced"
		}

	case platform.StatusUpToDate:
		if newSHA != "" {
			msg = fmt.Sprintf("Already up to date (SHA: %s)", newSHA)
		} else {
			msg = "Already up to date"
		}

	default:
		return "", fmt.Errorf("unknown clone status %d", result.Status)
	}

	// Recoverable incidents (repair counts, skipped-step warnings) are notes,
	// never failures — surface them alongside the status message.
	for _, note := range result.Notes {
		msg += "; " + note
	}
	return msg, nil
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
	if h.Rule.CloneBare {
		if h.Rule.Branch != "" {
			return fmt.Sprintf("git clone --bare -b %s %s %s/.git", h.Rule.Branch, h.Rule.CloneURL, h.Rule.ClonePath)
		}
		return fmt.Sprintf("git clone --bare %s %s/.git", h.Rule.CloneURL, h.Rule.ClonePath)
	}

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
	if h.Rule.CloneBare {
		state["layout"] = "bare"
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
			currentCloneURLs[giturl.NormalizeGitURL(rule.CloneURL)] = true
		}
	}

	// Find clones to uninstall (in status but not in current rules)
	var rules []parser.Rule
	if status.Clones != nil {
		for _, clone := range status.Clones {
			normalizedStatusBlueprint := normalizeBlueprint(clone.Blueprint)
			normalizedStatusURL := giturl.NormalizeGitURL(clone.URL)
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
		provider := h.Container.GitProvider()
		fs := h.Container.SystemProvider().Filesystem()

		// Found a matching status entry — now check SHA currency. An
		// unreachable remote (error or unknown SHA) means we cannot judge
		// currency — trust the status entry as-is.
		remoteSHA, remoteErr := provider.RemoteSHA(platform.RepoID{URL: h.Rule.CloneURL, Branch: h.Rule.Branch})
		if remoteErr != nil || remoteSHA == "" {
			return true
		}

		// Bare clones track the remote branch tip, not the pinned local branch
		// (moving it would disturb whichever worktree has it checked out).
		// The layout is also detected on disk because uninstall rules are
		// rebuilt from status and don't carry the bare flag.
		expandedPath := fs.ExpandPath(h.Rule.ClonePath)
		if h.Rule.CloneBare || provider.Layout(platform.RepoPath(expandedPath)) == platform.LayoutBare {
			hasGit := fs.Exists(filepath.Join(expandedPath, ".git"))
			bareSHA, _ := provider.BareSHA(platform.RepoPath(expandedPath), h.Rule.Branch)
			return hasGit && bareSHA == remoteSHA
		}

		// When workdir is set, we only care about the target directory — skip the
		// clean storage check (which is for two-stage clones) and verify .git exists.
		if h.Rule.CloneWorkdir {
			gitDir := expandedPath + "/.git"
			hasGit := fs.Exists(gitDir)
			localSHAVal, _ := provider.LocalSHA(platform.RepoPath(expandedPath))
			return hasGit && localSHAVal == remoteSHA
		}

		// Two-stage clone — try clean repository storage first (prevents pollution issues)
		cleanSHA, _ := provider.StorageSHA(platform.RepoID{URL: h.Rule.CloneURL, Branch: h.Rule.Branch})
		if cleanSHA != "" {
			// Clean storage exists, use it for SHA comparison
			return cleanSHA == remoteSHA
		}

		// Fall back to checking target directory for backward compatibility
		// This handles existing installations that don't have clean storage yet
		localSHAVal, _ := provider.LocalSHA(platform.RepoPath(expandedPath))
		return localSHAVal == remoteSHA
	}
	return false
}
