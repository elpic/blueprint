package handlers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/elpic/blueprint/internal/parser"
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
	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	sshDir := filepath.Join(homeDir, ".ssh")
	knownHostsPath := filepath.Join(sshDir, "known_hosts")

	// Create .ssh directory with proper permissions (700)
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	// Create known_hosts file if it doesn't exist with proper permissions (600)
	if _, err := os.Stat(knownHostsPath); os.IsNotExist(err) {
		if err := os.WriteFile(knownHostsPath, []byte{}, 0600); err != nil {
			return "", fmt.Errorf("failed to create known_hosts file: %w", err)
		}
	}

	// Determine which key types to try
	keyTypes := []string{}
	if h.Rule.KnownHostsKey != "" {
		// If a specific key type was specified, use only that
		keyTypes = []string{h.Rule.KnownHostsKey}
	} else {
		// Default to trying multiple key types in order of preference
		keyTypes = []string{"ed25519", "ecdsa", "rsa"}
	}

	// Try each key type until one succeeds
	var errors []string
	for _, keyType := range keyTypes {
		// Use shell to execute the command with pipes
		shellCmd := fmt.Sprintf(`
content=$(ssh-keyscan -t %s %s 2>&1)
exit_code=$?
if [ $exit_code -eq 0 ] && [ -n "$content" ]; then
  if ! grep -q "$content" ~/.ssh/known_hosts 2>/dev/null; then
    printf '%%s\n' "$content" >> ~/.ssh/known_hosts
  fi
  exit 0
fi
# Print the actual error from ssh-keyscan
echo "$content" >&2
exit 1
`, keyType, h.Rule.KnownHosts)

		cmd := exec.Command("sh", "-c", shellCmd)
		output, err := cmd.CombinedOutput()

		if err == nil {
			// Successfully added the host
			return fmt.Sprintf("Added %s to known_hosts (key type: %s)", h.Rule.KnownHosts, keyType), nil
		}

		// Collect error messages for each key type
		errMsg := strings.TrimSpace(string(output))
		if errMsg == "" {
			errMsg = "unknown error"
		}
		errors = append(errors, fmt.Sprintf("%s: %s", keyType, errMsg))
	}

	// If we got here, none of the key types worked
	return "", fmt.Errorf("failed to add host to known_hosts - tried: %s\nDetails:\n%s", strings.Join(keyTypes, ", "), strings.Join(errors, "\n"))
}

// Down removes the host from known_hosts file
func (h *KnownHostsHandler) Down() (string, error) {
	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	knownHostsPath := filepath.Join(homeDir, ".ssh", "known_hosts")

	// Check if known_hosts file exists
	if _, err := os.Stat(knownHostsPath); os.IsNotExist(err) {
		return "known_hosts file does not exist", nil
	}

	// Remove the host entry using sed
	// sed removes lines that start with the host (including variations of IP addresses)
	removeHostCmd := fmt.Sprintf(`
sed -i.bak '/^%s[, ]/d' ~/.ssh/known_hosts 2>/dev/null || true
rm -f ~/.ssh/known_hosts.bak 2>/dev/null || true
`, escapeForSed(h.Rule.KnownHosts))

	cmd := exec.Command("sh", "-c", removeHostCmd)
	if _, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not remove %s from known_hosts\n", h.Rule.KnownHosts)
	}

	return fmt.Sprintf("Removed %s from known_hosts", h.Rule.KnownHosts), nil
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

// UpdateStatus updates the status after adding or removing a known host
func (h *KnownHostsHandler) UpdateStatus(status *Status, records []ExecutionRecord, blueprint string, osName string) error {
	// Normalize blueprint path for consistent storage and comparison
	blueprint = normalizePath(blueprint)

	if h.Rule.Action == "known_hosts" {
		// Check if this rule's command was executed successfully
		// Look for a record indicating success
		commandExecuted := false
		var keyType string
		for _, record := range records {
			if record.Status == "success" && strings.Contains(record.Command, "known_hosts") && strings.Contains(record.Command, h.Rule.KnownHosts) {
				commandExecuted = true
				// Extract key type from the output
				if strings.Contains(record.Output, "ed25519") {
					keyType = "ed25519"
				} else if strings.Contains(record.Output, "ecdsa") {
					keyType = "ecdsa"
				} else if strings.Contains(record.Output, "rsa") {
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
	} else if h.Rule.Action == "uninstall" && h.Rule.Tool == "known_hosts" {
		// Remove known host from status if uninstall was successful
		status.KnownHosts = removeKnownHostsStatus(status.KnownHosts, h.Rule.KnownHosts, blueprint, osName)
	}

	return nil
}
