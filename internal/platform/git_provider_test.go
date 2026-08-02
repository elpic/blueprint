package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRealGitProvider_IsRepository(t *testing.T) {
	g := &realGitProvider{}

	t.Run("directory containing .git", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
			t.Fatalf("Mkdir .git: %v", err)
		}
		if !g.IsRepository(dir) {
			t.Errorf("IsRepository(%q) = false, want true", dir)
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		dir := t.TempDir()
		if g.IsRepository(dir) {
			t.Errorf("IsRepository(%q) = true, want false", dir)
		}
	})

	t.Run(".git as regular file (worktree/gitfile)", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: ../.git/worktrees/foo\n"), 0o644); err != nil {
			t.Fatalf("WriteFile .git: %v", err)
		}
		if !g.IsRepository(dir) {
			t.Errorf("IsRepository(%q) = false, want true for a worktree .git file", dir)
		}
	})

	t.Run("nonexistent path", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "does-not-exist")
		if g.IsRepository(path) {
			t.Errorf("IsRepository(%q) = true, want false", path)
		}
	})
}

func TestRealGitProvider_Update_NotARepository(t *testing.T) {
	g := &realGitProvider{}

	// Update on a non-repository directory must error out (no origin to read).
	dir := t.TempDir()
	if _, err := g.Update(dir, "main"); err == nil {
		t.Errorf("Update(%q) = nil error, want error", dir)
	}
}
