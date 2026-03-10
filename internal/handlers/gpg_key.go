package handlers

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/elpic/blueprint/internal"
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

// buildUpCommand returns the components needed to install the GPG key.
// Exposed for testing — callers should not embed these in shell strings.
func (h *GPGKeyHandler) buildUpCommand() (gpgKeyURL, keyringPath, tmpFile, sourcesListPath string) {
	keyring := h.Rule.GPGKeyring
	gpgKeyURL = h.Rule.GPGKeyURL
	keyringPath = fmt.Sprintf("/usr/share/keyrings/%s.gpg", keyring)
	tmpFile = filepath.Join(os.TempDir(), fmt.Sprintf("sources-%s.list", keyring))
	sourcesListPath = fmt.Sprintf("/etc/apt/sources.list.d/%s.list", keyring)
	return
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
	if err := os.WriteFile(tmpFile, []byte(debSourceLine+"\n"), internal.FilePermission); err != nil {
		return "", fmt.Errorf("failed to write sources file: %w", err)
	}
	defer func() { _ = os.Remove(tmpFile) }()

	// Download GPG key and pipe to gpg --dearmor using discrete exec.Command calls
	// to prevent shell injection from user-controlled URL/keyring values.
	if err := h.downloadAndDearmor(gpgKeyURL, keyringPath); err != nil {
		return "", fmt.Errorf("failed to add GPG key: %w", err)
	}

	// Copy sources list and update apt using discrete exec.Command calls.
	cpOut, err := exec.Command("sudo", "cp", tmpFile, sourcesListPath).CombinedOutput() // #nosec G204 -- args passed discretely
	if err != nil {
		return "", fmt.Errorf("failed to copy sources list: %w\n%s", err, cpOut)
	}
	_, _ = exec.Command("sudo", "apt", "update").CombinedOutput() // #nosec G204 -- no user data in args

	return fmt.Sprintf("Added GPG key %s and repository %s", keyring, debURL), nil
}

// downloadAndDearmor downloads a GPG key from url and writes the dearmored binary
// to destPath using discrete exec.Command invocations (no shell string interpolation).
var downloadAndDearmor = func(url, destPath string) error {
	// curl -fsSL <url>
	curl := exec.Command("curl", "-fsSL", url) // #nosec G204 -- URL is user-supplied; passed as argument, not shell string
	pr, pw := io.Pipe()
	curl.Stdout = pw

	// sudo gpg --yes --dearmor -o <destPath>
	gpg := exec.Command("sudo", "gpg", "--yes", "--dearmor", "-o", destPath) // #nosec G204 -- destPath is derived from user keyring name; passed as argument
	gpg.Stdin = pr

	if err := curl.Start(); err != nil {
		return fmt.Errorf("curl start: %w", err)
	}
	if err := gpg.Start(); err != nil {
		_ = pw.Close()
		return fmt.Errorf("gpg start: %w", err)
	}

	curlErr := curl.Wait()
	_ = pw.Close()
	gpgErr := gpg.Wait()

	if curlErr != nil {
		return fmt.Errorf("curl failed: %w", curlErr)
	}
	if gpgErr != nil {
		return fmt.Errorf("gpg failed: %w", gpgErr)
	}
	return nil
}

func (h *GPGKeyHandler) downloadAndDearmor(url, destPath string) error {
	return downloadAndDearmor(url, destPath)
}

// Down removes the GPG key and repository
func (h *GPGKeyHandler) Down() (string, error) {
	keyring := h.Rule.GPGKeyring

	keyringPath := fmt.Sprintf("/usr/share/keyrings/%s.gpg", keyring)
	sourcesListPath := fmt.Sprintf("/etc/apt/sources.list.d/%s.list", keyring)

	// Use discrete exec.Command calls — never interpolate user-controlled keyring into a shell string.
	_, _ = exec.Command("sudo", "rm", "-f", sourcesListPath).CombinedOutput() // #nosec G204 -- args passed discretely
	_, _ = exec.Command("sudo", "rm", "-f", keyringPath).CombinedOutput()     // #nosec G204 -- args passed discretely
	_, _ = exec.Command("sudo", "apt", "update").CombinedOutput()             // #nosec G204 -- no user data in args

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

// GetState returns handler-specific state as key-value pairs
func (h *GPGKeyHandler) GetState(isUninstall bool) map[string]string {
	return map[string]string{
		"summary": h.GetDisplayDetails(isUninstall),
		"keyring": h.Rule.GPGKeyring,
	}
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
