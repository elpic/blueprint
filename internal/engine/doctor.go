package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	gitpkg "github.com/elpic/blueprint/internal/git"
	handlerskg "github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
)

// doctorIssue describes a single category of problems found by doctor.
type doctorIssue struct {
	description string
	count       int
	examples    []string // up to a small number of representative entries
	fix         func()   // optional: called in --fix mode to repair the issue in-place
}

// checkBlueprintURLs scans all Blueprint fields in the status for entries
// that are not in normalized form. Returns one issue per category if any
// stale entries are found, or nil if everything is clean.
func checkBlueprintURLs(status *handlerskg.Status) []doctorIssue {
	type entry struct {
		raw        string
		normalized string
	}

	var stale []entry

	collect := func(raw string) {
		norm := handlerskg.NormalizeBlueprint(raw)
		if raw != norm {
			stale = append(stale, entry{raw: raw, normalized: norm})
		}
	}

	for _, e := range status.AllEntries() {
		collect(e.GetBlueprint())
	}

	if len(stale) == 0 {
		return nil
	}

	// Deduplicate examples (show at most 3 unique raw → normalized pairs).
	seen := map[string]bool{}
	var examples []string
	for _, e := range stale {
		key := e.raw
		if !seen[key] {
			seen[key] = true
			if len(examples) < 3 {
				examples = append(examples, fmt.Sprintf("%s → %s", e.raw, e.normalized))
			}
		}
	}

	return []doctorIssue{
		{
			description: fmt.Sprintf("%d entries have stale blueprint URLs (run 'blueprint doctor --fix' to repair)", len(stale)),
			count:       len(stale),
			examples:    examples,
		},
	}
}

// normalizeBlueprintForDoctor normalizes a blueprint value for duplicate comparison.
// It handles both well-formed and malformed URL forms (e.g. single-slash https:/)
// by always delegating to NormalizeBlueprint, which now recognises all known variants.
func normalizeBlueprintForDoctor(bp string) string {
	return handlerskg.NormalizeBlueprint(bp)
}

// checkDuplicates detects entries where the same resource+OS pair appears more than
// once under blueprint URLs that normalize to the same value. This is more precise
// than ignoring blueprint entirely: resources genuinely installed from two different
// blueprints are not flagged, while entries that only differ because of URL form
// variants (e.g. "https:/host/repo.git" vs "https://host/repo") are caught.
func checkDuplicates(status *handlerskg.Status) []doctorIssue {
	type key struct{ resource, os, blueprint string }
	seen := map[key]bool{}
	count := 0

	track := func(resource, os, blueprint string) {
		k := key{resource, os, normalizeBlueprintForDoctor(blueprint)}
		if seen[k] {
			count++
		}
		seen[k] = true
	}

	for _, e := range status.AllEntries() {
		track(e.GetResourceKey(), e.GetOS(), e.GetBlueprint())
	}

	if count == 0 {
		return nil
	}

	return []doctorIssue{
		{
			description: fmt.Sprintf("%d duplicate entries (same resource recorded more than once)", count),
			count:       count,
		},
	}
}

// findBlueprintSetupFile returns the path to setup.bp inside localPath, or an
// error if the file does not exist.
func findBlueprintSetupFile(localPath string) (string, error) {
	p := filepath.Join(localPath, "setup.bp")
	if _, err := os.Stat(p); err != nil {
		return "", err
	}
	return p, nil
}

// rulesForBlueprint loads the parsed rules for a blueprint URL at a specific
// git SHA. It clones/updates the repo on demand, then checks out the given SHA
// so the orphan check uses the exact version that was applied. If sha is empty
// it uses HEAD (safe fallback for local blueprints or old status files).
func rulesForBlueprint(blueprintURL, sha string) ([]parser.Rule, error) {
	if !gitpkg.IsGitURL(blueprintURL) {
		// Local file — parse directly.
		return parser.ParseFile(blueprintURL)
	}

	localPath := blueprintRepoPath(blueprintURL)

	// Clone or update so we have the repo locally.
	params := gitpkg.ParseGitURL(blueprintURL)
	if _, _, _, err := gitpkg.CloneOrUpdateRepository(params.URL, localPath, params.Branch); err != nil {
		return nil, fmt.Errorf("failed to fetch blueprint %s: %w", blueprintURL, err)
	}

	// Checkout the specific SHA that was applied so we compare against the
	// exact version of the blueprint the user ran, not the current HEAD.
	if sha != "" {
		if err := gitpkg.CheckoutSHA(localPath, sha); err != nil {
			// Non-fatal: fall through and use whatever is checked out.
			fmt.Printf("  Warning: could not checkout SHA %s for %s: %v\n", sha, blueprintURL, err)
		}
	}

	setupPath, err := findBlueprintSetupFile(localPath)
	if err != nil {
		return nil, fmt.Errorf("setup.bp not found in %s: %w", blueprintURL, err)
	}
	return parser.ParseFile(setupPath)
}

// filterOrphans removes all entries from status whose (normalized resource key,
// normalized blueprint, OS) triple appears in the orphaned set. The orphaned map
// key is "<normalizedBlueprint>\x00<os>\x00<normalizedResourceKey>".
func filterOrphans(status *handlerskg.Status, orphaned map[string]bool) {
	norm := handlerskg.NormalizeBlueprint
	status.FilterEntries(func(e handlerskg.StatusEntry) bool {
		resource := e.GetResourceKey()
		bp := norm(e.GetBlueprint())
		os := e.GetOS()
		k := bp + "\x00" + os + "\x00" + resource
		kn := bp + "\x00" + os + "\x00" + norm(resource)
		return !orphaned[k] && !orphaned[kn]
	})
}

// checkOrphansWithLoader is the testable core of checkOrphans.
// loader is called with a normalized blueprint URL and returns the parsed rules
// for that blueprint, or nil if the blueprint is not available locally.
func checkOrphansWithLoader(status *handlerskg.Status, loader func(string) []parser.Rule) []doctorIssue {
	// Build a map of blueprint URL → set of rule keys present in that blueprint.
	type ruleSet map[string]bool
	cache := map[string]ruleSet{} // normalized blueprint URL → rule keys

	rulesFor := func(bp string) ruleSet {
		norm := handlerskg.NormalizeBlueprint(bp)
		if rs, ok := cache[norm]; ok {
			return rs
		}
		rules := loader(norm)
		if rules == nil {
			cache[norm] = nil
			return nil
		}
		rs := ruleSet{}
		for _, r := range rules {
			rs[handlerskg.RuleKey(r)] = true
			// Use each action's registered OrphanIndex to index all resource
			// keys that may appear in status entries, so that key-based orphan
			// detection works correctly even when the rule has an id:.
			if def := handlerskg.GetAction(r.Action); def != nil && def.OrphanIndex != nil {
				def.OrphanIndex(r, func(key string) {
					if key != "" {
						rs[key] = true
					}
				})
			}
		}
		cache[norm] = rs
		return rs
	}

	type orphanEntry struct {
		display string
		// orphanKey is the lookup key used to build the orphaned set passed to
		// filterOrphans: "<normalizedBlueprint>\x00<os>\x00<resource>".
		orphanKey string
	}
	var orphans []orphanEntry

	// Some actions are excluded from key-based orphan detection — see
	// OrphanCheckExcluded on their ActionDef for the reason.
	isExcluded := func(e handlerskg.StatusEntry) bool {
		def := handlerskg.GetAction(e.GetAction())
		return def != nil && def.OrphanCheckExcluded
	}

	for _, e := range status.AllEntries() {
		if isExcluded(e) {
			continue
		}
		resource := e.GetResourceKey()
		bp := e.GetBlueprint()
		os := e.GetOS()

		rs := rulesFor(bp)
		if rs == nil {
			continue // blueprint not cached — skip
		}
		// Check both the raw resource value and its normalized form (handles
		// git URLs stored with/without .git suffix, e.g. schedule sources).
		if !rs[resource] && !rs[handlerskg.NormalizeBlueprint(resource)] {
			normBP := handlerskg.NormalizeBlueprint(bp)
			orphans = append(orphans, orphanEntry{
				display:   fmt.Sprintf("%s (blueprint: %s)", resource, normBP),
				orphanKey: normBP + "\x00" + os + "\x00" + resource,
			})
		}
	}

	if len(orphans) == 0 {
		return nil
	}

	examples := make([]string, 0, 3)
	for i, o := range orphans {
		if i >= 3 {
			break
		}
		examples = append(examples, o.display)
	}

	// Build the orphaned set and attach the removal function.
	orphanedSet := make(map[string]bool, len(orphans))
	for _, o := range orphans {
		orphanedSet[o.orphanKey] = true
	}
	removeOrphans := func() {
		filterOrphans(status, orphanedSet)
	}

	return []doctorIssue{
		{
			description: fmt.Sprintf("%d orphaned entries (resource no longer exists in blueprint)", len(orphans)),
			count:       len(orphans),
			examples:    examples,
			fix:         removeOrphans,
		},
	}
}

// checkStaleSymlinks scans all DotfilesStatus entries and reports symlinks
// whose target path no longer resolves to a real file (broken symlink or
// symlink that was deleted). With --fix it removes them from the filesystem
// and from the status entry's Links list; entries with no remaining links are
// removed from status entirely.
func checkStaleSymlinks(status *handlerskg.Status) []doctorIssue {
	type staleLink struct {
		link      string // symlink path
		entryURL  string // dotfiles URL owning this link
		entryOS   string
		entryBlue string
	}
	var stale []staleLink

	for i := range status.Dotfiles {
		entry := &status.Dotfiles[i]
		for _, link := range entry.Links {
			expanded := expandHomedir(link)
			info, err := os.Lstat(expanded)
			if err != nil {
				// path does not exist at all
				stale = append(stale, staleLink{link: link, entryURL: entry.URL, entryOS: entry.OS, entryBlue: entry.Blueprint})
				continue
			}
			if info.Mode()&os.ModeSymlink != 0 {
				// path exists and is a symlink — check if target resolves
				if _, err := os.Stat(expanded); err != nil {
					stale = append(stale, staleLink{link: link, entryURL: entry.URL, entryOS: entry.OS, entryBlue: entry.Blueprint})
				}
			}
		}
	}

	if len(stale) == 0 {
		return nil
	}

	examples := make([]string, 0, 3)
	for i, s := range stale {
		if i >= 3 {
			break
		}
		examples = append(examples, s.link)
	}

	fixFn := func() {
		for _, s := range stale {
			_ = os.Remove(expandHomedir(s.link))
		}
		// Remove stale links from each entry's Links slice; drop entries with
		// no remaining links.
		staleSet := make(map[string]bool, len(stale))
		for _, s := range stale {
			staleSet[s.link] = true
		}
		var kept []handlerskg.DotfilesStatus
		for _, entry := range status.Dotfiles {
			var links []string
			for _, l := range entry.Links {
				if !staleSet[l] {
					links = append(links, l)
				}
			}
			entry.Links = links
			if len(links) > 0 {
				kept = append(kept, entry)
			}
		}
		status.Dotfiles = kept
	}

	return []doctorIssue{
		{
			description: fmt.Sprintf("%d stale dotfile symlink(s) (target missing or broken)", len(stale)),
			count:       len(stale),
			examples:    examples,
			fix:         fixFn,
		},
	}
}

// expandHomedir replaces a leading ~ with the current user's home directory.
func expandHomedir(path string) string {
	if len(path) == 0 || path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}

// checkMissingCloneDirs scans all CloneStatus entries and reports entries
// whose local clone directory no longer exists on disk. With --fix it removes
// those entries from status.
func checkMissingCloneDirs(status *handlerskg.Status) []doctorIssue {
	var missing []handlerskg.CloneStatus

	for _, entry := range status.Clones {
		expanded := expandHomedir(entry.Path)
		if _, err := os.Stat(expanded); os.IsNotExist(err) {
			missing = append(missing, entry)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	examples := make([]string, 0, 3)
	for i, m := range missing {
		if i >= 3 {
			break
		}
		examples = append(examples, m.Path)
	}

	fixFn := func() {
		missingPaths := make(map[string]bool, len(missing))
		for _, m := range missing {
			missingPaths[m.Path] = true
		}
		status.FilterEntries(func(e handlerskg.StatusEntry) bool {
			if e.GetAction() != "clone" {
				return true
			}
			return !missingPaths[e.GetResourceKey()]
		})
	}

	return []doctorIssue{
		{
			description: fmt.Sprintf("%d clone director(ies) missing from disk", len(missing)),
			count:       len(missing),
			examples:    examples,
			fix:         fixFn,
		},
	}
}

// checkOrphans detects status entries whose resource no longer exists in the
// blueprint file they were installed from. Uses status.BlueprintSHA to check
// against the exact version that was applied.
func checkOrphans(status *handlerskg.Status) []doctorIssue {
	fetched := map[string]bool{}
	sha := status.BlueprintSHA
	return checkOrphansWithLoader(status, func(norm string) []parser.Rule {
		if !fetched[norm] {
			fetched[norm] = true
			if sha != "" {
				fmt.Printf("  Fetching blueprint %s @ %s...\n", norm, sha[:8])
			} else {
				fmt.Printf("  Fetching blueprint %s...\n", norm)
			}
		}
		rules, err := rulesForBlueprint(norm, sha)
		if err != nil {
			fmt.Printf("  %s\n", ui.FormatError(fmt.Sprintf("could not fetch %s: %v", norm, err)))
			return nil
		}
		return rules
	})
}

// DoctorCheck reads ~/.blueprint/status.json, reports all issues found, and
// optionally rewrites the file with issues fixed when fix is true.
// Exits with code 1 if issues are found and fix is false.
func DoctorCheck(fix bool) {
	if fix {
		fmt.Printf("\n%s\n", ui.FormatHighlight("=== Blueprint Doctor (fix mode) ==="))
	} else {
		fmt.Printf("\n%s\n", ui.FormatHighlight("=== Blueprint Doctor ==="))
	}

	statusPath, err := getStatusPath()
	if err != nil {
		fmt.Printf("%s\n", ui.FormatError(fmt.Sprintf("Error getting status path: %v", err)))
		os.Exit(1)
	}

	fmt.Printf("\nChecking status file...\n")

	data, err := readBlueprintFile(statusPath)
	if err != nil {
		// No status file yet — nothing to check.
		fmt.Printf("  %s\n", ui.FormatSuccess("no status file found — nothing to check"))
		fmt.Printf("\n%s\n\n", ui.FormatSuccess("No issues found."))
		return
	}

	var status handlerskg.Status
	if err := json.Unmarshal(data, &status); err != nil {
		fmt.Printf("%s\n", ui.FormatError(fmt.Sprintf("Error parsing status file: %v", err)))
		os.Exit(1)
	}

	issues := checkBlueprintURLs(&status)
	issues = append(issues, checkDuplicates(&status)...)
	fmt.Printf("\nChecking for orphaned entries...\n")
	issues = append(issues, checkOrphans(&status)...)
	fmt.Printf("\nChecking for stale symlinks...\n")
	issues = append(issues, checkStaleSymlinks(&status)...)
	fmt.Printf("\nChecking for missing clone directories...\n")
	issues = append(issues, checkMissingCloneDirs(&status)...)

	if len(issues) == 0 {
		fmt.Printf("  %s\n", ui.FormatSuccess("blueprint URLs are normalized"))
		fmt.Printf("  %s\n", ui.FormatSuccess("no duplicate entries"))
		fmt.Printf("  %s\n", ui.FormatSuccess("no orphaned entries"))
		fmt.Printf("  %s\n", ui.FormatSuccess("no stale symlinks"))
		fmt.Printf("  %s\n", ui.FormatSuccess("no missing clone directories"))
		fmt.Printf("\n%s\n\n", ui.FormatSuccess("No issues found."))
		return
	}

	if fix {
		// Normalize URLs and deduplicate first, then run any issue-specific fixes.
		handlerskg.MigrateStatus(&status)
		handlerskg.DeduplicateStatus(&status)
		for _, issue := range issues {
			if issue.fix != nil {
				issue.fix()
			}
		}

		fixed, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			fmt.Printf("%s\n", ui.FormatError(fmt.Sprintf("Error serializing status: %v", err)))
			os.Exit(1)
		}
		if err := os.WriteFile(statusPath, fixed, 0600); err != nil {
			fmt.Printf("%s\n", ui.FormatError(fmt.Sprintf("Error writing status file: %v", err)))
			os.Exit(1)
		}

		for _, issue := range issues {
			fmt.Printf("  %s\n", ui.FormatSuccess(fmt.Sprintf("Fixed: %s", issue.description)))
		}
		fmt.Printf("\n%s\n\n", ui.FormatSuccess("All issues fixed."))
		return
	}

	// Report mode: print issues and exit 1.
	for _, issue := range issues {
		fmt.Printf("  %s\n", ui.FormatError(issue.description))
		for _, ex := range issue.examples {
			fmt.Printf("    %s\n", ui.FormatDim(ex))
		}
	}

	issueWord := "issue"
	if len(issues) != 1 {
		issueWord = "issues"
	}
	fmt.Printf("\n%s\n\n", ui.FormatError(fmt.Sprintf("%d %s found. Run 'blueprint doctor --fix' to repair.", len(issues), issueWord)))
	os.Exit(1)
}
