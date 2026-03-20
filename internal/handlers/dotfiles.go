package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	gitpkg "github.com/elpic/blueprint/internal/git"
	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
)

// DotfilesHandler handles dotfiles repository cloning and symlink management
type DotfilesHandler struct {
	BaseHandler
}

// NewDotfilesHandler creates a new dotfiles handler
func NewDotfilesHandler(rule parser.Rule, basePath string) *DotfilesHandler {
	return &DotfilesHandler{
		BaseHandler: BaseHandler{
			Rule:     rule,
			BasePath: basePath,
		},
	}
}

// expandedDotfilesPath returns the expanded clone path for the dotfiles repo
func (h *DotfilesHandler) expandedDotfilesPath() string {
	return expandPath(h.Rule.DotfilesPath)
}

// shouldSkipEntry returns true for top-level repo entries that should not be symlinked.
// userSkip is the optional per-rule skip list from the blueprint.
func shouldSkipEntry(name string, userSkip []string) bool {
	lower := strings.ToLower(name)
	if name == ".git" || name == ".github" || strings.HasPrefix(lower, "readme") {
		return true
	}
	for _, s := range userSkip {
		if s == name {
			return true
		}
	}
	return false
}

// ensureSymlink creates a symlink at dst pointing to src.
// Returns true if created, false if already correct, and an error description if skipped.
func ensureSymlink(src, dst string) (created bool, skipReason string) {
	info, statErr := os.Lstat(dst)
	if statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			if target, err := os.Readlink(dst); err == nil && target == src {
				return false, "" // already correct
			}
		} else {
			return false, dst // real file/dir exists — skip
		}
	}
	if err := os.Symlink(src, dst); err != nil {
		return false, dst
	}
	return true, ""
}

// Up clones/updates the dotfiles repo and creates symlinks in the home directory.
// Top-level files and symlinks are linked directly into ~.
// Top-level directories are descended one level: each item inside is linked into ~/dir/.
// Stale symlinks (pointing into the clone but whose source no longer exists) are removed.
func (h *DotfilesHandler) Up() (string, error) {
	clonePath := h.expandedDotfilesPath()

	_, _, _, err := gitpkg.CloneOrUpdateRepository(
		h.Rule.DotfilesURL,
		clonePath,
		h.Rule.DotfilesBranch,
	)
	if err != nil {
		return "", fmt.Errorf("failed to clone/update dotfiles repository: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	var created, already int
	var skippedNames []string

	entries, err := os.ReadDir(clonePath)
	if err != nil {
		return "", fmt.Errorf("failed to read dotfiles directory: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if shouldSkipEntry(name, h.Rule.DotfilesSkip) {
			continue
		}

		src := filepath.Join(clonePath, name)
		dst := filepath.Join(homeDir, name)

		if entry.IsDir() {
			// Ensure the target directory exists in ~
			if mkErr := os.MkdirAll(dst, 0750); mkErr != nil {
				skippedNames = append(skippedNames, name)
				continue
			}
			// Symlink each item inside the directory one level deep
			subEntries, readErr := os.ReadDir(src)
			if readErr != nil {
				skippedNames = append(skippedNames, name)
				continue
			}
			for _, sub := range subEntries {
				subSrc := filepath.Join(src, sub.Name())
				subDst := filepath.Join(dst, sub.Name())
				ok, reason := ensureSymlink(subSrc, subDst)
				if ok {
					created++
				} else if reason == "" {
					already++
				} else {
					skippedNames = append(skippedNames, filepath.Join(name, sub.Name()))
				}
			}
		} else {
			ok, reason := ensureSymlink(src, dst)
			if ok {
				created++
			} else if reason == "" {
				already++
			} else {
				skippedNames = append(skippedNames, name)
			}
		}
	}

	// Remove stale symlinks: symlinks in ~ (or ~/dir/) pointing into clonePath
	// whose source file no longer exists in the repo.
	removed := h.removeStaleSymlinks(clonePath, homeDir)

	msg := fmt.Sprintf("Dotfiles linked: %d created, %d already linked, %d stale removed", created, already, removed)
	if len(skippedNames) > 0 {
		msg += fmt.Sprintf(", skipped: %s", strings.Join(skippedNames, ", "))
	}
	return msg, nil
}

// removeStaleSymlinks removes symlinks in homeDir (and one level into subdirs) that point
// into clonePath but whose source no longer exists OR whose top-level entry is now skipped.
// Returns the count removed.
func (h *DotfilesHandler) removeStaleSymlinks(clonePath, homeDir string) int {
	removed := 0

	checkAndRemove := func(linkPath string) {
		info, err := os.Lstat(linkPath)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			return
		}
		target, err := os.Readlink(linkPath)
		if err != nil {
			return
		}
		// Only touch symlinks that point into our managed clone
		if !strings.HasPrefix(target, clonePath+string(filepath.Separator)) && target != clonePath {
			return
		}
		// Determine the top-level entry name within the clone that this symlink originates from
		rel, relErr := filepath.Rel(clonePath, target)
		if relErr == nil {
			topLevel := strings.SplitN(rel, string(filepath.Separator), 2)[0]
			if shouldSkipEntry(topLevel, h.Rule.DotfilesSkip) {
				if os.Remove(linkPath) == nil {
					removed++
				}
				return
			}
		}
		// If the source no longer exists in the repo, remove the symlink
		if _, statErr := os.Stat(target); os.IsNotExist(statErr) {
			if os.Remove(linkPath) == nil {
				removed++
			}
		}
	}

	// Check top-level entries in ~
	topEntries, err := os.ReadDir(homeDir)
	if err != nil {
		return removed
	}
	for _, e := range topEntries {
		fullPath := filepath.Join(homeDir, e.Name())
		if e.Type()&os.ModeSymlink != 0 {
			checkAndRemove(fullPath)
		} else if e.IsDir() {
			// One level deep into real directories
			subEntries, readErr := os.ReadDir(fullPath)
			if readErr != nil {
				continue
			}
			for _, sub := range subEntries {
				checkAndRemove(filepath.Join(fullPath, sub.Name()))
			}
		}
	}

	return removed
}

// Down removes all symlinks pointing into the clone and then removes the clone directory.
func (h *DotfilesHandler) Down() (string, error) {
	clonePath := h.expandedDotfilesPath()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	// Remove all symlinks in ~ (and one level deep) that point into clonePath
	removedLinks := h.removeAllManagedSymlinks(clonePath, homeDir)

	// Remove the clone directory
	var cloneRemoved bool
	if _, statErr := os.Stat(clonePath); statErr == nil {
		if removeErr := os.RemoveAll(clonePath); removeErr == nil {
			cloneRemoved = true
		}
	}

	msg := fmt.Sprintf("Removed %d symlinks", removedLinks)
	if cloneRemoved {
		msg += fmt.Sprintf(", removed clone at %s", clonePath)
	}
	return msg, nil
}

// removeAllManagedSymlinks removes every symlink in homeDir (and one level deep into real dirs)
// that points into clonePath. Returns the count removed.
func (h *DotfilesHandler) removeAllManagedSymlinks(clonePath, homeDir string) int {
	removed := 0

	removeIfManaged := func(linkPath string) {
		info, err := os.Lstat(linkPath)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			return
		}
		target, err := os.Readlink(linkPath)
		if err != nil {
			return
		}
		if strings.HasPrefix(target, clonePath+string(filepath.Separator)) || target == clonePath {
			if os.Remove(linkPath) == nil {
				removed++
			}
		}
	}

	topEntries, err := os.ReadDir(homeDir)
	if err != nil {
		return removed
	}
	for _, e := range topEntries {
		fullPath := filepath.Join(homeDir, e.Name())
		if e.Type()&os.ModeSymlink != 0 {
			removeIfManaged(fullPath)
		} else if e.IsDir() {
			subEntries, readErr := os.ReadDir(fullPath)
			if readErr != nil {
				continue
			}
			for _, sub := range subEntries {
				removeIfManaged(filepath.Join(fullPath, sub.Name()))
			}
		}
	}

	return removed
}

// GetCommand returns the command used for display/plan output
func (h *DotfilesHandler) GetCommand() string {
	if h.Rule.DotfilesBranch != "" {
		return fmt.Sprintf("git clone -b %s %s %s", h.Rule.DotfilesBranch, h.Rule.DotfilesURL, h.Rule.DotfilesPath)
	}
	return fmt.Sprintf("git clone %s %s", h.Rule.DotfilesURL, h.Rule.DotfilesPath)
}

// UpdateStatus updates the status after dotfiles are installed or removed
func (h *DotfilesHandler) UpdateStatus(status *Status, records []ExecutionRecord, blueprint string, osName string) error {
	blueprint = normalizePath(blueprint)

	if h.Rule.Action == "dotfiles" {
		cloneCmd := h.GetCommand()
		_, commandExecuted := commandSuccessfullyExecuted(cloneCmd, records)
		if commandExecuted {
			// Collect all symlinks currently pointing into the clone
			var links []string
			var currentSHA string
			homeDir, err := os.UserHomeDir()
			if err == nil {
				clonePath := h.expandedDotfilesPath()

				// Get current SHA of the cloned repository
				currentSHA = gitpkg.LocalSHA(clonePath)

				collectManaged := func(linkPath string) {
					info, lerr := os.Lstat(linkPath)
					if lerr != nil || info.Mode()&os.ModeSymlink == 0 {
						return
					}
					target, rerr := os.Readlink(linkPath)
					if rerr != nil {
						return
					}
					// Try to resolve the target path to handle ~ and relative paths
					resolvedTarget, terr := filepath.EvalSymlinks(target)
					if terr == nil {
						target = resolvedTarget
					}
					// Also resolve clonePath for comparison
					resolvedClonePath, cerr := filepath.EvalSymlinks(clonePath)
					if cerr == nil {
						clonePath = resolvedClonePath
					}
					// Check if target points into the clone directory
					if strings.HasPrefix(target, clonePath+string(filepath.Separator)) || target == clonePath {
						links = append(links, linkPath)
					}
				}
				if topEntries, readErr := os.ReadDir(homeDir); readErr == nil {
					for _, e := range topEntries {
						fullPath := filepath.Join(homeDir, e.Name())
						if e.Type()&os.ModeSymlink != 0 {
							collectManaged(fullPath)
						} else if e.IsDir() {
							if subEntries, serr := os.ReadDir(fullPath); serr == nil {
								for _, sub := range subEntries {
									collectManaged(filepath.Join(fullPath, sub.Name()))
								}
							}
						}
					}
				}
			}

			status.Dotfiles = removeDotfilesStatus(status.Dotfiles, h.Rule.DotfilesURL, blueprint, osName)
			status.Dotfiles = append(status.Dotfiles, DotfilesStatus{
				URL:       h.Rule.DotfilesURL,
				Path:      h.Rule.DotfilesPath,
				Branch:    h.Rule.DotfilesBranch,
				SHA:       currentSHA,
				Links:     links,
				ClonedAt:  time.Now().Format(time.RFC3339),
				Blueprint: blueprint,
				OS:        osName,
			})
		}
	} else if h.Rule.Action == "uninstall" && DetectRuleType(h.Rule) == "dotfiles" {
		// Remove from status if clone dir no longer exists
		expandedPath := h.expandedDotfilesPath()
		if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
			status.Dotfiles = removeDotfilesStatus(status.Dotfiles, h.Rule.DotfilesURL, blueprint, osName)
		}
	}

	return nil
}

// GetDependencyKey returns the unique key for dependency resolution
func (h *DotfilesHandler) GetDependencyKey() string {
	return getDependencyKey(h.Rule, h.Rule.DotfilesURL)
}

// GetDisplayDetails returns the URL to display during execution
func (h *DotfilesHandler) GetDisplayDetails(isUninstall bool) string {
	return h.Rule.DotfilesURL
}

// GetState returns handler-specific state as key-value pairs
func (h *DotfilesHandler) GetState(isUninstall bool) map[string]string {
	return map[string]string{
		"summary": h.Rule.DotfilesURL,
		"url":     h.Rule.DotfilesURL,
		"path":    h.Rule.DotfilesPath,
	}
}

// DisplayInfo displays handler-specific information
func (h *DotfilesHandler) DisplayInfo() {
	formatFunc := ui.FormatInfo
	if h.Rule.Action == "uninstall" {
		formatFunc = ui.FormatDim
	}

	fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("URL: %s", h.Rule.DotfilesURL)))
	fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Path: %s", h.Rule.DotfilesPath)))
	if h.Rule.DotfilesBranch != "" {
		fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Branch: %s", h.Rule.DotfilesBranch)))
	}
}

// DisplayStatusFromStatus displays dotfiles status from the Status object
func (h *DotfilesHandler) DisplayStatusFromStatus(status *Status) {
	if status == nil || len(status.Dotfiles) == 0 {
		return
	}

	fmt.Printf("\n%s\n", ui.FormatHighlight("Dotfiles:"))
	for _, d := range status.Dotfiles {
		t, err := time.Parse(time.RFC3339, d.ClonedAt)
		var timeStr string
		if err == nil {
			timeStr = t.Format("2006-01-02 15:04:05")
		} else {
			timeStr = d.ClonedAt
		}

		// Show SHA (abbreviated to 7 chars) if available
		shaStr := ""
		if d.SHA != "" {
			if len(d.SHA) >= 7 {
				shaStr = fmt.Sprintf(" @ %s", d.SHA[:7])
			} else {
				shaStr = fmt.Sprintf(" @ %s", d.SHA)
			}
		}

		fmt.Printf("  %s %s%s (%s) [%s, %s]\n",
			ui.FormatSuccess("●"),
			ui.FormatInfo(d.URL),
			ui.FormatDim(shaStr),
			ui.FormatDim(timeStr),
			ui.FormatDim(d.OS),
			ui.FormatDim(abbreviateBlueprintPath(d.Blueprint)),
		)
		fmt.Printf("     %s %s\n", ui.FormatDim("Path:"), ui.FormatInfo(d.Path))
		fmt.Printf("     %s %d links\n", ui.FormatDim("Links:"), len(d.Links))
	}
}

// FindUninstallRules compares dotfiles status against current rules and returns uninstall rules
func (h *DotfilesHandler) FindUninstallRules(status *Status, currentRules []parser.Rule, blueprintFile, osName string) []parser.Rule {
	normalizedBlueprint := normalizePath(blueprintFile)

	// Build set of current dotfiles URLs (using normalized URLs for comparison)
	currentURLs := make(map[string]bool)
	for _, rule := range currentRules {
		if rule.Action == "dotfiles" && rule.DotfilesURL != "" {
			currentURLs[gitpkg.NormalizeGitURL(rule.DotfilesURL)] = true
		}
	}

	var rules []parser.Rule
	for _, d := range status.Dotfiles {
		normalizedStatusBlueprint := normalizePath(d.Blueprint)
		normalizedStatusURL := gitpkg.NormalizeGitURL(d.URL)
		if normalizedStatusBlueprint == normalizedBlueprint && d.OS == osName && !currentURLs[normalizedStatusURL] {
			rules = append(rules, parser.Rule{
				Action:         "uninstall",
				DotfilesURL:    d.URL,
				DotfilesPath:   d.Path,
				DotfilesBranch: d.Branch,
				OSList:         []string{osName},
			})
		}
	}

	return rules
}

// IsInstalled returns true if the dotfiles URL is already installed and up to date.
// It checks the URL, SHA, and verifies all expected symlinks are present and correct.
func (h *DotfilesHandler) IsInstalled(status *Status, blueprintFile, osName string) bool {
	normalizedBlueprint := normalizePath(blueprintFile)
	normalizedRuleURL := gitpkg.NormalizeGitURL(h.Rule.DotfilesURL)
	for _, d := range status.Dotfiles {
		normalizedStatusURL := gitpkg.NormalizeGitURL(d.URL)
		if normalizedStatusURL == normalizedRuleURL && normalizePath(d.Blueprint) == normalizedBlueprint && d.OS == osName {
			// Check if SHA matches - if different, repo has changes that need to be applied
			if d.SHA != "" {
				currentSHA := gitpkg.LocalSHA(h.expandedDotfilesPath())
				if currentSHA != "" && currentSHA != d.SHA {
					// SHA changed - new files might have been added, need to re-process
					return false
				}
			}

			// Check if all expected links are present and correct
			if len(d.Links) > 0 {
				clonePath := h.expandedDotfilesPath()

				for _, linkPath := range d.Links {
					// Check if the link exists
					info, err := os.Lstat(linkPath)
					if err != nil {
						// Link doesn't exist
						return false
					}

					// Check if it's a symlink
					if info.Mode()&os.ModeSymlink == 0 {
						// Not a symlink anymore
						return false
					}

					// Check if it points to the correct location
					target, err := os.Readlink(linkPath)
					if err != nil {
						return false
					}

					// Resolve both paths for comparison
					resolvedTarget, _ := filepath.EvalSymlinks(target)
					resolvedClonePath, _ := filepath.EvalSymlinks(clonePath)

					// Check if the symlink target is within the clone directory
					if !strings.HasPrefix(resolvedTarget, resolvedClonePath+string(filepath.Separator)) && resolvedTarget != resolvedClonePath {
						// Symlink points to wrong location
						return false
					}
				}
			}

			return true
		}
	}
	return false
}
