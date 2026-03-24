package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
)

func init() {
	RegisterAction(ActionDef{
		Name:   "download",
		Prefix: "download ",
		NewHandler: func(rule parser.Rule, basePath string, passwordCache map[string]string) Handler {
			return NewDownloadHandler(rule, basePath)
		},
		RuleKey: func(rule parser.Rule) string {
			return rule.DownloadPath
		},
		Detect: func(rule parser.Rule) bool {
			return rule.DownloadURL != ""
		},
		Summary: func(rule parser.Rule) string {
			return rule.DownloadURL + " → " + rule.DownloadPath
		},
		OrphanIndex: func(rule parser.Rule, index func(string)) {
			index(rule.DownloadPath)
		},
	})
}

// DownloadHandler handles downloading files from URLs
type DownloadHandler struct {
	BaseHandler
}

// NewDownloadHandler creates a new download handler
func NewDownloadHandler(rule parser.Rule, basePath string) *DownloadHandler {
	return &DownloadHandler{
		BaseHandler: BaseHandler{
			Rule:     rule,
			BasePath: basePath,
		},
	}
}

// Up downloads the file from the URL to the destination path
// httpClient returns the HTTP client used for downloading files.
func (h *DownloadHandler) httpClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

func (h *DownloadHandler) Up() (string, error) {
	destPath := expandPath(h.Rule.DownloadPath)

	// If overwrite is false and file already exists, skip
	if !h.Rule.DownloadOverwrite {
		if _, err := os.Stat(destPath); err == nil {
			return fmt.Sprintf("already exists, skipping: %s", destPath), nil
		}
	}

	// Create parent directories if needed
	parentDir := filepath.Dir(destPath)
	if err := os.MkdirAll(parentDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create parent directory %s: %w", parentDir, err)
	}

	// HTTP GET the URL
	resp, err := h.httpClient().Get(h.Rule.DownloadURL) // #nosec G107 -- URL is user-supplied via blueprint file
	if err != nil {
		return "", fmt.Errorf("failed to download %s: %w", h.Rule.DownloadURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status %d: %s", resp.StatusCode, h.Rule.DownloadURL)
	}

	// Write to a temp file then rename atomically
	tmpPath := destPath + ".tmp"
	tmpFile, err := os.Create(tmpPath) // #nosec G304 -- destPath is user-supplied via blueprint file
	if err != nil {
		return "", fmt.Errorf("failed to create temp file %s: %w", tmpPath, err)
	}

	_, err = io.Copy(tmpFile, resp.Body)
	if closeErr := tmpFile.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to write download to %s: %w", tmpPath, err)
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to move file to %s: %w", destPath, err)
	}

	// Apply permissions if specified
	if h.Rule.DownloadPerms != "" {
		var octal int
		_, _ = fmt.Sscanf(h.Rule.DownloadPerms, "%o", &octal)
		if err := os.Chmod(destPath, os.FileMode(octal)); err != nil { // #nosec G302 -- permissions explicitly chosen by user
			return "", fmt.Errorf("failed to set permissions on %s: %w", destPath, err)
		}
	}

	msg := fmt.Sprintf("Downloaded %s to %s", h.Rule.DownloadURL, destPath)
	if h.Rule.DownloadPerms != "" {
		msg += fmt.Sprintf(" (permissions: %s)", h.Rule.DownloadPerms)
	}
	return msg, nil
}

// Down removes the downloaded file
func (h *DownloadHandler) Down() (string, error) {
	destPath := expandPath(h.Rule.DownloadPath)

	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		return fmt.Sprintf("File %s does not exist", destPath), nil
	}

	if err := os.Remove(destPath); err != nil {
		return "", fmt.Errorf("failed to remove file %s: %w", destPath, err)
	}

	return fmt.Sprintf("Removed file %s", destPath), nil
}

// GetCommand returns a representative command string for display
func (h *DownloadHandler) GetCommand() string {
	destPath := h.Rule.DownloadPath
	if h.Rule.Action == "uninstall" {
		return fmt.Sprintf("rm -f %s", destPath)
	}

	cmd := fmt.Sprintf("curl -fsSL %s -o %s", h.Rule.DownloadURL, destPath)
	if h.Rule.DownloadPerms != "" {
		cmd += fmt.Sprintf(" && chmod %s %s", h.Rule.DownloadPerms, destPath)
	}
	return cmd
}

// UpdateStatus updates the blueprint status after executing download or uninstall-download
func (h *DownloadHandler) UpdateStatus(status *Status, records []ExecutionRecord, blueprint string, osName string) error {
	blueprint = normalizeBlueprint(blueprint)

	if h.Rule.Action == "download" {
		expectedCmd := h.GetCommand()
		downloadExecuted := false
		for _, record := range records {
			if record.Status == "success" && record.Command == expectedCmd {
				downloadExecuted = true
				break
			}
		}

		// Also check if the file was skipped (already exists) — treat as success
		if !downloadExecuted {
			skippedCmd := fmt.Sprintf("already exists, skipping: %s", expandPath(h.Rule.DownloadPath))
			for _, record := range records {
				if record.Status == "success" && strings.Contains(record.Output, skippedCmd) {
					downloadExecuted = true
					break
				}
			}
		}

		if downloadExecuted {
			status.Downloads = removeDownloadStatus(status.Downloads, h.Rule.DownloadPath, blueprint, osName)
			status.Downloads = append(status.Downloads, DownloadStatus{
				URL:          h.Rule.DownloadURL,
				Path:         h.Rule.DownloadPath,
				DownloadedAt: time.Now().Format(time.RFC3339),
				Blueprint:    blueprint,
				OS:           osName,
			})
		}
	} else if h.Rule.Action == "uninstall" && DetectRuleType(h.Rule) == "download" {
		expandedPath := expandPath(h.Rule.DownloadPath)
		if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
			status.Downloads = removeDownloadStatus(status.Downloads, h.Rule.DownloadPath, blueprint, osName)
		}
	}

	return nil
}

// DisplayInfo displays handler-specific information
func (h *DownloadHandler) DisplayInfo() {
	formatFunc := ui.FormatInfo
	if h.Rule.Action == "uninstall" {
		formatFunc = ui.FormatDim
	}

	fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("URL: %s", h.Rule.DownloadURL)))
	fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Destination: %s", h.Rule.DownloadPath)))
	if h.Rule.DownloadPerms != "" {
		fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Permissions: %s", h.Rule.DownloadPerms)))
	}
}

// GetDependencyKey returns the unique key for this rule in dependency resolution
func (h *DownloadHandler) GetDependencyKey() string {
	return getDependencyKey(h.Rule, h.Rule.DownloadPath)
}

// GetDisplayDetails returns the destination path to display during execution
func (h *DownloadHandler) GetDisplayDetails(isUninstall bool) string {
	return h.Rule.DownloadPath
}

// GetState returns handler-specific state as key-value pairs
func (h *DownloadHandler) GetState(isUninstall bool) map[string]string {
	return map[string]string{
		"summary": h.Rule.DownloadPath,
		"url":     h.Rule.DownloadURL,
		"path":    h.Rule.DownloadPath,
	}
}

// FindUninstallRules compares download status against current rules and returns uninstall rules
func (h *DownloadHandler) FindUninstallRules(status *Status, currentRules []parser.Rule, blueprintFile, osName string) []parser.Rule {
	normalizedBlueprint := normalizeBlueprint(blueprintFile)

	currentDownloadPaths := make(map[string]bool)
	for _, rule := range currentRules {
		if rule.Action == "download" && rule.DownloadPath != "" {
			currentDownloadPaths[rule.DownloadPath] = true
		}
	}

	var rules []parser.Rule
	if status.Downloads != nil {
		for _, dl := range status.Downloads {
			normalizedStatusBlueprint := normalizeBlueprint(dl.Blueprint)
			if normalizedStatusBlueprint == normalizedBlueprint && dl.OS == osName && !currentDownloadPaths[dl.Path] {
				rules = append(rules, parser.Rule{
					Action:       "uninstall",
					DownloadURL:  dl.URL,
					DownloadPath: dl.Path,
					OSList:       []string{osName},
				})
			}
		}
	}

	return rules
}

// IsInstalled returns true if the download path in this rule is already in status.
func (h *DownloadHandler) IsInstalled(status *Status, blueprintFile, osName string) bool {
	normalizedBlueprint := normalizeBlueprint(blueprintFile)
	for _, dl := range status.Downloads {
		if dl.Path == h.Rule.DownloadPath && normalizeBlueprint(dl.Blueprint) == normalizedBlueprint && dl.OS == osName {
			return true
		}
	}
	return false
}

// DisplayStatusFromStatus displays download status from Status object
func (h *DownloadHandler) DisplayStatusFromStatus(status *Status) {
	if status == nil || len(status.Downloads) == 0 {
		return
	}

	fmt.Printf("\n%s\n", ui.FormatHighlight("Downloaded Files:"))
	for _, dl := range status.Downloads {
		t, err := time.Parse(time.RFC3339, dl.DownloadedAt)
		var timeStr string
		if err == nil {
			timeStr = t.Format("2006-01-02 15:04:05")
		} else {
			timeStr = dl.DownloadedAt
		}

		fmt.Printf("  %s %s (%s) [%s, %s]\n",
			ui.FormatSuccess("●"),
			ui.FormatInfo(dl.Path),
			ui.FormatDim(timeStr),
			ui.FormatDim(dl.OS),
			ui.FormatDim(abbreviateBlueprintPath(dl.Blueprint)),
		)
	}
}
