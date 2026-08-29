package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/elpic/blueprint/internal/platform"
	"github.com/elpic/blueprint/internal/platform/mocks"
)

// recordingGitProvider wraps a GitProvider and records every Clone spec it
// receives, so tests can assert exactly what the parser asked the seam to do.
type recordingGitProvider struct {
	platform.GitProvider
	cloneSpecs []platform.CloneSpec
}

func (r *recordingGitProvider) Clone(spec platform.CloneSpec) (platform.CloneResult, error) {
	r.cloneSpecs = append(r.cloneSpecs, spec)
	return r.GitProvider.Clone(spec)
}

// TestLoadGitIncludeUsesGitSeam pins the seam adoption of git includes —
// the engine call that previously hid behind the bare `git` import alias.
// A git include resolves through one Direct-mode Clone spec and the rules are
// parsed from the cached working copy.
func TestLoadGitIncludeUsesGitSeam(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// localPathForGitInclude("https://github.com/org/repo.git@main:infra/setup.bp")
	// → ~/.blueprint/repos/github.com/org/repo
	cacheDir := filepath.Join(home, ".blueprint", "repos", "github.com", "org", "repo")
	if err := os.MkdirAll(filepath.Join(cacheDir, "infra"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "infra", "setup.bp"), []byte("install git\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	wantSpec := platform.CloneSpec{
		URL:    "https://github.com/org/repo.git",
		Path:   cacheDir,
		Branch: "main",
		Mode:   platform.ModeDirect,
	}
	recording := &recordingGitProvider{GitProvider: mocks.NewMockGitProvider()}
	origGit := Git
	Git = recording
	t.Cleanup(func() { Git = origGit })

	rules, err := loadGitInclude("https://github.com/org/repo.git@main:infra/setup.bp", map[string]bool{})
	if err != nil {
		t.Fatalf("loadGitInclude(): %v", err)
	}
	if len(rules) != 1 || rules[0].Action != "install" {
		t.Fatalf("rules = %+v, want a single install rule", rules)
	}

	if len(recording.cloneSpecs) != 1 || recording.cloneSpecs[0] != wantSpec {
		t.Errorf("Clone specs = %+v, want exactly [%+v]", recording.cloneSpecs, wantSpec)
	}
}

// TestLoadGitIncludeCloneError ensures a clone failure from the seam surfaces
// as an include error rather than falling through to a missing file.
func TestLoadGitIncludeCloneError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	wantSpec := platform.CloneSpec{
		URL:  "https://github.com/org/repo.git",
		Path: filepath.Join(home, ".blueprint", "repos", "github.com", "org", "repo"),
		Mode: platform.ModeDirect,
	}
	cloneErr := &cloneError{}
	recording := &recordingGitProvider{
		GitProvider: mocks.NewMockGitProvider().WithCloneError(wantSpec, cloneErr),
	}
	origGit := Git
	Git = recording
	t.Cleanup(func() { Git = origGit })

	if _, err := loadGitInclude("https://github.com/org/repo.git", map[string]bool{}); err == nil {
		t.Fatal("loadGitInclude() succeeded, want the clone error to propagate")
	}
}

// cloneError is a distinct error type so the assertion doesn't depend on text.
type cloneError struct{}

func (*cloneError) Error() string { return "clone failed" }
