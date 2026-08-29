package git

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
)

// #031: go-git's HardReset deletes untracked files.
//
// Real `git reset --hard` never touches untracked files. go-git v5.19.1's
// resetWorktree maps worktree files absent from the index to a Delete action,
// so the update path's Reset{HardReset} destroys them. The protection here
// moves untracked paths aside before the reset and puts them back after —
// the files are physically absent while the reset runs, so no go-git
// regression can reach them either.
//
// Crash-safety: every step leaves the data on disk somewhere recoverable.
//   - A crash before or during the move leaves files in the worktree (the
//     reset has not run).
//   - A crash between move and restore leaves everything in the backup
//     directory, which is named after the clone and sits beside it.
//   - A crash during the restore leaves moved-back files in the worktree and
//     the rest in the backup directory.
// The backup is removed only after every entry was restored; a leftover
// `.blueprint-untracked-backup-*` directory therefore means user data waiting
// for manual recovery, and later runs surface it as a note.

// backupDirPrefix is the backup directory name prefix. The clone's base name
// and a nanosecond timestamp follow it, in the clone's PARENT directory —
// inside the worktree the reset would delete the backup along with everything
// else untracked.
const backupDirPrefix = ".blueprint-untracked-backup-"

// maxNotePaths caps how many protected paths are listed in a note before the
// count takes over; a dotfiles clone can hold hundreds of untracked files.
const maxNotePaths = 5

// untrackedBackup holds untracked paths moved out of a worktree.
type untrackedBackup struct {
	// dir is the absolute backup directory (in the worktree's parent).
	dir string
	// entries are worktree-relative paths of everything moved: files,
	// symlinks and whole nested-repository directories, sorted.
	entries []string
}

// addNotes appends to an optional notes accumulator (nil = caller ignores
// notes). The accumulator is threaded from cloneOrUpdateRepository to the
// seam's CloneResult.Notes.
func addNotes(notes *[]string, add ...string) {
	if notes != nil {
		*notes = append(*notes, add...)
	}
}

// protectUntrackedFiles enumerates the worktree's untracked paths and moves
// them into a fresh backup directory, so a HardReset cannot delete them.
//
// Failure policy (#031 is a data-loss fix — fail loudly, never proceed into a
// reset that might destroy files we could not account for):
//   - Enumeration failure is an error: the update aborts with the worktree
//     untouched.
//   - A partial move is undone (best effort) and reported as an error.
//   - Stale backups from earlier interrupted updates are noted, never touched.
//
// Returns a nil backup (with no error) when there is nothing to protect.
func protectUntrackedFiles(repo *git.Repository, worktree string) (*untrackedBackup, []string, error) {
	var notes []string

	// Leftovers from an earlier interrupted update: user data waiting to be
	// recovered by hand. Checked before creating this run's backup so it is
	// never listed itself.
	if stale := staleBackupDirs(worktree); len(stale) > 0 {
		notes = append(notes, fmt.Sprintf(
			"found untracked-file backup from an earlier interrupted update of %s: %s",
			worktree, strings.Join(stale, ", ")))
	}

	untracked, err := untrackedWorktreePaths(repo, worktree)
	if err != nil {
		return nil, notes, fmt.Errorf(
			"could not enumerate untracked files in %s — refusing to reset: %w", worktree, err)
	}
	if len(untracked) == 0 {
		return nil, notes, nil
	}

	backup, err := moveUntrackedToBackup(worktree, untracked)
	if err != nil {
		return nil, notes, err
	}
	return backup, notes, nil
}

// untrackedWorktreePaths returns the worktree-relative paths of entries
// present in the working tree but absent from the index: files, symlinks and
// nested-repository directories. Ignored (.gitignore'd) files are untracked
// too and are included — real `git reset --hard` leaves them alone as well.
//
// This is a targeted index-vs-disk diff, not worktree.Status(): Status hashes
// the content of every file, tracked and untracked alike (see the #030
// benchmark), while this walk reads the index once and then only directory
// entries — no file content is ever read.
func untrackedWorktreePaths(repo *git.Repository, worktree string) ([]string, error) {
	idx, err := repo.Storer.Index()
	if err != nil {
		return nil, fmt.Errorf("read index: %w", err)
	}
	// Index entry names are slash-separated regardless of OS.
	tracked := make(map[string]bool, len(idx.Entries))
	for _, e := range idx.Entries {
		tracked[e.Name] = true
	}

	var untracked []string
	err = filepath.WalkDir(worktree, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// An unreadable subtree means the untracked set is incomplete;
			// protectUntrackedFiles refuses the reset rather than risk it.
			return walkErr
		}
		rel, relErr := filepath.Rel(worktree, p)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			// A directory with its own .git is a nested repository (a repo
			// the user cloned inside the clone, or an unregistered
			// submodule). Protect it as a unit: walking inside would shred
			// its worktree file by file.
			if _, err := os.Lstat(filepath.Join(p, ".git")); err == nil {
				untracked = append(untracked, rel)
				return fs.SkipDir
			}
			return nil
		}
		if d.Name() == ".git" {
			return nil // .git file (linked-worktree gitfile) — never ours
		}
		if !tracked[filepath.ToSlash(rel)] {
			untracked = append(untracked, rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk worktree: %w", err)
	}
	return untracked, nil
}

// moveUntrackedToBackup renames each untracked path into a fresh timestamped
// backup directory beside the worktree, mirroring the worktree layout.
func moveUntrackedToBackup(worktree string, untracked []string) (*untrackedBackup, error) {
	dir := filepath.Join(filepath.Dir(worktree),
		fmt.Sprintf("%s%s-%d", backupDirPrefix, filepath.Base(worktree), time.Now().UnixNano()))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create untracked-file backup %s: %w", dir, err)
	}

	backup := &untrackedBackup{dir: dir, entries: append([]string(nil), untracked...)}
	sort.Strings(backup.entries)

	for i, rel := range backup.entries {
		if err := moveEntry(filepath.Join(worktree, rel), filepath.Join(dir, rel)); err != nil {
			// Put back what was already moved — an aborted update must not
			// leave user files in a hidden backup directory. Whatever cannot
			// be put back stays in the backup (recoverable, see restore).
			backup.entries = backup.entries[:i]
			kept := backup.restore(worktree)
			if len(kept) == 0 {
				_ = os.RemoveAll(dir)
			}
			return nil, fmt.Errorf("move untracked path %s aside: %w", rel, err)
		}
	}
	return backup, nil
}

// moveEntry renames one entry (file, symlink or directory) into the backup,
// creating its parent directories there as needed.
func moveEntry(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return fmt.Errorf("create backup parent directory: %w", err)
	}
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// restore moves every backed-up entry back into the worktree.
//
// Collision rule: when the target path now exists — the update checked out a
// tracked file where the user had an untracked one — the checked-out file
// wins and the user's copy stays in the backup. The alternative (restoring
// the user's file under a suffixed name) would plant files the user never
// created into the clone, and those would themselves be untracked files the
// next update has to reason about.
//
// The returned strings are the entries still in the backup: collisions and
// restore failures. An empty slice means every entry is back in the worktree
// and the backup directory can be removed.
func (b *untrackedBackup) restore(worktree string) []string {
	var kept []string
	for _, rel := range b.entries {
		target := filepath.Join(worktree, rel)
		if _, err := os.Lstat(target); err == nil {
			kept = append(kept, rel) // collision — checked-out file wins
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not recreate parent directory for %s: %v\n", rel, err)
			kept = append(kept, rel)
			continue
		}
		if err := os.Rename(filepath.Join(b.dir, rel), target); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not restore untracked path %s: %v\n", rel, err)
			kept = append(kept, rel)
		}
	}
	return kept
}

// notes renders the protection outcome for the seam's Notes channel
// (CloneResult.Notes): what was protected, and which copies stayed in the
// backup because their path now holds a file from the update.
func (b *untrackedBackup) notes(worktree string, kept []string) []string {
	shown := b.entries
	suffix := ""
	if len(shown) > maxNotePaths {
		shown = shown[:maxNotePaths]
		suffix = fmt.Sprintf(" (+%d more)", len(b.entries)-maxNotePaths)
	}
	plural := "s"
	if len(b.entries) == 1 {
		plural = ""
	}
	notes := []string{fmt.Sprintf("protected %d untracked file%s from reset in %s: %s%s",
		len(b.entries), plural, worktree, strings.Join(shown, ", "), suffix)}

	for _, rel := range kept {
		notes = append(notes, fmt.Sprintf(
			"kept untracked copy of %s at %s — its path now holds a file added by the update",
			rel, filepath.Join(b.dir, rel)))
	}
	return notes
}

// staleBackupDirs lists backup directories beside the worktree left by
// earlier interrupted updates of the same clone.
func staleBackupDirs(worktree string) []string {
	parent := filepath.Dir(worktree)
	prefix := backupDirPrefix + filepath.Base(worktree) + "-"

	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil
	}
	var stale []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) {
			stale = append(stale, filepath.Join(parent, e.Name()))
		}
	}
	sort.Strings(stale)
	return stale
}
