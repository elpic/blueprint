package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// ---- #9: IsGitURL false positive on substring ".git" ----------------------
//
// strings.Contains(input, ".git") matches local paths that happen to contain
// the string ".git", causing blueprint to treat them as remote git URLs.

func TestIsGitURLFalsePositives(t *testing.T) {
	falsePositives := []string{
		"~/projects/dotfiles.git/setup.bp",  // local path with .git directory
		"/home/user/mygit-configs/setup.bp", // contains "git" but not .git
		"./setup.bp",                        // plain relative path
	}
	for _, input := range falsePositives {
		if IsGitURL(input) {
			t.Errorf("BUG: IsGitURL(%q) = true, want false — .git substring false positive", input)
		}
	}
}

func TestIsGitURLTruePositives(t *testing.T) {
	truePositives := []string{
		"git@github.com:user/repo.git",
		"https://github.com/user/repo.git",
		"http://github.com/user/repo.git",
		"git://github.com/user/repo.git",
		"https://github.com/user/repo",
	}
	for _, input := range truePositives {
		if !IsGitURL(input) {
			t.Errorf("IsGitURL(%q) = false, want true", input)
		}
	}
}

// ---- #10: CloneOrUpdateRepository stale origin/HEAD after fetch -----------
//
// When no branch is specified, CloneOrUpdateRepository must NOT rely on
// refs/remotes/origin/HEAD to determine the reset target after fetch. go-git's
// Fetch does not update origin/HEAD the way system git does, so it can be
// stale — pointing to an old commit. The function must use the live remote HEAD
// SHA (resolved via ls-remote before fetch) as the authoritative target.

func TestCloneOrUpdateRepository_NoBranch_DetectsUpdateDespiteStaleOriginHEAD(t *testing.T) {
	// --- Setup: create a remote bare repo and a source repo with initial commit ---
	remoteDir := filepath.Join(t.TempDir(), "remote.git")
	sourceDir := t.TempDir()
	cloneDir := filepath.Join(t.TempDir(), "clone")

	// Create bare remote
	runGit(t, "init", "--bare", remoteDir)

	// Create source repo with an initial commit
	runGit(t, "init", sourceDir)
	configureGitUser(t, sourceDir)
	os.WriteFile(filepath.Join(sourceDir, "README.md"), []byte("initial"), 0644)
	runGit(t, "-C", sourceDir, "add", ".")
	runGit(t, "-C", sourceDir, "commit", "-m", "initial")
	runGit(t, "-C", sourceDir, "branch", "-m", "main") // rename default branch to main
	runGit(t, "-C", sourceDir, "remote", "add", "origin", remoteDir)
	runGit(t, "-C", sourceDir, "push", "-u", "origin", "main")

	// Ensure remote HEAD points to main
	runGit(t, "-C", remoteDir, "symbolic-ref", "HEAD", "refs/heads/main")

	// --- First clone via CloneOrUpdateRepository (no branch) ---
	_, newSHA, status, err := CloneOrUpdateRepository(remoteDir, cloneDir, "")
	if err != nil {
		t.Fatalf("first clone failed: %v", err)
	}
	if status != "Cloned" {
		t.Fatalf("expected 'Cloned', got %q", status)
	}
	oldSHA := newSHA

	// --- Push a new commit to the remote ---
	os.WriteFile(filepath.Join(sourceDir, "README.md"), []byte("updated"), 0644)
	runGit(t, "-C", sourceDir, "add", ".")
	runGit(t, "-C", sourceDir, "commit", "-m", "update")
	runGit(t, "-C", sourceDir, "push", "origin", "main")

	// --- Simulate stale origin/HEAD: write it as a direct hash ref to old SHA ---
	repo, err := git.PlainOpen(cloneDir)
	if err != nil {
		t.Fatalf("failed to open cloned repo: %v", err)
	}
	originHEADRef := plumbing.NewHashReference(
		plumbing.NewRemoteReferenceName("origin", "HEAD"),
		plumbing.NewHash(oldSHA),
	)
	if err := repo.Storer.SetReference(originHEADRef); err != nil {
		t.Fatalf("failed to set stale origin/HEAD: %v", err)
	}

	// Verify origin/HEAD is now stale
	staleCheck, err := repo.Reference(plumbing.NewRemoteReferenceName("origin", "HEAD"), true)
	if err != nil {
		t.Fatalf("failed to read origin/HEAD: %v", err)
	}
	if staleCheck.Hash().String() != oldSHA {
		t.Fatalf("origin/HEAD should be stale at %s, got %s", oldSHA, staleCheck.Hash().String())
	}

	// --- CloneOrUpdateRepository again — must detect the update despite stale origin/HEAD ---
	gotOld, gotNew, gotStatus, err := CloneOrUpdateRepository(remoteDir, cloneDir, "")
	if err != nil {
		t.Fatalf("update clone failed: %v", err)
	}
	if gotStatus != "Updated" {
		t.Fatalf("expected 'Updated', got %q (oldSHA=%s, remote has new commits)", gotStatus, gotOld)
	}
	if gotNew == "" || gotNew == oldSHA {
		t.Fatalf("expected new SHA different from %s, got %s", oldSHA, gotNew)
	}

	// Verify the working tree was actually updated
	content, err := os.ReadFile(filepath.Join(cloneDir, "README.md"))
	if err != nil {
		t.Fatalf("failed to read README.md in clone: %v", err)
	}
	if strings.TrimSpace(string(content)) != "updated" {
		t.Fatalf("expected README.md content 'updated', got %q", string(content))
	}
}

// runGit executes a system git command, failing the test on error.
func runGit(t *testing.T, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, string(out))
	}
	return strings.TrimSpace(string(out))
}

// configureGitUser sets a known git user.email and user.name so commits work.
func configureGitUser(t *testing.T, repoDir string) {
	t.Helper()
	runGit(t, "-C", repoDir, "config", "user.email", "test@blueprint.test")
	runGit(t, "-C", repoDir, "config", "user.name", "Blueprint Test")
}
