// Package platform provides abstractions for system operations to enable testability.
// This package separates I/O and system dependencies from business logic, allowing
// for fast, reliable unit testing with mocks.
package platform

import (
	"io"
	"net/http"
	"os"
	"time"
)

// SystemProvider is the main interface that combines all platform operations.
// It serves as the entry point for all system interactions.
type SystemProvider interface {
	OS() OSDetector
	Process() ProcessExecutor
	Filesystem() FilesystemProvider
	Network() NetworkProvider
}

// OSDetector provides operating system detection capabilities.
type OSDetector interface {
	// Name returns the normalized OS name (mac, linux, windows)
	Name() string
	// Architecture returns the system architecture (amd64, arm64, etc.)
	Architecture() string
	// IsRoot returns true if running with root/admin privileges
	IsRoot() bool
	// CurrentUser returns information about the current user
	CurrentUser() (UserInfo, error)
}

// UserInfo represents user information
type UserInfo struct {
	Username string
	UID      string
	GID      string
	HomeDir  string
}

// ProcessExecutor handles command execution and process management.
type ProcessExecutor interface {
	// Execute runs a command and returns the result
	Execute(cmd string, options ExecuteOptions) (*ExecuteResult, error)
	// ExecuteWithContext runs a command with context/timeout
	ExecuteWithContext(cmd string, options ExecuteOptions, timeout time.Duration) (*ExecuteResult, error)
	// IsCommandAvailable checks if a command exists in PATH
	IsCommandAvailable(cmd string) bool
	// GetEnvironmentVar returns an environment variable value
	GetEnvironmentVar(key string) string
	// SetEnvironmentVar sets an environment variable for child processes
	SetEnvironmentVar(key, value string) error
}

// ExecuteOptions configures command execution
type ExecuteOptions struct {
	// WorkingDir sets the working directory for the command
	WorkingDir string
	// Environment provides additional environment variables
	Environment map[string]string
	// Input provides stdin input to the command
	Input string
	// StreamOutput determines if output should be streamed live
	StreamOutput bool
	// SudoPassword provides sudo password if needed
	SudoPassword string
}

// ExecuteResult represents the result of command execution
type ExecuteResult struct {
	// ExitCode is the process exit code
	ExitCode int
	// Stdout contains standard output
	Stdout string
	// Stderr contains standard error output
	Stderr string
	// Duration is how long the command took to execute
	Duration time.Duration
	// Success indicates if the command succeeded (exit code 0)
	Success bool
}

// FilesystemProvider handles file and directory operations.
type FilesystemProvider interface {
	// Exists checks if a file or directory exists
	Exists(path string) bool
	// IsDirectory checks if path is a directory
	IsDirectory(path string) bool
	// IsFile checks if path is a regular file
	IsFile(path string) bool
	// ReadFile reads entire file contents
	ReadFile(path string) ([]byte, error)
	// WriteFile writes data to a file
	WriteFile(path string, data []byte, perm os.FileMode) error
	// AppendToFile appends data to a file
	AppendToFile(path string, data []byte) error
	// CreateDirectory creates a directory and any necessary parent directories
	CreateDirectory(path string, perm os.FileMode) error
	// RemoveFile removes a file
	RemoveFile(path string) error
	// RemoveDirectory removes a directory and all its contents
	RemoveDirectory(path string) error
	// CopyFile copies a file from source to destination
	CopyFile(src, dst string) error
	// MoveFile moves/renames a file
	MoveFile(src, dst string) error
	// ListDirectory lists files and directories in a path
	ListDirectory(path string) ([]FileInfo, error)
	// GetPermissions returns file permissions
	GetPermissions(path string) (os.FileMode, error)
	// SetPermissions sets file permissions
	SetPermissions(path string, perm os.FileMode) error
	// CreateSymlink creates a symbolic link
	CreateSymlink(oldPath, newPath string) error
	// ReadSymlink reads the target of a symbolic link
	ReadSymlink(path string) (string, error)
	// GetFileInfo returns detailed file information
	GetFileInfo(path string) (FileInfo, error)
	// TempDirectory creates a temporary directory
	TempDirectory(prefix string) (string, error)
	// ExpandPath expands ~ and environment variables in paths
	ExpandPath(path string) string
	// OpenFile opens a file with specified flags
	OpenFile(path string, flag int, perm os.FileMode) (io.ReadWriteCloser, error)
}

// FileInfo represents file or directory information
type FileInfo struct {
	Name    string
	Path    string
	Size    int64
	Mode    os.FileMode
	ModTime time.Time
	IsDir   bool
}

// NetworkProvider handles network operations.
type NetworkProvider interface {
	// HTTPClient returns an HTTP client for making requests
	HTTPClient() HTTPClient
	// DownloadFile downloads a file from URL to local path
	DownloadFile(url, path string) error
	// GetURLContent fetches content from a URL
	GetURLContent(url string) ([]byte, error)
	// IsReachable checks if a host is reachable
	IsReachable(host string, port int, timeout time.Duration) bool
}

// HTTPClient provides HTTP request capabilities
type HTTPClient interface {
	// Get performs an HTTP GET request
	Get(url string) (*http.Response, error)
	// Post performs an HTTP POST request
	Post(url, contentType string, body io.Reader) (*http.Response, error)
	// Do performs an HTTP request
	Do(req *http.Request) (*http.Response, error)
}

// GitProvider handles Git operations (specialized network/filesystem operations).
// This is separated from other providers as Git operations are common enough
// to warrant their own interface but complex enough to need abstraction.
type GitProvider interface {
	// Clone clones a repository
	Clone(url, path, branch string) (*GitResult, error)
	// Update updates a repository to latest
	Update(path, branch string) (*GitResult, error)
	// GetLocalSHA returns the current HEAD SHA of a local repository
	GetLocalSHA(path string) (string, error)
	// GetRemoteHeadSHA returns the HEAD SHA of a remote repository
	GetRemoteHeadSHA(url, branch string) (string, error)
	// IsRepository checks if a path is a Git repository
	IsRepository(path string) bool
}

// GitResult represents the result of a Git operation
type GitResult struct {
	// Status indicates what happened (Cloned, Updated, Already up to date)
	Status string
	// OldSHA is the SHA before the operation (for updates)
	OldSHA string
	// NewSHA is the SHA after the operation
	NewSHA string
	// Message provides additional information
	Message string
}

// CryptoProvider handles encryption and decryption operations.
type CryptoProvider interface {
	// Decrypt decrypts a file using the provided password
	Decrypt(inputPath, outputPath, password string) error
	// Encrypt encrypts a file using the provided password
	Encrypt(inputPath, outputPath, password string) error
	// IsEncrypted checks if a file appears to be encrypted
	IsEncrypted(path string) bool
}

// PackageManagerProvider handles package manager operations.
// This abstracts different package managers (brew, apt, snap, etc.) behind a common interface.
type PackageManagerProvider interface {
	// Install installs packages using the appropriate package manager
	Install(packages []string, manager string) (*ExecuteResult, error)
	// Uninstall removes packages using the appropriate package manager
	Uninstall(packages []string, manager string) (*ExecuteResult, error)
	// IsInstalled checks if a package is installed
	IsInstalled(packageName, manager string) bool
	// GetInstalledVersion returns the installed version of a package
	GetInstalledVersion(packageName, manager string) (string, error)
	// IsManagerAvailable checks if a package manager is available
	IsManagerAvailable(manager string) bool
	// GetDefaultManager returns the default package manager for the current OS
	GetDefaultManager() string
}

// CommandExecutor interface for executing commands - allows dependency injection
// This replaces the old testMode pattern with clean dependency injection
type CommandExecutor interface {
	Execute(cmd string) (string, error)
}

// Container interface for dependency injection
type Container interface {
	// SystemProvider returns the system provider instance
	SystemProvider() SystemProvider
	// GitProvider returns the git provider instance
	GitProvider() GitProvider
	// CryptoProvider returns the crypto provider instance
	CryptoProvider() CryptoProvider
	// PackageManagerProvider returns the package manager provider instance
	PackageManagerProvider() PackageManagerProvider
}
