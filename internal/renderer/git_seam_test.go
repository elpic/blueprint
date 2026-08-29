package renderer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/elpic/blueprint/internal/platform"
	"github.com/elpic/blueprint/internal/platform/mocks"
)

// recordingGitProvider wraps a GitProvider and records every Clone spec it
// receives, so tests can assert exactly what the renderer asked the seam to do.
type recordingGitProvider struct {
	platform.GitProvider
	cloneSpecs []platform.CloneSpec
}

func (r *recordingGitProvider) Clone(spec platform.CloneSpec) (platform.CloneResult, error) {
	r.cloneSpecs = append(r.cloneSpecs, spec)
	return r.GitProvider.Clone(spec)
}

// TestResolveTemplatePathUsesGitSeam pins the seam adoption of
// ResolveTemplatePath: a remote template resolves through one Direct-mode
// Clone spec (working-copy cache) and returns the template directory from it.
func TestResolveTemplatePathUsesGitSeam(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	localRepo := filepath.Join(home, ".blueprint", "repos", "github.com", "org", "repo")
	tmplDir := filepath.Join(localRepo, "templates")
	if err := os.MkdirAll(tmplDir, 0o750); err != nil {
		t.Fatal(err)
	}

	wantSpec := platform.CloneSpec{
		URL:    "https://github.com/org/repo.git",
		Path:   localRepo,
		Branch: "main",
		Mode:   platform.ModeDirect,
	}
	recording := &recordingGitProvider{GitProvider: mocks.NewMockGitProvider()}
	origGit := Git
	Git = recording
	t.Cleanup(func() { Git = origGit })

	local, root, cleanup, err := ResolveTemplatePath("https://github.com/org/repo.git@main:templates", false)
	defer cleanup()
	if err != nil {
		t.Fatalf("ResolveTemplatePath(): %v", err)
	}
	if local != tmplDir || root != tmplDir {
		t.Errorf("ResolveTemplatePath() = (%q, %q), want (%q, %q)", local, root, tmplDir, tmplDir)
	}

	if len(recording.cloneSpecs) != 1 || recording.cloneSpecs[0] != wantSpec {
		t.Errorf("Clone specs = %+v, want exactly [%+v]", recording.cloneSpecs, wantSpec)
	}
}
