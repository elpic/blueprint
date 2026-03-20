package handlers

import (
	"os"
	"path/filepath"
	"strings"
)

// homeDirProvider is a port interface for getting the home directory.
// This allows dependency injection for testing error scenarios.
type homeDirProvider interface {
	// UserHomeDir returns the current user's home directory.
	UserHomeDir() (string, error)
}

// defaultHomeDirProvider is the production adapter using os.UserHomeDir.
type defaultHomeDirProvider struct{}

func (d *defaultHomeDirProvider) UserHomeDir() (string, error) {
	return os.UserHomeDir()
}

// homeDir is a variable for testability.
var homeDir homeDirProvider = &defaultHomeDirProvider{}

// expandPath expands a leading ~ to the current user's home directory.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := homeDir.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}
