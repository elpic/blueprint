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
			// Also index the natural resource identity for each action type so
			// that status entries (which store the resource path/name, not the
			// rule id) are matched correctly even when the rule has an id:.
			switch r.Action {
			case "clone":
				rs[r.ClonePath] = true
			case "decrypt":
				rs[r.DecryptPath] = true
			case "download":
				rs[r.DownloadPath] = true
			case "mkdir":
				rs[r.Mkdir] = true
			case "known_hosts":
				rs[r.KnownHosts] = true
			case "gpg_key":
				rs[r.GPGKeyring] = true
			case "dotfiles":
				rs[r.DotfilesURL] = true
			case "schedule":
				rs[r.ScheduleSource] = true
				rs[handlerskg.NormalizeBlueprint(r.ScheduleSource)] = true
			case "shell":
				rs[r.ShellName] = true
			case "authorized_keys":
				rs[r.AuthorizedKeysFile] = true
				rs[r.AuthorizedKeysEncrypted] = true
			case "run":
				rs[r.RunCommand] = true
			case "run-sh":
				rs[r.RunShURL] = true
			}
			// Index every package / homebrew formula / ollama model individually.
			for _, pkg := range r.Packages {
				rs[pkg.Name] = true
			}
			for _, formula := range r.HomebrewPackages {
				rs[formula] = true
			}
			for _, model := range r.OllamaModels {
				rs[model] = true
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

	// Asdf and Mise use compound keys (plugin+version, tool+version) that cannot
	// be matched against individual blueprint rule resource keys, so they are
	// intentionally excluded from orphan detection.
	isExcluded := func(e handlerskg.StatusEntry) bool {
		switch e.(type) {
		case *handlerskg.AsdfStatus, *handlerskg.MiseStatus, *handlerskg.SudoersStatus:
			return true
		}
		return false
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

	if len(issues) == 0 {
		fmt.Printf("  %s\n", ui.FormatSuccess("blueprint URLs are normalized"))
		fmt.Printf("  %s\n", ui.FormatSuccess("no duplicate entries"))
		fmt.Printf("  %s\n", ui.FormatSuccess("no orphaned entries"))
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
