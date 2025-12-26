package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	cryptopkg "github.com/elpic/blueprint/internal/crypto"
	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
)

// DecryptHandler handles file decryption and cleanup
type DecryptHandler struct {
	BaseHandler
	passwordCache map[string]string // Reference to password cache
}

// NewDecryptHandler creates a new decrypt handler
func NewDecryptHandler(rule parser.Rule, basePath string, passwordCache map[string]string) *DecryptHandler {
	return &DecryptHandler{
		BaseHandler: BaseHandler{
			Rule:     rule,
			BasePath: basePath,
		},
		passwordCache: passwordCache,
	}
}

// Up decrypts the file to the destination path
func (h *DecryptHandler) Up() (string, error) {
	// Get password from cache
	passwordID := h.Rule.DecryptPasswordID
	if passwordID == "" {
		passwordID = "default"
	}

	password, ok := h.passwordCache[passwordID]
	if !ok {
		return "", fmt.Errorf("no password cached for password-id: %s", passwordID)
	}

	// Resolve source file path
	sourceFile := h.resolveFilePath(h.Rule.DecryptFile)
	if _, err := os.Stat(sourceFile); err != nil {
		return "", fmt.Errorf("encrypted file not found: %s", sourceFile)
	}

	// Read encrypted file
	encryptedData, err := os.ReadFile(sourceFile)
	if err != nil {
		return "", fmt.Errorf("failed to read encrypted file: %w", err)
	}

	// Decrypt file
	decryptedData, err := cryptopkg.DecryptFile(encryptedData, password)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	// Expand destination path
	destPath := expandPath(h.Rule.DecryptPath)

	// Create directory if needed
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Write decrypted file
	if err := os.WriteFile(destPath, decryptedData, 0600); err != nil {
		return "", fmt.Errorf("failed to write decrypted file: %w", err)
	}

	return fmt.Sprintf("Decrypted to %s", destPath), nil
}

// Down removes the decrypted file
func (h *DecryptHandler) Down() (string, error) {
	destPath := expandPath(h.Rule.DecryptPath)

	// Remove file if it exists
	if _, err := os.Stat(destPath); err == nil {
		if err := os.Remove(destPath); err != nil {
			return "", fmt.Errorf("failed to remove decrypted file: %w", err)
		}
		return fmt.Sprintf("Removed decrypted file at %s", destPath), nil
	}

	return "Decrypted file not found", nil
}

// GetCommand returns the actual command(s) that will be executed
func (h *DecryptHandler) GetCommand() string {
	if h.Rule.Action == "uninstall" {
		return fmt.Sprintf("rm -f %s", h.Rule.DecryptPath)
	}

	// Decrypt action - show description since it's not a shell command
	return fmt.Sprintf("Decrypt file: %s → %s", h.Rule.DecryptFile, h.Rule.DecryptPath)
}

// UpdateStatus updates the status after decrypting or removing a file
func (h *DecryptHandler) UpdateStatus(status *Status, records []ExecutionRecord, blueprint string, osName string) error {
	// Normalize blueprint path for consistent storage and comparison
	blueprint = normalizePath(blueprint)

	if h.Rule.Action == "decrypt" {
		decryptCmd := h.GetCommand()

		_, commandExecuted := commandSuccessfullyExecuted(decryptCmd, records)

		if commandExecuted {
			// Remove existing entry if present
			status.Decrypts = removeDecryptStatus(status.Decrypts, h.Rule.DecryptPath, blueprint, osName)
			// Add new entry
			status.Decrypts = append(status.Decrypts, DecryptStatus{
				SourceFile:  h.Rule.DecryptFile,
				DestPath:    h.Rule.DecryptPath,
				DecryptedAt: time.Now().Format(time.RFC3339),
				Blueprint:   blueprint,
				OS:          osName,
			})
		}
	} else if h.Rule.Action == "uninstall" && DetectRuleType(h.Rule) == "decrypt" {
		// Check if decrypt file was removed by checking if file doesn't exist
		expandedPath := expandPath(h.Rule.DecryptPath)
		if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
			// File has been removed, update status
			status.Decrypts = removeDecryptStatus(status.Decrypts, h.Rule.DecryptPath, blueprint, osName)
		}
	}

	return nil
}

// DisplayInfo displays handler-specific information
func (h *DecryptHandler) DisplayInfo() {
	formatFunc := ui.FormatInfo
	if h.Rule.Action == "uninstall" {
		formatFunc = ui.FormatDim
	}

	fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("File: %s", h.Rule.DecryptFile)))
	fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Path: %s", h.Rule.DecryptPath)))
	if h.Rule.Group != "" {
		fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Group: %s", h.Rule.Group)))
	}
	if h.Rule.DecryptPasswordID != "" {
		fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Password ID: %s", h.Rule.DecryptPasswordID)))
	}
}

// resolveFilePath resolves the file path, checking multiple locations
func (h *DecryptHandler) resolveFilePath(file string) string {
	// If absolute path, use it directly
	if filepath.IsAbs(file) {
		return expandPath(file)
	}

	// Try relative to basePath first
	basePath := h.BasePath
	if basePath != "" {
		candidate := filepath.Join(basePath, file)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Try relative to current working directory
	if _, err := os.Stat(file); err == nil {
		return file
	}

	// Try home directory expansion if it starts with ~
	if file[0] == '~' {
		return expandPath(file)
	}

	// Return original (will fail at runtime with proper error message)
	return file
}

// DisplayStatus displays decrypted file status information
func (h *DecryptHandler) DisplayStatus(decrypts []DecryptStatus) {
	if len(decrypts) == 0 {
		return
	}

	fmt.Printf("\n%s\n", ui.FormatHighlight("Decrypted Files:"))
	for _, decrypt := range decrypts {
		// Parse timestamp for display
		t, err := time.Parse(time.RFC3339, decrypt.DecryptedAt)
		var timeStr string
		if err == nil {
			timeStr = t.Format("2006-01-02 15:04:05")
		} else {
			timeStr = decrypt.DecryptedAt
		}

		fmt.Printf("  %s %s (%s) [%s, %s]\n",
			ui.FormatSuccess("●"),
			ui.FormatInfo(decrypt.DestPath),
			ui.FormatDim(timeStr),
			ui.FormatDim(decrypt.OS),
			ui.FormatDim(decrypt.Blueprint),
		)
		fmt.Printf("     %s %s\n",
			ui.FormatDim("From:"),
			ui.FormatInfo(decrypt.SourceFile),
		)
	}
}
