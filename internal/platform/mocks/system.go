// Package mocks provides mock implementations of platform interfaces for testing.
// All mocks use fluent interfaces for easy configuration in tests.
package mocks

import (
	"io"
	"net/http"
	"os"
	"time"

	"github.com/elpic/blueprint/internal/platform"
)

// MockSystemProvider provides a complete mock of SystemProvider with fluent configuration.
type MockSystemProvider struct {
	osDetector *MockOSDetector
	process    *MockProcessExecutor
	filesystem *MockFilesystemProvider
	network    *MockNetworkProvider
}

// NewMockSystemProvider creates a new mock system provider with sensible defaults.
func NewMockSystemProvider() *MockSystemProvider {
	return &MockSystemProvider{
		osDetector: NewMockOSDetector(),
		process:    NewMockProcessExecutor(),
		filesystem: NewMockFilesystemProvider(),
		network:    NewMockNetworkProvider(),
	}
}

// OS returns the mock OS detector.
func (m *MockSystemProvider) OS() platform.OSDetector {
	return m.osDetector
}

// Process returns the mock process executor.
func (m *MockSystemProvider) Process() platform.ProcessExecutor {
	return m.process
}

// Filesystem returns the mock filesystem provider.
func (m *MockSystemProvider) Filesystem() platform.FilesystemProvider {
	return m.filesystem
}

// Network returns the mock network provider.
func (m *MockSystemProvider) Network() platform.NetworkProvider {
	return m.network
}

// Fluent configuration methods

// WithOS configures the OS detector and returns the system provider for chaining.
func (m *MockSystemProvider) WithOS(name string) *MockSystemProvider {
	m.osDetector.WithName(name)
	return m
}

// WithUser configures the current user and returns the system provider for chaining.
func (m *MockSystemProvider) WithUser(username, uid, gid, homeDir string) *MockSystemProvider {
	m.osDetector.WithUser(username, uid, gid, homeDir)
	return m
}

// WithCommandResult configures a command result and returns the system provider for chaining.
func (m *MockSystemProvider) WithCommandResult(cmd string, result *platform.ExecuteResult) *MockSystemProvider {
	m.process.WithCommandResult(cmd, result)
	return m
}

// WithCommandError configures a command error and returns the system provider for chaining.
func (m *MockSystemProvider) WithCommandError(cmd string, err error) *MockSystemProvider {
	m.process.WithCommandError(cmd, err)
	return m
}

// WithFile configures a file in the mock filesystem and returns the system provider for chaining.
func (m *MockSystemProvider) WithFile(path string, content []byte) *MockSystemProvider {
	m.filesystem.WithFile(path, content)
	return m
}

// WithDirectory configures a directory in the mock filesystem and returns the system provider for chaining.
func (m *MockSystemProvider) WithDirectory(path string) *MockSystemProvider {
	m.filesystem.WithDirectory(path)
	return m
}

// WithEnvironmentVar configures an environment variable and returns the system provider for chaining.
func (m *MockSystemProvider) WithEnvironmentVar(key, value string) *MockSystemProvider {
	m.process.WithEnvironmentVar(key, value)
	return m
}

// MockOSDetector provides a mock implementation of OSDetector.
type MockOSDetector struct {
	name         string
	architecture string
	isRoot       bool
	userInfo     platform.UserInfo
	userError    error
}

// NewMockOSDetector creates a new mock OS detector with sensible defaults.
func NewMockOSDetector() *MockOSDetector {
	return &MockOSDetector{
		name:         "mac",
		architecture: "amd64",
		isRoot:       false,
		userInfo: platform.UserInfo{
			Username: "testuser",
			UID:      "1000",
			GID:      "1000",
			HomeDir:  "/Users/testuser",
		},
	}
}

// Name returns the configured OS name.
func (m *MockOSDetector) Name() string {
	return m.name
}

// Architecture returns the configured architecture.
func (m *MockOSDetector) Architecture() string {
	return m.architecture
}

// IsRoot returns the configured root status.
func (m *MockOSDetector) IsRoot() bool {
	return m.isRoot
}

// CurrentUser returns the configured user info or error.
func (m *MockOSDetector) CurrentUser() (platform.UserInfo, error) {
	return m.userInfo, m.userError
}

// Fluent configuration methods

// WithName sets the OS name and returns the detector for chaining.
func (m *MockOSDetector) WithName(name string) *MockOSDetector {
	m.name = name
	return m
}

// WithArchitecture sets the architecture and returns the detector for chaining.
func (m *MockOSDetector) WithArchitecture(arch string) *MockOSDetector {
	m.architecture = arch
	return m
}

// WithRoot sets the root status and returns the detector for chaining.
func (m *MockOSDetector) WithRoot(isRoot bool) *MockOSDetector {
	m.isRoot = isRoot
	return m
}

// WithUser sets the user information and returns the detector for chaining.
func (m *MockOSDetector) WithUser(username, uid, gid, homeDir string) *MockOSDetector {
	m.userInfo = platform.UserInfo{
		Username: username,
		UID:      uid,
		GID:      gid,
		HomeDir:  homeDir,
	}
	return m
}

// WithUserError sets a user error and returns the detector for chaining.
func (m *MockOSDetector) WithUserError(err error) *MockOSDetector {
	m.userError = err
	return m
}

// MockProcessExecutor provides a mock implementation of ProcessExecutor.
type MockProcessExecutor struct {
	commandResults    map[string]*platform.ExecuteResult
	commandErrors     map[string]error
	availableCommands map[string]bool
	environmentVars   map[string]string
}

// NewMockProcessExecutor creates a new mock process executor.
func NewMockProcessExecutor() *MockProcessExecutor {
	return &MockProcessExecutor{
		commandResults:    make(map[string]*platform.ExecuteResult),
		commandErrors:     make(map[string]error),
		availableCommands: make(map[string]bool),
		environmentVars:   make(map[string]string),
	}
}

// Execute executes a command and returns the configured result or error.
func (m *MockProcessExecutor) Execute(cmd string, options platform.ExecuteOptions) (*platform.ExecuteResult, error) {
	if err, exists := m.commandErrors[cmd]; exists {
		return nil, err
	}
	if result, exists := m.commandResults[cmd]; exists {
		return result, nil
	}
	// Default result for unconfigured commands
	return &platform.ExecuteResult{
		ExitCode: 0,
		Stdout:   "mock output",
		Stderr:   "",
		Duration: time.Millisecond,
		Success:  true,
	}, nil
}

// ExecuteWithContext executes a command with timeout and returns the configured result or error.
func (m *MockProcessExecutor) ExecuteWithContext(cmd string, options platform.ExecuteOptions, timeout time.Duration) (*platform.ExecuteResult, error) {
	return m.Execute(cmd, options) // For simplicity, ignore timeout in mock
}

// IsCommandAvailable returns whether a command is configured as available.
func (m *MockProcessExecutor) IsCommandAvailable(cmd string) bool {
	if available, exists := m.availableCommands[cmd]; exists {
		return available
	}
	return true // Default to available
}

// GetEnvironmentVar returns the configured environment variable value.
func (m *MockProcessExecutor) GetEnvironmentVar(key string) string {
	return m.environmentVars[key]
}

// SetEnvironmentVar sets an environment variable in the mock.
func (m *MockProcessExecutor) SetEnvironmentVar(key, value string) error {
	m.environmentVars[key] = value
	return nil
}

// Fluent configuration methods

// WithCommandResult configures a command result and returns the executor for chaining.
func (m *MockProcessExecutor) WithCommandResult(cmd string, result *platform.ExecuteResult) *MockProcessExecutor {
	m.commandResults[cmd] = result
	return m
}

// WithCommandError configures a command error and returns the executor for chaining.
func (m *MockProcessExecutor) WithCommandError(cmd string, err error) *MockProcessExecutor {
	m.commandErrors[cmd] = err
	return m
}

// WithCommandAvailable configures command availability and returns the executor for chaining.
func (m *MockProcessExecutor) WithCommandAvailable(cmd string, available bool) *MockProcessExecutor {
	m.availableCommands[cmd] = available
	return m
}

// WithEnvironmentVar configures an environment variable and returns the executor for chaining.
func (m *MockProcessExecutor) WithEnvironmentVar(key, value string) *MockProcessExecutor {
	m.environmentVars[key] = value
	return m
}

// MockFilesystemProvider provides an in-memory filesystem for testing.
type MockFilesystemProvider struct {
	files       map[string][]byte
	directories map[string]bool
	permissions map[string]os.FileMode
	symlinks    map[string]string
	fileInfo    map[string]platform.FileInfo
}

// NewMockFilesystemProvider creates a new mock filesystem provider.
func NewMockFilesystemProvider() *MockFilesystemProvider {
	return &MockFilesystemProvider{
		files:       make(map[string][]byte),
		directories: make(map[string]bool),
		permissions: make(map[string]os.FileMode),
		symlinks:    make(map[string]string),
		fileInfo:    make(map[string]platform.FileInfo),
	}
}

// Exists checks if a file or directory exists in the mock filesystem.
func (m *MockFilesystemProvider) Exists(path string) bool {
	_, fileExists := m.files[path]
	_, dirExists := m.directories[path]
	return fileExists || dirExists
}

// IsDirectory checks if path is a directory in the mock filesystem.
func (m *MockFilesystemProvider) IsDirectory(path string) bool {
	return m.directories[path]
}

// IsFile checks if path is a file in the mock filesystem.
func (m *MockFilesystemProvider) IsFile(path string) bool {
	_, exists := m.files[path]
	return exists
}

// ReadFile reads file contents from the mock filesystem.
func (m *MockFilesystemProvider) ReadFile(path string) ([]byte, error) {
	if content, exists := m.files[path]; exists {
		return content, nil
	}
	return nil, os.ErrNotExist
}

// WriteFile writes data to the mock filesystem.
func (m *MockFilesystemProvider) WriteFile(path string, data []byte, perm os.FileMode) error {
	m.files[path] = data
	m.permissions[path] = perm
	return nil
}

// AppendToFile appends data to a file in the mock filesystem.
func (m *MockFilesystemProvider) AppendToFile(path string, data []byte) error {
	existing := m.files[path]
	m.files[path] = append(existing, data...)
	return nil
}

// CreateDirectory creates a directory in the mock filesystem.
func (m *MockFilesystemProvider) CreateDirectory(path string, perm os.FileMode) error {
	m.directories[path] = true
	m.permissions[path] = perm
	return nil
}

// RemoveFile removes a file from the mock filesystem.
func (m *MockFilesystemProvider) RemoveFile(path string) error {
	delete(m.files, path)
	delete(m.permissions, path)
	return nil
}

// RemoveDirectory removes a directory from the mock filesystem.
func (m *MockFilesystemProvider) RemoveDirectory(path string) error {
	delete(m.directories, path)
	delete(m.permissions, path)
	return nil
}

// CopyFile copies a file in the mock filesystem.
func (m *MockFilesystemProvider) CopyFile(src, dst string) error {
	if content, exists := m.files[src]; exists {
		m.files[dst] = make([]byte, len(content))
		copy(m.files[dst], content)
		if perm, exists := m.permissions[src]; exists {
			m.permissions[dst] = perm
		}
		return nil
	}
	return os.ErrNotExist
}

// MoveFile moves a file in the mock filesystem.
func (m *MockFilesystemProvider) MoveFile(src, dst string) error {
	if err := m.CopyFile(src, dst); err != nil {
		return err
	}
	return m.RemoveFile(src)
}

// ListDirectory lists files in the mock filesystem.
func (m *MockFilesystemProvider) ListDirectory(path string) ([]platform.FileInfo, error) {
	if !m.directories[path] {
		return nil, os.ErrNotExist
	}
	var infos []platform.FileInfo
	// Add configured file info for this directory
	for _, info := range m.fileInfo {
		if info.Path == path {
			infos = append(infos, info)
		}
	}
	return infos, nil
}

// GetPermissions returns file permissions from the mock filesystem.
func (m *MockFilesystemProvider) GetPermissions(path string) (os.FileMode, error) {
	if perm, exists := m.permissions[path]; exists {
		return perm, nil
	}
	return 0, os.ErrNotExist
}

// SetPermissions sets file permissions in the mock filesystem.
func (m *MockFilesystemProvider) SetPermissions(path string, perm os.FileMode) error {
	if !m.Exists(path) {
		return os.ErrNotExist
	}
	m.permissions[path] = perm
	return nil
}

// CreateSymlink creates a symbolic link in the mock filesystem.
func (m *MockFilesystemProvider) CreateSymlink(oldPath, newPath string) error {
	m.symlinks[newPath] = oldPath
	return nil
}

// ReadSymlink reads a symbolic link target from the mock filesystem.
func (m *MockFilesystemProvider) ReadSymlink(path string) (string, error) {
	if target, exists := m.symlinks[path]; exists {
		return target, nil
	}
	return "", os.ErrNotExist
}

// GetFileInfo returns file information from the mock filesystem.
func (m *MockFilesystemProvider) GetFileInfo(path string) (platform.FileInfo, error) {
	if info, exists := m.fileInfo[path]; exists {
		return info, nil
	}
	return platform.FileInfo{}, os.ErrNotExist
}

// TempDirectory creates a temporary directory path (doesn't actually create it).
func (m *MockFilesystemProvider) TempDirectory(prefix string) (string, error) {
	return "/tmp/" + prefix + "test", nil
}

// ExpandPath expands paths (simple implementation for testing).
func (m *MockFilesystemProvider) ExpandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		return "/Users/testuser" + path[1:]
	}
	return path
}

// OpenFile opens a file (returns a mock reader/writer).
func (m *MockFilesystemProvider) OpenFile(path string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	return &mockFile{data: m.files[path]}, nil
}

// Fluent configuration methods

// WithFile configures a file in the mock filesystem and returns the provider for chaining.
func (m *MockFilesystemProvider) WithFile(path string, content []byte) *MockFilesystemProvider {
	m.files[path] = content
	return m
}

// WithDirectory configures a directory in the mock filesystem and returns the provider for chaining.
func (m *MockFilesystemProvider) WithDirectory(path string) *MockFilesystemProvider {
	m.directories[path] = true
	return m
}

// WithPermissions configures file permissions and returns the provider for chaining.
func (m *MockFilesystemProvider) WithPermissions(path string, perm os.FileMode) *MockFilesystemProvider {
	m.permissions[path] = perm
	return m
}

// WithSymlink configures a symbolic link and returns the provider for chaining.
func (m *MockFilesystemProvider) WithSymlink(linkPath, targetPath string) *MockFilesystemProvider {
	m.symlinks[linkPath] = targetPath
	return m
}

// WithFileInfo configures file information and returns the provider for chaining.
func (m *MockFilesystemProvider) WithFileInfo(path string, info platform.FileInfo) *MockFilesystemProvider {
	m.fileInfo[path] = info
	return m
}

// mockFile provides a simple implementation of io.ReadWriteCloser for testing.
type mockFile struct {
	data []byte
}

func (f *mockFile) Read(p []byte) (n int, err error) {
	n = copy(p, f.data)
	return n, nil
}

func (f *mockFile) Write(p []byte) (n int, err error) {
	f.data = append(f.data, p...)
	return len(p), nil
}

func (f *mockFile) Close() error {
	return nil
}

// MockNetworkProvider provides a mock implementation of NetworkProvider.
type MockNetworkProvider struct {
	httpClient   *MockHTTPClient
	urlContents  map[string][]byte
	urlErrors    map[string]error
	reachability map[string]bool
}

// NewMockNetworkProvider creates a new mock network provider.
func NewMockNetworkProvider() *MockNetworkProvider {
	return &MockNetworkProvider{
		httpClient:   NewMockHTTPClient(),
		urlContents:  make(map[string][]byte),
		urlErrors:    make(map[string]error),
		reachability: make(map[string]bool),
	}
}

// HTTPClient returns the mock HTTP client.
func (m *MockNetworkProvider) HTTPClient() platform.HTTPClient {
	return m.httpClient
}

// DownloadFile simulates downloading a file.
func (m *MockNetworkProvider) DownloadFile(url, path string) error {
	if err, exists := m.urlErrors[url]; exists {
		return err
	}
	// In a real mock, you might write to the filesystem provider
	return nil
}

// GetURLContent returns the configured content for a URL.
func (m *MockNetworkProvider) GetURLContent(url string) ([]byte, error) {
	if err, exists := m.urlErrors[url]; exists {
		return nil, err
	}
	if content, exists := m.urlContents[url]; exists {
		return content, nil
	}
	return []byte("mock content"), nil
}

// IsReachable returns the configured reachability for a host.
func (m *MockNetworkProvider) IsReachable(host string, port int, timeout time.Duration) bool {
	key := host
	if reachable, exists := m.reachability[key]; exists {
		return reachable
	}
	return true // Default to reachable
}

// Fluent configuration methods

// WithURLContent configures content for a URL and returns the provider for chaining.
func (m *MockNetworkProvider) WithURLContent(url string, content []byte) *MockNetworkProvider {
	m.urlContents[url] = content
	return m
}

// WithURLError configures an error for a URL and returns the provider for chaining.
func (m *MockNetworkProvider) WithURLError(url string, err error) *MockNetworkProvider {
	m.urlErrors[url] = err
	return m
}

// WithReachability configures host reachability and returns the provider for chaining.
func (m *MockNetworkProvider) WithReachability(host string, reachable bool) *MockNetworkProvider {
	m.reachability[host] = reachable
	return m
}

// MockHTTPClient provides a mock HTTP client.
type MockHTTPClient struct {
	responses map[string]*http.Response
	errors    map[string]error
}

// NewMockHTTPClient creates a new mock HTTP client.
func NewMockHTTPClient() *MockHTTPClient {
	return &MockHTTPClient{
		responses: make(map[string]*http.Response),
		errors:    make(map[string]error),
	}
}

// Get performs a mock HTTP GET.
func (m *MockHTTPClient) Get(url string) (*http.Response, error) {
	if err, exists := m.errors[url]; exists {
		return nil, err
	}
	if resp, exists := m.responses[url]; exists {
		return resp, nil
	}
	// Default response
	return &http.Response{StatusCode: 200}, nil
}

// Post performs a mock HTTP POST.
func (m *MockHTTPClient) Post(url, contentType string, body io.Reader) (*http.Response, error) {
	return m.Get(url) // Simplified for mock
}

// Do performs a mock HTTP request.
func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.Get(req.URL.String())
}

// WithResponse configures a response for a URL.
func (m *MockHTTPClient) WithResponse(url string, resp *http.Response) *MockHTTPClient {
	m.responses[url] = resp
	return m
}

// WithError configures an error for a URL.
func (m *MockHTTPClient) WithError(url string, err error) *MockHTTPClient {
	m.errors[url] = err
	return m
}
