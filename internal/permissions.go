package internal

import "os"

// File and directory permission constants
const (
	// DirectoryPermission is the default permission for directories (rwxr-x---)
	// Used for: blueprint directory, history directory
	DirectoryPermission os.FileMode = 0o750

	// SensitiveDirectoryPermission is the permission for sensitive directories (rwx------)
	// Used for: SSH directory, decrypt destination directory
	SensitiveDirectoryPermission os.FileMode = 0o700

	// FilePermission is the permission for sensitive files (rw-------)
	// Used for: encrypted files, status files, history files, temporary files
	FilePermission os.FileMode = 0o600
)
