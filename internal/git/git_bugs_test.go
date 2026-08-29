package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// ---- #9: IsGitURL false positive on substring ".git" ----------------------
//
// strings.Contains(input, ".git") matches local paths that happen to contain
// the string ".git", causing blueprint to treat them as remote git URLs.

// ---- #26: worktree silently missing tracked files -------------------------
//
// When local HEAD matched the remote HEAD, CloneOrUpdateRepository returned
// "Already up to date" without fetching, resetting or restoring. A worktree
// that lost tracked files (so HEAD never diverges from the remote) could
// therefore never self-heal, and blueprint apply then symlinked nothing while
// still reporting success.
//
// The repair is deliberately narrow: it puts back tracked files that are
// MISSING from the worktree and leaves everything else alone. Blueprint clones
// hold user work — a locally edited .zshrc in a dotfiles clone, or a whole
// repo someone develops in via `clone workdir: true` — so anything broader
// would trade a missing-file bug for silent data loss.

// commitFiles writes files (including nested paths) into the origin repo and
// commits them, leaving HEAD on its current branch.
func commitFiles(t *testing.T, origin string, files map[string]string) {
	t.Helper()

	for rel, content := range files {
		full := filepath.Join(origin, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	identity := []string{
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
	}
	for _, args := range [][]string{
		{"git", "-C", origin, "add", "-A"},
		{"git", "-C", origin, "commit", "-qm", "add fixture files"},
	} {
		cmd := exec.Command(args[0], args[1:]...) // #nosec G204 -- test fixture
		cmd.Env = append(os.Environ(), identity...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}
}

// statusEntries returns `git status --porcelain` entries, sorted and with each
// line trimmed, so assertions don't depend on git's output order or on the
// leading column of the porcelain format.
func statusEntries(t *testing.T, dir string) []string {
	t.Helper()

	out, err := exec.Command("git", "-C", dir, "status", "--porcelain").Output() // #nosec G204 -- test fixture
	if err != nil {
		t.Fatalf("git status: %v", err)
	}
	var entries []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		entries = append(entries, strings.TrimSpace(line))
	}
	sort.Strings(entries)
	return entries
}

// readFile reads a file, failing the test if it is missing.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// assertFileContent compares a file's bytes against want.
func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	if got := readFile(t, path); got != want {
		t.Errorf("%s content = %q, want %q", filepath.Base(path), got, want)
	}
}

// TC-01: a single deleted tracked file is restored.
func TestCloneOrUpdateRepository_TC01_RestoresDeletedTrackedFile(t *testing.T) {
	origin := newOriginRepo(t, "main")
	commitFiles(t, origin, map[string]string{".config/nushell/config.nu": "# nushell config\n"})

	target := filepath.Join(t.TempDir(), "dotfiles")
	if _, _, _, err := CloneOrUpdateRepository(origin, target, ""); err != nil {
		t.Fatalf("first clone: %v", err)
	}

	// The failure mode: files gone from disk, HEAD untouched.
	missing := filepath.Join(target, ".config/nushell/config.nu")
	if err := os.Remove(missing); err != nil {
		t.Fatalf("remove tracked file: %v", err)
	}

	oldSHA, newSHA, status, err := CloneOrUpdateRepository(origin, target, "")
	if err != nil {
		t.Fatalf("second clone: %v", err)
	}
	// The status string must not change: cloneOrUpdateRepositoryTwoStageImpl
	// decides whether to re-copy storage to target based on it.
	if status != "Already up to date" {
		t.Errorf("status = %q, want %q", status, "Already up to date")
	}
	if oldSHA != newSHA {
		t.Errorf("oldSHA = %q, newSHA = %q — want equal (remote is unchanged)", oldSHA, newSHA)
	}
	assertFileContent(t, missing, "# nushell config\n")
	if got := gitIn(t, target, "status", "--porcelain"); got != "" {
		t.Errorf("worktree still differs from HEAD after repair: %q", got)
	}
}

// TC-02: a deleted tracked directory is restored file by file.
func TestCloneOrUpdateRepository_TC02_RestoresDeletedTrackedDirectory(t *testing.T) {
	origin := newOriginRepo(t, "main")
	commitFiles(t, origin, map[string]string{
		".config/nushell/config.nu": "# config\n",
		".config/nushell/env.nu":    "# env\n",
		".config/starship.toml":     "format = \"$all\"\n",
	})

	target := filepath.Join(t.TempDir(), "dotfiles")
	if _, _, _, err := CloneOrUpdateRepository(origin, target, ""); err != nil {
		t.Fatalf("first clone: %v", err)
	}

	if err := os.RemoveAll(filepath.Join(target, ".config/nushell")); err != nil {
		t.Fatalf("remove tracked directory: %v", err)
	}

	_, _, status, err := CloneOrUpdateRepository(origin, target, "")
	if err != nil {
		t.Fatalf("second clone: %v", err)
	}
	if status != "Already up to date" {
		t.Errorf("status = %q, want %q", status, "Already up to date")
	}
	assertFileContent(t, filepath.Join(target, ".config/nushell/config.nu"), "# config\n")
	assertFileContent(t, filepath.Join(target, ".config/nushell/env.nu"), "# env\n")
	// The untouched sibling directory must be left alone.
	assertFileContent(t, filepath.Join(target, ".config/starship.toml"), "format = \"$all\"\n")
	if got := gitIn(t, target, "status", "--porcelain"); got != "" {
		t.Errorf("worktree still differs from HEAD after repair: %q", got)
	}
}

// TC-03 (merge gate): uncommitted modifications survive the repair.
func TestCloneOrUpdateRepository_TC03_ModifiedTrackedFileSurvivesRepair(t *testing.T) {
	origin := newOriginRepo(t, "main")
	commitFiles(t, origin, map[string]string{
		".zshrc":                    "# zshrc\n",
		".config/nushell/config.nu": "# nushell config\n",
	})

	target := filepath.Join(t.TempDir(), "dotfiles")
	if _, _, _, err := CloneOrUpdateRepository(origin, target, ""); err != nil {
		t.Fatalf("first clone: %v", err)
	}

	// User work: an uncommitted brew shellenv block appended to .zshrc.
	const brewBlock = "eval \"$(/opt/homebrew/bin/brew shellenv)\"\n"
	if err := os.WriteFile(filepath.Join(target, ".zshrc"), []byte("# zshrc\n"+brewBlock), 0o600); err != nil {
		t.Fatalf("modify .zshrc: %v", err)
	}
	deleted := filepath.Join(target, ".config/nushell/config.nu")
	if err := os.Remove(deleted); err != nil {
		t.Fatalf("remove tracked file: %v", err)
	}

	_, _, status, err := CloneOrUpdateRepository(origin, target, "")
	if err != nil {
		t.Fatalf("second clone: %v", err)
	}
	if status != "Already up to date" {
		t.Errorf("status = %q, want %q", status, "Already up to date")
	}
	// The deletion is repaired…
	assertFileContent(t, deleted, "# nushell config\n")
	// …and the user's uncommitted work is byte-identical, not reverted.
	assertFileContent(t, filepath.Join(target, ".zshrc"), "# zshrc\n"+brewBlock)
	if got := statusEntries(t, target); len(got) != 1 || got[0] != "M .zshrc" {
		t.Errorf("status entries = %q, want %q (only the modification remains)", got, []string{"M .zshrc"})
	}
}

// TC-04 (critical): the exact worktree state from the affected machine —
// three deletions next to two modified files, all in one run.
func TestCloneOrUpdateRepository_TC04_OnlyDeletionsAreRepaired(t *testing.T) {
	origin := newOriginRepo(t, "main")
	commitFiles(t, origin, map[string]string{
		".config/nushell/config.nu": "# nushell config\n",
		".config/nushell/env.nu":    "# nushell env\n",
		".config/starship.toml":     "format = \"$all\"\n",
		".zshrc":                    "# zshrc\nexport EDITOR=vim\n",
		"README.md":                 "# dotfiles\nline one\nline two\nline three\n",
	})

	target := filepath.Join(t.TempDir(), "dotfiles")
	if _, _, _, err := CloneOrUpdateRepository(origin, target, ""); err != nil {
		t.Fatalf("first clone: %v", err)
	}

	// Deletions: what the repair must fix.
	for _, rel := range []string{".config/nushell/config.nu", ".config/nushell/env.nu", ".config/starship.toml"} {
		if err := os.Remove(filepath.Join(target, rel)); err != nil {
			t.Fatalf("remove %s: %v", rel, err)
		}
	}
	// Modifications: user work that must survive untouched.
	const zshrcDirty = "# zshrc\nexport EDITOR=vim\neval \"$(/opt/homebrew/bin/brew shellenv)\"\n"
	if err := os.WriteFile(filepath.Join(target, ".zshrc"), []byte(zshrcDirty), 0o600); err != nil {
		t.Fatalf("modify .zshrc: %v", err)
	}
	const readmeDirty = "# dotfiles\nline three\n" // two lines deleted
	if err := os.WriteFile(filepath.Join(target, "README.md"), []byte(readmeDirty), 0o600); err != nil {
		t.Fatalf("modify README.md: %v", err)
	}

	_, _, status, err := CloneOrUpdateRepository(origin, target, "")
	if err != nil {
		t.Fatalf("second clone: %v", err)
	}
	if status != "Already up to date" {
		t.Errorf("status = %q, want %q", status, "Already up to date")
	}

	assertFileContent(t, filepath.Join(target, ".config/nushell/config.nu"), "# nushell config\n")
	assertFileContent(t, filepath.Join(target, ".config/nushell/env.nu"), "# nushell env\n")
	assertFileContent(t, filepath.Join(target, ".config/starship.toml"), "format = \"$all\"\n")
	assertFileContent(t, filepath.Join(target, ".zshrc"), zshrcDirty)
	assertFileContent(t, filepath.Join(target, "README.md"), readmeDirty)

	// Only the two modifications may remain.
	want := []string{"M .zshrc", "M README.md"}
	if got := statusEntries(t, target); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("status entries = %q, want %q (the two modifications, nothing else)", got, want)
	}
}

// TC-14: `clone workdir: true` targets are repos users develop in. Their
// uncommitted work must survive, while a deleted tracked file is still repaired.
func TestCloneOrUpdateRepositoryDirect_TC14_PreservesUncommittedWork(t *testing.T) {
	origin := newOriginRepo(t, "main")
	commitFiles(t, origin, map[string]string{
		"src/main.go": "package main\n",
		"README.md":   "# project\n",
	})

	target := filepath.Join(t.TempDir(), "project")
	if _, _, _, err := CloneOrUpdateRepositoryDirect(origin, target, ""); err != nil {
		t.Fatalf("first clone: %v", err)
	}

	// Work in progress the user has not committed.
	const mainDirty = "package main\n\nfunc main() {}\n"
	if err := os.WriteFile(filepath.Join(target, "src/main.go"), []byte(mainDirty), 0o600); err != nil {
		t.Fatalf("modify src/main.go: %v", err)
	}
	untracked := filepath.Join(target, "NOTES.md")
	if err := os.WriteFile(untracked, []byte("mine\n"), 0o600); err != nil {
		t.Fatalf("write untracked file: %v", err)
	}
	deleted := filepath.Join(target, "README.md")
	if err := os.Remove(deleted); err != nil {
		t.Fatalf("remove tracked file: %v", err)
	}

	_, _, status, err := CloneOrUpdateRepositoryDirect(origin, target, "")
	if err != nil {
		t.Fatalf("second clone: %v", err)
	}
	if status != "Already up to date" {
		t.Errorf("status = %q, want %q", status, "Already up to date")
	}
	assertFileContent(t, deleted, "# project\n")
	assertFileContent(t, filepath.Join(target, "src/main.go"), mainDirty)
	assertFileContent(t, untracked, "mine\n")
}

func TestDeletedTrackedPaths(t *testing.T) {
	origin := newOriginRepo(t, "main")
	target := filepath.Join(t.TempDir(), "dotfiles")
	if _, _, _, err := CloneOrUpdateRepository(origin, target, ""); err != nil {
		t.Fatalf("clone: %v", err)
	}

	if got, err := deletedTrackedPaths(target); err != nil {
		t.Fatalf("deletedTrackedPaths on a clean clone: %v", err)
	} else if len(got) != 0 {
		t.Errorf("clean clone reported deletions %v, want none", got)
	}

	// A modification is not a deletion — it is user work, not a repair target.
	if err := os.WriteFile(filepath.Join(target, "README.md"), []byte("# edited\n"), 0o600); err != nil {
		t.Fatalf("modify README.md: %v", err)
	}
	if got, err := deletedTrackedPaths(target); err != nil {
		t.Fatalf("deletedTrackedPaths with a modification: %v", err)
	} else if len(got) != 0 {
		t.Errorf("modified file reported as deleted: %v", got)
	}

	if err := os.Remove(filepath.Join(target, "README.md")); err != nil {
		t.Fatalf("remove tracked file: %v", err)
	}
	if got, err := deletedTrackedPaths(target); err != nil {
		t.Fatalf("deletedTrackedPaths with a deletion: %v", err)
	} else if len(got) != 1 || got[0] != "README.md" {
		t.Errorf("deletedTrackedPaths = %v, want [README.md]", got)
	}

	// A path that is not a repository must not be reported as having deletions.
	if _, err := deletedTrackedPaths(filepath.Join(t.TempDir(), "not-a-repo")); err == nil {
		t.Error("deletedTrackedPaths on a non-repository: want an error")
	}
}
