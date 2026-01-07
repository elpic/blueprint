package handlers

import (
	"github.com/elpic/blueprint/internal"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
)

// GPGKeyHandler handles GPG key addition and repository management
type GPGKeyHandler struct {
	BaseHandler
}

// NewGPGKeyHandler creates a new GPG key handler
func NewGPGKeyHandler(rule parser.Rule, basePath string) *GPGKeyHandler {
	return &GPGKeyHandler{
		BaseHandler: BaseHandler{
			Rule:     rule,
			BasePath: basePath,
		},
	}
}

// Up adds the GPG key and repository
func (h *GPGKeyHandler) Up() (string, error) {
	keyring := h.Rule.GPGKeyring
	gpgKeyURL := h.Rule.GPGKeyURL
	debURL := h.Rule.GPGDebURL

	keyringPath := fmt.Sprintf("/usr/share/keyrings/%s.gpg", keyring)
	sourcesListPath := fmt.Sprintf("/etc/apt/sources.list.d/%s.list", keyring)
	debSourceLine := fmt.Sprintf("deb [signed-by=%s] %s * *", keyringPath, debURL)

	// Write sources list content to temp file
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("sources-%s.list", keyring))
	err := os.WriteFile(tmpFile, []byte(debSourceLine+"\n"), internal.FilePermission)
	if err != nil {
		return "", fmt.Errorf("failed to write sources file: %w", err)
	}

	// Combine all operations into a single command
	// needsSudo will detect "sh" and handle sudo with password caching for single authentication
	combinedCmd := fmt.Sprintf("sh -c 'curl -fsSL %s | sudo gpg --yes --dearmor -o %s 2>/dev/null || true && sudo cp %s %s && sudo apt update 2>/dev/null || true'",
		gpgKeyURL, keyringPath, tmpFile, sourcesListPath)

	_, err = executeCommandWithCache(combinedCmd)
	if err != nil {
		_ = os.Remove(tmpFile)
		return "", fmt.Errorf("failed to add GPG key and repository: %w", err)
	}

	// Clean up temp file
	_ = os.Remove(tmpFile)

	return fmt.Sprintf("Added GPG key %s and repository %s", keyring, debURL), nil
}

// Down removes the GPG key and repository
func (h *GPGKeyHandler) Down() (string, error) {
	keyring := h.Rule.GPGKeyring

	keyringPath := fmt.Sprintf("/usr/share/keyrings/%s.gpg", keyring)
	sourcesListPath := fmt.Sprintf("/etc/apt/sources.list.d/%s.list", keyring)

	// Combine all removal operations into a single command
	// needsSudo will detect "sh" and handle sudo with password caching
	combinedCmd := fmt.Sprintf("sh -c 'sudo rm -f %s && sudo rm -f %s && sudo apt update 2>/dev/null || true'",
		sourcesListPath, keyringPath)

	_, _ = executeCommandWithCache(combinedCmd) // Don't fail - files might not exist or apt update might fail

	return fmt.Sprintf("Removed GPG key %s and repository", keyring), nil
}

// GetCommand returns the actual command(s) that will be executed
func (h *GPGKeyHandler) GetCommand() string {
	if h.Rule.Action == "uninstall" {
		keyring := h.Rule.GPGKeyring
		keyringPath := fmt.Sprintf("/usr/share/keyrings/%s.gpg", keyring)
		sourcesListPath := fmt.Sprintf("/etc/apt/sources.list.d/%s.list", keyring)

		// Show the main removal steps
		return fmt.Sprintf(
			"sudo rm -f %s && sudo rm -f %s && sudo apt update",
			sourcesListPath,
			keyringPath,
		)
	}

	// Up action (install) - show the combined command
	keyring := h.Rule.GPGKeyring
	gpgKeyURL := h.Rule.GPGKeyURL

	keyringPath := fmt.Sprintf("/usr/share/keyrings/%s.gpg", keyring)
	sourcesListPath := fmt.Sprintf("/etc/apt/sources.list.d/%s.list", keyring)
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("sources-%s.list", keyring))

	// Show the combined command that runs in a single sudo session
	return fmt.Sprintf(
		"sh -c 'curl -fsSL %s | gpg --yes --dearmor -o %s && cp %s %s && sudo apt update'",
		gpgKeyURL, keyringPath, tmpFile, sourcesListPath,
	)
}

// UpdateStatus updates the status after adding or removing a GPG key
func (h *GPGKeyHandler) UpdateStatus(status *Status, records []ExecutionRecord, blueprint string, osName string) error {
	// Normalize blueprint path for consistent storage and comparison
	blueprint = normalizePath(blueprint)

	if h.Rule.Action == "gpg-key" {
		// Check if this rule's command was executed successfully
		commandExecuted := false
		for _, record := range records {
			if record.Status == "success" && strings.Contains(record.Command, "gpg-key") && strings.Contains(record.Command, h.Rule.GPGKeyring) {
				commandExecuted = true
				break
			}
		}

		if commandExecuted {
			// Remove existing entry if present
			status.GPGKeys = removeGPGKeyStatus(status.GPGKeys, h.Rule.GPGKeyring, blueprint, osName)
			// Add new entry
			status.GPGKeys = append(status.GPGKeys, GPGKeyStatus{
				Keyring:   h.Rule.GPGKeyring,
				URL:       h.Rule.GPGKeyURL,
				DebURL:    h.Rule.GPGDebURL,
				AddedAt:   time.Now().Format(time.RFC3339),
				Blueprint: blueprint,
				OS:        osName,
			})
		}
	} else if h.Rule.Action == "uninstall" && DetectRuleType(h.Rule) == "gpg-key" {
		// Remove GPG key from status if uninstall was successful
		status.GPGKeys = removeGPGKeyStatus(status.GPGKeys, h.Rule.GPGKeyring, blueprint, osName)
	}

	return nil
}

// NeedsSudo returns true because GPG key operations always require sudo
func (h *GPGKeyHandler) NeedsSudo() bool {
	return true
}

// DisplayInfo displays handler-specific information
func (h *GPGKeyHandler) DisplayInfo() {
	formatFunc := ui.FormatInfo
	if h.Rule.Action == "uninstall" {
		formatFunc = ui.FormatDim
	}

	fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Keyring: %s", h.Rule.GPGKeyring)))
	fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Repository: %s", h.Rule.GPGDebURL)))
	fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Key URL: %s", h.Rule.GPGKeyURL)))
}

// DisplayStatus displays GPG key status information
func (h *GPGKeyHandler) DisplayStatus(keys []GPGKeyStatus) {
	if len(keys) == 0 {
		return
	}

	fmt.Printf("\n%s\n", ui.FormatHighlight("GPG Keys:"))
	for _, key := range keys {
		// Parse timestamp for display
		t, err := time.Parse(time.RFC3339, key.AddedAt)
		var timeStr string
		if err == nil {
			timeStr = t.Format("2006-01-02 15:04:05")
		} else {
			timeStr = key.AddedAt
		}

		fmt.Printf("  %s %s (%s) [%s, %s]\n",
			ui.FormatSuccess("●"),
			ui.FormatInfo(key.Keyring),
			ui.FormatDim(timeStr),
			ui.FormatDim(key.OS),
			ui.FormatDim(abbreviateBlueprintPath(key.Blueprint)),
		)
	}
}

// DisplayStatusFromStatus displays GPG key handler status from Status object
func (h *GPGKeyHandler) DisplayStatusFromStatus(status *Status) {
	if status == nil || status.GPGKeys == nil {
		return
	}
	h.DisplayStatus(status.GPGKeys)
}

// GetDependencyKey returns the unique key for this rule in dependency resolution
func (h *GPGKeyHandler) GetDependencyKey() string {
	return getDependencyKey(h.Rule, h.Rule.GPGKeyring)
}

// GetDisplayDetails returns the GPG keyring to display during execution
func (h *GPGKeyHandler) GetDisplayDetails(isUninstall bool) string {
	return h.Rule.GPGKeyring
}

// FindUninstallRules compares GPG key status against current rules and returns uninstall rules
func (h *GPGKeyHandler) FindUninstallRules(status *Status, currentRules []parser.Rule, blueprintFile, osName string) []parser.Rule {
	normalizedBlueprint := normalizePath(blueprintFile)

	// Build set of current GPG keyrings from gpg-key rules
	currentGPGKeys := make(map[string]bool)
	for _, rule := range currentRules {
		if rule.Action == "gpg-key" && rule.GPGKeyring != "" {
			currentGPGKeys[rule.GPGKeyring] = true
		}
	}

	// Find GPG keys to uninstall (in status but not in current rules)
	var rules []parser.Rule
	if status.GPGKeys != nil {
		for _, gpg := range status.GPGKeys {
			normalizedStatusBlueprint := normalizePath(gpg.Blueprint)
			if normalizedStatusBlueprint == normalizedBlueprint && gpg.OS == osName && !currentGPGKeys[gpg.Keyring] {
				rules = append(rules, parser.Rule{
					Action:     "uninstall",
					GPGKeyring: gpg.Keyring,
					OSList:     []string{osName},
				})
			}
		}
	}

	return rules
}
