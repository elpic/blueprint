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

// --- named scalar types: type-safety now, promotion path later ---

// SHA is a git commit hash.
type SHA string

// RepoPath is a working-copy or bare-layout repository path (~ allowed;
// expansion belongs to the engine below this seam).
type RepoPath string

// RepoID identifies a remote: URL in any accepted form (@github:, ssh, https)
// + optional branch. Struct because the pair already repeats across three
// methods; new fields (e.g. auth hints) won't touch signatures.
type RepoID struct {
	URL    string
	Branch string
}

// CloneMode selects the clone strategy. The zero value is ModeTwoStage, the
// engine's default, so a spec constructed without an explicit mode does what
// clone rules do today.
type CloneMode uint8

const (
	// ModeTwoStage clones into clean storage and mirrors the content into the
	// target (no .git in the target — it is a mirror, not a working repo).
	ModeTwoStage CloneMode = iota
	// ModeDirect clones a fully functional working copy. Updating an existing
	// copy reverts tracked changes, but untracked files MUST survive (#031).
	ModeDirect
	// ModeBare keeps the repository itself in <path>/.git (bare) with a
	// worktree per branch under <path>/; updates never touch existing
	// worktrees.
	ModeBare
)

// CloneSpec is resolved intent: the caller interpreted the rule, the engine
// executes the spec. Shorthand/`~` expansion belong to the engine.
type CloneSpec struct {
	URL    string
	Path   string
	Branch string
	Mode   CloneMode // ModeTwoStage (default) | ModeDirect | ModeBare
}

// CheckoutSpec is doctor's exact-SHA comparison. Force/Detach are realistic
// future fields (the current internal impl already forces) — spec type from
// day one so adding them touches zero call sites.
type CheckoutSpec struct {
	Path RepoPath
	SHA  SHA
}

// CloneStatus enumerates what a Clone did. Callers switch on the enum —
// never on status strings — so the compiler enumerates every case.
type CloneStatus uint8

const (
	StatusCloned   CloneStatus = iota // target did not exist; created
	StatusUpdated                     // existing target moved to a new SHA
	StatusSynced                      // content resynced, SHA unchanged
	StatusUpToDate                    // nothing to do
)

// CloneResult reports what a Clone did.
type CloneResult struct {
	Status CloneStatus
	OldSHA SHA
	NewSHA SHA
	// Notes carries recoverable incidents (repair counts, #031
	// untracked-file protection warnings). Never a failure — failures are
	// errors.
	Notes []string
}

// RepoLayout classifies what sits at a path.
type RepoLayout uint8

const (
	LayoutNone     RepoLayout = iota // not a repository
	LayoutStandard                   // working copy with a .git directory
	LayoutBare                       // bare repository (path itself, or <path>/.git in the bare-clone layout)
	LayoutWorktree                   // linked worktree / gitfile (.git is a regular file)
)

// GitProvider is the one seam for all git operations. Handlers, engine,
// doctor, renderer, and parser program against it; the implementation behind
// it (go-git v5 + system-git fallbacks today, go-git v6 after #032) is a
// swappable detail.
//
// Error contract:
//   - Hard failures (clone/fetch failure, checkout conflict) are errors.
//   - Recoverable incidents are CloneResult.Notes, never errors.
//   - An unknown SHA is ("", nil), not an error.
//
// Data-safety contract per CloneMode:
//   - ModeTwoStage: the target is a content mirror (no .git); re-cloning
//     replaces content but the clean storage is the source of truth.
//   - ModeDirect: updating an existing copy reverts tracked changes, but
//     untracked files MUST survive (#031).
//   - ModeBare: updates never touch existing worktrees registered under
//     <path>/.
type GitProvider interface {
	// Clone clones or updates per spec. The mode's data-safety contract above
	// is binding on every implementation.
	Clone(spec CloneSpec) (CloneResult, error)
	// Checkout checks out an exact SHA (forced; doctor inspects the version
	// that was applied).
	Checkout(spec CheckoutSpec) error

	// LocalSHA returns the HEAD SHA of the repository at path.
	// ("", nil) = unknown, not error.
	LocalSHA(path RepoPath) (SHA, error)
	// RemoteSHA returns the remote HEAD (or branch tip) SHA for id.
	// Branch "" resolves the remote default via symref.
	RemoteSHA(id RepoID) (SHA, error)
	// BareSHA returns the SHA a bare-layout clone at path is synced to, for
	// the given branch ("" for the remote default).
	BareSHA(path RepoPath, branch string) (SHA, error)
	// StorageSHA returns the HEAD SHA of the clean storage copy of id, or
	// ("", nil) when no clean storage exists.
	StorageSHA(id RepoID) (SHA, error)

	// Layout classifies what sits at path.
	Layout(path RepoPath) RepoLayout
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
