package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newOriginRepo creates a git repository with a single commit on branch and
// returns its path. Local paths are valid clone sources for both go-git and
// system git, so these tests need no network.
func newOriginRepo(t *testing.T, branch string) string {
	t.Helper()

	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...) // #nosec G204 -- test fixture
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	run("git", "init", "-q", "-b", branch, ".")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# fixture\n"), 0o600); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}
	run("git", "add", ".")
	run("git", "commit", "-qm", "initial commit")
	return dir
}

// commitOnBranch adds a commit to origin and returns its SHA.
func commitOnBranch(t *testing.T, origin, branch, file, content string) string {
	t.Helper()

	// -B creates the branch when it doesn't exist yet.
	cmd := exec.Command("git", "-C", origin, "checkout", "-q", "-B", branch) // #nosec G204 -- test fixture
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout %s: %v\n%s", branch, err, out)
	}
	if err := os.WriteFile(filepath.Join(origin, file), []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", file, err)
	}
	for _, args := range [][]string{
		{"git", "-C", origin, "add", "."},
		{"git", "-C", origin, "commit", "-qm", "add " + file},
	} {
		cmd := exec.Command(args[0], args[1:]...) // #nosec G204 -- test fixture
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}
	out, err := exec.Command("git", "-C", origin, "rev-parse", "HEAD").Output() // #nosec G204 -- test fixture
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// gitIn runs git in dir and returns trimmed stdout, failing the test on error.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...) // #nosec G204 -- test fixture
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

// assertBareLayout checks the worktrunk bare layout: a bare repo at
// <path>/.git with branches as worktrees beneath it.
func assertBareLayout(t *testing.T, path, branch string) {
	t.Helper()

	gitDir := filepath.Join(path, ".git")
	if got := gitIn(t, gitDir, "rev-parse", "--is-bare-repository"); got != "true" {
		t.Errorf("%s is not a bare repository (got %q)", gitDir, got)
	}

	worktree := filepath.Join(path, branch)
	gitFile := filepath.Join(worktree, ".git")
	info, err := os.Stat(gitFile)
	if err != nil {
		t.Fatalf("worktree %s has no .git: %v", worktree, err)
	}
	if info.IsDir() {
		t.Errorf("%s should be a gitfile linking to the bare repo, not a directory", gitFile)
	}
	if _, err := os.Stat(filepath.Join(worktree, "README.md")); err != nil {
		t.Errorf("worktree %s is not checked out: %v", worktree, err)
	}
	if got := gitIn(t, worktree, "rev-parse", "--abbrev-ref", "HEAD"); got != branch {
		t.Errorf("worktree HEAD branch = %q, want %q", got, branch)
	}
}

func TestCloneOrUpdateRepositoryBare_CreatesBareLayout(t *testing.T) {
	origin := newOriginRepo(t, "main")
	target := filepath.Join(t.TempDir(), "project")

	oldSHA, newSHA, status, err := CloneOrUpdateRepositoryBare(origin, target, "")
	if err != nil {
		t.Fatalf("CloneOrUpdateRepositoryBare: %v", err)
	}
	if status != "Cloned" {
		t.Errorf("status = %q, want %q", status, "Cloned")
	}
	if oldSHA != "" {
		t.Errorf("oldSHA = %q, want empty on first clone", oldSHA)
	}
	if len(newSHA) != 40 {
		t.Errorf("newSHA = %q, want a 40-char SHA", newSHA)
	}

	assertBareLayout(t, target, "main")

	// A bare clone writes no fetch refspec, so without this the repository
	// would never learn about new branches.
	gitDir := filepath.Join(target, ".git")
	if got := gitIn(t, gitDir, "config", "--get", "remote.origin.fetch"); got != fullFetchRefspec {
		t.Errorf("remote.origin.fetch = %q, want %q", got, fullFetchRefspec)
	}
	if got := gitIn(t, gitDir, "rev-parse", "--verify", "--quiet", "refs/remotes/origin/main"); len(got) != 40 {
		t.Errorf("refs/remotes/origin/main missing after clone (got %q)", got)
	}
}

func TestCloneOrUpdateRepositoryBare_UsesRequestedBranch(t *testing.T) {
	origin := newOriginRepo(t, "main")
	commitOnBranch(t, origin, "develop", "dev.txt", "dev\n")

	target := filepath.Join(t.TempDir(), "project")
	if _, _, _, err := CloneOrUpdateRepositoryBare(origin, target, "develop"); err != nil {
		t.Fatalf("CloneOrUpdateRepositoryBare: %v", err)
	}

	assertBareLayout(t, target, "develop")
	if _, err := os.Stat(filepath.Join(target, "develop", "dev.txt")); err != nil {
		t.Errorf("develop worktree missing dev.txt: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "main")); err == nil {
		t.Error("main worktree should not be created when branch: is set")
	}
}

func TestCloneOrUpdateRepositoryBare_FetchesWithoutTouchingWorktrees(t *testing.T) {
	origin := newOriginRepo(t, "main")
	target := filepath.Join(t.TempDir(), "project")

	_, firstSHA, _, err := CloneOrUpdateRepositoryBare(origin, target, "")
	if err != nil {
		t.Fatalf("first clone: %v", err)
	}
	if firstSHA == "" {
		t.Fatal("first clone returned no SHA")
	}

	// Uncommitted work an agent left behind must survive a re-run.
	uncommitted := filepath.Join(target, "main", "WIP.md")
	if err := os.WriteFile(uncommitted, []byte("work in progress\n"), 0o600); err != nil {
		t.Fatalf("write WIP file: %v", err)
	}

	commitOnBranch(t, origin, "main", "second.txt", "second\n")

	oldSHA, newSHA, status, err := CloneOrUpdateRepositoryBare(origin, target, "")
	if err != nil {
		t.Fatalf("second clone: %v", err)
	}
	if status != "Updated" {
		t.Errorf("status = %q, want %q", status, "Updated")
	}
	if oldSHA == newSHA {
		t.Errorf("SHA did not change after a new commit (both %s)", newSHA)
	}
	if firstSHA != oldSHA {
		t.Errorf("oldSHA = %q, want the SHA recorded by the first run (%q)", oldSHA, firstSHA)
	}
	if _, err := os.Stat(uncommitted); err != nil {
		t.Errorf("uncommitted work in worktree was not preserved: %v", err)
	}

	// The fetch makes the new commit visible without resetting the worktree.
	gitDir := filepath.Join(target, ".git")
	if got := gitIn(t, gitDir, "rev-parse", "refs/remotes/origin/main"); got != newSHA {
		t.Errorf("origin/main = %q, want %q", got, newSHA)
	}
}

func TestCloneOrUpdateRepositoryBare_ReportsUpToDate(t *testing.T) {
	origin := newOriginRepo(t, "main")
	target := filepath.Join(t.TempDir(), "project")

	if _, sha, _, err := CloneOrUpdateRepositoryBare(origin, target, ""); err != nil {
		t.Fatalf("first clone: %v", err)
	} else if sha == "" {
		t.Fatal("first clone returned no SHA")
	}

	oldSHA, newSHA, status, err := CloneOrUpdateRepositoryBare(origin, target, "")
	if err != nil {
		t.Fatalf("second clone: %v", err)
	}
	if status != "Already up to date" {
		t.Errorf("status = %q, want %q", status, "Already up to date")
	}
	if oldSHA != newSHA || oldSHA == "" {
		t.Errorf("oldSHA = %q, newSHA = %q — want equal, non-empty", oldSHA, newSHA)
	}
}

func TestCloneOrUpdateRepositoryBare_RecreatesMissingWorktree(t *testing.T) {
	origin := newOriginRepo(t, "main")
	target := filepath.Join(t.TempDir(), "project")

	if _, _, _, err := CloneOrUpdateRepositoryBare(origin, target, ""); err != nil {
		t.Fatalf("first clone: %v", err)
	}

	if err := os.RemoveAll(filepath.Join(target, "main")); err != nil {
		t.Fatalf("remove worktree: %v", err)
	}

	_, _, status, err := CloneOrUpdateRepositoryBare(origin, target, "")
	if err != nil {
		t.Fatalf("second clone: %v", err)
	}
	if status != "Updated" {
		t.Errorf("status = %q, want %q (worktree was recreated)", status, "Updated")
	}
	assertBareLayout(t, target, "main")
}

func TestCloneOrUpdateRepositoryBare_KeepsOtherWorktrees(t *testing.T) {
	origin := newOriginRepo(t, "main")
	commitOnBranch(t, origin, "feature", "feature.txt", "feature\n")
	// Leave origin on main so the default-branch worktree is the main one.
	gitIn(t, origin, "checkout", "-q", "main")

	target := filepath.Join(t.TempDir(), "project")

	if _, _, _, err := CloneOrUpdateRepositoryBare(origin, target, ""); err != nil {
		t.Fatalf("first clone: %v", err)
	}

	// A second worktree, as `wt switch --create feature` would leave it.
	gitDir := filepath.Join(target, ".git")
	gitIn(t, gitDir, "worktree", "add", filepath.Join(target, "feature"), "feature")
	marker := filepath.Join(target, "feature", "UNCOMMITTED.md")
	if err := os.WriteFile(marker, []byte("keep me\n"), 0o600); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	if _, _, _, err := CloneOrUpdateRepositoryBare(origin, target, ""); err != nil {
		t.Fatalf("second clone: %v", err)
	}

	assertBareLayout(t, target, "main")
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("sibling worktree was disturbed: %v", err)
	}
}

func TestCloneOrUpdateRepositoryBare_RefusesNonEmptyWorktreePath(t *testing.T) {
	origin := newOriginRepo(t, "main")
	target := filepath.Join(t.TempDir(), "project")

	if err := os.MkdirAll(filepath.Join(target, "main"), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "main", "notes.txt"), []byte("mine\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if _, _, _, err := CloneOrUpdateRepositoryBare(origin, target, ""); err == nil {
		t.Fatal("expected an error when the worktree path holds unrelated files")
	} else if !strings.Contains(err.Error(), "directory is not empty") {
		t.Errorf("error = %v, want it to mention a non-empty directory", err)
	}
	if _, err := os.Stat(filepath.Join(target, "main", "notes.txt")); err != nil {
		t.Errorf("unrelated files were destroyed: %v", err)
	}
}

func TestCloneOrUpdateRepositoryBare_RejectsNonBareRepository(t *testing.T) {
	origin := newOriginRepo(t, "main")
	target := filepath.Join(t.TempDir(), "project")

	// A clone made without bare: keeps its repository in the same place.
	cmd := exec.Command("git", "clone", "-q", origin, target) // #nosec G204 -- test fixture
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone: %v\n%s", err, out)
	}

	_, _, _, err := CloneOrUpdateRepositoryBare(origin, target, "")
	if err == nil {
		t.Fatal("expected an error when <path>/.git is not a bare repository")
	}
	if !strings.Contains(err.Error(), "non-bare repository") {
		t.Errorf("error = %v, want it to mention the non-bare repository", err)
	}
	if _, err := os.Stat(filepath.Join(target, "README.md")); err != nil {
		t.Errorf("existing clone was disturbed: %v", err)
	}
}

func TestBareDefaultBranch(t *testing.T) {
	origin := newOriginRepo(t, "trunk")
	target := filepath.Join(t.TempDir(), "project")

	if _, _, _, err := CloneOrUpdateRepositoryBare(origin, target, ""); err != nil {
		t.Fatalf("CloneOrUpdateRepositoryBare: %v", err)
	}

	// The remote default is trunk, not main/master — the worktree must follow it.
	assertBareLayout(t, target, "trunk")
}

func TestExpandHomePath(t *testing.T) {
	t.Run("absolute path is unchanged", func(t *testing.T) {
		got, err := expandHomePath("/tmp/project")
		if err != nil {
			t.Fatalf("expandHomePath: %v", err)
		}
		if got != "/tmp/project" {
			t.Errorf("expandHomePath = %q, want %q", got, "/tmp/project")
		}
	})

	t.Run("tilde expands to home", func(t *testing.T) {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("UserHomeDir: %v", err)
		}
		got, err := expandHomePath("~/project")
		if err != nil {
			t.Fatalf("expandHomePath: %v", err)
		}
		want := filepath.Join(home, "project")
		if got != want {
			t.Errorf("expandHomePath = %q, want %q", got, want)
		}
	})
}
