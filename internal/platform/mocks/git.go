package mocks

import (
	"os"

	"github.com/elpic/blueprint/internal/platform"
)

// cloneKey identifies a configured clone outcome. Mode+URL+branch — the path
// is where the clone lands, not what it clones, so it is not part of the key.
type cloneKey struct {
	mode   platform.CloneMode
	url    string
	branch string
}

func keyFor(spec platform.CloneSpec) cloneKey {
	return cloneKey{mode: spec.Mode, url: spec.URL, branch: spec.Branch}
}

// MockGitProvider provides a mock implementation of GitProvider with fluent
// configuration. Clone outcomes are keyed on the CloneSpec (mode+URL+branch);
// every other method is keyed on its natural identity (path, RepoID,
// path+branch). Not thread-safe — tests run serially.
//
// Unconfigured calls fall back to defaults in the spirit of the original
// mock: Clone succeeds with StatusCloned, LocalSHA/RemoteSHA return
// well-known fixture SHAs, Layout reports LayoutNone.
type MockGitProvider struct {
	cloneResults map[cloneKey]platform.CloneResult
	cloneErrors  map[cloneKey]error

	checkoutErrors map[platform.CheckoutSpec]error

	localSHAs      map[platform.RepoPath]platform.SHA
	localSHAErrors map[platform.RepoPath]error

	remoteSHAs      map[platform.RepoID]platform.SHA
	remoteSHAErrors map[platform.RepoID]error

	bareSHAs      map[cloneKey]platform.SHA
	bareSHAErrors map[cloneKey]error

	storageSHAs      map[platform.RepoID]platform.SHA
	storageSHAErrors map[platform.RepoID]error

	layouts map[platform.RepoPath]platform.RepoLayout
}

// NewMockGitProvider creates a new mock Git provider.
func NewMockGitProvider() *MockGitProvider {
	return &MockGitProvider{
		cloneResults:     make(map[cloneKey]platform.CloneResult),
		cloneErrors:      make(map[cloneKey]error),
		checkoutErrors:   make(map[platform.CheckoutSpec]error),
		localSHAs:        make(map[platform.RepoPath]platform.SHA),
		localSHAErrors:   make(map[platform.RepoPath]error),
		remoteSHAs:       make(map[platform.RepoID]platform.SHA),
		remoteSHAErrors:  make(map[platform.RepoID]error),
		bareSHAs:         make(map[cloneKey]platform.SHA),
		bareSHAErrors:    make(map[cloneKey]error),
		storageSHAs:      make(map[platform.RepoID]platform.SHA),
		storageSHAErrors: make(map[platform.RepoID]error),
		layouts:          make(map[platform.RepoPath]platform.RepoLayout),
	}
}

// Clone returns the configured outcome for spec (mode+URL+branch), or a
// default successful clone.
func (m *MockGitProvider) Clone(spec platform.CloneSpec) (platform.CloneResult, error) {
	key := keyFor(spec)
	if err, exists := m.cloneErrors[key]; exists {
		return platform.CloneResult{}, err
	}
	if result, exists := m.cloneResults[key]; exists {
		return result, nil
	}
	return platform.CloneResult{
		Status: platform.StatusCloned,
		NewSHA: "abc123def456",
	}, nil
}

// Checkout returns the configured error for spec, or nil.
func (m *MockGitProvider) Checkout(spec platform.CheckoutSpec) error {
	return m.checkoutErrors[spec]
}

// LocalSHA returns the configured local SHA for a repository.
func (m *MockGitProvider) LocalSHA(path platform.RepoPath) (platform.SHA, error) {
	if err, exists := m.localSHAErrors[path]; exists {
		return "", err
	}
	if sha, exists := m.localSHAs[path]; exists {
		return sha, nil
	}
	return "abc123def456", nil // Default SHA
}

// RemoteSHA returns the configured remote SHA for a repository.
func (m *MockGitProvider) RemoteSHA(id platform.RepoID) (platform.SHA, error) {
	if err, exists := m.remoteSHAErrors[id]; exists {
		return "", err
	}
	if sha, exists := m.remoteSHAs[id]; exists {
		return sha, nil
	}
	return "def456abc123", nil // Default SHA (different from local)
}

// BareSHA returns the configured bare-layout SHA for a path+branch.
func (m *MockGitProvider) BareSHA(path platform.RepoPath, branch string) (platform.SHA, error) {
	key := cloneKey{url: string(path), branch: branch}
	if err, exists := m.bareSHAErrors[key]; exists {
		return "", err
	}
	if sha, exists := m.bareSHAs[key]; exists {
		return sha, nil
	}
	return "", nil // Default: unknown bare SHA
}

// StorageSHA returns the configured clean-storage SHA for a repository.
func (m *MockGitProvider) StorageSHA(id platform.RepoID) (platform.SHA, error) {
	if err, exists := m.storageSHAErrors[id]; exists {
		return "", err
	}
	if sha, exists := m.storageSHAs[id]; exists {
		return sha, nil
	}
	return "", nil // Default: no clean storage
}

// Layout returns the configured layout for a path.
func (m *MockGitProvider) Layout(path platform.RepoPath) platform.RepoLayout {
	if layout, exists := m.layouts[path]; exists {
		return layout
	}
	return platform.LayoutNone // Default: not a repository
}

// Fluent configuration methods

// WithCloneResult configures a clone outcome for every spec with the same
// mode+URL+branch and returns the provider for chaining.
func (m *MockGitProvider) WithCloneResult(spec platform.CloneSpec, result platform.CloneResult) *MockGitProvider {
	m.cloneResults[keyFor(spec)] = result
	return m
}

// WithCloneError configures a clone error and returns the provider for chaining.
func (m *MockGitProvider) WithCloneError(spec platform.CloneSpec, err error) *MockGitProvider {
	m.cloneErrors[keyFor(spec)] = err
	return m
}

// WithCheckoutError configures a checkout error and returns the provider for chaining.
func (m *MockGitProvider) WithCheckoutError(spec platform.CheckoutSpec, err error) *MockGitProvider {
	m.checkoutErrors[spec] = err
	return m
}

// WithLocalSHA configures a local SHA and returns the provider for chaining.
func (m *MockGitProvider) WithLocalSHA(path platform.RepoPath, sha platform.SHA) *MockGitProvider {
	m.localSHAs[path] = sha
	return m
}

// WithLocalSHAError configures a local SHA error and returns the provider for chaining.
func (m *MockGitProvider) WithLocalSHAError(path platform.RepoPath, err error) *MockGitProvider {
	m.localSHAErrors[path] = err
	return m
}

// WithRemoteSHA configures a remote SHA and returns the provider for chaining.
func (m *MockGitProvider) WithRemoteSHA(id platform.RepoID, sha platform.SHA) *MockGitProvider {
	m.remoteSHAs[id] = sha
	return m
}

// WithRemoteSHAError configures a remote SHA error and returns the provider for chaining.
func (m *MockGitProvider) WithRemoteSHAError(id platform.RepoID, err error) *MockGitProvider {
	m.remoteSHAErrors[id] = err
	return m
}

// WithBareSHA configures a bare-layout SHA for a path+branch and returns the
// provider for chaining.
func (m *MockGitProvider) WithBareSHA(path platform.RepoPath, branch string, sha platform.SHA) *MockGitProvider {
	m.bareSHAs[cloneKey{url: string(path), branch: branch}] = sha
	return m
}

// WithBareSHAError configures a bare-layout SHA error and returns the provider for chaining.
func (m *MockGitProvider) WithBareSHAError(path platform.RepoPath, branch string, err error) *MockGitProvider {
	m.bareSHAErrors[cloneKey{url: string(path), branch: branch}] = err
	return m
}

// WithStorageSHA configures a clean-storage SHA and returns the provider for chaining.
func (m *MockGitProvider) WithStorageSHA(id platform.RepoID, sha platform.SHA) *MockGitProvider {
	m.storageSHAs[id] = sha
	return m
}

// WithStorageSHAError configures a clean-storage SHA error and returns the provider for chaining.
func (m *MockGitProvider) WithStorageSHAError(id platform.RepoID, err error) *MockGitProvider {
	m.storageSHAErrors[id] = err
	return m
}

// WithLayout configures the layout reported for a path and returns the
// provider for chaining.
func (m *MockGitProvider) WithLayout(path platform.RepoPath, layout platform.RepoLayout) *MockGitProvider {
	m.layouts[path] = layout
	return m
}

// Common test scenarios as convenience methods

// WithSuccessfulClone configures a successful clone of spec landing at
// spec.Path with the given SHA, recording the on-disk reality each mode
// produces: Direct leaves a working copy (standard layout, readable local
// SHA), Bare leaves <path>/.git plus worktrees (bare layout, readable bare
// SHA), TwoStage leaves a content mirror (no .git; the SHA lives in clean
// storage).
func (m *MockGitProvider) WithSuccessfulClone(spec platform.CloneSpec, sha platform.SHA) *MockGitProvider {
	m.WithCloneResult(spec, platform.CloneResult{
		Status: platform.StatusCloned,
		NewSHA: sha,
	})
	switch spec.Mode {
	case platform.ModeDirect:
		m.WithLayout(platform.RepoPath(spec.Path), platform.LayoutStandard)
		m.WithLocalSHA(platform.RepoPath(spec.Path), sha)
	case platform.ModeBare:
		m.WithLayout(platform.RepoPath(spec.Path), platform.LayoutBare)
		m.WithBareSHA(platform.RepoPath(spec.Path), spec.Branch, sha)
	default: // ModeTwoStage: mirror, no .git in the target
		m.WithLayout(platform.RepoPath(spec.Path), platform.LayoutNone)
		m.WithStorageSHA(platform.RepoID{URL: spec.URL, Branch: spec.Branch}, sha)
	}
	return m
}

// WithSuccessfulUpdate configures an update of spec from oldSHA to newSHA,
// refreshing the per-mode SHA view the way a real update would.
func (m *MockGitProvider) WithSuccessfulUpdate(spec platform.CloneSpec, oldSHA, newSHA platform.SHA) *MockGitProvider {
	m.WithCloneResult(spec, platform.CloneResult{
		Status: platform.StatusUpdated,
		OldSHA: oldSHA,
		NewSHA: newSHA,
	})
	switch spec.Mode {
	case platform.ModeDirect:
		m.WithLayout(platform.RepoPath(spec.Path), platform.LayoutStandard)
		m.WithLocalSHA(platform.RepoPath(spec.Path), newSHA)
	case platform.ModeBare:
		m.WithLayout(platform.RepoPath(spec.Path), platform.LayoutBare)
		m.WithBareSHA(platform.RepoPath(spec.Path), spec.Branch, newSHA)
	default: // ModeTwoStage
		m.WithLayout(platform.RepoPath(spec.Path), platform.LayoutNone)
		m.WithStorageSHA(platform.RepoID{URL: spec.URL, Branch: spec.Branch}, newSHA)
	}
	return m
}

// WithUpToDateRepository configures spec as already up to date at sha,
// refreshing the per-mode SHA view.
func (m *MockGitProvider) WithUpToDateRepository(spec platform.CloneSpec, sha platform.SHA) *MockGitProvider {
	m.WithCloneResult(spec, platform.CloneResult{
		Status: platform.StatusUpToDate,
		OldSHA: sha,
		NewSHA: sha,
	})
	switch spec.Mode {
	case platform.ModeDirect:
		m.WithLayout(platform.RepoPath(spec.Path), platform.LayoutStandard)
		m.WithLocalSHA(platform.RepoPath(spec.Path), sha)
	case platform.ModeBare:
		m.WithLayout(platform.RepoPath(spec.Path), platform.LayoutBare)
		m.WithBareSHA(platform.RepoPath(spec.Path), spec.Branch, sha)
	default: // ModeTwoStage
		m.WithLayout(platform.RepoPath(spec.Path), platform.LayoutNone)
		m.WithStorageSHA(platform.RepoID{URL: spec.URL, Branch: spec.Branch}, sha)
	}
	return m
}

// WithUnreachableRemote configures an unreachable remote (network error).
func (m *MockGitProvider) WithUnreachableRemote(id platform.RepoID) *MockGitProvider {
	return m.WithRemoteSHAError(id, os.ErrNotExist)
}
