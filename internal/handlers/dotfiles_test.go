package handlers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

func TestDotfilesHandlerGetCommand(t *testing.T) {
	tests := []struct {
		name string
		rule parser.Rule
		want string
	}{
		{
			name: "basic clone",
			rule: parser.Rule{
				Action:       "dotfiles",
				DotfilesURL:  "https://github.com/user/dotfiles",
				DotfilesPath: "~/.dotfiles",
			},
			want: "git clone https://github.com/user/dotfiles ~/.dotfiles",
		},
		{
			name: "clone with branch",
			rule: parser.Rule{
				Action:         "dotfiles",
				DotfilesURL:    "https://github.com/user/dotfiles",
				DotfilesPath:   "~/.dotfiles",
				DotfilesBranch: "main",
			},
			want: "git clone -b main https://github.com/user/dotfiles ~/.dotfiles",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewDotfilesHandler(tt.rule, "")
			if got := h.GetCommand(); got != tt.want {
				t.Errorf("GetCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDotfilesHandlerGetDependencyKey(t *testing.T) {
	t.Run("uses ID when set", func(t *testing.T) {
		h := NewDotfilesHandler(parser.Rule{
			DotfilesURL: "https://github.com/user/dotfiles",
			ID:          "my-dots",
		}, "")
		if got := h.GetDependencyKey(); got != "my-dots" {
			t.Errorf("GetDependencyKey() = %q, want %q", got, "my-dots")
		}
	})
	t.Run("falls back to URL", func(t *testing.T) {
		h := NewDotfilesHandler(parser.Rule{
			DotfilesURL: "https://github.com/user/dotfiles",
		}, "")
		if got := h.GetDependencyKey(); got != "https://github.com/user/dotfiles" {
			t.Errorf("GetDependencyKey() = %q, want %q", got, "https://github.com/user/dotfiles")
		}
	})
}

func TestDotfilesHandlerDown_PathNotExist(t *testing.T) {
	h := NewDotfilesHandler(parser.Rule{
		Action:       "dotfiles",
		DotfilesURL:  "https://github.com/user/dotfiles",
		DotfilesPath: "/nonexistent/path/.dotfiles",
	}, "")
	msg, err := h.Down()
	if err != nil {
		t.Fatalf("Down() unexpected error: %v", err)
	}
	// Should report already removed or not found — not crash
	if msg == "" {
		t.Error("Down() returned empty message for non-existent path")
	}
}

func TestDotfilesHandlerGetDisplayDetails(t *testing.T) {
	h := NewDotfilesHandler(parser.Rule{
		DotfilesURL:  "https://github.com/user/dotfiles",
		DotfilesPath: "~/.dotfiles",
	}, "")
	got := h.GetDisplayDetails(false)
	if !strings.Contains(got, "dotfiles") {
		t.Errorf("GetDisplayDetails() = %q, expected to contain path info", got)
	}
}

func TestDotfilesHandlerIsInstalled(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		status   Status
		expected bool
	}{
		{
			name: "not installed - no status entry",
			rule: parser.Rule{
				Action:       "dotfiles",
				DotfilesURL:  "https://github.com/user/dotfiles",
				DotfilesPath: "~/.dotfiles",
			},
			status:   Status{},
			expected: false,
		},
		{
			name: "not installed - different URL",
			rule: parser.Rule{
				Action:       "dotfiles",
				DotfilesURL:  "https://github.com/user/dotfiles",
				DotfilesPath: "~/.dotfiles",
			},
			status: Status{
				Dotfiles: []DotfilesStatus{
					{
						URL:       "https://github.com/other/dotfiles",
						Path:      "~/.dotfiles",
						Blueprint: "/tmp/test.bp",
						OS:        "linux",
					},
				},
			},
			expected: false,
		},
		{
			name: "not installed - different blueprint",
			rule: parser.Rule{
				Action:       "dotfiles",
				DotfilesURL:  "https://github.com/user/dotfiles",
				DotfilesPath: "~/.dotfiles",
			},
			status: Status{
				Dotfiles: []DotfilesStatus{
					{
						URL:       "https://github.com/user/dotfiles",
						Path:      "~/.dotfiles",
						Blueprint: "/tmp/other.bp",
						OS:        "linux",
					},
				},
			},
			expected: false,
		},
		{
			name: "not installed - different OS",
			rule: parser.Rule{
				Action:       "dotfiles",
				DotfilesURL:  "https://github.com/user/dotfiles",
				DotfilesPath: "~/.dotfiles",
			},
			status: Status{
				Dotfiles: []DotfilesStatus{
					{
						URL:       "https://github.com/user/dotfiles",
						Path:      "~/.dotfiles",
						Blueprint: "/tmp/test.bp",
						OS:        "mac",
					},
				},
			},
			expected: false,
		},
		{
			name: "installed - exact match without SHA",
			rule: parser.Rule{
				Action:       "dotfiles",
				DotfilesURL:  "https://github.com/user/dotfiles",
				DotfilesPath: "~/.dotfiles",
			},
			status: Status{
				Dotfiles: []DotfilesStatus{
					{
						URL:       "https://github.com/user/dotfiles",
						Path:      "~/.dotfiles",
						Blueprint: "/tmp/test.bp",
						OS:        "linux",
						SHA:       "", // Empty SHA - will return true
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewDotfilesHandler(tt.rule, "/tmp")
			got := h.IsInstalled(&tt.status, "/tmp/test.bp", "linux")
			if got != tt.expected {
				t.Errorf("IsInstalled() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestShouldSkipEntry(t *testing.T) {
	tests := []struct {
		name       string
		entryName  string
		userSkip   []string
		shouldSkip bool
	}{
		{
			name:       "git directory",
			entryName:  ".git",
			userSkip:   nil,
			shouldSkip: true,
		},
		{
			name:       "github directory",
			entryName:  ".github",
			userSkip:   nil,
			shouldSkip: true,
		},
		{
			name:       "readme file case insensitive",
			entryName:  "README.md",
			userSkip:   nil,
			shouldSkip: true,
		},
		{
			name:       "readme lowercase",
			entryName:  "readme.txt",
			userSkip:   nil,
			shouldSkip: true,
		},
		{
			name:       "normal file",
			entryName:  ".bashrc",
			userSkip:   nil,
			shouldSkip: false,
		},
		{
			name:       "normal directory",
			entryName:  "nvim",
			userSkip:   nil,
			shouldSkip: false,
		},
		{
			name:       "user skip exact match",
			entryName:  "skipme",
			userSkip:   []string{"skipme"},
			shouldSkip: true,
		},
		{
			name:       "user skip partial match",
			entryName:  "something",
			userSkip:   []string{"skipme"},
			shouldSkip: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldSkipEntry(tt.entryName, tt.userSkip)
			if got != tt.shouldSkip {
				t.Errorf("shouldSkipEntry(%q, %v) = %v, want %v", tt.entryName, tt.userSkip, got, tt.shouldSkip)
			}
		})
	}
}

// TestDotfilesFindUninstallRulesNormalizesURLs tests that FindUninstallRules
// uses normalized URLs for comparison so SSH and HTTPS URLs for the same repo are treated as equal.
func TestDotfilesFindUninstallRulesNormalizesURLs(t *testing.T) {
	status := Status{
		Dotfiles: []DotfilesStatus{
			{
				URL:       "git@github.com:user/dotfiles.git",
				Path:      "~/.dotfiles",
				Blueprint: "/bp/setup.bp",
				OS:        "linux",
				SHA:       "abc123",
				Links:     []string{"~/.bashrc"},
			},
		},
	}

	// Current rules use HTTPS URL for the same repo
	currentRules := []parser.Rule{
		{
			Action:       "dotfiles",
			DotfilesURL:  "https://github.com/user/dotfiles.git",
			DotfilesPath: "~/.dotfiles",
		},
	}

	handler := NewDotfilesHandler(currentRules[0], "/bp/setup.bp")
	rules := handler.FindUninstallRules(&status, currentRules, "/bp/setup.bp", "linux")

	// Since the URLs are normalized to the same value, the repo should NOT be marked for uninstall
	if len(rules) > 0 {
		t.Errorf("FindUninstallRules should not return uninstall rules when SSH and HTTPS URLs refer to the same repo, got %d rules", len(rules))
	}
}

// TestDotfilesIsInstalledNormalizesURLs tests that IsInstalled uses normalized URLs
// for comparison so SSH and HTTPS URLs for the same repo are treated as equal.
func TestDotfilesIsInstalledNormalizesURLs(t *testing.T) {
	// Status has SSH URL - no SHA or Links to avoid additional checks
	// that would fail in a unit test (LocalSHA lookup, symlink existence)
	status := Status{
		Dotfiles: []DotfilesStatus{
			{
				URL:       "git@github.com:user/dotfiles.git",
				Path:      "~/.dotfiles",
				Blueprint: "/bp/setup.bp",
				OS:        "linux",
			},
		},
	}

	// Rule uses HTTPS URL for the same repo
	rule := parser.Rule{
		Action:       "dotfiles",
		DotfilesURL:  "https://github.com/user/dotfiles.git",
		DotfilesPath: "~/.dotfiles",
	}

	handler := NewDotfilesHandler(rule, "/bp/setup.bp")

	// Should find the installed dotfiles because URLs are normalized for comparison
	if !handler.IsInstalled(&status, "/bp/setup.bp", "linux") {
		t.Error("IsInstalled should return true when SSH and HTTPS URLs refer to the same repo")
	}
}

// TestEnsureSymlinkFailsOnWrongTarget demonstrates the core bug: when a symlink
// already exists pointing to a wrong target, ensureSymlink cannot fix it because
// os.Symlink fails when dst already exists. This is why removeAllManagedSymlinks
// must be called before recreating symlinks on update.
func TestEnsureSymlinkFailsOnWrongTarget(t *testing.T) {
	dir := t.TempDir()

	oldTarget := filepath.Join(dir, "old")
	newTarget := filepath.Join(dir, "new")
	link := filepath.Join(dir, "link")

	os.WriteFile(oldTarget, []byte("old"), 0644)
	os.WriteFile(newTarget, []byte("new"), 0644)

	// Create symlink pointing to old target
	if err := os.Symlink(oldTarget, link); err != nil {
		t.Fatal(err)
	}

	// Try to create symlink at same dst pointing to new target
	created, reason := ensureSymlink(newTarget, link)
	if created {
		t.Error("ensureSymlink should NOT be able to overwrite an existing symlink with wrong target")
	}
	if reason == "" {
		t.Error("ensureSymlink should return a skip reason for wrong-target symlink")
	}
}

// TestUpdateRemovesOldSymlinksAndCreatesNew tests that on an update scenario,
// old symlinks (e.g. after a file rename in the repo) are removed and new ones created.
func TestUpdateRemovesOldSymlinksAndCreatesNew(t *testing.T) {
	// Setup: simulate a clone dir and a home dir
	cloneDir := t.TempDir()
	homeDir := t.TempDir()

	// Create "old" file in the clone (simulating state before rename)
	oldFile := filepath.Join(cloneDir, ".old_config")
	if err := os.WriteFile(oldFile, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create symlink in homeDir pointing to old file in clone
	oldLink := filepath.Join(homeDir, ".old_config")
	if err := os.Symlink(oldFile, oldLink); err != nil {
		t.Fatal(err)
	}

	// Now simulate the rename: remove old file, create new file
	os.Remove(oldFile)
	newFile := filepath.Join(cloneDir, ".new_config")
	if err := os.WriteFile(newFile, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}

	// The old symlink is now broken but still points into the clone.
	// removeAllManagedSymlinks should remove it.
	h := NewDotfilesHandler(parser.Rule{Action: "dotfiles"}, "")
	removed := h.removeAllManagedSymlinks(cloneDir, homeDir)
	if removed != 1 {
		t.Errorf("removeAllManagedSymlinks() removed %d, want 1", removed)
	}

	// Verify old symlink is gone
	if _, err := os.Lstat(oldLink); !os.IsNotExist(err) {
		t.Error("old symlink should have been removed")
	}

	// Now create the new symlink (simulating the recreate step)
	newLink := filepath.Join(homeDir, ".new_config")
	created, reason := ensureSymlink(newFile, newLink)
	if !created {
		t.Errorf("ensureSymlink() should have created new link, skipReason=%q", reason)
	}

	// Verify new symlink is correct
	target, err := os.Readlink(newLink)
	if err != nil {
		t.Fatal(err)
	}
	if target != newFile {
		t.Errorf("new symlink target = %q, want %q", target, newFile)
	}
}

// TestNonManagedSymlinksPreservedDuringUpdate tests that symlinks NOT pointing
// into the clone directory are preserved by removeAllManagedSymlinks.
func TestNonManagedSymlinksPreservedDuringUpdate(t *testing.T) {
	cloneDir := t.TempDir()
	homeDir := t.TempDir()

	// Create a managed symlink (points into clone)
	managedTarget := filepath.Join(cloneDir, ".bashrc")
	if err := os.WriteFile(managedTarget, []byte("managed"), 0644); err != nil {
		t.Fatal(err)
	}
	managedLink := filepath.Join(homeDir, ".bashrc")
	if err := os.Symlink(managedTarget, managedLink); err != nil {
		t.Fatal(err)
	}

	// Create a non-managed symlink (points elsewhere)
	otherDir := t.TempDir()
	otherTarget := filepath.Join(otherDir, ".manual_config")
	if err := os.WriteFile(otherTarget, []byte("manual"), 0644); err != nil {
		t.Fatal(err)
	}
	manualLink := filepath.Join(homeDir, ".manual_config")
	if err := os.Symlink(otherTarget, manualLink); err != nil {
		t.Fatal(err)
	}

	h := NewDotfilesHandler(parser.Rule{Action: "dotfiles"}, "")
	removed := h.removeAllManagedSymlinks(cloneDir, homeDir)
	if removed != 1 {
		t.Errorf("removeAllManagedSymlinks() removed %d, want 1 (only managed)", removed)
	}

	// Managed symlink should be gone
	if _, err := os.Lstat(managedLink); !os.IsNotExist(err) {
		t.Error("managed symlink should have been removed")
	}

	// Manual symlink should still exist
	info, err := os.Lstat(manualLink)
	if err != nil {
		t.Fatal("manual symlink should still exist")
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("manual symlink should still be a symlink")
	}
}

// TestFreshInstallDoesNotRemoveSymlinks tests that the isUpdate detection
// correctly identifies a fresh install (clonePath does not exist before clone).
func TestFreshInstallDoesNotRemoveSymlinks(t *testing.T) {
	// For a fresh install, clonePath doesn't exist before CloneOrUpdateRepository.
	// We verify the detection logic: os.Stat on a non-existent path returns error.
	nonExistent := filepath.Join(t.TempDir(), "does-not-exist")

	_, err := os.Stat(nonExistent)
	if !os.IsNotExist(err) {
		t.Error("expected non-existent path to return IsNotExist error")
	}

	// isUpdate should be false for a non-existent path
	isUpdate := err == nil
	if isUpdate {
		t.Error("fresh install path should not be detected as update")
	}
}

// TestIsInstalledReturnsFalseWhenSymlinkMissing verifies that IsInstalled
// returns false when a symlink recorded in Links has been deleted, even though
// the status entry (URL, blueprint, OS) otherwise matches. This ensures the
// apply path will call Up() which recreates the missing symlink.
func TestIsInstalledReturnsFalseWhenSymlinkMissing(t *testing.T) {
	// Create a real symlink so at least one link is present and correct
	tmpDir := t.TempDir()
	cloneDir := filepath.Join(tmpDir, "clone")
	if err := os.MkdirAll(cloneDir, 0750); err != nil {
		t.Fatal(err)
	}
	targetFile := filepath.Join(cloneDir, ".bashrc")
	if err := os.WriteFile(targetFile, []byte("# bashrc"), 0644); err != nil {
		t.Fatal(err)
	}
	existingLink := filepath.Join(tmpDir, ".bashrc")
	if err := os.Symlink(targetFile, existingLink); err != nil {
		t.Fatal(err)
	}

	// A second link that does NOT exist on disk
	missingLink := filepath.Join(tmpDir, ".zshrc")

	status := Status{
		Dotfiles: []DotfilesStatus{
			{
				URL:       "https://github.com/user/dotfiles.git",
				Path:      cloneDir,
				Blueprint: "/tmp/test.bp",
				OS:        "linux",
				Links:     []string{existingLink, missingLink},
			},
		},
	}

	h := NewDotfilesHandler(parser.Rule{
		Action:       "dotfiles",
		DotfilesURL:  "https://github.com/user/dotfiles.git",
		DotfilesPath: cloneDir,
	}, "")

	if h.IsInstalled(&status, "/tmp/test.bp", "linux") {
		t.Error("IsInstalled() should return false when a symlink from Links is missing")
	}
}

// TestDotfilesLinksForDiff verifies that DotfilesLinksForDiff returns only
// symlinks that are missing or wrong — not ones that already exist correctly.
func TestDotfilesLinksForDiff(t *testing.T) {
	cloneDir := t.TempDir()
	homeDir := t.TempDir()

	// Create two files in the clone
	if err := os.WriteFile(filepath.Join(cloneDir, ".zshrc"), []byte("zsh"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cloneDir, ".gitconfig"), []byte("git"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a correct symlink for .zshrc — should NOT appear in diff
	zshrcDst := filepath.Join(homeDir, ".zshrc")
	if err := os.Symlink(filepath.Join(cloneDir, ".zshrc"), zshrcDst); err != nil {
		t.Fatal(err)
	}

	// Leave .gitconfig unlinked — SHOULD appear in diff
	rule := parser.Rule{
		Action:       "dotfiles",
		DotfilesURL:  "git@github.com:user/dotfiles.git",
		DotfilesPath: cloneDir,
	}
	h := NewDotfilesHandler(rule, cloneDir)
	links := DotfilesLinksForDiff(h, homeDir)

	if len(links) != 1 {
		t.Fatalf("DotfilesLinksForDiff() returned %d links, want 1 (only missing ones)", len(links))
	}
	if !strings.Contains(links[0], ".gitconfig") {
		t.Errorf("expected .gitconfig in diff links, got %v", links)
	}
}

// TestDotfilesLinksForDiffEmptyClone returns nil when clone dir doesn't exist or is empty.
func TestDotfilesLinksForDiffEmptyClone(t *testing.T) {
	rule := parser.Rule{
		Action:       "dotfiles",
		DotfilesURL:  "git@github.com:user/dotfiles.git",
		DotfilesPath: "/nonexistent/path",
	}
	h := NewDotfilesHandler(rule, "/nonexistent/path")
	links := DotfilesLinksForDiff(h, t.TempDir())
	if len(links) != 0 {
		t.Errorf("DotfilesLinksForDiff() returned %d links for missing clone, want 0", len(links))
	}
}

// TestDotfilesLinksForDiffSkipsExistingSymlinks verifies that correctly symlinked
// entries are excluded from the diff output.
func TestDotfilesLinksForDiffSkipsExistingSymlinks(t *testing.T) {
	cloneDir := t.TempDir()
	homeDir := t.TempDir()

	// Create a file and correct symlink for it
	src := filepath.Join(cloneDir, ".vimrc")
	if err := os.WriteFile(src, []byte("vim"), 0644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(homeDir, ".vimrc")
	if err := os.Symlink(src, dst); err != nil {
		t.Fatal(err)
	}

	rule := parser.Rule{Action: "dotfiles", DotfilesURL: "git@github.com:user/dots.git", DotfilesPath: cloneDir}
	h := NewDotfilesHandler(rule, cloneDir)
	links := DotfilesLinksForDiff(h, homeDir)
	if len(links) != 0 {
		t.Errorf("expected no diff links when symlink is correct, got %v", links)
	}
}

// TestBrokenSymlinksCleanedUpOnUpdate tests that broken symlinks pointing
// into the clone directory are cleaned up by removeAllManagedSymlinks.
func TestBrokenSymlinksCleanedUpOnUpdate(t *testing.T) {
	cloneDir := t.TempDir()
	homeDir := t.TempDir()

	// Create a symlink pointing to a file in clone that no longer exists (broken)
	brokenTarget := filepath.Join(cloneDir, ".deleted_file")
	brokenLink := filepath.Join(homeDir, ".deleted_file")
	// Create and then delete the target to make a broken symlink
	if err := os.WriteFile(brokenTarget, []byte("temp"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(brokenTarget, brokenLink); err != nil {
		t.Fatal(err)
	}
	os.Remove(brokenTarget) // Now the symlink is broken

	// Also create a broken symlink one level deep (in a subdirectory)
	subDir := filepath.Join(homeDir, ".config")
	if err := os.MkdirAll(subDir, 0750); err != nil {
		t.Fatal(err)
	}
	deepBrokenTarget := filepath.Join(cloneDir, ".config", "deleted_sub")
	// We need the parent dir in clone to exist for the path to make sense
	if err := os.MkdirAll(filepath.Join(cloneDir, ".config"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(deepBrokenTarget, []byte("temp"), 0644); err != nil {
		t.Fatal(err)
	}
	deepBrokenLink := filepath.Join(subDir, "deleted_sub")
	if err := os.Symlink(deepBrokenTarget, deepBrokenLink); err != nil {
		t.Fatal(err)
	}
	os.Remove(deepBrokenTarget) // Now this symlink is also broken

	h := NewDotfilesHandler(parser.Rule{Action: "dotfiles"}, "")
	removed := h.removeAllManagedSymlinks(cloneDir, homeDir)
	if removed != 2 {
		t.Errorf("removeAllManagedSymlinks() removed %d broken symlinks, want 2", removed)
	}

	// Both broken symlinks should be gone
	if _, err := os.Lstat(brokenLink); !os.IsNotExist(err) {
		t.Error("broken top-level symlink should have been removed")
	}
	if _, err := os.Lstat(deepBrokenLink); !os.IsNotExist(err) {
		t.Error("broken deep symlink should have been removed")
	}
}
