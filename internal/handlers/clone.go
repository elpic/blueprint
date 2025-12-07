package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gitpkg "github.com/elpic/blueprint/internal/git"
	"github.com/elpic/blueprint/internal/parser"
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
