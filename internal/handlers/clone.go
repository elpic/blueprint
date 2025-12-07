package handlers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

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

// Up clones the repository
func (h *CloneHandler) Up() (string, error) {
	// Expand home directory if needed
	clonePath := expandPath(h.Rule.ClonePath)

	// Check if already cloned
	if _, err := os.Stat(clonePath); err == nil {
		// Already exists, try to update
		return h.updateRepository(clonePath)
	}

	// Clone the repository
	cmd := h.buildCloneCommand(clonePath)
	cloneOutput, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("clone failed: %w", err)
	}

	// Extract SHA from output
	sha := extractSHAFromOutput(string(cloneOutput))
	if sha != "" {
		return fmt.Sprintf("Cloned (SHA: %s)", sha), nil
	}
	return "Cloned", nil
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

// buildCloneCommand builds the git clone command
func (h *CloneHandler) buildCloneCommand(clonePath string) string {
	if h.Rule.Branch != "" {
		return fmt.Sprintf("git clone -b %s %s %s", h.Rule.Branch, h.Rule.CloneURL, clonePath)
	}
	return fmt.Sprintf("git clone %s %s", h.Rule.CloneURL, clonePath)
}

// updateRepository updates an existing repository
func (h *CloneHandler) updateRepository(clonePath string) (string, error) {
	// Get current SHA
	getSHACmd := fmt.Sprintf("cd %s && git rev-parse HEAD", clonePath)
	oldSHAOutput, _ := exec.Command("sh", "-c", getSHACmd).CombinedOutput()
	oldSHA := strings.TrimSpace(string(oldSHAOutput))

	// Pull updates
	pullCmd := fmt.Sprintf("cd %s && git pull", clonePath)
	if err := exec.Command("sh", "-c", pullCmd).Run(); err != nil {
		return "", fmt.Errorf("pull failed: %w", err)
	}

	// Get new SHA
	newSHAOutput, _ := exec.Command("sh", "-c", getSHACmd).CombinedOutput()
	newSHA := strings.TrimSpace(string(newSHAOutput))

	// Check if SHA changed
	if oldSHA != "" && newSHA != "" && oldSHA != newSHA {
		return fmt.Sprintf("Updated (SHA changed: %s → %s) (SHA: %s)", oldSHA[:8], newSHA[:8], newSHA), nil
	} else if newSHA != "" {
		return fmt.Sprintf("Already up to date (SHA: %s)", newSHA), nil
	}

	return "Already up to date", nil
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

// extractSHAFromOutput extracts the SHA from git output
func extractSHAFromOutput(output string) string {
	// Look for patterns like "SHA: abc123def456..."
	re := regexp.MustCompile(`\(SHA:\s*([a-fA-F0-9]+)\)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) > 1 {
		return matches[1]
	}

	// Also look for direct SHA (40 hex characters)
	re2 := regexp.MustCompile(`\b([a-fA-F0-9]{40})\b`)
	matches = re2.FindStringSubmatch(output)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}
