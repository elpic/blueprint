package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	git "github.com/go-git/go-git/v5"
)

// ---- #031: go-git's HardReset deletes untracked files ----------------------
//
// Real `git reset --hard` never touches untracked files. go-git v5.19.1's
// HardReset maps worktree files absent from the index to a Delete action, so
// the update path (fetch + Reset{HardReset}) destroys them.
//
// These tests reproduce QA's verified table:
//
//	| File | Local state                 | After a correct update    |
//	|------|-----------------------------|---------------------------|
//	| X    | modified, same size         | rewritten correctly       |
//	| Y    | modified, unchanged upstream| reverted (expected)       |
//	| Z    | deleted locally             | restored                  |
//	| W    | added upstream              | checked out               |
//	| U    | untracked user file         | survives byte-for-byte    |
//
// backupDirs returns the #031 backup directories left beside a clone.
func backupDirs(t *testing.T, target string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(target), ".blueprint-untracked-backup-*"))
	if err != nil {
		t.Fatalf("glob backup dirs: %v", err)
	}
	return matches
}

// TC-U1: QA's full table — especially U (untracked) and X (same-size modified
// tracked file; the protection must not weaken the reset's rewrite of X).
func TestCloneOrUpdateRepository_TC_U1_UntrackedFileSurvivesReset(t *testing.T) {
	origin := newOriginRepo(t, "main")
	commitFiles(t, origin, map[string]string{
		".zshrc":            "# zshrc v1\n", // X: will be modified, same size
		"README.md":         "# readme\nline two\n", // Y: will be modified
		".config/git/config": "# git config\n", // Z: will be deleted
	})

	target := filepath.Join(t.TempDir(), "dotfiles")
	if _, _, _, err := CloneOrUpdateRepository(origin, target, ""); err != nil {
		t.Fatalf("first clone: %v", err)
	}

	// X: same-size modification (stat-cache adversarial) — tracked, must be
	// reverted by the reset regardless.
	if err := os.WriteFile(filepath.Join(target, ".zshrc"), []byte("# zshrc v2\n"), 0o600); err != nil {
		t.Fatalf("modify .zshrc: %v", err)
	}
	// Y: modification of a file upstream never touched — reverted (policy).
	if err := os.WriteFile(filepath.Join(target, "README.md"), []byte("# readme\nline two\nline three\n"), 0o600); err != nil {
		t.Fatalf("modify README.md: %v", err)
	}
	// Z: local deletion of a tracked file — restored.
	if err := os.Remove(filepath.Join(target, ".config/git/config")); err != nil {
		t.Fatalf("delete .config/git/config: %v", err)
	}
	// U: untracked user file — MUST survive the update.
	const userNotes = "local scratch notes\n"
	if err := os.WriteFile(filepath.Join(target, "user-notes.md"), []byte(userNotes), 0o600); err != nil {
		t.Fatalf("write untracked file: %v", err)
	}

	// W: a new file upstream makes the second run a real update (reset runs).
	commitFiles(t, origin, map[string]string{"new-upstream.txt": "upstream content\n"})

	_, _, status, err := CloneOrUpdateRepository(origin, target, "")
	if err != nil {
		t.Fatalf("second clone: %v", err)
	}
	if status != "Updated" {
		t.Errorf("status = %q, want %q", status, "Updated")
	}

	// X rewritten correctly: the same-size modification is reverted.
	assertFileContent(t, filepath.Join(target, ".zshrc"), "# zshrc v1\n")
	// Y reverted (expected — managed-clone policy on tracked files).
	assertFileContent(t, filepath.Join(target, "README.md"), "# readme\nline two\n")
	// Z restored.
	assertFileContent(t, filepath.Join(target, ".config/git/config"), "# git config\n")
	// W checked out.
	assertFileContent(t, filepath.Join(target, "new-upstream.txt"), "upstream content\n")
	// U survives — the bug was its deletion.
	assertFileContent(t, filepath.Join(target, "user-notes.md"), userNotes)

	// A successful update with everything restored leaves no backup behind.
	if dirs := backupDirs(t, target); len(dirs) != 0 {
		t.Errorf("backup dirs left behind after successful update: %v", dirs)
	}
}

// TC-U2: an untracked file inside an untracked directory survives.
func TestCloneOrUpdateRepository_TC_U2_UntrackedDirectorySurvivesReset(t *testing.T) {
	origin := newOriginRepo(t, "main")
	commitFiles(t, origin, map[string]string{".zshrc": "# zshrc\n"})

	target := filepath.Join(t.TempDir(), "dotfiles")
	if _, _, _, err := CloneOrUpdateRepository(origin, target, ""); err != nil {
		t.Fatalf("first clone: %v", err)
	}

	const journal = "dear diary\n"
	if err := os.MkdirAll(filepath.Join(target, "notes"), 0o750); err != nil {
		t.Fatalf("mkdir notes: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "notes/journal.md"), []byte(journal), 0o600); err != nil {
		t.Fatalf("write untracked file: %v", err)
	}

	commitFiles(t, origin, map[string]string{"upstream.txt": "v2\n"})

	if _, _, _, err := CloneOrUpdateRepository(origin, target, ""); err != nil {
		t.Fatalf("second clone: %v", err)
	}
	assertFileContent(t, filepath.Join(target, "notes/journal.md"), journal)
	if dirs := backupDirs(t, target); len(dirs) != 0 {
		t.Errorf("backup dirs left behind after successful update: %v", dirs)
	}
}

// TC-U3: an untracked file whose path is added upstream. The upstream file
// must win at that path; the user's copy is preserved in the backup directory.
func TestCloneOrUpdateRepository_TC_U3_UntrackedCollisionWithUpstreamFile(t *testing.T) {
	origin := newOriginRepo(t, "main")
	commitFiles(t, origin, map[string]string{".zshrc": "# zshrc\n"})

	target := filepath.Join(t.TempDir(), "dotfiles")
	if _, _, _, err := CloneOrUpdateRepository(origin, target, ""); err != nil {
		t.Fatalf("first clone: %v", err)
	}

	const userDraft = "user's local draft\n"
	if err := os.WriteFile(filepath.Join(target, "feature.txt"), []byte(userDraft), 0o600); err != nil {
		t.Fatalf("write untracked file: %v", err)
	}

	// Upstream adds a tracked file at the same path.
	commitFiles(t, origin, map[string]string{"feature.txt": "upstream feature\n"})

	if _, _, _, err := CloneOrUpdateRepository(origin, target, ""); err != nil {
		t.Fatalf("second clone: %v", err)
	}

	// The upstream file wins at the path.
	assertFileContent(t, filepath.Join(target, "feature.txt"), "upstream feature\n")

	// The user's copy is preserved in a backup directory, not destroyed.
	dirs := backupDirs(t, target)
	if len(dirs) != 1 {
		t.Fatalf("backup dirs = %v, want exactly one (the collision keeps the backup)", dirs)
	}
	assertFileContent(t, filepath.Join(dirs[0], "feature.txt"), userDraft)
}

// TC-U4: an untracked symlink survives the reset as a symlink.
func TestCloneOrUpdateRepository_TC_U4_UntrackedSymlinkSurvivesReset(t *testing.T) {
	origin := newOriginRepo(t, "main")
	commitFiles(t, origin, map[string]string{".zshrc": "# zshrc\n"})

	target := filepath.Join(t.TempDir(), "dotfiles")
	if _, _, _, err := CloneOrUpdateRepository(origin, target, ""); err != nil {
		t.Fatalf("first clone: %v", err)
	}

	if err := os.Symlink(".zshrc", filepath.Join(target, ".zshrc.local")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	commitFiles(t, origin, map[string]string{"upstream.txt": "v2\n"})

	if _, _, _, err := CloneOrUpdateRepository(origin, target, ""); err != nil {
		t.Fatalf("second clone: %v", err)
	}

	info, err := os.Lstat(filepath.Join(target, ".zshrc.local"))
	if err != nil {
		t.Fatalf("untracked symlink deleted by update: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf(".zshrc.local is no longer a symlink (mode %v)", info.Mode())
	}
	if got, err := os.Readlink(filepath.Join(target, ".zshrc.local")); err != nil || got != ".zshrc" {
		t.Errorf(".zshrc.local link target = %q, %v; want %q", got, err, ".zshrc")
	}
}

// TC-U5: a nested repository inside the clone is protected as a unit, not
// shredded file by file.
func TestCloneOrUpdateRepository_TC_U5_NestedRepoSurvivesReset(t *testing.T) {
	origin := newOriginRepo(t, "main")
	commitFiles(t, origin, map[string]string{".zshrc": "# zshrc\n"})

	target := filepath.Join(t.TempDir(), "dotfiles")
	if _, _, _, err := CloneOrUpdateRepository(origin, target, ""); err != nil {
		t.Fatalf("first clone: %v", err)
	}

	// A nested git repository the user cloned inside the dotfiles clone.
	nested := filepath.Join(target, "vendored")
	nestedOrigin := newOriginRepo(t, "main")
	clone := exec.Command("git", "clone", "-q", nestedOrigin, nested) // #nosec G204 -- test fixture
	if out, err := clone.CombinedOutput(); err != nil {
		t.Fatalf("clone nested repo: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(nested, "local.txt"), []byte("nested work\n"), 0o600); err != nil {
		t.Fatalf("write file in nested repo: %v", err)
	}

	commitFiles(t, origin, map[string]string{"upstream.txt": "v2\n"})

	if _, _, _, err := CloneOrUpdateRepository(origin, target, ""); err != nil {
		t.Fatalf("second clone: %v", err)
	}

	// The nested repo is intact: .git still there, working file still there.
	if _, err := os.Stat(filepath.Join(nested, ".git")); err != nil {
		t.Errorf("nested repository's .git missing after update: %v", err)
	}
	assertFileContent(t, filepath.Join(nested, "local.txt"), "nested work\n")
	if got := gitIn(t, nested, "rev-parse", "--is-inside-work-tree"); got != "true" {
		t.Errorf("nested repo no longer a working tree: %q", got)
	}
}

// TC-U6: protection events surface as Notes through the seam's channel
// (CloneResult.Notes via CloneOrUpdateRepositoryDirectWithNotes), naming the
// clone and the protected paths.
func TestCloneOrUpdateRepository_TC_U6_ProtectionNotes(t *testing.T) {
	origin := newOriginRepo(t, "main")
	commitFiles(t, origin, map[string]string{".zshrc": "# zshrc\n"})

	target := filepath.Join(t.TempDir(), "dotfiles")
	if _, _, _, err := CloneOrUpdateRepositoryDirect(origin, target, ""); err != nil {
		t.Fatalf("first clone: %v", err)
	}

	if err := os.WriteFile(filepath.Join(target, "user-notes.md"), []byte("mine\n"), 0o600); err != nil {
		t.Fatalf("write untracked file: %v", err)
	}
	commitFiles(t, origin, map[string]string{"upstream.txt": "v2\n"})

	_, _, _, notes, err := CloneOrUpdateRepositoryDirectWithNotes(origin, target, "")
	if err != nil {
		t.Fatalf("second clone: %v", err)
	}
	wantNote := "protected 1 untracked file from reset in " + target + ": user-notes.md"
	found := false
	for _, n := range notes {
		if n == wantNote {
			found = true
		}
	}
	if !found {
		t.Errorf("notes = %q, want to contain %q", notes, wantNote)
	}
	// Nothing was left in a backup — no collision, no failure.
	for _, n := range notes {
		if strings.Contains(n, "kept untracked copy") {
			t.Errorf("unexpected kept-copy note: %q", n)
		}
	}
	if dirs := backupDirs(t, target); len(dirs) != 0 {
		t.Errorf("backup dirs left behind: %v", dirs)
	}
}

// TC-U7: the collision note tells the user where their copy landed.
func TestCloneOrUpdateRepository_TC_U7_CollisionNote(t *testing.T) {
	origin := newOriginRepo(t, "main")
	commitFiles(t, origin, map[string]string{".zshrc": "# zshrc\n"})

	target := filepath.Join(t.TempDir(), "dotfiles")
	if _, _, _, err := CloneOrUpdateRepository(origin, target, ""); err != nil {
		t.Fatalf("first clone: %v", err)
	}

	if err := os.WriteFile(filepath.Join(target, "feature.txt"), []byte("draft\n"), 0o600); err != nil {
		t.Fatalf("write untracked file: %v", err)
	}
	commitFiles(t, origin, map[string]string{"feature.txt": "upstream\n"})

	var notes []string
	if _, _, _, err := cloneOrUpdateRepository(origin, target, "", &notes); err != nil {
		t.Fatalf("second clone: %v", err)
	}

	keptNote := ""
	for _, n := range notes {
		if strings.Contains(n, "kept untracked copy of feature.txt") {
			keptNote = n
		}
	}
	if keptNote == "" {
		t.Fatalf("notes = %q, want a kept-copy note for feature.txt", notes)
	}
	// The note names where the user's copy is recoverable.
	if !strings.Contains(keptNote, filepath.Join(filepath.Dir(target), backupDirPrefix)) {
		t.Errorf("kept-copy note %q does not name the backup directory", keptNote)
	}
}

// TC-U8: a backup directory left by an earlier interrupted update is surfaced
// as a note so the user can recover the files by hand.
func TestCloneOrUpdateRepository_TC_U8_StaleBackupNoted(t *testing.T) {
	origin := newOriginRepo(t, "main")
	commitFiles(t, origin, map[string]string{".zshrc": "# zshrc\n"})

	target := filepath.Join(t.TempDir(), "dotfiles")
	if _, _, _, err := CloneOrUpdateRepository(origin, target, ""); err != nil {
		t.Fatalf("first clone: %v", err)
	}

	// Simulate the debris of an interrupted update: a timestamped backup dir
	// beside the clone, holding a user file that never made it back.
	stale := filepath.Join(filepath.Dir(target), backupDirPrefix+"dotfiles-1690000000000000000")
	if err := os.MkdirAll(stale, 0o750); err != nil {
		t.Fatalf("mkdir stale backup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stale, "orphan.md"), []byte("stale\n"), 0o600); err != nil {
		t.Fatalf("write stale backup file: %v", err)
	}

	commitFiles(t, origin, map[string]string{"upstream.txt": "v2\n"})

	var notes []string
	if _, _, _, err := cloneOrUpdateRepository(origin, target, "", &notes); err != nil {
		t.Fatalf("second clone: %v", err)
	}
	found := false
	for _, n := range notes {
		if strings.Contains(n, "found untracked-file backup from an earlier interrupted update") &&
			strings.Contains(n, stale) {
			found = true
		}
	}
	if !found {
		t.Errorf("notes = %q, want a stale-backup note naming %s", notes, stale)
	}
	// The stale backup is surfaced, never deleted.
	if _, err := os.Stat(filepath.Join(stale, "orphan.md")); err != nil {
		t.Errorf("stale backup was deleted: %v", err)
	}
}

// TC-U9 (crash simulation): a backup directory on disk is recoverable by a
// fresh process that knows nothing but the backup layout — the crash-safety
// contract from git_untracked.go. The "crash" happens right after
// moveUntrackedToBackup, before any reset ran.
func TestUntrackedBackup_CrashRestore(t *testing.T) {
	origin := newOriginRepo(t, "main")
	commitFiles(t, origin, map[string]string{".zshrc": "# zshrc\n"})

	target := filepath.Join(t.TempDir(), "dotfiles")
	if _, _, _, err := CloneOrUpdateRepository(origin, target, ""); err != nil {
		t.Fatalf("first clone: %v", err)
	}

	const journal = "crash survivor\n"
	if err := os.MkdirAll(filepath.Join(target, "notes"), 0o750); err != nil {
		t.Fatalf("mkdir notes: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "notes/journal.md"), []byte(journal), 0o600); err != nil {
		t.Fatalf("write untracked file: %v", err)
	}

	// "Process 1": enumerate and move aside, then die (never reset, never
	// restore). The backup directory is the only surviving state.
	repo, err := git.PlainOpen(target)
	if err != nil {
		t.Fatalf("open repo: %v", err)
	}
	untracked, err := untrackedWorktreePaths(repo, target)
	if err != nil {
		t.Fatalf("enumerate untracked: %v", err)
	}
	backup, err := moveUntrackedToBackup(target, untracked)
	if err != nil {
		t.Fatalf("move untracked aside: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "notes/journal.md")); !os.IsNotExist(err) {
		t.Fatalf("journal.md should be out of the worktree after the move, stat err = %v", err)
	}

	// "Process 2": rediscover the backup from disk and restore it.
	discovered := backupDirs(t, target)
	if len(discovered) != 1 || discovered[0] != backup.dir {
		t.Fatalf("rediscovered backup dirs = %v, want [%s]", discovered, backup.dir)
	}
	fresh := &untrackedBackup{dir: discovered[0], entries: []string{"notes/journal.md"}}
	if kept := fresh.restore(target); len(kept) != 0 {
		t.Fatalf("restore kept entries %v, want none", kept)
	}
	assertFileContent(t, filepath.Join(target, "notes/journal.md"), journal)
}
