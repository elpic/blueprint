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

func init() {
	RegisterAction(ActionDef{
		Name:   "dotfiles",
		Prefix: "dotfiles ",
		NewHandler: func(rule parser.Rule, basePath string, passwordCache map[string]string) Handler {
			return NewDotfilesHandler(rule, basePath)
		},
		RuleKey: func(rule parser.Rule) string {
			return rule.DotfilesURL
		},
		Detect: func(rule parser.Rule) bool {
			return rule.DotfilesURL != ""
		},
		Summary: func(rule parser.Rule) string {
			return rule.DotfilesURL
		},
		OrphanIndex: func(rule parser.Rule, index func(string)) {
			index(rule.DotfilesURL)
		},
		AlwaysRunUp: true,
		ShellExport: func(rule parser.Rule, _, _ string) []string {
			clonePath := rule.DotfilesPath
			if clonePath == "" {
				parts := strings.Split(strings.TrimSuffix(rule.DotfilesURL, ".git"), "/")
				repoName := parts[len(parts)-1]
				clonePath = "~/.blueprint/dotfiles/" + repoName
			}
			path := shellHome(clonePath)
			cloneCmd := "git clone"
			if rule.DotfilesBranch != "" {
				cloneCmd += " -b " + shellQ(rule.DotfilesBranch)
			}
			cloneCmd += " " + shellQ(rule.DotfilesURL) + " " + path
			skipCase := ".|..|.git|.github|README*|readme*|LICENSE*|license*"
			for _, s := range rule.DotfilesSkip {
				skipCase += "|" + s
			}
			resetRef := "origin/HEAD"
			if rule.DotfilesBranch != "" {
				resetRef = "origin/" + rule.DotfilesBranch
			}
			return []string{
				fmt.Sprintf("if [ ! -d %s ]; then", path),
				"  " + cloneCmd,
				"else",
				fmt.Sprintf("  git -C %s fetch -q origin", path),
				fmt.Sprintf("  git -C %s reset --hard %s -q 2>/dev/null || git -C %s reset --hard FETCH_HEAD -q", path, resetRef, path),
				"fi",
				"# Symlink dotfiles to home directory",
				fmt.Sprintf(`for f in %s/.* %s/*; do`, path, path),
				`  name="$(basename "$f")"`,
				`  case "$name" in`,
				`    ` + skipCase + `) continue ;;`,
				`  esac`,
				`  if [ -f "$f" ] || [ -L "$f" ]; then`,
				`    ln -sf "$f" "$HOME/$name"`,
				`  elif [ -d "$f" ]; then`,
				`    mkdir -p "$HOME/$name"`,
				`    for child in "$f"/*; do`,
				`      [ -e "$child" ] || continue`,
				`      ln -sf "$child" "$HOME/$name/$(basename "$child")"`,
				`    done`,
				`  fi`,
				`done`,
			}
		},
	})
}

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

	// Detect update scenario: clone directory already exists before pull
	_, statErr := os.Stat(clonePath)
	isUpdate := statErr == nil

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

	// On update, remove all managed symlinks first so renames, deletions, and
	// reorganizations in the repo are reflected correctly. See ADR:
	// .brain/adr-dotfiles-recreate-on-update.md
	if isUpdate {
		h.removeAllManagedSymlinks(clonePath, homeDir)
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
	blueprint = normalizeBlueprint(blueprint)

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
	normalizedBlueprint := normalizeBlueprint(blueprintFile)

	// Build set of current dotfiles URLs (using normalized URLs for comparison)
	currentURLs := make(map[string]bool)
	for _, rule := range currentRules {
		if rule.Action == "dotfiles" && rule.DotfilesURL != "" {
			currentURLs[gitpkg.NormalizeGitURL(rule.DotfilesURL)] = true
		}
	}

	var rules []parser.Rule
	for _, d := range status.Dotfiles {
		normalizedStatusBlueprint := normalizeBlueprint(d.Blueprint)
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

// DotfilesLinksForDiff walks the local dotfiles clone and returns symlink paths
// that would be created by Up() but don't exist yet as correct symlinks.
// This mirrors the Up() linking logic: top-level files/symlinks link into ~,
// top-level directories are descended one level. Entries that are already
// correctly symlinked are excluded — only missing or wrong ones are returned.
// Paths are abbreviated with ~ for readability.
// homeDirOverride is used in tests to inject a custom home directory; pass ""
// to use the real home directory.
func DotfilesLinksForDiff(h *DotfilesHandler, homeDirOverride ...string) []string {
	clonePath := h.expandedDotfilesPath()
	var homeDir string
	if len(homeDirOverride) > 0 && homeDirOverride[0] != "" {
		homeDir = homeDirOverride[0]
	} else {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return nil
		}
	}

	entries, err := os.ReadDir(clonePath)
	if err != nil {
		return nil
	}

	abbrev := func(p string) string {
		if strings.HasPrefix(p, homeDir) {
			return "~" + p[len(homeDir):]
		}
		return p
	}

	// symlinkNeeded returns true if dst is not already a correct symlink to src.
	symlinkNeeded := func(src, dst string) bool {
		info, err := os.Lstat(dst)
		if err != nil {
			return true // doesn't exist
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return false // real file/dir exists — Up() would skip it too
		}
		target, err := os.Readlink(dst)
		return err != nil || target != src
	}

	var missing []string
	for _, entry := range entries {
		if shouldSkipEntry(entry.Name(), h.Rule.DotfilesSkip) {
			continue
		}
		src := filepath.Join(clonePath, entry.Name())
		if entry.IsDir() {
			// Descend one level — link each item inside into ~/dir/
			subEntries, err := os.ReadDir(src)
			if err != nil {
				continue
			}
			dstDir := filepath.Join(homeDir, entry.Name())
			for _, sub := range subEntries {
				subSrc := filepath.Join(src, sub.Name())
				subDst := filepath.Join(dstDir, sub.Name())
				if symlinkNeeded(subSrc, subDst) {
					missing = append(missing, abbrev(subDst))
				}
			}
		} else {
			dst := filepath.Join(homeDir, entry.Name())
			if symlinkNeeded(src, dst) {
				missing = append(missing, abbrev(dst))
			}
		}
	}
	return missing
}

// IsInstalled returns true if the dotfiles repo is up to date and all expected
// symlinks are present and correct. It compares the local clone SHA against the
// remote HEAD — if the remote has new commits, it returns false so PrintDiff
// shows the rule as needing an update. The engine always calls Up() for
// dotfiles regardless of this result (see command.go).
func (h *DotfilesHandler) IsInstalled(status *Status, blueprintFile, osName string) bool {
	normalizedBlueprint := normalizeBlueprint(blueprintFile)
	normalizedRuleURL := gitpkg.NormalizeGitURL(h.Rule.DotfilesURL)
	for _, d := range status.Dotfiles {
		normalizedStatusURL := gitpkg.NormalizeGitURL(d.URL)
		if normalizedStatusURL == normalizedRuleURL && normalizeBlueprint(d.Blueprint) == normalizedBlueprint && d.OS == osName {
			// Check if remote has new commits
			clonePath := h.expandedDotfilesPath()
			localSHA := gitpkg.LocalSHA(clonePath)
			if localSHA != "" {
				remoteSHA := gitpkg.RemoteHeadSHA(h.Rule.DotfilesURL, "")
				if remoteSHA != "" && remoteSHA != localSHA {
					return false
				}
			}
			// Check all expected symlinks are present and correct
			for _, linkPath := range d.Links {
				info, err := os.Lstat(linkPath)
				if err != nil || info.Mode()&os.ModeSymlink == 0 {
					return false
				}
				target, err := os.Readlink(linkPath)
				if err != nil {
					return false
				}
				resolvedTarget, _ := filepath.EvalSymlinks(target)
				resolvedClonePath, _ := filepath.EvalSymlinks(clonePath)
				if !strings.HasPrefix(resolvedTarget, resolvedClonePath+string(filepath.Separator)) && resolvedTarget != resolvedClonePath {
					return false
				}
			}
			return true
		}
	}
	return false
}
