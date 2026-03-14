package handlers

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
)

// GPGKeyHandler handles GPG key addition and repository management
type GPGKeyHandler struct {
	BaseHandler
	sudoPassword string
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

// NewGPGKeyHandlerWithPassword creates a new GPG key handler with a cached sudo password.
func NewGPGKeyHandlerWithPassword(rule parser.Rule, basePath, sudoPassword string) *GPGKeyHandler {
	return &GPGKeyHandler{
		BaseHandler: BaseHandler{
			Rule:     rule,
			BasePath: basePath,
		},
		sudoPassword: sudoPassword,
	}
}

// sudoCommand returns a sudo command that injects the cached password via stdin
// when available, falling back to plain sudo otherwise.
func (h *GPGKeyHandler) sudoCommand(args ...string) *exec.Cmd {
	if h.sudoPassword != "" {
		allArgs := append([]string{"-S"}, args...)
		cmd := exec.Command("sudo", allArgs...) // #nosec G204
		cmd.Stdin = strings.NewReader(h.sudoPassword + "\n")
		return cmd
	}
	return exec.Command("sudo", args...) // #nosec G204
}

// keyringPath returns the path where the ASCII-armored key is stored.
// Uses /etc/apt/keyrings/ (modern admin-managed location, apt 2.4+).
func (h *GPGKeyHandler) keyringPath() string {
	return fmt.Sprintf("/etc/apt/keyrings/%s.asc", h.Rule.GPGKeyring)
}

// sourcesListPath returns the apt sources list path for this rule.
func (h *GPGKeyHandler) sourcesListPath() string {
	return fmt.Sprintf("/etc/apt/sources.list.d/%s.list", h.Rule.GPGKeyring)
}

// isKeyringInstalled returns true if the keyring file already exists on disk.
func isKeyringInstalled(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// isRepoConfigured returns true if the repo URL is already present in any
// file under /etc/apt/sources.list.d/.
func isRepoConfigured(repoURL string) bool {
	entries, err := os.ReadDir("/etc/apt/sources.list.d/")
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile("/etc/apt/sources.list.d/" + e.Name()) // #nosec G304
		if err != nil {
			continue
		}
		if strings.Contains(string(data), repoURL) {
			return true
		}
	}
	return false
}

// Up adds the GPG key and repository, skipping both if already configured.
func (h *GPGKeyHandler) Up() (string, error) {
	keyringPath := h.keyringPath()
	sourcesPath := h.sourcesListPath()
	debURL := h.Rule.GPGDebURL
	debSourceLine := fmt.Sprintf("deb [signed-by=%s] %s * *", keyringPath, debURL)

	keyExists := isKeyringInstalled(keyringPath)
	repoExists := isRepoConfigured(debURL)

	if keyExists && repoExists {
		return fmt.Sprintf("already configured: %s", h.Rule.GPGKeyring), nil
	}

	needsUpdate := false

	// Ensure /etc/apt/keyrings exists (not present on Ubuntu < 22.04)
	if mkdirOut, err := h.sudoCommand("install", "-m", "0755", "-d", "/etc/apt/keyrings").CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to create keyrings directory: %w\n%s", err, mkdirOut)
	}

	if !keyExists {
		// Download the ASCII-armored key directly — APT 1.4+ reads .asc natively,
		// so we can skip gpg --dearmor entirely.
		if err := h.downloadKey(h.Rule.GPGKeyURL, keyringPath); err != nil {
			return "", fmt.Errorf("failed to download GPG key: %w", err)
		}
		// Ensure the key is world-readable so apt (running as root) can read it.
		if chmodOut, err := h.sudoCommand("chmod", "go+r", keyringPath).CombinedOutput(); err != nil {
			return "", fmt.Errorf("failed to set key permissions: %w\n%s", err, chmodOut)
		}
		needsUpdate = true
	}

	if !repoExists {
		// Write sources list to a temp file and copy with sudo (avoids shell redirection with elevated privileges).
		tmpFile, err := os.CreateTemp("", fmt.Sprintf("blueprint-sources-%s-*.list", h.Rule.GPGKeyring))
		if err != nil {
			return "", fmt.Errorf("failed to create temp sources file: %w", err)
		}
		tmpPath := tmpFile.Name()
		defer func() { _ = os.Remove(tmpPath) }()

		if _, err := tmpFile.WriteString(debSourceLine + "\n"); err != nil {
			_ = tmpFile.Close()
			return "", fmt.Errorf("failed to write sources file: %w", err)
		}
		_ = tmpFile.Close()

		if cpOut, err := h.sudoCommand("cp", tmpPath, sourcesPath).CombinedOutput(); err != nil {
			return "", fmt.Errorf("failed to install sources list: %w\n%s", err, cpOut)
		}
		needsUpdate = true
	}

	if needsUpdate {
		_, _ = h.sudoCommand("apt-get", "update").CombinedOutput()
	}

	return fmt.Sprintf("added GPG key %s and repository %s", h.Rule.GPGKeyring, debURL), nil
}

// downloadKey fetches a GPG key from url and writes it (as-is) to destPath via sudo tee.
// Storing the raw .asc avoids gpg --dearmor; APT 1.4+ reads ASCII-armored keys directly.
var downloadKey = func(url, destPath, sudoPassword string) error {
	// First download to a temp file to avoid piping directly into sudo, which
	// complicates password injection (sudo -S reads password from stdin, leaving
	// no clean way to also feed the key data through the same stdin).
	tmp, err := os.CreateTemp("", "blueprint-gpgkey-*.asc")
	if err != nil {
		return fmt.Errorf("temp file: %w", err)
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	// curl -fsSL <url> -o <tmpPath>
	curlOut, err := exec.Command("curl", "-fsSL", url, "-o", tmpPath).CombinedOutput() // #nosec G204 -- URL is user-supplied; passed as arg, never in shell string
	if err != nil {
		return fmt.Errorf("curl failed: %w\n%s", err, curlOut)
	}

	// sudo [-S] cp <tmpPath> <destPath>
	var cpCmd *exec.Cmd
	if sudoPassword != "" {
		cpCmd = exec.Command("sudo", "-S", "cp", tmpPath, destPath) // #nosec G204
		cpCmd.Stdin = strings.NewReader(sudoPassword + "\n")
	} else {
		cpCmd = exec.Command("sudo", "cp", tmpPath, destPath) // #nosec G204
	}
	if cpOut, err := cpCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sudo cp failed: %w\n%s", err, cpOut)
	}
	return nil
}

func (h *GPGKeyHandler) downloadKey(url, destPath string) error {
	return downloadKey(url, destPath, h.sudoPassword)
}

// Down removes the GPG key and repository
func (h *GPGKeyHandler) Down() (string, error) {
	keyring := h.Rule.GPGKeyring
	keyringPath := h.keyringPath()
	sourcesPath := h.sourcesListPath()

	_, _ = h.sudoCommand("rm", "-f", sourcesPath).CombinedOutput()
	_, _ = h.sudoCommand("rm", "-f", keyringPath).CombinedOutput()
	_, _ = h.sudoCommand("apt-get", "update").CombinedOutput()

	return fmt.Sprintf("removed GPG key %s and repository", keyring), nil
}

// GetCommand returns the actual command(s) that will be executed
func (h *GPGKeyHandler) GetCommand() string {
	keyringPath := h.keyringPath()
	sourcesPath := h.sourcesListPath()

	if h.Rule.Action == "uninstall" {
		return fmt.Sprintf(
			"sudo rm -f %s && sudo rm -f %s && sudo apt-get update",
			sourcesPath, keyringPath,
		)
	}

	return fmt.Sprintf(
		"curl -fsSL %s | sudo tee %s && echo 'deb [signed-by=%s] %s * *' | sudo tee %s && sudo apt-get update",
		h.Rule.GPGKeyURL, keyringPath, keyringPath, h.Rule.GPGDebURL, sourcesPath,
	)
}

// UpdateStatus updates the status after adding or removing a GPG key
func (h *GPGKeyHandler) UpdateStatus(status *Status, records []ExecutionRecord, blueprint string, osName string) error {
	blueprint = normalizePath(blueprint)

	if h.Rule.Action == "gpg-key" {
		commandExecuted := false
		for _, record := range records {
			if record.Status == "success" && strings.Contains(record.Command, "gpg-key") && strings.Contains(record.Command, h.Rule.GPGKeyring) {
				commandExecuted = true
				break
			}
		}

		if commandExecuted {
			status.GPGKeys = removeGPGKeyStatus(status.GPGKeys, h.Rule.GPGKeyring, blueprint, osName)
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

	currentGPGKeys := make(map[string]bool)
	for _, rule := range currentRules {
		if rule.Action == "gpg-key" && rule.GPGKeyring != "" {
			currentGPGKeys[rule.GPGKeyring] = true
		}
	}

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

// IsInstalled returns true if the GPG keyring in this rule is already in status.
func (h *GPGKeyHandler) IsInstalled(status *Status, blueprintFile, osName string) bool {
	normalizedBlueprint := normalizePath(blueprintFile)
	for _, gpg := range status.GPGKeys {
		if gpg.Keyring == h.Rule.GPGKeyring && normalizePath(gpg.Blueprint) == normalizedBlueprint && gpg.OS == osName {
			return true
		}
	}
	return false
}
