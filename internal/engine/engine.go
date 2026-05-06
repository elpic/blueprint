package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	gitpkg "github.com/elpic/blueprint/internal/git"
	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
)

// ExecutableName is the name to use in user-facing command hints (e.g. "Run to fix").
// Defaults to "blueprint"; set to "go run ./cmd/blueprint" when running via go run.
var ExecutableName = "blueprint"

type ExecutionRecord struct {
	Timestamp  string `json:"timestamp"`
	Blueprint  string `json:"blueprint"`
	OS         string `json:"os"`
	Command    string `json:"command"`
	Status     string `json:"status"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	Output     string `json:"output,omitempty"`
	Error      string `json:"error,omitempty"`
}

// passwordStore is a mutex-protected map of password-id → password.
type passwordStore struct {
	mu sync.RWMutex
	m  map[string]string
}

func (p *passwordStore) get(key string) (string, bool) {
	p.mu.RLock()
	v, ok := p.m[key]
	p.mu.RUnlock()
	return v, ok
}

func (p *passwordStore) set(key, value string) {
	p.mu.Lock()
	p.m[key] = value
	p.mu.Unlock()
}

func (p *passwordStore) snapshot() map[string]string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make(map[string]string, len(p.m))
	for k, v := range p.m {
		out[k] = v
	}
	return out
}

// passwordCache stores decryption passwords by password-id to avoid re-prompting
var passwordCache = &passwordStore{m: make(map[string]string)}

// RunWithSkip executes the blueprint and returns an exit code:
// 0 = success (all rules applied or dry-run completed),
// 1 = one or more rules failed or a fatal error occurred.
func RunWithSkip(file string, dry bool, skipGroup string, skipID string, onlyID string, skipDecrypt bool, preferSSH bool, noStatus bool) int {
	if preferSSH {
		file = gitpkg.ExpandShorthandSSH(file)
	} else {
		file = gitpkg.ExpandShorthand(file)
	}
	var runNumber int

	// Get next run number (only for non-dry runs)
	if !dry {
		var err error
		runNumber, err = getNextRunNumber()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to get run number: %v\n", err)
			runNumber = 0 // Disable history saving if we can't get run number
		}
	}

	setupPath, blueprintSHA, cleanup, err := resolveBlueprintFile(file, dry, preferSSH)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return 1
	}
	defer cleanup()

	// Parse the setup file (with include support for both local and git repositories)
	var rules []parser.Rule
	// Use ParseFile for both local files and git repositories
	// This enables include directive support in both cases
	rules, err = parser.ParseFile(setupPath)
	if err != nil {
		fmt.Println("Parse error:", err)
		return 1
	}

	// Interpolate ${VAR_NAME} in all rules before any further processing so that
	// paths like ${WORKSPACE}/repo are resolved consistently everywhere — OS
	// filtering, skip/only flags, auto-uninstall comparisons, execution, and
	// status saving all see the same expanded values.
	{
		vars := resolveVarMap(rules, nil)
		for i, r := range rules {
			rules[i] = interpolateRule(r, vars)
		}
	}

	// Filter rules by current OS first, before applying skip flags.
	// We keep the full OS-filtered set separately so auto-uninstall comparisons
	// see the complete blueprint (skipped rules should not trigger uninstalls).
	currentOS := getOSName()
	allOSRules := filterRulesByOS(rules)

	// Filter rules by skip/only flags
	var filteredRules []parser.Rule
	for _, rule := range allOSRules {
		if onlyID != "" {
			// --only: keep only the rule with this ID
			if rule.ID == onlyID {
				filteredRules = append(filteredRules, rule)
			}
			continue
		}
		if skipGroup != "" && rule.Group == skipGroup {
			continue
		}
		if skipID != "" && rule.ID == skipID {
			continue
		}
		if skipDecrypt && rule.Action == "decrypt" {
			continue
		}
		filteredRules = append(filteredRules, rule)
	}

	if onlyID != "" && len(filteredRules) == 0 {
		fmt.Printf("No rule found with id: %s\n", onlyID)
		return 1
	}

	// Check history and add auto-uninstall rules for removed packages.
	// Skip auto-uninstall when --only is set (we're targeting one specific rule).
	// Use allOSRules (not filteredRules) so that rules excluded by skip flags
	// are not mistakenly treated as "removed from the blueprint".
	var autoUninstallRules []parser.Rule
	if onlyID == "" {
		autoUninstallRules = getAutoUninstallRules(allOSRules, file, currentOS)
	}
	allRules := append(filteredRules, autoUninstallRules...)

	// Count cleanup operations only when not using skip/only options
	var numCleanups int
	if skipGroup == "" && skipID == "" && onlyID == "" {
		numCleanups = len(autoUninstallRules)
	}

	// Extract base directory from setupPath for resolving relative file paths
	basePath := filepath.Dir(setupPath)

	if dry {
		ui.PrintExecutionHeader(false, currentOS, file, len(filteredRules), len(autoUninstallRules), numCleanups)
		displayRules(filteredRules)
		if len(autoUninstallRules) > 0 {
			ui.PrintAutoUninstallSection()
			displayRules(autoUninstallRules)
		}
		ui.PrintPlanFooter()
		return 0
	}

	ui.PrintExecutionHeader(true, currentOS, file, len(filteredRules), len(autoUninstallRules), numCleanups)

	// Prompt for sudo password upfront (before decrypt passwords)
	// Check all rules including auto-uninstall rules
	if err := promptForSudoPasswordWithOS(allRules, currentOS); err != nil {
		fmt.Printf("%s\n", ui.FormatError(fmt.Sprintf("Error prompting for sudo password: %v", err)))
		return 1
	}

	// Prompt for all decrypt passwords upfront
	if err := promptForDecryptPasswords(allRules); err != nil {
		fmt.Printf("%s\n", ui.FormatError(fmt.Sprintf("Error prompting for passwords: %v", err)))
		return 1
	}

	records := executeRules(allRules, file, currentOS, basePath, runNumber)
	if err := saveHistory(records); err != nil {
		fmt.Printf("Warning: Failed to save history: %v\n", err)
	}
	// Use the original file path/URL for status (never temp paths)
	if !noStatus {
		if err := saveStatus(allRules, records, file, blueprintSHA, currentOS); err != nil {
			fmt.Printf("Warning: Failed to save status: %v\n", err)
		}
	}

	// Clear sudo cache on all operating systems
	clearSudoCache()

	// Return exit code 1 if any rule failed.
	for _, r := range records {
		if r.Status == "error" {
			return 1
		}
	}
	return 0
}

func Run(file string, dry bool) int {
	return RunWithSkip(file, dry, "", "", "", false, false, false)
}
