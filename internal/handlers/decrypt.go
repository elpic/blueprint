package handlers

import (
	"fmt"
	"os"
	"path/filepath"

	cryptopkg "github.com/elpic/blueprint/internal/crypto"
	"github.com/elpic/blueprint/internal/parser"
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
