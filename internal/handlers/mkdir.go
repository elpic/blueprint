package handlers

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
)

// MkdirHandler handles directory creation with optional permissions
type MkdirHandler struct {
	BaseHandler
}

// NewMkdirHandler creates a new mkdir handler
func NewMkdirHandler(rule parser.Rule, basePath string) *MkdirHandler {
	return &MkdirHandler{
		BaseHandler: BaseHandler{
			Rule:     rule,
			BasePath: basePath,
		},
	}
}

// Up creates the directory with optional permissions
func (h *MkdirHandler) Up() (string, error) {
	// Expand path (handle ~)
	path := h.Rule.Mkdir
	path = mkdirExpandPath(path)

	// Validate permissions if specified
	if h.Rule.MkdirPerms != "" {
		if !mkdirIsValidOctalPermissions(h.Rule.MkdirPerms) {
			return "", fmt.Errorf("invalid permissions '%s': must be valid octal (0-777)", h.Rule.MkdirPerms)
		}
	}

	// Create directory with mkdir -p (use default 0750 if no perms specified)
	mode := os.FileMode(0750)
	if h.Rule.MkdirPerms != "" {
		// Parse the octal permission string
		var octal int
		_, _ = fmt.Sscanf(h.Rule.MkdirPerms, "%o", &octal)
		mode = os.FileMode(octal)
	}

	if err := os.MkdirAll(path, mode); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", path, err)
	}

	// Return success message
	msg := fmt.Sprintf("Created directory %s", path)
	if h.Rule.MkdirPerms != "" {
		msg += fmt.Sprintf(" with permissions %s", h.Rule.MkdirPerms)
	}
	return msg, nil
}

// Down removes the directory
func (h *MkdirHandler) Down() (string, error) {
	// Expand path (handle ~)
	path := h.Rule.Mkdir
	path = mkdirExpandPath(path)

	// Check if directory exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Sprintf("Directory %s does not exist", path), nil
	}

	// Remove directory recursively
	if err := os.RemoveAll(path); err != nil {
		return "", fmt.Errorf("failed to remove directory %s: %w", path, err)
	}

	return fmt.Sprintf("Removed directory %s", path), nil
}

// GetCommand returns the actual command(s) that will be executed
func (h *MkdirHandler) GetCommand() string {
	path := h.Rule.Mkdir

	// If this is an uninstall, show the directory removal
	if h.Rule.Action == "uninstall" {
		return fmt.Sprintf("rm -rf %s", path)
	}

	// For mkdir action, show the directory creation
	// Escape path if it contains spaces
	escapedPath := mkdirEscapePath(path)
	msg := fmt.Sprintf("mkdir -p %s", escapedPath)

	if h.Rule.MkdirPerms != "" {
		msg += fmt.Sprintf(" && chmod %s %s", h.Rule.MkdirPerms, escapedPath)
	}

	return msg
}

// UpdateStatus updates the blueprint status after executing mkdir or uninstall-mkdir
func (h *MkdirHandler) UpdateStatus(status *Status, records []ExecutionRecord, blueprint string, osName string) error {
	// Normalize blueprint path for comparison
	blueprint = normalizePath(blueprint)

	if h.Rule.Action == "mkdir" {
		// Check if mkdir was executed successfully by looking for the GetCommand output
		expectedCmd := h.GetCommand()
		mkdirExecuted := false
		for _, record := range records {
			if record.Status == "success" && record.Command == expectedCmd {
				mkdirExecuted = true
				break
			}
		}

		if mkdirExecuted {
			// Remove existing entry if present
			status.Mkdirs = removeMkdirStatus(status.Mkdirs, h.Rule.Mkdir, blueprint, osName)
			// Add new entry
			status.Mkdirs = append(status.Mkdirs, MkdirStatus{
				Path:      h.Rule.Mkdir,
				CreatedAt: time.Now().Format(time.RFC3339),
				Blueprint: blueprint,
				OS:        osName,
			})
		}
	} else if h.Rule.Action == "uninstall" && DetectRuleType(h.Rule) == "mkdir" {
		// Check if mkdir was uninstalled successfully by checking if the directory no longer exists
		expandedPath := h.Rule.Mkdir
		if strings.HasPrefix(expandedPath, "~") {
			homeDir, err := os.UserHomeDir()
			if err == nil {
				expandedPath = filepath.Join(homeDir, expandedPath[1:])
			}
		}

		// If directory doesn't exist, remove from status
		if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
			status.Mkdirs = removeMkdirStatus(status.Mkdirs, h.Rule.Mkdir, blueprint, osName)
		}
	}

	return nil
}

// DisplayInfo displays handler-specific information
func (h *MkdirHandler) DisplayInfo() {
	formatFunc := ui.FormatInfo
	if h.Rule.Action == "uninstall" {
		formatFunc = ui.FormatDim
	}

	fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Path: %s", h.Rule.Mkdir)))
	if h.Rule.MkdirPerms != "" {
		fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Permissions: %s", h.Rule.MkdirPerms)))
	}
}

// mkdirExpandPath expands ~ to home directory
func mkdirExpandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		usr, err := user.Current()
		if err != nil {
			return path
		}
		return filepath.Join(usr.HomeDir, path[1:])
	}
	return path
}

// mkdirEscapePath escapes special characters for shell
func mkdirEscapePath(path string) string {
	// If path contains spaces or special characters, quote it
	if strings.ContainsAny(path, " \t\"'$`\\") {
		// Use single quotes but escape any single quotes in the path
		path = strings.ReplaceAll(path, "'", "'\\''")
		return fmt.Sprintf("'%s'", path)
	}
	return path
}

// mkdirIsValidOctalPermissions validates that the string is valid octal (0-777)
func mkdirIsValidOctalPermissions(perms string) bool {
	// Check if it matches octal pattern (0-7 digits only)
	matched, err := regexp.MatchString(`^[0-7]{1,3}$`, perms)
	if err != nil || !matched {
		return false
	}

	// Parse as octal and check range
	var octal int
	_, err = fmt.Sscanf(perms, "%o", &octal)
	if err != nil {
		return false
	}

	// Valid range is 0 to 777 (octal)
	return octal >= 0 && octal <= 0777
}

// removeMkdirStatus removes a mkdir from the status mkdirs list
func removeMkdirStatus(mkdirs []MkdirStatus, path string, blueprint string, osName string) []MkdirStatus {
	var result []MkdirStatus
	// Normalize blueprint for comparison to handle path variations like /tmp vs /private/tmp
	normalizedBlueprint := normalizePath(blueprint)
	for _, mkdir := range mkdirs {
		// Also normalize the stored blueprint for comparison
		normalizedStoredBlueprint := normalizePath(mkdir.Blueprint)
		if mkdir.Path != path || normalizedStoredBlueprint != normalizedBlueprint || mkdir.OS != osName {
			result = append(result, mkdir)
		}
	}
	return result
}

// DisplayStatus displays created directory status information
func (h *MkdirHandler) DisplayStatus(mkdirs []MkdirStatus) {
	if len(mkdirs) == 0 {
		return
	}

	fmt.Printf("\n%s\n", ui.FormatHighlight("Created Directories:"))
	for _, mkdir := range mkdirs {
		// Parse timestamp for display
		t, err := time.Parse(time.RFC3339, mkdir.CreatedAt)
		var timeStr string
		if err == nil {
			timeStr = t.Format("2006-01-02 15:04:05")
		} else {
			timeStr = mkdir.CreatedAt
		}

		fmt.Printf("  %s %s (%s) [%s, %s]\n",
			ui.FormatSuccess("●"),
			ui.FormatInfo(mkdir.Path),
			ui.FormatDim(timeStr),
			ui.FormatDim(mkdir.OS),
			ui.FormatDim(mkdir.Blueprint),
		)
	}
}
