package platform

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/elpic/blueprint/internal"
	cryptopkg "github.com/elpic/blueprint/internal/crypto"
	gitpkg "github.com/elpic/blueprint/internal/git"
	"github.com/go-git/go-git/v5"
)

// container implements the Container interface and manages all platform dependencies.
// It uses lazy initialization to only create providers when needed.
type container struct {
	// Provider instances (lazily initialized)
	systemProvider         SystemProvider
	gitProvider            GitProvider
	cryptoProvider         CryptoProvider
	packageManagerProvider PackageManagerProvider

	// Factory functions for creating providers
	systemProviderFactory         func() SystemProvider
	gitProviderFactory            func() GitProvider
	cryptoProviderFactory         func() CryptoProvider
	packageManagerProviderFactory func() PackageManagerProvider

	// Synchronization for lazy initialization
	mu sync.RWMutex
}

// NewContainer creates a new production container with real implementations.
func NewContainer() Container {
	return &container{
		systemProviderFactory:         NewSystemProvider,
		gitProviderFactory:            NewGitProvider,
		cryptoProviderFactory:         NewCryptoProvider,
		packageManagerProviderFactory: NewPackageManagerProvider,
	}
}

// NewTestContainer creates a new test container that can be configured with mocks.
// All providers are initially nil and must be set via the configuration functions.
func NewTestContainer() *TestContainer {
	c := &container{}
	return &TestContainer{container: c}
}

// SystemProvider returns the system provider instance, creating it if necessary.
func (c *container) SystemProvider() SystemProvider {
	c.mu.RLock()
	if c.systemProvider != nil {
		defer c.mu.RUnlock()
		return c.systemProvider
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check pattern to avoid creating multiple instances
	if c.systemProvider == nil && c.systemProviderFactory != nil {
		c.systemProvider = c.systemProviderFactory()
	}

	return c.systemProvider
}

// GitProvider returns the git provider instance, creating it if necessary.
func (c *container) GitProvider() GitProvider {
	c.mu.RLock()
	if c.gitProvider != nil {
		defer c.mu.RUnlock()
		return c.gitProvider
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.gitProvider == nil && c.gitProviderFactory != nil {
		c.gitProvider = c.gitProviderFactory()
	}

	return c.gitProvider
}

// CryptoProvider returns the crypto provider instance, creating it if necessary.
func (c *container) CryptoProvider() CryptoProvider {
	c.mu.RLock()
	if c.cryptoProvider != nil {
		defer c.mu.RUnlock()
		return c.cryptoProvider
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cryptoProvider == nil && c.cryptoProviderFactory != nil {
		c.cryptoProvider = c.cryptoProviderFactory()
	}

	return c.cryptoProvider
}

// PackageManagerProvider returns the package manager provider instance, creating it if necessary.
func (c *container) PackageManagerProvider() PackageManagerProvider {
	c.mu.RLock()
	if c.packageManagerProvider != nil {
		defer c.mu.RUnlock()
		return c.packageManagerProvider
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.packageManagerProvider == nil && c.packageManagerProviderFactory != nil {
		c.packageManagerProvider = c.packageManagerProviderFactory()
	}

	return c.packageManagerProvider
}

// TestContainer provides a fluent interface for configuring test containers.
// It allows easy setup of mocks and test doubles for unit testing.
type TestContainer struct {
	container *container
}

// WithSystemProvider configures the test container to use a specific system provider.
func (tc *TestContainer) WithSystemProvider(provider SystemProvider) *TestContainer {
	tc.container.mu.Lock()
	defer tc.container.mu.Unlock()
	tc.container.systemProvider = provider
	return tc
}

// WithGitProvider configures the test container to use a specific git provider.
func (tc *TestContainer) WithGitProvider(provider GitProvider) *TestContainer {
	tc.container.mu.Lock()
	defer tc.container.mu.Unlock()
	tc.container.gitProvider = provider
	return tc
}

// WithCryptoProvider configures the test container to use a specific crypto provider.
func (tc *TestContainer) WithCryptoProvider(provider CryptoProvider) *TestContainer {
	tc.container.mu.Lock()
	defer tc.container.mu.Unlock()
	tc.container.cryptoProvider = provider
	return tc
}

// WithPackageManagerProvider configures the test container to use a specific package manager provider.
func (tc *TestContainer) WithPackageManagerProvider(provider PackageManagerProvider) *TestContainer {
	tc.container.mu.Lock()
	defer tc.container.mu.Unlock()
	tc.container.packageManagerProvider = provider
	return tc
}

// Build returns the configured Container.
func (tc *TestContainer) Build() Container {
	return tc.container
}

// Factory functions for creating real implementations.
// These are defined as variables to allow easy stubbing in tests if needed.

var (
	// NewSystemProvider creates a real system provider
	NewSystemProvider = func() SystemProvider {
		return &realSystemProvider{
			osDetector:         &realOSDetector{},
			processExecutor:    &realProcessExecutor{},
			filesystemProvider: &realFilesystemProvider{},
			networkProvider:    &realNetworkProvider{},
		}
	}

	// NewGitProvider creates a real git provider
	NewGitProvider = func() GitProvider {
		return &realGitProvider{}
	}

	// NewCryptoProvider creates a real crypto provider
	NewCryptoProvider = func() CryptoProvider {
		return &realCryptoProvider{}
	}

	// NewPackageManagerProvider creates a real package manager provider
	NewPackageManagerProvider = func() PackageManagerProvider {
		return &realPackageManagerProvider{}
	}
)

// Real implementations for production use.
// These implement the actual platform-specific operations.

type realSystemProvider struct {
	osDetector         OSDetector
	processExecutor    ProcessExecutor
	filesystemProvider FilesystemProvider
	networkProvider    NetworkProvider
}

func (p *realSystemProvider) OS() OSDetector                 { return p.osDetector }
func (p *realSystemProvider) Process() ProcessExecutor       { return p.processExecutor }
func (p *realSystemProvider) Filesystem() FilesystemProvider { return p.filesystemProvider }
func (p *realSystemProvider) Network() NetworkProvider       { return p.networkProvider }

type realOSDetector struct{}

// Name returns the normalized OS name (mac, linux, windows).
func (d *realOSDetector) Name() string {
	detector := internal.NewOSDetector()
	return detector.Name()
}

// Architecture returns the system architecture.
func (d *realOSDetector) Architecture() string {
	return runtime.GOARCH
}

// IsRoot returns true if running with root/admin privileges.
func (d *realOSDetector) IsRoot() bool {
	currentUser, err := user.Current()
	if err != nil {
		return false
	}

	uid, err := strconv.Atoi(currentUser.Uid)
	if err != nil {
		return false
	}

	return uid == 0
}

// CurrentUser returns information about the current user.
func (d *realOSDetector) CurrentUser() (UserInfo, error) {
	currentUser, err := user.Current()
	if err != nil {
		return UserInfo{}, err
	}

	return UserInfo{
		Username: currentUser.Username,
		UID:      currentUser.Uid,
		GID:      currentUser.Gid,
		HomeDir:  currentUser.HomeDir,
	}, nil
}

type realProcessExecutor struct{}

// Execute runs a command and returns the result.
func (e *realProcessExecutor) Execute(cmd string, options ExecuteOptions) (*ExecuteResult, error) {
	return e.ExecuteWithContext(cmd, options, 0)
}

// ExecuteWithContext runs a command with context/timeout.
func (e *realProcessExecutor) ExecuteWithContext(cmd string, options ExecuteOptions, timeout time.Duration) (*ExecuteResult, error) {
	start := time.Now()

	var ctx context.Context
	var cancel context.CancelFunc

	if timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
		defer cancel()
	} else {
		ctx = context.Background()
	}

	// Split command into parts for proper execution
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	execCmd := exec.CommandContext(ctx, parts[0], parts[1:]...)

	// Set working directory if specified
	if options.WorkingDir != "" {
		execCmd.Dir = options.WorkingDir
	}

	// Set environment variables
	if len(options.Environment) > 0 {
		envVars := os.Environ()
		for key, value := range options.Environment {
			envVars = append(envVars, fmt.Sprintf("%s=%s", key, value))
		}
		execCmd.Env = envVars
	}

	// Set up input if provided
	if options.Input != "" {
		execCmd.Stdin = strings.NewReader(options.Input)
	}

	// Execute command
	output, err := execCmd.CombinedOutput()
	duration := time.Since(start)
	exitCode := 0

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = 1
		}
	}

	result := &ExecuteResult{
		ExitCode: exitCode,
		Stdout:   string(output),
		Stderr:   "", // Combined output includes stderr
		Duration: duration,
		Success:  exitCode == 0,
	}

	if !result.Success {
		return result, fmt.Errorf("command failed with exit code %d: %s", exitCode, string(output))
	}

	return result, nil
}

// IsCommandAvailable checks if a command exists in PATH.
func (e *realProcessExecutor) IsCommandAvailable(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// GetEnvironmentVar returns an environment variable value.
func (e *realProcessExecutor) GetEnvironmentVar(key string) string {
	return os.Getenv(key)
}

// SetEnvironmentVar sets an environment variable for child processes.
func (e *realProcessExecutor) SetEnvironmentVar(key, value string) error {
	return os.Setenv(key, value)
}

type realFilesystemProvider struct{}

// Exists checks if a file or directory exists.
func (f *realFilesystemProvider) Exists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// IsDirectory checks if path is a directory.
func (f *realFilesystemProvider) IsDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// IsFile checks if path is a regular file.
func (f *realFilesystemProvider) IsFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}

// ReadFile reads entire file contents.
func (f *realFilesystemProvider) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// WriteFile writes data to a file.
func (f *realFilesystemProvider) WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}

// AppendToFile appends data to a file.
func (f *realFilesystemProvider) AppendToFile(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	_, err = file.Write(data)
	return err
}

// CreateDirectory creates a directory and any necessary parent directories.
func (f *realFilesystemProvider) CreateDirectory(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// RemoveFile removes a file.
func (f *realFilesystemProvider) RemoveFile(path string) error {
	return os.Remove(path)
}

// RemoveDirectory removes a directory and all its contents.
func (f *realFilesystemProvider) RemoveDirectory(path string) error {
	return os.RemoveAll(path)
}

// CopyFile copies a file from source to destination.
func (f *realFilesystemProvider) CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = sourceFile.Close() }()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = destFile.Close() }()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// MoveFile moves/renames a file.
func (f *realFilesystemProvider) MoveFile(src, dst string) error {
	return os.Rename(src, dst)
}

// ListDirectory lists files and directories in a path.
func (f *realFilesystemProvider) ListDirectory(path string) ([]FileInfo, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var result []FileInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		result = append(result, FileInfo{
			Name:    entry.Name(),
			Path:    filepath.Join(path, entry.Name()),
			Size:    info.Size(),
			Mode:    info.Mode(),
			ModTime: info.ModTime(),
			IsDir:   entry.IsDir(),
		})
	}

	return result, nil
}

// GetPermissions returns file permissions.
func (f *realFilesystemProvider) GetPermissions(path string) (os.FileMode, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Mode(), nil
}

// SetPermissions sets file permissions.
func (f *realFilesystemProvider) SetPermissions(path string, perm os.FileMode) error {
	return os.Chmod(path, perm)
}

// CreateSymlink creates a symbolic link.
func (f *realFilesystemProvider) CreateSymlink(oldPath, newPath string) error {
	return os.Symlink(oldPath, newPath)
}

// ReadSymlink reads the target of a symbolic link.
func (f *realFilesystemProvider) ReadSymlink(path string) (string, error) {
	return os.Readlink(path)
}

// GetFileInfo returns detailed file information.
func (f *realFilesystemProvider) GetFileInfo(path string) (FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return FileInfo{}, err
	}

	return FileInfo{
		Name:    info.Name(),
		Path:    path,
		Size:    info.Size(),
		Mode:    info.Mode(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
	}, nil
}

// TempDirectory creates a temporary directory.
func (f *realFilesystemProvider) TempDirectory(prefix string) (string, error) {
	return os.MkdirTemp("", prefix)
}

// ExpandPath expands ~ and environment variables in paths.
func (f *realFilesystemProvider) ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		if currentUser, err := user.Current(); err == nil {
			path = filepath.Join(currentUser.HomeDir, path[2:])
		}
	}
	return os.ExpandEnv(path)
}

// OpenFile opens a file with specified flags.
func (f *realFilesystemProvider) OpenFile(path string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	return os.OpenFile(path, flag, perm)
}

type realNetworkProvider struct{}

// HTTPClient returns an HTTP client for making requests.
func (n *realNetworkProvider) HTTPClient() HTTPClient {
	return &http.Client{
		Timeout: 30 * time.Second,
	}
}

// DownloadFile downloads a file from URL to local path.
func (n *realNetworkProvider) DownloadFile(url, path string) error {
	resp, err := n.HTTPClient().Get(url)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	_, err = io.Copy(file, resp.Body)
	return err
}

// GetURLContent fetches content from a URL.
func (n *realNetworkProvider) GetURLContent(url string) ([]byte, error) {
	resp, err := n.HTTPClient().Get(url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// IsReachable checks if a host is reachable.
func (n *realNetworkProvider) IsReachable(host string, port int, timeout time.Duration) bool {
	// For simplicity, we'll just try to make an HTTP request
	// In a real implementation, this could use net.Dial
	client := &http.Client{Timeout: timeout}
	url := fmt.Sprintf("http://%s:%d", host, port)
	_, err := client.Get(url)
	return err == nil
}

type realGitProvider struct{}

func (g *realGitProvider) Clone(url, path, branch string) (*GitResult, error) {
	oldSHA, newSHA, status, err := gitpkg.CloneOrUpdateRepository(url, path, branch)
	if err != nil {
		return nil, err
	}
	return &GitResult{Status: status, OldSHA: oldSHA, NewSHA: newSHA}, nil
}

func (g *realGitProvider) Update(path, branch string) (*GitResult, error) {
	// Update has no URL argument, so read the "origin" remote URL from the
	// existing local repository first. Use repo.Remote("origin") rather than
	// iterating Remotes() — it returns a direct error when origin is absent.
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository at %s: %w", path, err)
	}
	origin, err := repo.Remote("origin")
	if err != nil || len(origin.Config().URLs) == 0 {
		return nil, fmt.Errorf("no origin remote found in repository: %s", path)
	}

	oldSHA, newSHA, status, err := gitpkg.CloneOrUpdateRepository(origin.Config().URLs[0], path, branch)
	if err != nil {
		return nil, err
	}
	return &GitResult{Status: status, OldSHA: oldSHA, NewSHA: newSHA}, nil
}

func (g *realGitProvider) GetLocalSHA(path string) (string, error) {
	return gitpkg.LocalSHAWithError(path)
}

func (g *realGitProvider) GetRemoteHeadSHA(url, branch string) (string, error) {
	return gitpkg.RemoteHeadSHAWithError(url, branch)
}

// IsRepository reports whether path looks like a git repository.
// The path itself must be a directory, and it must contain a .git entry.
// The .git entry may be a directory (standard and bare repositories) or a
// regular file (worktrees and gitfiles such as submodules), so its mere
// existence is the check rather than requiring a directory.
func (g *realGitProvider) IsRepository(path string) bool {
	pathInfo, err := os.Stat(path)
	if err != nil || !pathInfo.IsDir() {
		return false
	}
	gitEntry, err := os.Stat(filepath.Join(path, ".git"))
	if err != nil {
		return false
	}
	return gitEntry.IsDir() || gitEntry.Mode().IsRegular()
}

type realCryptoProvider struct{}

// minEncryptedFileSize is the minimum plausible size of an EncryptFile output:
// 32-byte salt + 12-byte GCM nonce + 16-byte GCM tag = 60 bytes. Anything
// smaller cannot have been produced by EncryptFile.
const minEncryptedFileSize = 60

func (c *realCryptoProvider) Encrypt(inputPath, outputPath, password string) error {
	plaintext, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}
	encrypted, err := cryptopkg.EncryptFile(plaintext, password)
	if err != nil {
		return fmt.Errorf("encryption failed: %w", err)
	}
	if dir := filepath.Dir(outputPath); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, internal.SensitiveDirectoryPermission); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}
	if err := os.WriteFile(outputPath, encrypted, internal.FilePermission); err != nil { // #nosec G703 -- outputPath is a caller-supplied path
		return fmt.Errorf("failed to write encrypted file: %w", err)
	}
	return nil
}

func (c *realCryptoProvider) Decrypt(inputPath, outputPath, password string) error {
	encrypted, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}
	decrypted, err := cryptopkg.DecryptFile(encrypted, password)
	if err != nil {
		return fmt.Errorf("decryption failed: %w", err)
	}
	if dir := filepath.Dir(outputPath); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, internal.SensitiveDirectoryPermission); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}
	if err := os.WriteFile(outputPath, decrypted, internal.FilePermission); err != nil { // #nosec G703 -- outputPath is a caller-supplied path
		return fmt.Errorf("failed to write decrypted file: %w", err)
	}
	return nil
}

// IsEncrypted uses a size heuristic: EncryptFile output is always at least
// minEncryptedFileSize bytes (salt + nonce + GCM tag) and is indistinguishable
// from random data. A file that exists and is at least that large is treated
// as encrypted; anything smaller, or unreadable, is not.
func (c *realCryptoProvider) IsEncrypted(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Size() >= minEncryptedFileSize
}

type realPackageManagerProvider struct {
	// osDetector is injected for testability; when nil the real detector is used.
	osDetector OSDetector
}

// os returns the configured OS detector, falling back to the real one.
func (p *realPackageManagerProvider) os() OSDetector {
	if p.osDetector != nil {
		return p.osDetector
	}
	return &realOSDetector{}
}

// shouldAddSudo reports whether apt commands need the sudo prefix.
// Mirrors install.go's logic: Linux requires sudo unless running as root.
func (p *realPackageManagerProvider) shouldAddSudo() bool {
	return p.os().Name() == "linux" && !p.os().IsRoot()
}

// probeCommand returns the command used to probe a package's presence/version,
// or "" for unsupported package managers.
func (p *realPackageManagerProvider) probeCommand(packageName, manager string) string {
	switch manager {
	case "brew", "homebrew":
		return "brew list --versions " + packageName
	case "apt", "apt-get":
		return "dpkg -s " + packageName
	default:
		return ""
	}
}

// installCommand builds the install command for the given packages and manager
// without executing anything.
func (p *realPackageManagerProvider) installCommand(packages []string, manager string) (string, error) {
	if len(packages) == 0 {
		return "", fmt.Errorf("no packages specified")
	}
	pkgStr := strings.Join(packages, " ")
	switch manager {
	case "brew", "homebrew":
		return fmt.Sprintf("brew install %s", pkgStr), nil
	case "apt", "apt-get":
		cmd := fmt.Sprintf("apt-get install -y %s", pkgStr)
		if p.shouldAddSudo() {
			cmd = "sudo " + cmd
		}
		return cmd, nil
	default:
		return "", fmt.Errorf("unsupported package manager: %s", manager)
	}
}

// uninstallCommand builds the uninstall command for the given packages and
// manager without executing anything.
func (p *realPackageManagerProvider) uninstallCommand(packages []string, manager string) (string, error) {
	if len(packages) == 0 {
		return "", fmt.Errorf("no packages specified")
	}
	pkgStr := strings.Join(packages, " ")
	switch manager {
	case "brew", "homebrew":
		return fmt.Sprintf("brew uninstall -y %s", pkgStr), nil
	case "apt", "apt-get":
		cmd := fmt.Sprintf("apt-get remove -y %s", pkgStr)
		if p.shouldAddSudo() {
			cmd = "sudo " + cmd
		}
		return cmd, nil
	default:
		return "", fmt.Errorf("unsupported package manager: %s", manager)
	}
}

// runCommand executes a probe command and reports success (exit 0).
func (p *realPackageManagerProvider) runCommand(cmd string) bool {
	result, err := (&realProcessExecutor{}).Execute(cmd, ExecuteOptions{})
	return err == nil && result.Success
}

// parseBrewVersion extracts the version from "brew list --versions <pkg>"
// output, e.g. "git 2.43.0" → "2.43.0".
func parseBrewVersion(output string) (string, error) {
	fields := strings.Fields(output)
	if len(fields) < 2 {
		return "", fmt.Errorf("unable to parse version from brew output: %q", output)
	}
	return fields[len(fields)-1], nil
}

// parseDpkgVersion extracts the version from "dpkg -s <pkg>" output by finding
// the "Version: x.y.z" line.
func parseDpkgVersion(output string) (string, error) {
	for _, line := range strings.Split(output, "\n") {
		if trimmed, ok := strings.CutPrefix(line, "Version:"); ok {
			if version := strings.TrimSpace(trimmed); version != "" {
				return version, nil
			}
		}
	}
	return "", fmt.Errorf("unable to find version line in dpkg output")
}

func (p *realPackageManagerProvider) Install(packages []string, manager string) (*ExecuteResult, error) {
	cmd, err := p.installCommand(packages, manager)
	if err != nil {
		return nil, err
	}
	return (&realProcessExecutor{}).Execute(cmd, ExecuteOptions{})
}

func (p *realPackageManagerProvider) Uninstall(packages []string, manager string) (*ExecuteResult, error) {
	cmd, err := p.uninstallCommand(packages, manager)
	if err != nil {
		return nil, err
	}
	return (&realProcessExecutor{}).Execute(cmd, ExecuteOptions{})
}

func (p *realPackageManagerProvider) IsInstalled(packageName, manager string) bool {
	cmd := p.probeCommand(packageName, manager)
	if cmd == "" {
		return false
	}
	return p.runCommand(cmd)
}

func (p *realPackageManagerProvider) GetInstalledVersion(packageName, manager string) (string, error) {
	cmd := p.probeCommand(packageName, manager)
	if cmd == "" {
		return "", fmt.Errorf("unsupported package manager: %s", manager)
	}
	result, err := (&realProcessExecutor{}).Execute(cmd, ExecuteOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to determine installed version of %s: %w", packageName, err)
	}
	switch manager {
	case "brew", "homebrew":
		version, parseErr := parseBrewVersion(result.Stdout)
		if parseErr != nil {
			return "", fmt.Errorf("failed to parse version for %s: %w", packageName, parseErr)
		}
		return version, nil
	case "apt", "apt-get":
		version, parseErr := parseDpkgVersion(result.Stdout)
		if parseErr != nil {
			return "", fmt.Errorf("failed to parse version for %s: %w", packageName, parseErr)
		}
		return version, nil
	default:
		return "", fmt.Errorf("unsupported package manager: %s", manager)
	}
}

func (p *realPackageManagerProvider) IsManagerAvailable(manager string) bool {
	switch manager {
	case "brew", "homebrew":
		_, err := exec.LookPath("brew")
		return err == nil
	case "apt", "apt-get":
		// Install/uninstall commands are built around apt-get, so accept either
		// the "apt" or "apt-get" binary being present.
		if _, err := exec.LookPath("apt-get"); err == nil {
			return true
		}
		_, err := exec.LookPath("apt")
		return err == nil
	default:
		return false
	}
}

func (p *realPackageManagerProvider) GetDefaultManager() string {
	switch p.os().Name() {
	case "mac":
		return "brew"
	case "linux":
		return "apt"
	default:
		return ""
	}
}
