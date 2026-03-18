package platform

import (
	"io"
	"os"
	"sync"
	"time"
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
		return &realSystemProvider{}
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

// Placeholder implementations for real providers.
// These should be implemented in separate files (real_*.go) to contain
// the actual platform-specific implementations.

type realSystemProvider struct{}

func (p *realSystemProvider) OS() OSDetector                 { return &realOSDetector{} }
func (p *realSystemProvider) Process() ProcessExecutor       { return &realProcessExecutor{} }
func (p *realSystemProvider) Filesystem() FilesystemProvider { return &realFilesystemProvider{} }
func (p *realSystemProvider) Network() NetworkProvider       { return &realNetworkProvider{} }

type realOSDetector struct{}

func (d *realOSDetector) Name() string                   { panic("not implemented") }
func (d *realOSDetector) Architecture() string           { panic("not implemented") }
func (d *realOSDetector) IsRoot() bool                   { panic("not implemented") }
func (d *realOSDetector) CurrentUser() (UserInfo, error) { panic("not implemented") }

type realProcessExecutor struct{}

func (e *realProcessExecutor) Execute(cmd string, options ExecuteOptions) (*ExecuteResult, error) {
	panic("not implemented")
}
func (e *realProcessExecutor) ExecuteWithContext(cmd string, options ExecuteOptions, timeout time.Duration) (*ExecuteResult, error) {
	panic("not implemented")
}
func (e *realProcessExecutor) IsCommandAvailable(cmd string) bool        { panic("not implemented") }
func (e *realProcessExecutor) GetEnvironmentVar(key string) string       { panic("not implemented") }
func (e *realProcessExecutor) SetEnvironmentVar(key, value string) error { panic("not implemented") }

type realFilesystemProvider struct{}

func (f *realFilesystemProvider) Exists(path string) bool              { panic("not implemented") }
func (f *realFilesystemProvider) IsDirectory(path string) bool         { panic("not implemented") }
func (f *realFilesystemProvider) IsFile(path string) bool              { panic("not implemented") }
func (f *realFilesystemProvider) ReadFile(path string) ([]byte, error) { panic("not implemented") }
func (f *realFilesystemProvider) WriteFile(path string, data []byte, perm os.FileMode) error {
	panic("not implemented")
}
func (f *realFilesystemProvider) AppendToFile(path string, data []byte) error {
	panic("not implemented")
}
func (f *realFilesystemProvider) CreateDirectory(path string, perm os.FileMode) error {
	panic("not implemented")
}
func (f *realFilesystemProvider) RemoveFile(path string) error      { panic("not implemented") }
func (f *realFilesystemProvider) RemoveDirectory(path string) error { panic("not implemented") }
func (f *realFilesystemProvider) CopyFile(src, dst string) error    { panic("not implemented") }
func (f *realFilesystemProvider) MoveFile(src, dst string) error    { panic("not implemented") }
func (f *realFilesystemProvider) ListDirectory(path string) ([]FileInfo, error) {
	panic("not implemented")
}
func (f *realFilesystemProvider) GetPermissions(path string) (os.FileMode, error) {
	panic("not implemented")
}
func (f *realFilesystemProvider) SetPermissions(path string, perm os.FileMode) error {
	panic("not implemented")
}
func (f *realFilesystemProvider) CreateSymlink(oldPath, newPath string) error {
	panic("not implemented")
}
func (f *realFilesystemProvider) ReadSymlink(path string) (string, error)   { panic("not implemented") }
func (f *realFilesystemProvider) GetFileInfo(path string) (FileInfo, error) { panic("not implemented") }
func (f *realFilesystemProvider) TempDirectory(prefix string) (string, error) {
	panic("not implemented")
}
func (f *realFilesystemProvider) ExpandPath(path string) string { panic("not implemented") }
func (f *realFilesystemProvider) OpenFile(path string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	panic("not implemented")
}

type realNetworkProvider struct{}

func (n *realNetworkProvider) HTTPClient() HTTPClient                   { panic("not implemented") }
func (n *realNetworkProvider) DownloadFile(url, path string) error      { panic("not implemented") }
func (n *realNetworkProvider) GetURLContent(url string) ([]byte, error) { panic("not implemented") }
func (n *realNetworkProvider) IsReachable(host string, port int, timeout time.Duration) bool {
	panic("not implemented")
}

type realGitProvider struct{}

func (g *realGitProvider) Clone(url, path, branch string) (*GitResult, error) {
	panic("not implemented")
}
func (g *realGitProvider) Update(path, branch string) (*GitResult, error) { panic("not implemented") }
func (g *realGitProvider) GetLocalSHA(path string) (string, error)        { panic("not implemented") }
func (g *realGitProvider) GetRemoteHeadSHA(url, branch string) (string, error) {
	panic("not implemented")
}
func (g *realGitProvider) IsRepository(path string) bool { panic("not implemented") }

type realCryptoProvider struct{}

func (c *realCryptoProvider) Decrypt(inputPath, outputPath, password string) error {
	panic("not implemented")
}
func (c *realCryptoProvider) Encrypt(inputPath, outputPath, password string) error {
	panic("not implemented")
}
func (c *realCryptoProvider) IsEncrypted(path string) bool { panic("not implemented") }

type realPackageManagerProvider struct{}

func (p *realPackageManagerProvider) Install(packages []string, manager string) (*ExecuteResult, error) {
	panic("not implemented")
}
func (p *realPackageManagerProvider) Uninstall(packages []string, manager string) (*ExecuteResult, error) {
	panic("not implemented")
}
func (p *realPackageManagerProvider) IsInstalled(packageName, manager string) bool {
	panic("not implemented")
}
func (p *realPackageManagerProvider) GetInstalledVersion(packageName, manager string) (string, error) {
	panic("not implemented")
}
func (p *realPackageManagerProvider) IsManagerAvailable(manager string) bool {
	panic("not implemented")
}
func (p *realPackageManagerProvider) GetDefaultManager() string { panic("not implemented") }
