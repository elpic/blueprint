package handlers

import (
	"fmt"
	"github.com/elpic/blueprint/internal"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
)

// KnownHostsHandler handles SSH known_hosts file management
type KnownHostsHandler struct {
	BaseHandler
}

// NewKnownHostsHandler creates a new known_hosts handler
func NewKnownHostsHandler(rule parser.Rule, basePath string) *KnownHostsHandler {
	return &KnownHostsHandler{
		BaseHandler: BaseHandler{
			Rule:     rule,
			BasePath: basePath,
		},
	}
}

// Up adds the host to known_hosts file
func (h *KnownHostsHandler) Up() (string, error) {
	// Validate hostname
	if !isValidHostname(h.Rule.KnownHosts) {
		return "", fmt.Errorf("invalid hostname: %s (contains invalid characters)", h.Rule.KnownHosts)
	}

	knownHostsPath, err := knownHostsFile(true)
	if err != nil {
		return "", err
	}

	cmd := exec.Command("sh", "-c", h.GetCommand())
	output, err := cmd.Output()

	if err != nil {
		// Collect error messages for each key type
		errMsg := strings.TrimSpace(string(output))
		if errMsg == "" {
			errMsg = "unknown error"
		}
		return "", fmt.Errorf("failed to add host to known_hosts - \nDetails:\n%s", errMsg)
	}

	// Append the scanned key to known_hosts
	f, openErr := os.OpenFile(knownHostsPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if openErr != nil {
		return "", fmt.Errorf("failed to open known_hosts for writing: %w", openErr)
	}
	defer f.Close() //nolint:errcheck
	if _, writeErr := f.Write(output); writeErr != nil {
		return "", fmt.Errorf("failed to write to known_hosts: %w", writeErr)
	}

	return fmt.Sprintf("Added %s to known_hosts", h.Rule.KnownHosts), nil
}

// DownWithRunner removes the host using an injectable command runner (for testing).
// BUG: currently ignores runner errors — test will catch this.
func (h *KnownHostsHandler) DownWithRunner(run func(cmd string) error) (string, error) {
	if !isValidHostname(h.Rule.KnownHosts) {
		return "", fmt.Errorf("invalid hostname: %s (contains invalid characters)", h.Rule.KnownHosts)
	}

	removeHostCmd := fmt.Sprintf(`sed -i.bak '/^%s[, ]/d' ~/.ssh/known_hosts 2>/dev/null || true &&  rm -f ~/.ssh/known_hosts.bak 2>/dev/null || true`, escapeForSed(h.Rule.KnownHosts))

	if err := run(removeHostCmd); err != nil {
		return "", fmt.Errorf("could not remove %s from known_hosts: %w", h.Rule.KnownHosts, err)
	}

	return fmt.Sprintf("Removed %s from known_hosts", h.Rule.KnownHosts), nil
}

// Down removes the host from known_hosts file
func (h *KnownHostsHandler) Down() (string, error) {
	// Validate hostname
	if !isValidHostname(h.Rule.KnownHosts) {
		return "", fmt.Errorf("invalid hostname: %s (contains invalid characters)", h.Rule.KnownHosts)
	}

	// Check if known_hosts file exists
	if _, err := knownHostsFile(false); err != nil {
		return "", err
	}

	return h.DownWithRunner(func(cmd string) error {
		out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%w: %s", err, out)
		}
		return nil
	})
}

func sshDir(create bool) (string, error) {
	var homePath string
	var err error

	if homePath, err = os.UserHomeDir(); err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	sshPath := filepath.Join(homePath, ".ssh")

	if create {
		// Create .ssh directory with proper permissions (700)
		if err := os.MkdirAll(sshPath, internal.SensitiveDirectoryPermission); err != nil {
			return "", fmt.Errorf("failed to create .ssh directory: %w", err)
		}
	}

	return sshPath, nil
}

func knownHostsFile(create bool) (string, error) {
	sshPath, err := sshDir(create)

	if err != nil {
		return "", err
	}

	knownHostsPath := filepath.Join(sshPath, "known_hosts")

	if create {
		if _, err := os.Stat(knownHostsPath); os.IsNotExist(err) {
			if err := os.WriteFile(knownHostsPath, []byte{}, internal.FilePermission); err != nil {
				return "", fmt.Errorf("failed to create known_hosts file: %w", err)
			}
		}
	} else {
		if _, err := os.Stat(knownHostsPath); os.IsNotExist(err) {
			return "", fmt.Errorf("known_hosts file does not exist")
		}
	}

	return knownHostsPath, nil
}

// escapeForSed escapes special characters for use in sed regex
func escapeForSed(s string) string {
	// Escape special sed characters
	replacer := strings.NewReplacer(
		".", "\\.",
		"[", "\\[",
		"]", "\\]",
		"*", "\\*",
		"^", "\\^",
		"$", "\\$",
		"\\", "\\\\",
	)
	return replacer.Replace(s)
}

// GetCommand returns the actual command(s) that will be executed
func (h *KnownHostsHandler) GetCommand() string {
	if h.Rule.Action == "uninstall" {
		// Return the sed command for removing host
		return fmt.Sprintf(`sed -i.bak '/^%s[, ]/d' ~/.ssh/known_hosts && rm -f ~/.ssh/known_hosts.bak`, escapeForSed(h.Rule.KnownHosts))
	}

	// For known_hosts add action, return the ssh-keyscan command
	keyType := getKeyType(h)
	return fmt.Sprintf("ssh-keyscan -t %s %s", keyType, h.Rule.KnownHosts)
}

// UpdateStatus updates the status after adding or removing a known host
func (h *KnownHostsHandler) UpdateStatus(status *Status, records []ExecutionRecord, blueprint string, osName string) error {
	// Normalize blueprint path for consistent storage and comparison
	blueprint = normalizeBlueprint(blueprint)

	if h.Rule.Action == "known_hosts" {
		// Check if this rule's command was executed successfully
		// Look for a record indicating success
		commandExecuted := false
		var keyType string
		for _, record := range records {
			if record.Status == "success" && strings.Contains(record.Command, "ssh-keyscan") && strings.Contains(record.Command, h.Rule.KnownHosts) {
				commandExecuted = true
				// Extract key type from the command
				if strings.Contains(record.Command, "ed25519") {
					keyType = "ed25519"
				} else if strings.Contains(record.Command, "ecdsa") {
					keyType = "ecdsa"
				} else if strings.Contains(record.Command, "rsa") {
					keyType = "rsa"
				}
				break
			}
		}

		if commandExecuted {
			// Remove existing entry if present
			status.KnownHosts = removeKnownHostsStatus(status.KnownHosts, h.Rule.KnownHosts, blueprint, osName)
			// Add new entry
			status.KnownHosts = append(status.KnownHosts, KnownHostsStatus{
				Host:      h.Rule.KnownHosts,
				KeyType:   keyType,
				AddedAt:   time.Now().Format(time.RFC3339),
				Blueprint: blueprint,
				OS:        osName,
			})
		}
	} else if h.Rule.Action == "uninstall" && DetectRuleType(h.Rule) == "known_hosts" {
		// Remove known host from status if uninstall was successful
		status.KnownHosts = removeKnownHostsStatus(status.KnownHosts, h.Rule.KnownHosts, blueprint, osName)
	}

	return nil
}

func getKeyType(h *KnownHostsHandler) string {
	keyType := "ed25519" // Default to ed25519

	if h.Rule.KnownHostsKey != "" {
		keyType = h.Rule.KnownHostsKey
	}

	return keyType
}

// DisplayInfo displays handler-specific information
func (h *KnownHostsHandler) DisplayInfo() {
	formatFunc := ui.FormatInfo
	if h.Rule.Action == "uninstall" {
		formatFunc = ui.FormatDim
	}

	fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Host: %s", h.Rule.KnownHosts)))

	keyTypeDisplay := h.Rule.KnownHostsKey
	if keyTypeDisplay == "" {
		keyTypeDisplay = "auto-detect (ed25519, ecdsa, rsa)"
	}
	fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Key Type: %s", keyTypeDisplay)))
}

// isValidHostname validates that a hostname is safe to use in shell commands
// Allows alphanumeric, dots, hyphens, and underscores (valid DNS names and IPs)
func isValidHostname(hostname string) bool {
	if hostname == "" {
		return false
	}
	// Match valid hostname pattern: alphanumeric, dots, hyphens, underscores
	matched, err := regexp.MatchString(`^[a-zA-Z0-9._\-]+$`, hostname)
	if err != nil {
		return false
	}
	return matched
}

// DisplayStatus displays SSH known host status information
func (h *KnownHostsHandler) DisplayStatus(hosts []KnownHostsStatus) {
	if len(hosts) == 0 {
		return
	}

	fmt.Printf("\n%s\n", ui.FormatHighlight("SSH Known Hosts:"))
	for _, kh := range hosts {
		// Parse timestamp for display
		t, err := time.Parse(time.RFC3339, kh.AddedAt)
		var timeStr string
		if err == nil {
			timeStr = t.Format("2006-01-02 15:04:05")
		} else {
			timeStr = kh.AddedAt
		}

		keyTypeStr := kh.KeyType
		if keyTypeStr == "" {
			keyTypeStr = "unknown"
		}

		fmt.Printf("  %s %s (%s, %s) [%s, %s]\n",
			ui.FormatSuccess("●"),
			ui.FormatInfo(kh.Host),
			ui.FormatDim(keyTypeStr),
			ui.FormatDim(timeStr),
			ui.FormatDim(kh.OS),
			ui.FormatDim(abbreviateBlueprintPath(kh.Blueprint)),
		)
	}
}

// DisplayStatusFromStatus displays known hosts handler status from Status object
func (h *KnownHostsHandler) DisplayStatusFromStatus(status *Status) {
	if status == nil || status.KnownHosts == nil {
		return
	}
	h.DisplayStatus(status.KnownHosts)
}

// GetDependencyKey returns the unique key for this rule in dependency resolution
func (h *KnownHostsHandler) GetDependencyKey() string {
	return getDependencyKey(h.Rule, h.Rule.KnownHosts)
}

// GetDisplayDetails returns the known host to display during execution
func (h *KnownHostsHandler) GetDisplayDetails(isUninstall bool) string {
	return h.Rule.KnownHosts
}

// GetState returns handler-specific state as key-value pairs
func (h *KnownHostsHandler) GetState(isUninstall bool) map[string]string {
	return map[string]string{
		"summary": h.GetDisplayDetails(isUninstall),
		"host":    h.Rule.KnownHosts,
	}
}

// FindUninstallRules compares known hosts status against current rules and returns uninstall rules
func (h *KnownHostsHandler) FindUninstallRules(status *Status, currentRules []parser.Rule, blueprintFile, osName string) []parser.Rule {
	normalizedBlueprint := normalizeBlueprint(blueprintFile)

	// Build set of current known hosts from known_hosts rules
	currentKnownHosts := make(map[string]bool)
	for _, rule := range currentRules {
		if rule.Action == "known_hosts" && rule.KnownHosts != "" {
			currentKnownHosts[rule.KnownHosts] = true
		}
	}

	// Find known hosts to uninstall (in status but not in current rules)
	var rules []parser.Rule
	if status.KnownHosts != nil {
		for _, host := range status.KnownHosts {
			normalizedStatusBlueprint := normalizeBlueprint(host.Blueprint)
			if normalizedStatusBlueprint == normalizedBlueprint && host.OS == osName && !currentKnownHosts[host.Host] {
				rules = append(rules, parser.Rule{
					Action:     "uninstall",
					KnownHosts: host.Host,
					OSList:     []string{osName},
				})
			}
		}
	}

	return rules
}

// IsInstalled returns true if the known host is recorded in status AND is present in ~/.ssh/known_hosts.
// Checking the file prevents stale status entries from causing the key write to be skipped.
func (h *KnownHostsHandler) IsInstalled(status *Status, blueprintFile, osName string) bool {
	normalizedBlueprint := normalizeBlueprint(blueprintFile)
	inStatus := false
	for _, host := range status.KnownHosts {
		if host.Host == h.Rule.KnownHosts && normalizeBlueprint(host.Blueprint) == normalizedBlueprint && host.OS == osName {
			inStatus = true
			break
		}
	}
	if !inStatus {
		return false
	}

	// Verify the key is actually present in the file — status can be stale if the file was deleted.
	knownHostsPath, err := knownHostsFile(false)
	if err != nil {
		return false
	}
	data, err := os.ReadFile(knownHostsPath)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, h.Rule.KnownHosts) {
			return true
		}
	}
	return false
}
