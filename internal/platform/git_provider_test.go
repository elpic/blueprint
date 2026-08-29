package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Compile-time interface compliance: the real provider satisfies GitProvider.
var _ GitProvider = (*realGitProvider)(nil)

// --- fixture helpers (local repos only — no network) ---

// runGitIn runs git in dir and fails the test on error.
func runGitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...) // #nosec G204 -- test fixture
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return string(out)
}

// newOriginRepo creates a git repository with a single commit on branch and
// returns its path. Local paths are valid clone sources for both go-git and
// system git, so these tests need no network.
func newOriginRepo(t *testing.T, branch string) string {
	t.Helper()
	dir := t.TempDir()
	runGitIn(t, dir, "init", "-q", "-b", branch, ".")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# fixture\n"), 0o600); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}
	runGitIn(t, dir, "add", ".")
	runGitIn(t, dir, "commit", "-qm", "initial commit")
	return dir
}

// commitToRepo adds a commit to origin and returns the new HEAD SHA.
func commitToRepo(t *testing.T, origin, file, content string) SHA {
	t.Helper()
	if err := os.WriteFile(filepath.Join(origin, file), []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", file, err)
	}
	runGitIn(t, origin, "add", ".")
	runGitIn(t, origin, "commit", "-qm", "add "+file)
	return SHA(strings.TrimSpace(runGitIn(t, origin, "rev-parse", "HEAD")))
}

// headSHA returns the current HEAD SHA of a repository.
func headSHA(t *testing.T, repo string) SHA {
	t.Helper()
	return SHA(strings.TrimSpace(runGitIn(t, repo, "rev-parse", "HEAD")))
}

// --- status string → enum mapping ---

func TestCloneStatusFromString(t *testing.T) {
	tests := []struct {
		status string
		want   CloneStatus
	}{
		{"Cloned", StatusCloned},
		{"Updated", StatusUpdated},
		{"Synced", StatusSynced},
		{"Already up to date", StatusUpToDate},
	}
	for _, tt := range tests {
		if got, err := cloneStatusFromString(tt.status); err != nil || got != tt.want {
			t.Errorf("cloneStatusFromString(%q) = (%v, %v), want (%v, nil)", tt.status, got, err, tt.want)
		}
	}

	// Unknown status strings must be errors, not silent zero values: today's
	// callers switch on exact strings, so an unnoticed new status would
	// silently misreport ("Synced" once escaped an audit that way).
	for _, status := range []string{"", "cloned", "Rebased", "Already up to date."} {
		if got, err := cloneStatusFromString(status); err == nil {
			t.Errorf("cloneStatusFromString(%q) = (%v, nil), want an error", status, got)
		}
	}
}

// --- Layout classification (ported from the old IsRepository test) ---

func TestRealGitProvider_Layout(t *testing.T) {
	g := &realGitProvider{}

	t.Run("standard working copy (.git directory)", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, ".git"), 0o750); err != nil {
			t.Fatalf("Mkdir .git: %v", err)
		}
		if got := g.Layout(RepoPath(dir)); got != LayoutStandard {
			t.Errorf("Layout(%q) = %v, want LayoutStandard", dir, got)
		}
	})

	t.Run(".git as regular file (worktree/gitfile)", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: ../.git/worktrees/foo\n"), 0o600); err != nil {
			t.Fatalf("WriteFile .git: %v", err)
		}
		if got := g.Layout(RepoPath(dir)); got != LayoutWorktree {
			t.Errorf("Layout(%q) = %v, want LayoutWorktree", dir, got)
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		dir := t.TempDir()
		if got := g.Layout(RepoPath(dir)); got != LayoutNone {
			t.Errorf("Layout(%q) = %v, want LayoutNone", dir, got)
		}
	})

	t.Run("nonexistent path", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "does-not-exist")
		if got := g.Layout(RepoPath(path)); got != LayoutNone {
			t.Errorf("Layout(%q) = %v, want LayoutNone", path, got)
		}
	})

	t.Run("true bare repository (path is the gitdir)", func(t *testing.T) {
		origin := newOriginRepo(t, "main")
		bare := filepath.Join(t.TempDir(), "repo.git")
		runGitIn(t, origin, "clone", "-q", "--bare", origin, bare)
		if got := g.Layout(RepoPath(bare)); got != LayoutBare {
			t.Errorf("Layout(%q) = %v, want LayoutBare", bare, got)
		}
	})
}

// --- behavioral smoke tests: one per non-trivial delegation ---

// TestRealGitProvider_CloneTwoStage covers the default mode end to end: the
// target is a content mirror (no .git), the SHA lives in clean storage, and a
// second run reports UpToDate before an origin commit turns it into Updated.
func TestRealGitProvider_CloneTwoStage(t *testing.T) {
	g := &realGitProvider{}
	origin := newOriginRepo(t, "main")
	sha1 := headSHA(t, origin)
	target := filepath.Join(t.TempDir(), "target")
	id := RepoID{URL: origin, Branch: "main"}
	spec := CloneSpec{URL: origin, Path: target, Branch: "main", Mode: ModeTwoStage}

	// Zero-value Mode is ModeTwoStage — assert the default before using it.
	if spec.Mode != ModeTwoStage {
		t.Fatalf("zero-value CloneMode = %v, want ModeTwoStage", spec.Mode)
	}

	res, err := g.Clone(spec)
	if err != nil {
		t.Fatalf("Clone(two-stage): %v", err)
	}
	if res.Status != StatusCloned || res.OldSHA != "" || res.NewSHA != sha1 {
		t.Errorf("Clone(two-stage) = %+v, want {Cloned %q %q}", res, "", sha1)
	}

	// The target is a mirror: content present, no .git, so Layout is None and
	// LocalSHA errors (the real provider propagates underlying errors).
	if _, err := os.Stat(filepath.Join(target, "README.md")); err != nil {
		t.Errorf("mirrored README.md missing: %v", err)
	}
	if got := g.Layout(RepoPath(target)); got != LayoutNone {
		t.Errorf("Layout(target) = %v, want LayoutNone for a two-stage mirror", got)
	}
	if _, err := g.LocalSHA(RepoPath(target)); err == nil {
		t.Errorf("LocalSHA(mirror) = nil error, want error (no .git in a two-stage target)")
	}

	// The authoritative SHA lives in clean storage.
	if got, err := g.StorageSHA(id); err != nil || got != sha1 {
		t.Errorf("StorageSHA(%+v) = (%q, %v), want (%q, nil)", id, got, err, sha1)
	}

	// Second run with no origin change: nothing to do.
	if res, err := g.Clone(spec); err != nil || res.Status != StatusUpToDate {
		t.Errorf("second Clone = (%+v, %v), want StatusUpToDate", res, err)
	}

	// New origin commit: storage advances, target is re-mirrored.
	sha2 := commitToRepo(t, origin, "second.txt", "v2\n")
	res, err = g.Clone(spec)
	if err != nil {
		t.Fatalf("Clone after origin commit: %v", err)
	}
	if res.Status != StatusUpdated || res.OldSHA != sha1 || res.NewSHA != sha2 {
		t.Errorf("Clone after origin commit = %+v, want {Updated %q %q}", res, sha1, sha2)
	}
}

// TestRealGitProvider_CloneDirect covers ModeDirect: a fully functional
// working copy (standard layout, readable LocalSHA), Updated on the second
// run, and Checkout round-tripping to the first commit.
func TestRealGitProvider_CloneDirect(t *testing.T) {
	g := &realGitProvider{}
	origin := newOriginRepo(t, "main")
	sha1 := headSHA(t, origin)
	target := filepath.Join(t.TempDir(), "target")
	spec := CloneSpec{URL: origin, Path: target, Branch: "main", Mode: ModeDirect}

	res, err := g.Clone(spec)
	if err != nil {
		t.Fatalf("Clone(direct): %v", err)
	}
	if res.Status != StatusCloned || res.NewSHA != sha1 {
		t.Errorf("Clone(direct) = %+v, want {Cloned %q}", res, sha1)
	}
	if got := g.Layout(RepoPath(target)); got != LayoutStandard {
		t.Errorf("Layout(target) = %v, want LayoutStandard", got)
	}
	if got, err := g.LocalSHA(RepoPath(target)); err != nil || got != sha1 {
		t.Errorf("LocalSHA(target) = (%q, %v), want (%q, nil)", got, err, sha1)
	}

	// Second run after an origin commit: fast-forward update. An untracked
	// user file must survive it (#031) and the protection must surface as a
	// Note through the seam.
	untracked := filepath.Join(target, "user-notes.md")
	if err := os.WriteFile(untracked, []byte("mine\n"), 0o600); err != nil {
		t.Fatalf("write untracked file: %v", err)
	}
	sha2 := commitToRepo(t, origin, "second.txt", "v2\n")
	res, err = g.Clone(spec)
	if err != nil {
		t.Fatalf("Clone(direct) update: %v", err)
	}
	if res.Status != StatusUpdated || res.OldSHA != sha1 || res.NewSHA != sha2 {
		t.Errorf("Clone(direct) update = %+v, want {Updated %q %q}", res, sha1, sha2)
	}
	if data, err := os.ReadFile(untracked); err != nil || string(data) != "mine\n" {
		t.Errorf("untracked file after update = (%q, %v), want %q", data, err, "mine\n")
	}
	wantNote := "protected 1 untracked file from reset in " + target + ": user-notes.md"
	if len(res.Notes) != 1 || res.Notes[0] != wantNote {
		t.Errorf("Clone(direct) update notes = %q, want [%q]", res.Notes, wantNote)
	}

	// Checkout pins the exact SHA doctor wants to inspect.
	if err := g.Checkout(CheckoutSpec{Path: RepoPath(target), SHA: sha1}); err != nil {
		t.Fatalf("Checkout(%q): %v", sha1, err)
	}
	if got, err := g.LocalSHA(RepoPath(target)); err != nil || got != sha1 {
		t.Errorf("LocalSHA after Checkout = (%q, %v), want (%q, nil)", got, err, sha1)
	}
}

// TestRealGitProvider_CloneBare covers ModeBare: the bare repository lives at
// <target>/.git with a worktree per branch, BareSHA reads the tracked tip,
// and the layout classification distinguishes bare root from worktree.
func TestRealGitProvider_CloneBare(t *testing.T) {
	g := &realGitProvider{}
	origin := newOriginRepo(t, "main")
	sha1 := headSHA(t, origin)
	target := filepath.Join(t.TempDir(), "target")
	spec := CloneSpec{URL: origin, Path: target, Branch: "main", Mode: ModeBare}

	res, err := g.Clone(spec)
	if err != nil {
		t.Fatalf("Clone(bare): %v", err)
	}
	if res.Status != StatusCloned || res.NewSHA != sha1 {
		t.Errorf("Clone(bare) = %+v, want {Cloned %q}", res, sha1)
	}

	if got := g.Layout(RepoPath(target)); got != LayoutBare {
		t.Errorf("Layout(bare root) = %v, want LayoutBare", got)
	}
	if got, err := g.BareSHA(RepoPath(target), "main"); err != nil || got != sha1 {
		t.Errorf("BareSHA(target, main) = (%q, %v), want (%q, nil)", got, err, sha1)
	}
	// The bare layout checks the branch out as a linked worktree at
	// <target>/<branch>: a .git regular file.
	if got := g.Layout(RepoPath(filepath.Join(target, "main"))); got != LayoutWorktree {
		t.Errorf("Layout(<target>/main) = %v, want LayoutWorktree", got)
	}

	// Second run with no origin change: nothing to do, worktrees untouched.
	if res, err := g.Clone(spec); err != nil || res.Status != StatusUpToDate {
		t.Errorf("second Clone(bare) = (%+v, %v), want StatusUpToDate", res, err)
	}
}

// TestRealGitProvider_UnknownSHAContract pins the seam's unknown-SHA rule:
// BareSHA and StorageSHA map their underlying empty string to ("", nil), while
// LocalSHA propagates the underlying error (internal/git cannot fail quietly
// there, so the real provider never returns ("", nil) for a bad path).
func TestRealGitProvider_UnknownSHAContract(t *testing.T) {
	g := &realGitProvider{}

	if got, err := g.BareSHA(RepoPath(filepath.Join(t.TempDir(), "missing")), "main"); err != nil || got != "" {
		t.Errorf("BareSHA(missing) = (%q, %v), want (\"\", nil)", got, err)
	}
	if got, err := g.StorageSHA(RepoID{URL: "file:///nonexistent/origin", Branch: "main"}); err != nil || got != "" {
		t.Errorf("StorageSHA(unknown) = (%q, %v), want (\"\", nil)", got, err)
	}
	if got, err := g.LocalSHA(RepoPath(filepath.Join(t.TempDir(), "missing"))); err == nil || got != "" {
		t.Errorf("LocalSHA(missing) = (%q, %v), want (\"\", error)", got, err)
	}
}

// TestRealGitProvider_CloneUnknownMode guards the dispatch: an invalid mode is
// a hard error, not a silent fallthrough.
func TestRealGitProvider_CloneUnknownMode(t *testing.T) {
	g := &realGitProvider{}
	spec := CloneSpec{URL: "file:///nonexistent", Path: t.TempDir(), Mode: CloneMode(99)}
	if _, err := g.Clone(spec); err == nil {
		t.Errorf("Clone(mode 99) = nil error, want error")
	}
}
