package handlers

import (
	"os"
	"path/filepath"
	"strings"
)

// expandPath expands a leading ~ to the current user's home directory.
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
