package handlers

import (
	"testing"
	"time"

	"github.com/elpic/blueprint/internal/parser"
)

// TestCloneIdempotencyBug reproduces the oh-my-zsh clone idempotency issue
func TestCloneIdempotencyBug(t *testing.T) {
	// Test to reproduce: Rule #10 (oh-my-zsh clone) runs again on subsequent runs,
	// overwriting directory and deleting antigen.zsh from Rule #11

	t.Run("Normal case should work", func(t *testing.T) {
		// Store original functions
		origLocal := localSHA
		origRemote := remoteHeadSHA
		defer func() {
			localSHA = origLocal
			remoteHeadSHA = origRemote
		}()

		rule := parser.Rule{
			ID:        "oh-my-zsh",
			Action:    "clone",
			CloneURL:  "https://github.com/ohmyzsh/ohmyzsh.git",
			ClonePath: "~/.oh-my-zsh",
			Branch:    "",
		}

		handler := NewCloneHandlerLegacy(rule, "/tmp")

		// Mock consistent SHAs (repository unchanged)
		testSHA := "abc123456789"
		localSHA = func(string) string { return testSHA }
		remoteHeadSHA = func(string, string) string { return testSHA }

		status := &Status{
			Clones: []CloneStatus{
				{
					Path:      "~/.oh-my-zsh",
					URL:       "https://github.com/ohmyzsh/ohmyzsh.git",
					SHA:       testSHA,
					Blueprint: "/tmp/test.bp",
					OS:        "darwin",
					ClonedAt:  time.Now().Format(time.RFC3339),
				},
			},
		}

		// Should be considered installed (idempotent)
		isInstalled := handler.IsInstalled(status, "/tmp/test.bp", "darwin")
		if !isInstalled {
			t.Error("Expected clone to be detected as installed when SHAs match")
		}
	})

	t.Run("Network failure should trust status", func(t *testing.T) {
		// This tests the fallback behavior when remote SHA can't be fetched
		origLocal := localSHA
		origRemote := remoteHeadSHA
		defer func() {
			localSHA = origLocal
			remoteHeadSHA = origRemote
		}()

		rule := parser.Rule{
			ID:        "oh-my-zsh",
			Action:    "clone",
			CloneURL:  "https://github.com/ohmyzsh/ohmyzsh.git",
			ClonePath: "~/.oh-my-zsh",
		}

		handler := NewCloneHandlerLegacy(rule, "/tmp")

		// Mock network failure (empty remote SHA) but valid local SHA
		localSHA = func(string) string { return "abc123456789" }
		remoteHeadSHA = func(string, string) string { return "" } // Network failure

		status := &Status{
			Clones: []CloneStatus{
				{
					Path:      "~/.oh-my-zsh",
					URL:       "https://github.com/ohmyzsh/ohmyzsh.git",
					SHA:       "abc123456789",
					Blueprint: "/tmp/test.bp",
					OS:        "darwin",
					ClonedAt:  time.Now().Format(time.RFC3339),
				},
			},
		}

		// Should trust the status when remote is unreachable
		isInstalled := handler.IsInstalled(status, "/tmp/test.bp", "darwin")
		if !isInstalled {
			t.Error("Expected clone to be trusted when remote SHA unavailable (network failure)")
		}
	})

	t.Run("Local repository corruption should cause re-clone", func(t *testing.T) {
		// This tests what happens when local repository is corrupted
		origLocal := localSHA
		origRemote := remoteHeadSHA
		defer func() {
			localSHA = origLocal
			remoteHeadSHA = origRemote
		}()

		rule := parser.Rule{
			ID:        "oh-my-zsh",
			Action:    "clone",
			CloneURL:  "https://github.com/ohmyzsh/ohmyzsh.git",
			ClonePath: "~/.oh-my-zsh",
		}

		handler := NewCloneHandlerLegacy(rule, "/tmp")

		// Mock local repository corruption (empty local SHA) but valid remote SHA
		localSHA = func(string) string { return "" } // Corrupted/missing .git
		remoteHeadSHA = func(string, string) string { return "def987654321" }

		status := &Status{
			Clones: []CloneStatus{
				{
					Path:      "~/.oh-my-zsh",
					URL:       "https://github.com/ohmyzsh/ohmyzsh.git",
					SHA:       "abc123456789",
					Blueprint: "/tmp/test.bp",
					OS:        "darwin",
					ClonedAt:  time.Now().Format(time.RFC3339),
				},
			},
		}

		// Should NOT be considered installed (will re-clone)
		isInstalled := handler.IsInstalled(status, "/tmp/test.bp", "darwin")
		if isInstalled {
			t.Error("Expected clone to NOT be trusted when local repository is corrupted")
		}

		// This is a potential source of the bug - if ~/.oh-my-zsh/.git gets corrupted,
		// it will trigger a re-clone, overwriting antigen.zsh
	})

	t.Run("Missing status entry should cause clone", func(t *testing.T) {
		rule := parser.Rule{
			ID:        "oh-my-zsh",
			Action:    "clone",
			CloneURL:  "https://github.com/ohmyzsh/ohmyzsh.git",
			ClonePath: "~/.oh-my-zsh",
		}

		handler := NewCloneHandlerLegacy(rule, "/tmp")

		// Empty status (no clone recorded)
		status := &Status{Clones: []CloneStatus{}}

		// Should NOT be considered installed
		isInstalled := handler.IsInstalled(status, "/tmp/test.bp", "darwin")
		if isInstalled {
			t.Error("Expected clone to NOT be considered installed when no status entry exists")
		}

		// This could be the bug - if status tracking is broken, entries might be missing
	})

	t.Run("Path normalization issues", func(t *testing.T) {
		origLocal := localSHA
		origRemote := remoteHeadSHA
		defer func() {
			localSHA = origLocal
			remoteHeadSHA = origRemote
		}()

		rule := parser.Rule{
			ID:        "oh-my-zsh",
			Action:    "clone",
			CloneURL:  "https://github.com/ohmyzsh/ohmyzsh.git",
			ClonePath: "~/.oh-my-zsh",
		}

		handler := NewCloneHandlerLegacy(rule, "/tmp")

		testSHA := "abc123456789"
		localSHA = func(string) string { return testSHA }
		remoteHeadSHA = func(string, string) string { return testSHA }

		status := &Status{
			Clones: []CloneStatus{
				{
					Path:      "~/.oh-my-zsh",
					URL:       "https://github.com/ohmyzsh/ohmyzsh.git",
					SHA:       testSHA,
					Blueprint: "/home/user/config/../config/blueprint.bp", // Different normalization
					OS:        "darwin",
					ClonedAt:  time.Now().Format(time.RFC3339),
				},
			},
		}

		// Test with different blueprint path formats
		testCases := []struct {
			name      string
			blueprint string
			expected  bool
		}{
			{"Exact match", "/home/user/config/blueprint.bp", true},                          // Should match after normalization
			{"Same path different format", "/home/user/config/../config/blueprint.bp", true}, // Should match
			{"Relative path", "blueprint.bp", false},                                         // Won't match
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				isInstalled := handler.IsInstalled(status, tc.blueprint, "darwin")
				if isInstalled != tc.expected {
					t.Errorf("Path normalization test %s: got %v, expected %v", tc.name, isInstalled, tc.expected)
				}
			})
		}
	})

	t.Run("OS name case sensitivity", func(t *testing.T) {
		origLocal := localSHA
		origRemote := remoteHeadSHA
		defer func() {
			localSHA = origLocal
			remoteHeadSHA = origRemote
		}()

		rule := parser.Rule{
			ID:        "oh-my-zsh",
			Action:    "clone",
			CloneURL:  "https://github.com/ohmyzsh/ohmyzsh.git",
			ClonePath: "~/.oh-my-zsh",
		}

		handler := NewCloneHandlerLegacy(rule, "/tmp")

		testSHA := "abc123456789"
		localSHA = func(string) string { return testSHA }
		remoteHeadSHA = func(string, string) string { return testSHA }

		status := &Status{
			Clones: []CloneStatus{
				{
					Path:      "~/.oh-my-zsh",
					URL:       "https://github.com/ohmyzsh/ohmyzsh.git",
					SHA:       testSHA,
					Blueprint: "/tmp/test.bp",
					OS:        "darwin", // Lowercase
					ClonedAt:  time.Now().Format(time.RFC3339),
				},
			},
		}

		// Test with different OS name cases
		testCases := []struct {
			osName   string
			expected bool
		}{
			{"darwin", true},  // Exact match
			{"Darwin", false}, // Different case (shouldn't match)
			{"mac", false},    // Different name
		}

		for _, tc := range testCases {
			isInstalled := handler.IsInstalled(status, "/tmp/test.bp", tc.osName)
			if isInstalled != tc.expected {
				t.Errorf("OS name test %s: got %v, expected %v", tc.osName, isInstalled, tc.expected)
			}
		}
	})
}
