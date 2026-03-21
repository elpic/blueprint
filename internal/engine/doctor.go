package engine

import (
	"encoding/json"
	"fmt"
	"os"

	handlerskg "github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/ui"
)

// doctorIssue describes a single category of problems found by doctor.
type doctorIssue struct {
	description string
	count       int
	examples    []string // up to a small number of representative entries
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

	for _, v := range status.Packages {
		collect(v.Blueprint)
	}
	for _, v := range status.Clones {
		collect(v.Blueprint)
	}
	for _, v := range status.Decrypts {
		collect(v.Blueprint)
	}
	for _, v := range status.Mkdirs {
		collect(v.Blueprint)
	}
	for _, v := range status.KnownHosts {
		collect(v.Blueprint)
	}
	for _, v := range status.GPGKeys {
		collect(v.Blueprint)
	}
	for _, v := range status.Asdfs {
		collect(v.Blueprint)
	}
	for _, v := range status.Mises {
		collect(v.Blueprint)
	}
	for _, v := range status.Sudoers {
		collect(v.Blueprint)
	}
	for _, v := range status.Brews {
		collect(v.Blueprint)
	}
	for _, v := range status.Ollamas {
		collect(v.Blueprint)
	}
	for _, v := range status.Downloads {
		collect(v.Blueprint)
	}
	for _, v := range status.Runs {
		collect(v.Blueprint)
	}
	for _, v := range status.Dotfiles {
		collect(v.Blueprint)
	}
	for _, v := range status.Schedules {
		collect(v.Blueprint)
	}
	for _, v := range status.Shells {
		collect(v.Blueprint)
	}
	for _, v := range status.AuthorizedKeys {
		collect(v.Blueprint)
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

// checkDuplicates detects entries where the same resource appears more than once
// for the same OS — regardless of which blueprint URL form was used. This catches
// duplicates even when the blueprint URL stored in the old entry is a malformed
// form that cannot yet be normalized (e.g. pre-fix single-slash URLs).
func checkDuplicates(status *handlerskg.Status) []doctorIssue {
	type key struct{ resource, os string }
	seen := map[key]bool{}
	count := 0

	track := func(resource, os string) {
		k := key{resource, os}
		if seen[k] {
			count++
		}
		seen[k] = true
	}

	for _, v := range status.Packages {
		track(v.Name, v.OS)
	}
	for _, v := range status.Clones {
		track(v.Path, v.OS)
	}
	for _, v := range status.Decrypts {
		track(v.DestPath, v.OS)
	}
	for _, v := range status.Mkdirs {
		track(v.Path, v.OS)
	}
	for _, v := range status.KnownHosts {
		track(v.Host, v.OS)
	}
	for _, v := range status.GPGKeys {
		track(v.Keyring, v.OS)
	}
	for _, v := range status.Asdfs {
		track(v.Plugin+"\x00"+v.Version, v.OS)
	}
	for _, v := range status.Mises {
		track(v.Tool+"\x00"+v.Version, v.OS)
	}
	for _, v := range status.Sudoers {
		track(v.User, v.OS)
	}
	for _, v := range status.Brews {
		track(v.Formula, v.OS)
	}
	for _, v := range status.Ollamas {
		track(v.Model, v.OS)
	}
	for _, v := range status.Downloads {
		track(v.Path, v.OS)
	}
	for _, v := range status.Runs {
		track(v.Command, v.OS)
	}
	for _, v := range status.Dotfiles {
		track(v.URL, v.OS)
	}
	for _, v := range status.Schedules {
		track(v.Source, v.OS)
	}
	for _, v := range status.Shells {
		track(v.Shell, v.OS)
	}
	for _, v := range status.AuthorizedKeys {
		track(v.Source, v.OS)
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

	if len(issues) == 0 {
		fmt.Printf("  %s\n", ui.FormatSuccess("blueprint URLs are normalized"))
		fmt.Printf("  %s\n", ui.FormatSuccess("no duplicate entries"))
		fmt.Printf("\n%s\n\n", ui.FormatSuccess("No issues found."))
		return
	}

	if fix {
		// First normalize URLs, then deduplicate (order matters).
		handlerskg.MigrateStatus(&status)
		handlerskg.DeduplicateStatus(&status)

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
		fmt.Printf("  %s\n", ui.FormatError(fmt.Sprintf("✗ %s", issue.description)))
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
