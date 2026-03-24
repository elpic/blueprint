package handlers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
)

func init() {
	RegisterAction(ActionDef{
		Name:   "sudoers",
		Prefix: "sudoers",
		NewHandler: func(rule parser.Rule, basePath string, passwordCache map[string]string) Handler {
			return NewSudoersHandler(rule, basePath)
		},
		RuleKey: func(rule parser.Rule) string {
			return rule.SudoersUser
		},
		Detect: func(rule parser.Rule) bool {
			return rule.SudoersUser != ""
		},
		Summary: func(rule parser.Rule) string {
			return rule.SudoersUser
		},
		ExcludeFromOrphanDetection: true,
	})
}

// SudoersHandler adds the current user (or a specified user) to sudoers
// with NOPASSWD: ALL, writing to /etc/sudoers.d/<username>.
type SudoersHandler struct {
	BaseHandler
}

// NewSudoersHandler creates a new sudoers handler
func NewSudoersHandler(rule parser.Rule, basePath string) *SudoersHandler {
	return &SudoersHandler{
		BaseHandler: BaseHandler{
			Rule:     rule,
			BasePath: basePath,
		},
	}
}

// resolveUser returns the user from the rule, falling back to the current $USER
func (h *SudoersHandler) resolveUser() (string, error) {
	if h.Rule.SudoersUser != "" {
		return h.Rule.SudoersUser, nil
	}
	user := os.Getenv("USER")
	if user == "" {
		// Fallback: ask the OS
		out, err := exec.Command("id", "-un").Output()
		if err != nil {
			return "", fmt.Errorf("could not determine current user: %w", err)
		}
		user = strings.TrimSpace(string(out))
	}
	if user == "" {
		return "", fmt.Errorf("could not determine current user")
	}
	return user, nil
}

// sudoersFilePath returns the drop-in file path for the given user
func sudoersFilePath(user string) string {
	return filepath.Join("/etc/sudoers.d", user)
}

// sudoersEntry returns the sudoers line for the given user
func sudoersEntry(user string) string {
	return fmt.Sprintf("%s ALL=(ALL) NOPASSWD: ALL\n", user)
}

// NeedsSudo returns true — writing to /etc/sudoers.d always requires sudo
func (h *SudoersHandler) NeedsSudo() bool {
	return true
}

// sudoersFileReader reads the contents of a sudoers drop-in file.
// Uses sudo cat since /etc/sudoers.d/ files are root-owned (mode 0440).
// Overridable for testing.
var sudoersFileReader = func(path string) ([]byte, error) {
	cmd := exec.Command("sudo", "cat", path)
	cmd.Stdin = nil
	out, err := cmd.Output()
	return out, err
}

// sudoRun runs a command under sudo directly, bypassing executeCommandWithCache
// to avoid the engine's double-sudo logic. Overridable for testing.
var sudoRun = func(args ...string) (string, error) {
	fullArgs := append([]string{"sudo"}, args...)
	cmd := exec.Command(fullArgs[0], fullArgs[1:]...) // #nosec G204
	cmd.Stdin = nil
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// sudoersTempDir returns the directory used for sudoers temp files.
// Overridable for testing.
var sudoersTempDir = func() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	dir := filepath.Join(home, ".blueprint")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("could not create ~/.blueprint: %w", err)
	}
	return dir, nil
}

// Up writes the sudoers drop-in file for the resolved user
func (h *SudoersHandler) Up() (string, error) {
	user, err := h.resolveUser()
	if err != nil {
		return "", err
	}

	filePath := sudoersFilePath(user)
	entry := sudoersEntry(user)

	// Skip if the correct entry is already present
	if existing, err := sudoersFileReader(filePath); err == nil &&
		strings.TrimSpace(string(existing)) == strings.TrimSpace(entry) {
		return fmt.Sprintf("%s already in sudoers", user), nil
	}

	// Write to a temp file in ~/.blueprint/ (mode 0700) to avoid TOCTOU races
	// in world-writable /tmp.
	tmpDir, err := sudoersTempDir()
	if err != nil {
		return "", err
	}
	tmpFile, err := os.CreateTemp(tmpDir, "blueprint-sudoers-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmpFile.WriteString(entry); err != nil {
		_ = tmpFile.Close()
		return "", fmt.Errorf("failed to write sudoers entry: %w", err)
	}
	_ = tmpFile.Close()

	// Validate with visudo -c -f (requires root).
	if out, err := sudoRun("visudo", "-c", "-f", tmpPath); err != nil {
		return "", fmt.Errorf("sudoers validation failed: %s", out)
	}

	// Copy validated file into /etc/sudoers.d/ and set correct permissions.
	if out, err := sudoRun("cp", tmpPath, filePath); err != nil {
		return "", fmt.Errorf("failed to install sudoers file at %s: %s", filePath, out)
	}
	if out, err := sudoRun("chmod", "0440", filePath); err != nil {
		return "", fmt.Errorf("failed to set permissions on %s: %s", filePath, out)
	}

	return fmt.Sprintf("Added %s to sudoers (NOPASSWD: ALL)", user), nil
}

// Down removes the sudoers drop-in file for the resolved user
func (h *SudoersHandler) Down() (string, error) {
	user, err := h.resolveUser()
	if err != nil {
		return "", err
	}

	filePath := sudoersFilePath(user)
	if out, err := sudoRun("rm", "-f", filePath); err != nil {
		return "", fmt.Errorf("failed to remove sudoers file %s: %s", filePath, out)
	}

	return fmt.Sprintf("Removed %s from sudoers", user), nil
}

// GetCommand returns the command that will be executed
func (h *SudoersHandler) GetCommand() string {
	if h.Rule.Action == "uninstall" {
		user := h.Rule.SudoersUser
		if user == "" {
			user = "$USER"
		}
		return fmt.Sprintf("sudo rm -f /etc/sudoers.d/%s", user)
	}
	user := h.Rule.SudoersUser
	if user == "" {
		user = "$USER"
	}
	return fmt.Sprintf("sudo install -m 0440 <entry> /etc/sudoers.d/%s", user)
}

// UpdateStatus updates the status after adding or removing a sudoers entry
func (h *SudoersHandler) UpdateStatus(status *Status, records []ExecutionRecord, blueprint string, osName string) error {
	blueprint = normalizeBlueprint(blueprint)

	if h.Rule.Action == "sudoers" {
		// Use the same command string as GetCommand() for record matching
		_, commandExecuted := commandSuccessfullyExecuted(h.GetCommand(), records)
		if !commandExecuted {
			return nil
		}

		// Resolve the user the same way Up() does
		resolvedUser, err := h.resolveUser()
		if err != nil || resolvedUser == "" {
			return nil
		}

		// Skip duplicates
		for _, s := range status.Sudoers {
			if s.User == resolvedUser && normalizeBlueprint(s.Blueprint) == blueprint && s.OS == osName {
				return nil
			}
		}

		status.Sudoers = append(status.Sudoers, SudoersStatus{
			User:      resolvedUser,
			AddedAt:   time.Now().Format(time.RFC3339),
			Blueprint: blueprint,
			OS:        osName,
		})
	} else if h.Rule.Action == "uninstall" && DetectRuleType(h.Rule) == "sudoers" {
		user := h.Rule.SudoersUser
		var newSudoers []SudoersStatus
		for _, s := range status.Sudoers {
			if s.User != user || normalizeBlueprint(s.Blueprint) != blueprint || s.OS != osName {
				newSudoers = append(newSudoers, s)
			}
		}
		status.Sudoers = newSudoers
	}

	return nil
}

// DisplayInfo displays handler-specific information
func (h *SudoersHandler) DisplayInfo() {
	formatFunc := ui.FormatInfo
	if h.Rule.Action == "uninstall" {
		formatFunc = ui.FormatDim
	}
	user := h.Rule.SudoersUser
	if user == "" {
		user = "$USER"
	}
	fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("User: %s → /etc/sudoers.d/%s (NOPASSWD: ALL)", user, user)))
}

// DisplayStatusFromStatus displays sudoers handler status
func (h *SudoersHandler) DisplayStatusFromStatus(status *Status) {
	if status == nil || len(status.Sudoers) == 0 {
		return
	}

	fmt.Printf("\n%s\n", ui.FormatHighlight("Sudoers:"))
	for _, s := range status.Sudoers {
		t, err := time.Parse(time.RFC3339, s.AddedAt)
		var timeStr string
		if err == nil {
			timeStr = t.Format("2006-01-02 15:04:05")
		} else {
			timeStr = s.AddedAt
		}
		fmt.Printf("  %s %s (%s) [%s, %s]\n",
			ui.FormatSuccess("●"),
			ui.FormatInfo(fmt.Sprintf("/etc/sudoers.d/%s", s.User)),
			ui.FormatDim(timeStr),
			ui.FormatDim(s.OS),
			ui.FormatDim(abbreviateBlueprintPath(s.Blueprint)),
		)
	}
}

// GetDependencyKey returns the unique key for this rule in dependency resolution
func (h *SudoersHandler) GetDependencyKey() string {
	fallback := "sudoers"
	if h.Rule.Action == "uninstall" && DetectRuleType(h.Rule) == "sudoers" {
		fallback = "uninstall-sudoers"
	}
	return getDependencyKey(h.Rule, fallback)
}

// GetDisplayDetails returns details to display during execution
func (h *SudoersHandler) GetDisplayDetails(isUninstall bool) string {
	user := h.Rule.SudoersUser
	if user == "" {
		user = "$USER"
	}
	return fmt.Sprintf("/etc/sudoers.d/%s", user)
}

// GetState returns handler-specific state as key-value pairs
func (h *SudoersHandler) GetState(isUninstall bool) map[string]string {
	user := h.Rule.SudoersUser
	if user == "" {
		user = "$USER"
	}
	return map[string]string{
		"summary": fmt.Sprintf("/etc/sudoers.d/%s", user),
		"user":    user,
	}
}

// FindUninstallRules compares sudoers status against current rules and returns uninstall rules
func (h *SudoersHandler) FindUninstallRules(status *Status, currentRules []parser.Rule, blueprintFile, osName string) []parser.Rule {
	normalizedBlueprint := normalizeBlueprint(blueprintFile)

	// Build set of users covered by current sudoers rules
	currentUsers := make(map[string]bool)
	for _, rule := range currentRules {
		if rule.Action == "sudoers" {
			user := rule.SudoersUser
			if user == "" {
				user = os.Getenv("USER")
			}
			if user != "" {
				currentUsers[user] = true
			}
		}
	}

	var rules []parser.Rule
	for _, s := range status.Sudoers {
		if normalizeBlueprint(s.Blueprint) == normalizedBlueprint && s.OS == osName {
			if !currentUsers[s.User] {
				rules = append(rules, parser.Rule{
					Action:      "uninstall",
					SudoersUser: s.User,
					OSList:      []string{osName},
				})
			}
		}
	}

	return rules
}

// IsInstalled returns true if the sudoers user in this rule is already in status.
func (h *SudoersHandler) IsInstalled(status *Status, blueprintFile, osName string) bool {
	normalizedBlueprint := normalizeBlueprint(blueprintFile)
	user := h.Rule.SudoersUser
	if user == "" {
		user = os.Getenv("USER")
	}
	for _, s := range status.Sudoers {
		if s.User == user && normalizeBlueprint(s.Blueprint) == normalizedBlueprint && s.OS == osName {
			return true
		}
	}
	return false
}
