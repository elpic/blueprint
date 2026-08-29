package git

// Table tests for the #030 go-git worktree repair: deleted-path detection and
// the restore that follows it.
//
// These cases began as a golden-master differential test against the shell
// implementation the repair was ported from. Once that reference was deleted
// the expectations stayed: each case pins a semantic the port had to preserve,
// and several (dangling symlink, skip-worktree, staged deletion) are exactly
// the ones a naive rewrite gets wrong.

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// repairFixture describes a worktree state to compare both implementations on.
type repairFixture struct {
	name string
	// files are committed at HEAD.
	files map[string]string
	// mutate shapes the cloned worktree into the state under test.
	mutate func(t *testing.T, work string)
	// wantDeleted is the set of repo-relative paths both implementations must
	// report as deleted.
	wantDeleted []string
	// wantContent, when set, is asserted after the go-git restore: path ->
	// exact bytes. Used for the byte-identity cases (CRLF).
	wantContent map[string]string
	// wantStatus, when non-nil, is asserted against `git status --porcelain`
	// after the go-git restore.
	wantStatus []string
}

func repairFixtures() []repairFixture {
	return []repairFixture{
		{
			name:  "deleted file",
			files: map[string]string{".config/nushell/config.nu": "# nushell config\n"},
			mutate: func(t *testing.T, work string) {
				rm(t, filepath.Join(work, ".config/nushell/config.nu"))
			},
			wantDeleted: []string{".config/nushell/config.nu"},
			wantContent: map[string]string{".config/nushell/config.nu": "# nushell config\n"},
		},
		{
			// A deleted directory takes its parents with it: this is the case
			// that forces os.MkdirAll in the go-git restore.
			name: "deleted file inside deleted directory",
			files: map[string]string{
				".config/nushell/config.nu": "# config\n",
				".config/nushell/env.nu":    "# env\n",
				".config/starship.toml":     "format = \"$all\"\n",
			},
			mutate: func(t *testing.T, work string) {
				rmAll(t, filepath.Join(work, ".config/nushell"))
			},
			wantDeleted: []string{".config/nushell/config.nu", ".config/nushell/env.nu"},
			wantContent: map[string]string{
				".config/nushell/config.nu": "# config\n",
				".config/nushell/env.nu":    "# env\n",
			},
		},
		{
			// The data-loss guard: an untracked file is not a deletion and must
			// never be reported or touched.
			name:  "untracked file present",
			files: map[string]string{".zshrc": "# tracked\n"},
			mutate: func(t *testing.T, work string) {
				write(t, filepath.Join(work, "notes.txt"), "untracked\n")
			},
			wantDeleted: nil,
			wantContent: map[string]string{"notes.txt": "untracked\n"},
		},
		{
			// The other half of the data-loss guard: a modified tracked file is
			// present on disk, so it is not a deletion and must survive intact.
			name:  "modified tracked file",
			files: map[string]string{".zshrc": "# committed\n"},
			mutate: func(t *testing.T, work string) {
				write(t, filepath.Join(work, ".zshrc"), "# locally edited\n")
			},
			wantDeleted: nil,
			wantContent: map[string]string{".zshrc": "# locally edited\n"},
		},
		{
			// A staged deletion is still a deletion — the file is missing from
			// the worktree. The repair must restore the file while leaving the
			// index alone, so status shows the staged `D ` plus an untracked `??`.
			name:  "staged deletion",
			files: map[string]string{".zshrc": "# tracked\n", "keep.txt": "keep\n"},
			mutate: func(t *testing.T, work string) {
				gitIn(t, work, "rm", "--cached", "--quiet", ".zshrc")
				rm(t, filepath.Join(work, ".zshrc"))
			},
			wantDeleted: []string{".zshrc"},
			wantContent: map[string]string{".zshrc": "# tracked\n"},
			wantStatus:  []string{"D  .zshrc", "?? .zshrc"},
		},
		{
			// Lstat, not Stat: a symlink whose target is missing is present on
			// disk. Stat would follow the dangling link and call it a deletion.
			name:  "symlink with dangling target",
			files: map[string]string{"real.txt": "real\n"},
			mutate: func(t *testing.T, work string) {
				commitSymlink(t, work, "link.txt", "does-not-exist")
			},
			wantDeleted: nil,
			wantStatus:  []string{},
		},
		{
			// Sparse checkout: a skip-worktree entry is absent from the
			// worktree by design, so it must not be reported or materialised.
			name:  "skip-worktree entry",
			files: map[string]string{"sparse/big.txt": "big\n", "keep.txt": "keep\n"},
			mutate: func(t *testing.T, work string) {
				gitIn(t, work, "update-index", "--skip-worktree", "sparse/big.txt")
				rmAll(t, filepath.Join(work, "sparse"))
			},
			wantDeleted: nil,
		},
		{
			// Byte-identity: go-git has no clean/smudge engine, so the restored
			// bytes must match HEAD exactly — no CRLF translation.
			name: "CRLF file restored byte-identical",
			files: map[string]string{
				".gitattributes": "* -text\n",
				"crlf.txt":       "line one\r\nline two\r\n",
			},
			mutate: func(t *testing.T, work string) {
				rm(t, filepath.Join(work, "crlf.txt"))
			},
			wantDeleted: []string{"crlf.txt"},
			wantContent: map[string]string{"crlf.txt": "line one\r\nline two\r\n"},
		},
	}
}

// TestRepairDetectsDeletedPaths asserts deletedTrackedPaths reports exactly
// the paths each fixture expects — and, crucially, nothing else.
func TestRepairDetectsDeletedPaths(t *testing.T) {
	for _, fx := range repairFixtures() {
		t.Run(fx.name, func(t *testing.T) {
			work := buildFixture(t, fx)

			got, err := deletedTrackedPaths(work)
			if err != nil {
				t.Fatalf("deletedTrackedPaths: %v", err)
			}
			assertSamePaths(t, "deleted", fx.wantDeleted, got)
		})
	}
}

// TestRepairRestoresDeletedPaths runs the full repair and asserts the resulting
// worktree: restored bytes, untouched user files, and (where the fixture cares)
// the exact `git status --porcelain` output.
func TestRepairRestoresDeletedPaths(t *testing.T) {
	for _, fx := range repairFixtures() {
		t.Run(fx.name, func(t *testing.T) {
			work := buildFixture(t, fx)

			if err := repairDeletedFiles(work); err != nil {
				t.Fatalf("repairDeletedFiles: %v", err)
			}

			if fx.wantStatus != nil {
				assertSamePaths(t, "status", fx.wantStatus, statusEntries(t, work))
			}
			for rel, want := range fx.wantContent {
				if got := readFile(t, filepath.Join(work, rel)); got != want {
					t.Errorf("%s = %q, want %q", rel, got, want)
				}
			}
			// No .blueprint-restore-* temp file may survive the rename.
			if leftovers := strayTempFiles(t, work); len(leftovers) > 0 {
				t.Errorf("repair left temp files behind: %v", leftovers)
			}
		})
	}
}

// strayTempFiles returns any temp file the atomic write failed to rename.
func strayTempFiles(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasPrefix(info.Name(), ".blueprint-restore-") {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return out
}

// buildFixture creates an origin repo, clones it and applies fx.mutate.
func buildFixture(t *testing.T, fx repairFixture) string {
	t.Helper()
	origin := newOriginRepo(t, "main")
	if fx.files != nil {
		commitFiles(t, origin, fx.files)
	}
	work := filepath.Join(t.TempDir(), "work")
	if _, _, _, err := CloneOrUpdateRepository(origin, work, ""); err != nil {
		t.Fatalf("clone: %v", err)
	}
	if fx.mutate != nil {
		fx.mutate(t, work)
	}
	return work
}

// commitSymlink creates a symlink and commits it so HEAD records it as 120000.
func commitSymlink(t *testing.T, work, name, target string) {
	t.Helper()
	if err := os.Symlink(target, filepath.Join(work, name)); err != nil {
		t.Fatalf("symlink %s: %v", name, err)
	}
	commitFiles(t, work, nil)
	// The link target deliberately does not exist: remove it if git created it.
	_ = os.Remove(filepath.Join(work, target))
}

func rm(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove %s: %v", path, err)
	}
}

func rmAll(t *testing.T, path string) {
	t.Helper()
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("remove all %s: %v", path, err)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertSamePaths(t *testing.T, what string, want, got []string) {
	t.Helper()
	sort.Strings(want)
	sort.Strings(got)
	if len(want) != len(got) {
		t.Fatalf("%s: got %v, want %v", what, got, want)
	}
	for i := range want {
		if want[i] != got[i] {
			t.Fatalf("%s: got %v, want %v", what, got, want)
		}
	}
}
