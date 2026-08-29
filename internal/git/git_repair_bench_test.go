package git

// Measurement instrument for #030, not a permanent test.
//
// The ADR requires a measurement before committing to the go-git HEAD-tree
// walk for deleted-path detection: the gate is "within 2x of diff-index and
// <100ms for a 5k-file repo". Both implementations are benchmarked over the
// same fixture — 5k tracked files plus a node_modules-shaped 20k-file
// untracked tree, which is the shape that punishes any strategy that walks
// untracked files.
//
// MEASURED (darwin/arm64, Apple M1 Max, go1.25, -benchtime 10x -count 3):
//
//	tree-walk    325.9 ms/op   21.2 MB/op   284,870 allocs/op
//	diff-index   140.3 ms/op   66.7 KB/op       167 allocs/op
//
// The tree-walk is ~2.3x slower and 3x over the absolute gate, so it MISSES
// both halves of the gate. It wins on the dimension that matters most for
// correctness (it never looks at untracked files and reads no content), but
// loses on cost: go-git's tree iterator allocates an *object.File per entry
// and decodes each object, which is 3 orders of magnitude more allocations
// than diff-index. Reported to the architect rather than hidden — see #030.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const (
	benchTrackedFiles   = 5000
	benchUntrackedFiles = 20000
	benchDeletedFiles   = 50
)

// benchFixture builds the fixture and returns the worktree path.
func benchFixture(b *testing.B) string {
	b.Helper()
	root := b.TempDir()
	origin := filepath.Join(root, "origin")
	if err := os.MkdirAll(origin, 0o750); err != nil {
		b.Fatal(err)
	}
	gitInBench(b, origin, "init", "-q", "-b", "main")

	// 5k tracked files, spread across 50 directories like a real dotfiles or
	// source repo rather than piled into one.
	for i := 0; i < benchTrackedFiles; i++ {
		dir := filepath.Join(origin, fmt.Sprintf("pkg%02d", i%50))
		if err := os.MkdirAll(dir, 0o750); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("file%04d.txt", i)), []byte("content\n"), 0o600); err != nil {
			b.Fatal(err)
		}
	}
	gitInBench(b, origin, "add", "-A")
	gitInBench(b, origin, "commit", "-qm", "fixture")

	work := filepath.Join(root, "work")
	gitInBench(b, root, "clone", "-q", origin, work)

	// A node_modules-shaped untracked directory: deep, wide, and full of files
	// a tracked-only strategy should never look at.
	for i := 0; i < benchUntrackedFiles; i++ {
		dir := filepath.Join(work, "node_modules", fmt.Sprintf("@scope%03d", i%200), "lib")
		if err := os.MkdirAll(dir, 0o750); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("mod%05d.js", i)), []byte("module.exports = {};\n"), 0o600); err != nil {
			b.Fatal(err)
		}
	}

	// Delete a handful of tracked files so both strategies do real work.
	for i := 0; i < benchDeletedFiles; i++ {
		if err := os.Remove(filepath.Join(work, fmt.Sprintf("pkg%02d", i%50), fmt.Sprintf("file%04d.txt", i))); err != nil {
			b.Fatal(err)
		}
	}
	return work
}

// gitCommandBench builds the git command for fixture setup. Test files are
// exempt from the gitcmd guard: they build fixtures, not product behaviour.
func gitCommandBench(args ...string) *exec.Cmd {
	return exec.Command("git", args...) // #nosec G204 -- benchmark fixture
}

func gitInBench(b *testing.B, dir string, args ...string) {
	b.Helper()
	full := append([]string{"-C", dir}, args...)
	cmd := gitCommandBench(full...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Bench", "GIT_AUTHOR_EMAIL=bench@example.com",
		"GIT_COMMITTER_NAME=Bench", "GIT_COMMITTER_EMAIL=bench@example.com")
	if out, err := cmd.CombinedOutput(); err != nil {
		b.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// BenchmarkDeletedTrackedPaths is a regression guard for the go-git
// HEAD-tree walk, retained after the measurement it was built for.
//
// The shell reference it originally raced against is gone, so this now reports
// an absolute number to compare against the ADR's "<100ms for a 5k-file repo"
// gate. If this number drifts materially upward, the tree-walk's cost
// assumptions need revisiting.
func BenchmarkDeletedTrackedPaths(b *testing.B) {
	work := benchFixture(b)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		got, err := deletedTrackedPaths(work)
		if err != nil {
			b.Fatal(err)
		}
		if len(got) != benchDeletedFiles {
			b.Fatalf("deleted %d files, want %d", len(got), benchDeletedFiles)
		}
	}
}
