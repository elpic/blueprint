package handlers

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
)

// ShellStatus tracks a shell change
type ShellStatus struct {
	Shell     string `json:"shell"`
	User      string `json:"user"`
	ChangedAt string `json:"changed_at"`
	Blueprint string `json:"blueprint"`
	OS        string `json:"os"`
}

// ShellHandler handles setting the default login shell
type ShellHandler struct {
	BaseHandler
}

// NewShellHandler creates a new shell handler
func NewShellHandler(rule parser.Rule, basePath string) *ShellHandler {
	return &ShellHandler{
		BaseHandler: BaseHandler{
			Rule:     rule,
			BasePath: basePath,
		},
	}
}

// validateShellName validates shell names to prevent command injection
func validateShellName(name string) error {
	// Only allow safe characters in shell names
	validChars := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !validChars.MatchString(name) {
		return fmt.Errorf("invalid shell name: contains unsafe characters")
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("invalid shell name: path traversal not allowed")
	}
	return nil
}

// Up sets the default login shell using chsh
func (h *ShellHandler) Up() (string, error) {
	shellName := h.Rule.ShellName

	// Always validate shell name for path traversal attacks, even if absolute
	if strings.Contains(shellName, "..") {
		return "", fmt.Errorf("shell name validation failed: path traversal not allowed")
	}

	// Validate shell name to prevent command injection
	if !filepath.IsAbs(shellName) {
		if err := validateShellName(shellName); err != nil {
			return "", fmt.Errorf("shell name validation failed: %w", err)
		}
	}

	// Resolve shell path
	shellPath, err := h.resolveShellPath(shellName)
	if err != nil {
		return "", fmt.Errorf("failed to resolve shell path: %w", err)
	}

	// Validate shell exists and is executable
	if err := h.validateShell(shellPath); err != nil {
		return "", err
	}

	// Check if shell is in /etc/shells
	if err := h.validateShellInEtcShells(shellPath); err != nil {
		return "", err
	}

	// Get current user
	currentUser, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}

	// Check if shell is already set (idempotency)
	currentShell, err := h.getCurrentShell(currentUser.Username)
	if err != nil {
		return "", fmt.Errorf("failed to get current shell: %w", err)
	}

	if currentShell == shellPath {
		return fmt.Sprintf("Shell already set to %s for user %s", shellPath, currentUser.Username), nil
	}

	// Change shell using chsh (with path sanitization)
	cmd := exec.Command("chsh", "-s", filepath.Clean(shellPath))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to change shell: %w (output: %s)", err, string(output))
	}

	return fmt.Sprintf("Changed default shell to %s for user %s", shellPath, currentUser.Username), nil
}

// Down reverts the shell change (not implemented as it's not safe to assume the previous shell)
func (h *ShellHandler) Down() (string, error) {
	return "Shell changes cannot be automatically reverted", fmt.Errorf("shell uninstall not supported - shell changes cannot be safely reverted")
}

// GetCommand returns the actual command that will be executed
func (h *ShellHandler) GetCommand() string {
	shellName := h.Rule.ShellName

	// For display purposes, show the basic chsh command
	shellPath, _ := h.resolveShellPath(shellName)

	if h.Rule.Action == "uninstall" {
		return "# Shell changes cannot be automatically reverted"
	}

	return fmt.Sprintf("chsh -s %s", shellPath)
}

// UpdateStatus updates the blueprint status after executing shell change
func (h *ShellHandler) UpdateStatus(status *Status, records []ExecutionRecord, blueprint string, osName string) error {
	// Normalize blueprint path for comparison
	blueprint = normalizePath(blueprint)

	if h.Rule.Action == "shell" {
		// Check if shell change was executed successfully
		expectedCmd := h.GetCommand()
		shellExecuted := false
		for _, record := range records {
			if record.Status == "success" && record.Command == expectedCmd {
				shellExecuted = true
				break
			}
		}

		if shellExecuted {
			// Get current user
			currentUser, err := user.Current()
			if err != nil {
				return fmt.Errorf("failed to get current user: %w", err)
			}

			// Resolve shell path for status
			shellPath, err := h.resolveShellPath(h.Rule.ShellName)
			if err != nil {
				return fmt.Errorf("failed to resolve shell path: %w", err)
			}

			// Remove existing entry if present
			status.Shells = removeShellStatus(status.Shells, currentUser.Username, blueprint, osName)

			// Add new entry
			status.Shells = append(status.Shells, ShellStatus{
				Shell:     shellPath,
				User:      currentUser.Username,
				ChangedAt: time.Now().Format(time.RFC3339),
				Blueprint: blueprint,
				OS:        osName,
			})
		}
	}
	// Note: We don't handle uninstall because shell changes cannot be safely reverted

	return nil
}

// DisplayInfo displays handler-specific information
func (h *ShellHandler) DisplayInfo() {
	formatFunc := ui.FormatInfo
	if h.Rule.Action == "uninstall" {
		formatFunc = ui.FormatDim
	}

	fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Shell: %s", h.Rule.ShellName)))

	// Try to resolve and show the full path
	if shellPath, err := h.resolveShellPath(h.Rule.ShellName); err == nil {
		fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Path: %s", shellPath)))
	}
}

// GetDependencyKey returns the unique key for this rule in dependency resolution
func (h *ShellHandler) GetDependencyKey() string {
	return getDependencyKey(h.Rule, h.Rule.ShellName)
}

// GetDisplayDetails returns the shell name to display during execution
func (h *ShellHandler) GetDisplayDetails(isUninstall bool) string {
	return h.Rule.ShellName
}

// GetState returns handler-specific state as key-value pairs
func (h *ShellHandler) GetState(isUninstall bool) map[string]string {
	return map[string]string{
		"summary": h.GetDisplayDetails(isUninstall),
		"shell":   h.Rule.ShellName,
	}
}

// IsInstalled returns true if the shell is already set for the current user
func (h *ShellHandler) IsInstalled(status *Status, blueprintFile, osName string) bool {
	normalizedBlueprint := normalizePath(blueprintFile)

	// Get current user
	currentUser, err := user.Current()
	if err != nil {
		return false
	}

	// Check if we have a shell status record
	for _, shell := range status.Shells {
		if shell.User == currentUser.Username &&
			normalizePath(shell.Blueprint) == normalizedBlueprint &&
			shell.OS == osName {

			// Also verify the shell is actually set
			currentShell, err := h.getCurrentShell(currentUser.Username)
			if err != nil {
				return false
			}

			// Check if current shell matches our expected shell
			expectedShell, err := h.resolveShellPath(h.Rule.ShellName)
			if err != nil {
				return false
			}

			return currentShell == expectedShell
		}
	}

	return false
}

// resolveShellPath resolves a shell name to its full path
func (h *ShellHandler) resolveShellPath(shellName string) (string, error) {
	// Always check for path traversal attempts
	if strings.Contains(shellName, "..") {
		return "", fmt.Errorf("invalid shell name: path traversal not allowed")
	}

	// If it's already an absolute path, clean and return it
	if filepath.IsAbs(shellName) {
		return filepath.Clean(shellName), nil
	}

	// Common shell paths to check
	commonPaths := []string{
		"/bin/" + shellName,
		"/usr/bin/" + shellName,
		"/usr/local/bin/" + shellName,
		"/opt/homebrew/bin/" + shellName, // Homebrew on Apple Silicon
		"/opt/local/bin/" + shellName,    // MacPorts
	}

	// Check each common path
	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// Try using 'which' command as fallback (with name validation)
	if err := validateShellName(shellName); err != nil {
		return "", fmt.Errorf("invalid shell name for 'which' command: %w", err)
	}
	cmd := exec.Command("which", shellName)
	output, err := cmd.Output()
	if err == nil {
		path := strings.TrimSpace(string(output))
		if path != "" {
			return filepath.Clean(path), nil
		}
	}

	return "", fmt.Errorf("shell '%s' not found in common locations", shellName)
}

// validateShell checks if the shell exists and is executable
func (h *ShellHandler) validateShell(shellPath string) error {
	info, err := os.Stat(shellPath)
	if err != nil {
		return fmt.Errorf("shell not found: %s", shellPath)
	}

	if info.IsDir() {
		return fmt.Errorf("shell path is a directory: %s", shellPath)
	}

	// Check if the file is executable
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("shell is not executable: %s", shellPath)
	}

	return nil
}

// validateShellInEtcShells checks if the shell is listed in /etc/shells
func (h *ShellHandler) validateShellInEtcShells(shellPath string) error {
	content, err := os.ReadFile("/etc/shells")
	if err != nil {
		// If /etc/shells doesn't exist, allow any valid shell
		return nil
	}

	shells := strings.Split(string(content), "\n")
	for _, line := range shells {
		line = strings.TrimSpace(line)
		if line == shellPath {
			return nil
		}
	}

	return fmt.Errorf("shell '%s' is not listed in /etc/shells - it may not be allowed as a login shell", shellPath)
}

// validateUsername validates usernames to prevent command injection
func validateUsername(username string) error {
	// Only allow safe characters in usernames (alphanumeric, dash, underscore, dot)
	validChars := regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)
	if !validChars.MatchString(username) {
		return fmt.Errorf("invalid username: contains unsafe characters")
	}
	if strings.Contains(username, "..") {
		return fmt.Errorf("invalid username: path traversal not allowed")
	}
	return nil
}

// getCurrentShell gets the current shell for the specified user
func (h *ShellHandler) getCurrentShell(username string) (string, error) {
	// Validate username to prevent command injection
	if err := validateUsername(username); err != nil {
		return "", fmt.Errorf("username validation failed: %w", err)
	}

	// Try using dscl on macOS first
	if h.isMacOS() {
		cmd := exec.Command("dscl", ".", "read", "/Users/"+username, "UserShell")
		output, err := cmd.Output()
		if err == nil {
			// Parse output like "UserShell: /bin/zsh"
			lines := strings.Split(strings.TrimSpace(string(output)), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "UserShell:") {
					parts := strings.Fields(line)
					if len(parts) >= 2 {
						return parts[1], nil
					}
				}
			}
		}
	}

	// Try getent (works on Linux and some Unix systems)
	cmd := exec.Command("getent", "passwd", username)
	output, err := cmd.Output()
	if err == nil {
		// Parse passwd entry: username:password:uid:gid:gecos:home:shell
		fields := strings.Split(strings.TrimSpace(string(output)), ":")
		if len(fields) >= 7 {
			return fields[6], nil
		}
	}

	// Fallback to reading /etc/passwd directly
	return h.getShellFromPasswd(username)
}

// getShellFromPasswd reads /etc/passwd to get the user's shell
func (h *ShellHandler) getShellFromPasswd(username string) (string, error) {
	// Validate username to prevent injection attacks
	if err := validateUsername(username); err != nil {
		return "", fmt.Errorf("username validation failed: %w", err)
	}
	content, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return "", fmt.Errorf("failed to read /etc/passwd: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, username+":") {
			fields := strings.Split(line, ":")
			if len(fields) >= 7 {
				return fields[6], nil
			}
		}
	}

	return "", fmt.Errorf("user not found in /etc/passwd")
}

// FindUninstallRules compares shell status against current rules and returns uninstall rules
func (h *ShellHandler) FindUninstallRules(status *Status, currentRules []parser.Rule, blueprintFile, osName string) []parser.Rule {
	// Shell changes cannot be automatically uninstalled, so return no rules
	return []parser.Rule{}
}

// NeedsSudo returns false because chsh typically works without sudo for the current user
func (h *ShellHandler) NeedsSudo() bool {
	// chsh normally works without sudo when changing your own shell
	return false
}

// isMacOS returns true if running on macOS
func (h *ShellHandler) isMacOS() bool {
	return runtime.GOOS == "darwin"
}

// removeShellStatus removes a shell entry from the status shells list
func removeShellStatus(shells []ShellStatus, user string, blueprint string, osName string) []ShellStatus {
	var result []ShellStatus
	normalizedBlueprint := normalizePath(blueprint)
	for _, shell := range shells {
		normalizedStoredBlueprint := normalizePath(shell.Blueprint)
		if shell.User != user || normalizedStoredBlueprint != normalizedBlueprint || shell.OS != osName {
			result = append(result, shell)
		}
	}
	return result
}
