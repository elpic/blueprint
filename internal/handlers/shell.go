package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/elpic/blueprint/internal"
	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
)

func init() {
	RegisterAction(ActionDef{
		Name:   "shell",
		Prefix: "shell ",
		NewHandler: func(rule parser.Rule, basePath string, passwordCache map[string]string) Handler {
			return NewShellHandler(rule, basePath)
		},
		RuleKey: func(rule parser.Rule) string {
			return rule.ShellName
		},
		Detect: func(rule parser.Rule) bool {
			return rule.ShellName != ""
		},
		Summary: func(rule parser.Rule) string {
			return rule.ShellName
		},
		OrphanIndex: func(rule parser.Rule, index func(string)) {
			index(rule.ShellName)
		},
		ShellExport: func(rule parser.Rule, _, _ string) []string {
			shell := rule.ShellName
			if !strings.HasPrefix(shell, "/") {
				return []string{
					fmt.Sprintf(`SHELL_PATH="$(command -v %s)"`, shellQ(shell)),
					`chsh -s "$SHELL_PATH"`,
				}
			}
			return []string{"chsh -s " + shellQ(shell)}
		},
	})
}

// ShellHandler handles setting the default login shell
type ShellHandler struct {
	BaseHandler
	previousShell string // Temporary storage for previous shell during execution
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

	// Store the current shell as the previous shell before making changes
	// This will be used for rollback in Down()
	h.previousShell = currentShell

	// Change shell using chsh (with path sanitization and timeout)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "chsh", "-s", filepath.Clean(shellPath))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to change shell: %w (output: %s)", err, string(output))
	}

	return fmt.Sprintf("Changed default shell to %s for user %s", shellPath, currentUser.Username), nil
}

// Down reverts the shell change using stored previous shell information
func (h *ShellHandler) Down() (string, error) {
	// Load current status to find the previous shell
	status := h.loadCurrentStatus()

	// Get current user
	currentUser, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}

	// Find the shell status entry for this user and blueprint
	var shellStatus *ShellStatus
	normalizedBlueprint := normalizeBlueprint(h.BasePath)
	for i, shell := range status.Shells {
		if shell.User == currentUser.Username &&
			normalizeBlueprint(shell.Blueprint) == normalizedBlueprint &&
			shell.OS == runtime.GOOS {
			shellStatus = &status.Shells[i]
			break
		}
	}

	if shellStatus == nil {
		return "", fmt.Errorf("no shell status found for user %s and blueprint %s", currentUser.Username, h.BasePath)
	}

	if shellStatus.PreviousShell == "" {
		return "", fmt.Errorf("no previous shell recorded - cannot revert (this may be an older status entry without rollback support)")
	}

	// Validate that the previous shell still exists and is valid
	if err := h.validateShell(shellStatus.PreviousShell); err != nil {
		return "", fmt.Errorf("previous shell %s is no longer valid: %w", shellStatus.PreviousShell, err)
	}

	// Check if previous shell is still in /etc/shells
	if err := h.validateShellInEtcShells(shellStatus.PreviousShell); err != nil {
		return "", fmt.Errorf("previous shell %s is not in /etc/shells: %w", shellStatus.PreviousShell, err)
	}

	// Check if we're already using the previous shell (idempotency)
	currentShell, err := h.getCurrentShell(currentUser.Username)
	if err != nil {
		return "", fmt.Errorf("failed to get current shell: %w", err)
	}

	if currentShell == shellStatus.PreviousShell {
		return fmt.Sprintf("Shell already reverted to %s for user %s", shellStatus.PreviousShell, currentUser.Username), nil
	}

	// Revert shell using chsh (with path sanitization and timeout)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "chsh", "-s", filepath.Clean(shellStatus.PreviousShell))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to revert shell: %w (output: %s)", err, string(output))
	}

	return fmt.Sprintf("Reverted shell to %s for user %s", shellStatus.PreviousShell, currentUser.Username), nil
}

// GetCommand returns the actual command that will be executed
func (h *ShellHandler) GetCommand() string {
	shellName := h.Rule.ShellName

	if h.Rule.Action == "uninstall" {
		// For uninstall, we show a generic message since the actual previous shell
		// is determined at execution time from status
		return "chsh -s <previous_shell>"
	}

	// For display purposes, show the basic chsh command
	shellPath, _ := h.resolveShellPath(shellName)
	return fmt.Sprintf("chsh -s %s", shellPath)
}

// UpdateStatus updates the blueprint status after executing shell change
func (h *ShellHandler) UpdateStatus(status *Status, records []ExecutionRecord, blueprint string, osName string) error {
	// Normalize blueprint path for comparison
	blueprint = normalizeBlueprint(blueprint)

	if h.Rule.Action == "shell" {
		// Check if shell change was executed successfully and extract the shell path
		shellExecuted := false
		var executedShellPath string
		for _, record := range records {
			if record.Status == "success" {
				// Check if this is a chsh command that matches our shell
				// This is more flexible than exact command matching
				if strings.HasPrefix(record.Command, "chsh -s ") {
					cmdShell := strings.TrimPrefix(record.Command, "chsh -s ")
					// Check if the shell in the command matches what we expect
					expectedShell, err := h.resolveShellPath(h.Rule.ShellName)
					if err == nil && cmdShell == expectedShell {
						shellExecuted = true
						executedShellPath = cmdShell
						break
					}
					// Also check if the command shell matches our rule shell name directly
					if cmdShell == h.Rule.ShellName {
						shellExecuted = true
						executedShellPath = cmdShell
						break
					}
					// Check basename match (e.g., "/bin/zsh" matches "zsh")
					if filepath.Base(cmdShell) == h.Rule.ShellName {
						shellExecuted = true
						executedShellPath = cmdShell
						break
					}
				}
			}
		}

		if shellExecuted {
			// Get current user
			currentUser, err := user.Current()
			if err != nil {
				return fmt.Errorf("failed to get current user: %w", err)
			}

			// Use the shell path from the executed command rather than trying to resolve it again
			shellPath := executedShellPath

			// Get previous shell from existing status (if any) to preserve rollback info
			var previousShell string
			existingEntry := findShellStatus(status.Shells, currentUser.Username, blueprint, osName)
			if existingEntry != nil {
				// Preserve existing PreviousShell if we're updating an entry
				previousShell = existingEntry.PreviousShell
			} else {
				// New entry - capture the previous shell from temporary storage
				// The Up() method stored this in h.previousShell
				previousShell = h.previousShell
			}

			// Remove existing entry if present
			status.Shells = removeShellStatus(status.Shells, currentUser.Username, blueprint, osName)

			// Add new entry with previous shell information
			status.Shells = append(status.Shells, ShellStatus{
				Shell:         shellPath,
				PreviousShell: previousShell,
				User:          currentUser.Username,
				ChangedAt:     time.Now().Format(time.RFC3339),
				Blueprint:     blueprint,
				OS:            osName,
			})
		}
	} else if h.Rule.Action == "uninstall" {
		// Handle shell uninstall/rollback
		shellRollbackExecuted := false
		for _, record := range records {
			if record.Status == "success" {
				// For uninstall, check if any chsh command was executed successfully
				// This handles the Down() method execution
				if strings.Contains(record.Command, "chsh -s") {
					shellRollbackExecuted = true
					break
				}
			}
		}

		if shellRollbackExecuted {
			// Get current user
			currentUser, err := user.Current()
			if err != nil {
				return fmt.Errorf("failed to get current user: %w", err)
			}

			// Remove the shell status entry since we've successfully rolled back
			status.Shells = removeShellStatus(status.Shells, currentUser.Username, blueprint, osName)
		}
	}

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
	normalizedBlueprint := normalizeBlueprint(blueprintFile)

	// Get current user
	currentUser, err := user.Current()
	if err != nil {
		return false
	}

	// Check if we have a shell status record
	for _, shell := range status.Shells {
		if shell.User == currentUser.Username &&
			normalizeBlueprint(shell.Blueprint) == normalizedBlueprint &&
			shell.OS == osName {

			// Cross-validate: verify the shell recorded in status matches the actual current shell
			currentShell, err := h.getCurrentShell(currentUser.Username)
			if err != nil {
				return false
			}

			// Check if current shell matches our expected shell
			expectedShell, err := h.resolveShellPath(h.Rule.ShellName)
			if err != nil {
				return false
			}

			// For maximum accuracy, check both status record and actual shell
			statusMatches := currentShell == shell.Shell
			expectedMatches := currentShell == expectedShell

			return statusMatches && expectedMatches
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

	// Try using 'which' command as fallback (with name validation and timeout)
	if err := validateShellName(shellName); err != nil {
		return "", fmt.Errorf("invalid shell name for 'which' command: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "which", shellName)
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

	// Create a timeout context for external commands
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try using dscl on macOS first
	if h.isMacOS() {
		cmd := exec.CommandContext(ctx, "dscl", ".", "read", "/Users/"+username, "UserShell")
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
	cmd := exec.CommandContext(ctx, "getent", "passwd", username)
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
	var uninstallRules []parser.Rule
	normalizedBlueprint := normalizeBlueprint(blueprintFile)

	// Get current user
	currentUser, err := user.Current()
	if err != nil {
		return uninstallRules
	}

	// Check each shell status entry
	for _, shell := range status.Shells {
		if shell.User == currentUser.Username &&
			normalizeBlueprint(shell.Blueprint) == normalizedBlueprint &&
			shell.OS == osName {

			// Check if this shell is still in the current rules
			stillInRules := false
			for _, rule := range currentRules {
				if rule.Action == "shell" {
					// Try to resolve the shell path for comparison
					expectedShell, err := h.resolveShellPath(rule.ShellName)
					if err == nil && expectedShell == shell.Shell {
						stillInRules = true
						break
					}
					// Also check direct shell name match (for cases where resolution fails)
					if rule.ShellName == shell.Shell {
						stillInRules = true
						break
					}
					// Also check if shell name is a basename of the stored shell path (e.g., "zsh" matches "/bin/zsh")
					if filepath.Base(shell.Shell) == rule.ShellName {
						stillInRules = true
						break
					}
				}
			}

			// If this shell is no longer in the rules and has rollback info, create uninstall rule
			if !stillInRules && shell.PreviousShell != "" {
				uninstallRules = append(uninstallRules, parser.Rule{
					Action:    "uninstall",
					ShellName: shell.Shell, // The shell we want to uninstall/revert
				})
			}
		}
	}

	return uninstallRules
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

// loadCurrentStatus loads the current status from the status file
func (h *ShellHandler) loadCurrentStatus() Status {
	var status Status
	statusPath, err := h.getStatusPath()
	if err != nil {
		return status
	}

	data, err := os.ReadFile(statusPath)
	if err != nil {
		return status
	}

	_ = json.Unmarshal(data, &status)
	return status
}

// getStatusPath returns the path to the status file in ~/.blueprint/
func (h *ShellHandler) getStatusPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	blueprintDir := filepath.Join(homeDir, ".blueprint")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(blueprintDir, internal.DirectoryPermission); err != nil {
		return "", fmt.Errorf("failed to create .blueprint directory: %w", err)
	}

	return filepath.Join(blueprintDir, "status.json"), nil
}

// findShellStatus finds a shell entry in the status shells list and returns it
func findShellStatus(shells []ShellStatus, user string, blueprint string, osName string) *ShellStatus {
	normalizedBlueprint := normalizeBlueprint(blueprint)
	for i, shell := range shells {
		normalizedStoredBlueprint := normalizeBlueprint(shell.Blueprint)
		if shell.User == user && normalizedStoredBlueprint == normalizedBlueprint && shell.OS == osName {
			return &shells[i]
		}
	}
	return nil
}
