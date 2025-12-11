package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	var addHostCmd string
	for _, keyType := range keyTypes {
		addHostCmd = fmt.Sprintf(`
content=$(ssh-keyscan -t %s %s 2>/dev/null)
if [ -n "$content" ]; then
  grep -q "$content" ~/.ssh/known_hosts || printf '%%s\n' "$content" >> ~/.ssh/known_hosts
  exit 0
fi
exit 1
`, keyType, h.Rule.KnownHosts)

		if _, err := executeCommandWithCache(addHostCmd); err == nil {
			// Successfully added the host
			return fmt.Sprintf("Added %s to known_hosts (key type: %s)", h.Rule.KnownHosts, keyType), nil
		}
	}

	// If we got here, none of the key types worked
	if len(keyTypes) == 1 {
		return "", fmt.Errorf("failed to add host to known_hosts with key type %s", keyTypes[0])
	}
	return "", fmt.Errorf("failed to add host to known_hosts - no suitable key types found (tried: %s)", strings.Join(keyTypes, ", "))
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

	if _, err := executeCommandWithCache(removeHostCmd); err != nil {
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
