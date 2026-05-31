package handlers

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/platform"
	"github.com/elpic/blueprint/internal/ui"
)

func init() {
	RegisterAction(ActionDef{
		Name:   "replace",
		Prefix: "replace ",
		NewHandler: func(rule parser.Rule, basePath string, passwordCache map[string]string) Handler {
			return NewReplaceHandler(rule, basePath, platform.NewContainer())
		},
		RuleKey: func(rule parser.Rule) string {
			return rule.ReplaceFile + "\x00" + rule.ReplaceMatch
		},
		Detect: func(rule parser.Rule) bool {
			return rule.ReplaceFile != "" && rule.ReplaceMatch != ""
		},
		Summary: func(rule parser.Rule) string {
			return fmt.Sprintf("%s (match: %q → %q)", rule.ReplaceFile, rule.ReplaceMatch, rule.ReplaceWith)
		},
		OrphanIndex: func(rule parser.Rule, index func(string)) {
			index(rule.ReplaceFile + "\x00" + rule.ReplaceMatch)
		},
		ShellExport: func(rule parser.Rule, _, _ string) []string {
			path := shellHome(rule.ReplaceFile)
			// Escape single quotes for shell safety
			match := strings.ReplaceAll(rule.ReplaceMatch, "'", "'\\''")
			with := strings.ReplaceAll(rule.ReplaceWith, "'", "'\\''")
			// Use sed for a simple first-occurrence replacement
			return []string{fmt.Sprintf("sed -i '0,/%s/s//%s/' %s", match, with, path)}
		},
	})
}

// ReplaceHandler handles find-and-replace operations in managed files
type ReplaceHandler struct {
	BaseHandler
}

// NewReplaceHandler creates a new replace handler with dependency injection
func NewReplaceHandler(rule parser.Rule, basePath string, container platform.Container) *ReplaceHandler {
	return &ReplaceHandler{
		BaseHandler: BaseHandler{
			Rule:      rule,
			BasePath:  basePath,
			Container: container,
		},
	}
}

// NewReplaceHandlerLegacy creates a new replace handler without container (for backward compatibility)
func NewReplaceHandlerLegacy(rule parser.Rule, basePath string) *ReplaceHandler {
	return NewReplaceHandler(rule, basePath, platform.NewContainer())
}

// Up performs the find-and-replace in the target file
func (h *ReplaceHandler) Up() (string, error) {
	filePath := expandPath(h.Rule.ReplaceFile)
	match := h.Rule.ReplaceMatch
	with := h.Rule.ReplaceWith

	// Read the file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	text := string(content)

	// Find the first occurrence
	idx := strings.Index(text, match)
	if idx < 0 {
		return "", fmt.Errorf("match %q not found in %s", match, filePath)
	}

	// Replace only the first occurrence
	newText := text[:idx] + with + text[idx+len(match):]

	// Write back to file
	if err := os.WriteFile(filePath, []byte(newText), 0644); err != nil {
		return "", fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	return fmt.Sprintf("Replaced %q with %q in %s", match, with, filePath), nil
}

// Down reverses the find-and-replace (finds the "with" text and replaces back with "match")
func (h *ReplaceHandler) Down() (string, error) {
	filePath := expandPath(h.Rule.ReplaceFile)
	match := h.Rule.ReplaceMatch
	with := h.Rule.ReplaceWith

	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Sprintf("File %s does not exist, nothing to undo", filePath), nil
		}
		return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	text := string(content)

	// Find the first occurrence of the "with" text (what we previously replaced)
	idx := strings.Index(text, with)
	if idx < 0 {
		return "", fmt.Errorf("cannot undo: %q not found in %s", with, filePath)
	}

	// Replace back to the original text
	newText := text[:idx] + match + text[idx+len(with):]

	if err := os.WriteFile(filePath, []byte(newText), 0644); err != nil {
		return "", fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	return fmt.Sprintf("Restored %q from %q in %s", match, with, filePath), nil
}

// GetCommand returns the actual command(s) that will be executed
func (h *ReplaceHandler) GetCommand() string {
	filePath := h.Rule.ReplaceFile
	match := h.Rule.ReplaceMatch
	with := h.Rule.ReplaceWith

	if h.Rule.Action == "uninstall" {
		return fmt.Sprintf("replace (undo) %s: restore %q from %q", filePath, match, with)
	}
	return fmt.Sprintf("replace %s: %q → %q", filePath, match, with)
}

// UpdateStatus updates the blueprint status after executing replace or undo
func (h *ReplaceHandler) UpdateStatus(status *Status, records []ExecutionRecord, blueprint string, osName string) error {
	blueprint = normalizeBlueprint(blueprint)

	if h.Rule.Action == "replace" {
		expectedCmd := h.GetCommand()
		executed := false
		for _, record := range records {
			if record.Status == "success" && record.Command == expectedCmd {
				executed = true
				break
			}
		}

		if executed {
			status.Replaces = removeReplaceStatus(status.Replaces, h.Rule.ReplaceFile, h.Rule.ReplaceMatch, blueprint, osName)
			status.Replaces = append(status.Replaces, ReplaceStatus{
				File:       h.Rule.ReplaceFile,
				Match:      h.Rule.ReplaceMatch,
				With:       h.Rule.ReplaceWith,
				ReplacedAt: time.Now().Format(time.RFC3339),
				Blueprint:  blueprint,
				OS:         osName,
			})
		}
	} else if h.Rule.Action == "uninstall" && DetectRuleType(h.Rule) == "replace" {
		// Check if the replace was undone successfully by checking if match text
		// is back in the file (i.e. the undo happened)
		filePath := expandPath(h.Rule.ReplaceFile)
		content, err := os.ReadFile(filePath)
		if err == nil {
			if strings.Contains(string(content), h.Rule.ReplaceMatch) {
				// Undo was successful, remove from status
				status.Replaces = removeReplaceStatus(status.Replaces, h.Rule.ReplaceFile, h.Rule.ReplaceMatch, blueprint, osName)
			}
		}
	}

	return nil
}

// DisplayInfo displays handler-specific information
func (h *ReplaceHandler) DisplayInfo() {
	formatFunc := ui.FormatInfo
	if h.Rule.Action == "uninstall" {
		formatFunc = ui.FormatDim
	}

	fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("File: %s", h.Rule.ReplaceFile)))
	fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Match: %s", h.Rule.ReplaceMatch)))
	fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("With: %s", h.Rule.ReplaceWith)))
}

// GetDependencyKey returns the unique key for this rule in dependency resolution
func (h *ReplaceHandler) GetDependencyKey() string {
	fallback := "replace"
	if h.Rule.ReplaceFile != "" && h.Rule.ReplaceMatch != "" {
		fallback = h.Rule.ReplaceFile + "\x00" + h.Rule.ReplaceMatch
	}
	return getDependencyKey(h.Rule, fallback)
}

// GetDisplayDetails returns the replace details to display during execution
func (h *ReplaceHandler) GetDisplayDetails(isUninstall bool) string {
	return fmt.Sprintf("%s: %q → %q", h.Rule.ReplaceFile, h.Rule.ReplaceMatch, h.Rule.ReplaceWith)
}

// GetState returns handler-specific state as key-value pairs
func (h *ReplaceHandler) GetState(isUninstall bool) map[string]string {
	return map[string]string{
		"summary": h.GetDisplayDetails(isUninstall),
		"file":    h.Rule.ReplaceFile,
		"match":   h.Rule.ReplaceMatch,
		"with":    h.Rule.ReplaceWith,
	}
}

// FindUninstallRules compares replace status against current rules and returns uninstall rules
func (h *ReplaceHandler) FindUninstallRules(status *Status, currentRules []parser.Rule, blueprintFile, osName string) []parser.Rule {
	normalizedBlueprint := normalizeBlueprint(blueprintFile)

	// Build set of current replace resource keys from replace rules
	currentReplaceKeys := make(map[string]bool)
	for _, rule := range currentRules {
		if rule.Action == "replace" && rule.ReplaceFile != "" && rule.ReplaceMatch != "" {
			key := rule.ReplaceFile + "\x00" + rule.ReplaceMatch
			currentReplaceKeys[key] = true
		}
	}

	// Find replaces to uninstall (in status but not in current rules)
	var rules []parser.Rule
	if status.Replaces != nil {
		for _, replace := range status.Replaces {
			resourceKey := replace.File + "\x00" + replace.Match
			normalizedStatusBlueprint := normalizeBlueprint(replace.Blueprint)
			if normalizedStatusBlueprint == normalizedBlueprint && replace.OS == osName && !currentReplaceKeys[resourceKey] {
				rules = append(rules, parser.Rule{
					Action:       "uninstall",
					ReplaceFile:  replace.File,
					ReplaceMatch: replace.Match,
					ReplaceWith:  replace.With,
					OSList:       []string{osName},
				})
			}
		}
	}

	return rules
}

// IsInstalled returns true if the replace entry for (file, match) is already in status
func (h *ReplaceHandler) IsInstalled(status *Status, blueprintFile, osName string) bool {
	normalizedBlueprint := normalizeBlueprint(blueprintFile)
	for _, replace := range status.Replaces {
		if replace.File == h.Rule.ReplaceFile &&
			replace.Match == h.Rule.ReplaceMatch &&
			normalizeBlueprint(replace.Blueprint) == normalizedBlueprint &&
			replace.OS == osName {
			return true
		}
	}
	return false
}

// DisplayStatus displays replace status information
func (h *ReplaceHandler) DisplayStatus(replaces []ReplaceStatus) {
	if len(replaces) == 0 {
		return
	}

	fmt.Printf("\n%s\n", ui.FormatHighlight("Replace Operations:"))
	for _, replace := range replaces {
		t, err := time.Parse(time.RFC3339, replace.ReplacedAt)
		var timeStr string
		if err == nil {
			timeStr = t.Format("2006-01-02 15:04:05")
		} else {
			timeStr = replace.ReplacedAt
		}

		fmt.Printf("  %s %s:%s (%s) [%s, %s]\n",
			ui.FormatSuccess("●"),
			ui.FormatInfo(replace.File),
			ui.FormatDim(fmt.Sprintf(" %q → %q", replace.Match, replace.With)),
			ui.FormatDim(timeStr),
			ui.FormatDim(replace.OS),
			ui.FormatDim(abbreviateBlueprintPath(replace.Blueprint)),
		)
	}
}

// DisplayStatusFromStatus displays replace handler status from Status object
func (h *ReplaceHandler) DisplayStatusFromStatus(status *Status) {
	if status == nil || status.Replaces == nil {
		return
	}
	h.DisplayStatus(status.Replaces)
}

// removeReplaceStatus removes a replace status entry by its resource key
func removeReplaceStatus(sl []ReplaceStatus, file, match, blueprint, osName string) []ReplaceStatus {
	return removeStatusEntry[ReplaceStatus, *ReplaceStatus](sl, file+"\x00"+match, blueprint, osName)
}
