package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	gitpkg "github.com/elpic/blueprint/internal/git"
	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
)

// CloneHandler handles git repository cloning and cleanup
type CloneHandler struct {
	BaseHandler
}

// NewCloneHandler creates a new clone handler
func NewCloneHandler(rule parser.Rule, basePath string) *CloneHandler {
	return &CloneHandler{
		BaseHandler: BaseHandler{
			Rule:     rule,
			BasePath: basePath,
		},
	}
}

// Up clones or updates the repository using go-git library
func (h *CloneHandler) Up() (string, error) {
	// Expand home directory if needed
	clonePath := expandPath(h.Rule.ClonePath)

	// Use go-git library to clone or update repository
	oldSHA, newSHA, status, err := gitpkg.CloneOrUpdateRepository(
		h.Rule.CloneURL,
		clonePath,
		h.Rule.Branch,
	)
	if err != nil {
		return "", fmt.Errorf("failed to clone/update repository: %w", err)
	}

	// Format output message with SHA tracking
	switch status {
	case "Cloned":
		if newSHA != "" {
			return fmt.Sprintf("Cloned (SHA: %s)", newSHA), nil
		}
		return "Cloned", nil

	case "Updated":
		if oldSHA != "" && newSHA != "" {
			return fmt.Sprintf("Updated (SHA changed: %s → %s) (SHA: %s)",
				oldSHA[:8], newSHA[:8], newSHA), nil
		}
		if newSHA != "" {
			return fmt.Sprintf("Updated (SHA: %s)", newSHA), nil
		}
		return "Updated", nil

	case "Already up to date":
		if newSHA != "" {
			return fmt.Sprintf("Already up to date (SHA: %s)", newSHA), nil
		}
		return "Already up to date", nil

	default:
		return status, nil
	}
}

// Down removes the cloned repository
func (h *CloneHandler) Down() (string, error) {
	clonePath := expandPath(h.Rule.ClonePath)

	// Remove directory if it exists
	if _, err := os.Stat(clonePath); err == nil {
		err := os.RemoveAll(clonePath)
		if err != nil {
			return "", fmt.Errorf("failed to remove directory %s: %w", clonePath, err)
		}
		return fmt.Sprintf("Removed cloned repository at %s", clonePath), nil
	}

	return "Repository not found", nil
}

// GetCommand returns the actual command(s) that will be executed
func (h *CloneHandler) GetCommand() string {
	if h.Rule.Action == "uninstall" {
		clonePath := h.Rule.ClonePath
		return fmt.Sprintf("rm -rf %s", clonePath)
	}

	// Clone action - use go-git, so return descriptive command
	if h.Rule.Branch != "" {
		return fmt.Sprintf("git clone -b %s %s %s", h.Rule.Branch, h.Rule.CloneURL, h.Rule.ClonePath)
	}

	return fmt.Sprintf("git clone %s %s", h.Rule.CloneURL, h.Rule.ClonePath)
}

// UpdateStatus updates the status after cloning or removing a repository
func (h *CloneHandler) UpdateStatus(status *Status, records []ExecutionRecord, blueprint string, osName string) error {
	// Normalize blueprint path for consistent storage and comparison
	blueprint = normalizePath(blueprint)

	if h.Rule.Action == "clone" {
		cloneCmd := h.GetCommand()

		record, commandExecuted := commandSuccessfullyExecuted(cloneCmd, records)

		if commandExecuted {
			cloneSHA := extractSHAFromOutput(record.Output)
			// Remove existing entry if present
			status.Clones = removeCloneStatus(status.Clones, h.Rule.ClonePath, blueprint, osName)
			// Add new entry
			status.Clones = append(status.Clones, CloneStatus{
				URL:       h.Rule.CloneURL,
				Path:      h.Rule.ClonePath,
				SHA:       cloneSHA,
				ClonedAt:  time.Now().Format(time.RFC3339),
				Blueprint: blueprint,
				OS:        osName,
			})
		}
	} else if h.Rule.Action == "uninstall" && DetectRuleType(h.Rule) == "clone" {
		// Check if clone was removed by checking if directory doesn't exist
		expandedPath := expandPath(h.Rule.ClonePath)
		if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
			// Directory has been removed, update status
			status.Clones = removeCloneStatus(status.Clones, h.Rule.ClonePath, blueprint, osName)
		}
	}

	return nil
}

// DisplayInfo displays handler-specific information
func (h *CloneHandler) DisplayInfo() {
	formatFunc := ui.FormatInfo
	if h.Rule.Action == "uninstall" {
		formatFunc = ui.FormatDim
	}

	fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("URL: %s", h.Rule.CloneURL)))
	fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Path: %s", h.Rule.ClonePath)))
	if h.Rule.Branch != "" {
		fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Branch: %s", h.Rule.Branch)))
	}
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(homeDir, path[1:])
	}
	return path
}

// DisplayStatus displays cloned repository status information
func (h *CloneHandler) DisplayStatus(clones []CloneStatus) {
	if len(clones) == 0 {
		return
	}

	fmt.Printf("\n%s\n", ui.FormatHighlight("Cloned Repositories:"))
	for _, clone := range clones {
		// Parse timestamp for display
		t, err := time.Parse(time.RFC3339, clone.ClonedAt)
		var timeStr string
		if err == nil {
			timeStr = t.Format("2006-01-02 15:04:05")
		} else {
			timeStr = clone.ClonedAt
		}

		fmt.Printf("  %s %s (%s) [%s, %s]\n",
			ui.FormatSuccess("●"),
			ui.FormatInfo(clone.Path),
			ui.FormatDim(timeStr),
			ui.FormatDim(clone.OS),
			ui.FormatDim(abbreviateBlueprintPath(clone.Blueprint)),
		)
		fmt.Printf("     %s %s\n",
			ui.FormatDim("URL:"),
			ui.FormatInfo(clone.URL),
		)
	}
}

// GetDependencyKey returns the unique key for this rule in dependency resolution
func (h *CloneHandler) GetDependencyKey() string {
	return getDependencyKey(h.Rule, h.Rule.ClonePath)
}

// GetDisplayDetails returns the clone path to display during execution
func (h *CloneHandler) GetDisplayDetails(isUninstall bool) string {
	return h.Rule.ClonePath
}
