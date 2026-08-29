package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/elpic/blueprint/internal/platform"
	"github.com/elpic/blueprint/internal/platform/mocks"
)

// recordingGitProvider wraps a GitProvider and records every Clone and
// Checkout spec it receives, so tests can assert exactly what the engine
// asked the seam to do.
type recordingGitProvider struct {
	platform.GitProvider
	cloneSpecs    []platform.CloneSpec
	checkoutSpecs []platform.CheckoutSpec
}

func (r *recordingGitProvider) Clone(spec platform.CloneSpec) (platform.CloneResult, error) {
	r.cloneSpecs = append(r.cloneSpecs, spec)
	return r.GitProvider.Clone(spec)
}

func (r *recordingGitProvider) Checkout(spec platform.CheckoutSpec) error {
	r.checkoutSpecs = append(r.checkoutSpecs, spec)
	return r.GitProvider.Checkout(spec)
}

// withGitSeam swaps the engine's Git package var for the given provider for
// the duration of the test, restoring the original afterwards.
func withGitSeam(t *testing.T, provider platform.GitProvider) {
	t.Helper()
	orig := Git
	Git = provider
	t.Cleanup(func() { Git = orig })
}

// blueprintCacheDir returns the stable cache path the engine derives for
// https://github.com/org/repo.git under the given home directory.
func blueprintCacheDir(home string) string {
	return filepath.Join(home, ".blueprint", "repos", "github.com", "org", "repo")
}

// TestResolveBlueprintFileUsesGitSeam pins the seam adoption of
// resolveBlueprintFile: a blueprint git URL resolves to one Direct-mode
// Clone spec (working-copy cache) and the returned SHA is the clone result's
// NewSHA.
func TestResolveBlueprintFileUsesGitSeam(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cacheDir := blueprintCacheDir(home)
	if err := os.MkdirAll(filepath.Join(cacheDir, "infra"), 0o750); err != nil {
		t.Fatal(err)
	}
	setupFile := filepath.Join(cacheDir, "infra", "setup.bp")
	if err := os.WriteFile(setupFile, []byte("install git\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	wantSpec := platform.CloneSpec{
		URL:    "https://github.com/org/repo.git",
		Path:   cacheDir,
		Branch: "main",
		Mode:   platform.ModeDirect,
	}
	recording := &recordingGitProvider{
		GitProvider: mocks.NewMockGitProvider().
			WithCloneResult(wantSpec, platform.CloneResult{
				Status: platform.StatusCloned,
				NewSHA: "feedbeefcafe",
			}),
	}
	withGitSeam(t, recording)

	setupPath, sha, _, err := resolveBlueprintFile("https://github.com/org/repo.git@main:infra/setup.bp", false, false)
	if err != nil {
		t.Fatalf("resolveBlueprintFile(): %v", err)
	}
	if setupPath != setupFile {
		t.Errorf("setupPath = %q, want %q", setupPath, setupFile)
	}
	if sha != "feedbeefcafe" {
		t.Errorf("sha = %q, want the clone result's NewSHA", sha)
	}

	if len(recording.cloneSpecs) != 1 || recording.cloneSpecs[0] != wantSpec {
		t.Errorf("Clone specs = %+v, want exactly [%+v]", recording.cloneSpecs, wantSpec)
	}
}

// TestRulesForBlueprintUsesGitSeam pins the seam adoption of
// rulesForBlueprint: one Direct-mode Clone spec plus a forced Checkout of the
// exact applied SHA on the cached working copy.
func TestRulesForBlueprintUsesGitSeam(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cacheDir := blueprintCacheDir(home)
	if err := os.MkdirAll(cacheDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "setup.bp"), []byte("install git\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	wantClone := platform.CloneSpec{
		URL:  "https://github.com/org/repo.git",
		Path: cacheDir,
		Mode: platform.ModeDirect,
	}
	recording := &recordingGitProvider{GitProvider: mocks.NewMockGitProvider()}
	withGitSeam(t, recording)

	rules, err := rulesForBlueprint("https://github.com/org/repo.git", "abc123def456")
	if err != nil {
		t.Fatalf("rulesForBlueprint(): %v", err)
	}
	if len(rules) != 1 || rules[0].Action != "install" {
		t.Fatalf("rules = %+v, want a single install rule", rules)
	}

	if len(recording.cloneSpecs) != 1 || recording.cloneSpecs[0] != wantClone {
		t.Errorf("Clone specs = %+v, want exactly [%+v]", recording.cloneSpecs, wantClone)
	}
	wantCheckout := platform.CheckoutSpec{Path: platform.RepoPath(cacheDir), SHA: platform.SHA("abc123def456")}
	if len(recording.checkoutSpecs) != 1 || recording.checkoutSpecs[0] != wantCheckout {
		t.Errorf("Checkout specs = %+v, want exactly [%+v]", recording.checkoutSpecs, wantCheckout)
	}
}
