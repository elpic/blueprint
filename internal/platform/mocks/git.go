package mocks

import (
	"os"

	"github.com/elpic/blueprint/internal/platform"
)

// MockGitProvider provides a mock implementation of GitProvider with fluent configuration.
type MockGitProvider struct {
	cloneResults    map[string]*platform.GitResult
	cloneErrors     map[string]error
	updateResults   map[string]*platform.GitResult
	updateErrors    map[string]error
	localSHAs       map[string]string
	localSHAErrors  map[string]error
	remoteSHAs      map[string]string
	remoteSHAErrors map[string]error
	repositories    map[string]bool
}

// NewMockGitProvider creates a new mock Git provider.
func NewMockGitProvider() *MockGitProvider {
	return &MockGitProvider{
		cloneResults:    make(map[string]*platform.GitResult),
		cloneErrors:     make(map[string]error),
		updateResults:   make(map[string]*platform.GitResult),
		updateErrors:    make(map[string]error),
		localSHAs:       make(map[string]string),
		localSHAErrors:  make(map[string]error),
		remoteSHAs:      make(map[string]string),
		remoteSHAErrors: make(map[string]error),
		repositories:    make(map[string]bool),
	}
}

// Clone performs a mock git clone operation.
func (m *MockGitProvider) Clone(url, path, branch string) (*platform.GitResult, error) {
	key := url + "|" + path + "|" + branch
	if err, exists := m.cloneErrors[key]; exists {
		return nil, err
	}
	if result, exists := m.cloneResults[key]; exists {
		return result, nil
	}
	// Default result
	return &platform.GitResult{
		Status:  "Cloned",
		OldSHA:  "",
		NewSHA:  "abc123def456",
		Message: "Successfully cloned repository",
	}, nil
}

// Update performs a mock git update operation.
func (m *MockGitProvider) Update(path, branch string) (*platform.GitResult, error) {
	key := path + "|" + branch
	if err, exists := m.updateErrors[key]; exists {
		return nil, err
	}
	if result, exists := m.updateResults[key]; exists {
		return result, nil
	}
	// Default result
	return &platform.GitResult{
		Status:  "Already up to date",
		OldSHA:  "abc123def456",
		NewSHA:  "abc123def456",
		Message: "Repository is up to date",
	}, nil
}

// GetLocalSHA returns the configured local SHA for a repository.
func (m *MockGitProvider) GetLocalSHA(path string) (string, error) {
	if err, exists := m.localSHAErrors[path]; exists {
		return "", err
	}
	if sha, exists := m.localSHAs[path]; exists {
		return sha, nil
	}
	return "abc123def456", nil // Default SHA
}

// GetRemoteHeadSHA returns the configured remote HEAD SHA for a repository.
func (m *MockGitProvider) GetRemoteHeadSHA(url, branch string) (string, error) {
	key := url + "|" + branch
	if err, exists := m.remoteSHAErrors[key]; exists {
		return "", err
	}
	if sha, exists := m.remoteSHAs[key]; exists {
		return sha, nil
	}
	return "def456abc123", nil // Default SHA (different from local)
}

// IsRepository returns whether a path is configured as a repository.
func (m *MockGitProvider) IsRepository(path string) bool {
	if isRepo, exists := m.repositories[path]; exists {
		return isRepo
	}
	return false // Default to not a repository
}

// Fluent configuration methods

// WithCloneResult configures a clone result and returns the provider for chaining.
func (m *MockGitProvider) WithCloneResult(url, path, branch string, result *platform.GitResult) *MockGitProvider {
	key := url + "|" + path + "|" + branch
	m.cloneResults[key] = result
	return m
}

// WithCloneError configures a clone error and returns the provider for chaining.
func (m *MockGitProvider) WithCloneError(url, path, branch string, err error) *MockGitProvider {
	key := url + "|" + path + "|" + branch
	m.cloneErrors[key] = err
	return m
}

// WithUpdateResult configures an update result and returns the provider for chaining.
func (m *MockGitProvider) WithUpdateResult(path, branch string, result *platform.GitResult) *MockGitProvider {
	key := path + "|" + branch
	m.updateResults[key] = result
	return m
}

// WithUpdateError configures an update error and returns the provider for chaining.
func (m *MockGitProvider) WithUpdateError(path, branch string, err error) *MockGitProvider {
	key := path + "|" + branch
	m.updateErrors[key] = err
	return m
}

// WithLocalSHA configures a local SHA and returns the provider for chaining.
func (m *MockGitProvider) WithLocalSHA(path, sha string) *MockGitProvider {
	m.localSHAs[path] = sha
	return m
}

// WithLocalSHAError configures a local SHA error and returns the provider for chaining.
func (m *MockGitProvider) WithLocalSHAError(path string, err error) *MockGitProvider {
	m.localSHAErrors[path] = err
	return m
}

// WithRemoteSHA configures a remote SHA and returns the provider for chaining.
func (m *MockGitProvider) WithRemoteSHA(url, branch, sha string) *MockGitProvider {
	key := url + "|" + branch
	m.remoteSHAs[key] = sha
	return m
}

// WithRemoteSHAError configures a remote SHA error and returns the provider for chaining.
func (m *MockGitProvider) WithRemoteSHAError(url, branch string, err error) *MockGitProvider {
	key := url + "|" + branch
	m.remoteSHAErrors[key] = err
	return m
}

// WithRepository configures whether a path is a repository and returns the provider for chaining.
func (m *MockGitProvider) WithRepository(path string, isRepo bool) *MockGitProvider {
	m.repositories[path] = isRepo
	return m
}

// Common test scenarios as convenience methods

// WithSuccessfulClone configures a successful clone operation.
func (m *MockGitProvider) WithSuccessfulClone(url, path, branch, sha string) *MockGitProvider {
	return m.WithCloneResult(url, path, branch, &platform.GitResult{
		Status:  "Cloned",
		OldSHA:  "",
		NewSHA:  sha,
		Message: "Successfully cloned repository",
	}).WithRepository(path, true).WithLocalSHA(path, sha)
}

// WithSuccessfulUpdate configures a successful update operation.
func (m *MockGitProvider) WithSuccessfulUpdate(path, branch, oldSHA, newSHA string) *MockGitProvider {
	return m.WithUpdateResult(path, branch, &platform.GitResult{
		Status:  "Updated",
		OldSHA:  oldSHA,
		NewSHA:  newSHA,
		Message: "Successfully updated repository",
	}).WithLocalSHA(path, newSHA)
}

// WithUpToDateRepository configures an already up-to-date repository.
func (m *MockGitProvider) WithUpToDateRepository(path, branch, sha string) *MockGitProvider {
	return m.WithUpdateResult(path, branch, &platform.GitResult{
		Status:  "Already up to date",
		OldSHA:  sha,
		NewSHA:  sha,
		Message: "Repository is already up to date",
	}).WithLocalSHA(path, sha).WithRepository(path, true)
}

// WithUnreachableRemote configures an unreachable remote (network error).
func (m *MockGitProvider) WithUnreachableRemote(url, branch string) *MockGitProvider {
	return m.WithRemoteSHAError(url, branch, os.ErrNotExist)
}
