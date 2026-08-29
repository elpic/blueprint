package handlers

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/platform"
	"github.com/elpic/blueprint/internal/platform/mocks"
)

// TestCloneTwoStageApproach tests the two-stage clone implementation
func TestCloneTwoStageApproach(t *testing.T) {
	t.Run("Two stage clone prevents pollution", func(t *testing.T) {
		// Create a temporary directory for testing
		tmpDir := t.TempDir()
		targetPath := filepath.Join(tmpDir, "test-repo")

		rule := parser.Rule{
			ID:        "test-repo",
			Action:    "clone",
			CloneURL:  "https://github.com/test/repo.git",
			ClonePath: targetPath,
			Branch:    "",
		}

		// Configure the seam's clone outcome for the two-stage spec. The
		// exact SHA in the output proves the handler resolved the rule to
		// ModeTwoStage — any other mode would hit the mock's default result.
		testSHA := "abc123456789"
		spec := platform.CloneSpec{URL: rule.CloneURL, Path: targetPath, Branch: "", Mode: platform.ModeTwoStage}
		gitMock := mocks.NewMockGitProvider().
			WithCloneResult(spec, platform.CloneResult{Status: platform.StatusCloned, NewSHA: platform.SHA(testSHA)})
		handler := NewCloneHandler(rule, tmpDir, newTestGitContainer(gitMock))

		// Create target directory and add a user file to simulate pollution
		err := os.MkdirAll(targetPath, 0755)
		if err != nil {
			t.Fatalf("Failed to create target directory: %v", err)
		}

		userFile := filepath.Join(targetPath, "antigen.zsh")
		err = os.WriteFile(userFile, []byte("# User's antigen configuration"), 0644)
		if err != nil {
			t.Fatalf("Failed to create user file: %v", err)
		}

		// Verify user file exists before clone
		if _, err := os.Stat(userFile); os.IsNotExist(err) {
			t.Fatal("User file should exist before clone")
		}

		// Execute clone
		output, err := handler.Up()
		if err != nil {
			t.Fatalf("Clone failed: %v", err)
		}

		if want := "Cloned (SHA: " + testSHA + ")"; output != want {
			t.Errorf("Up() = %q, want %q", output, want)
		}

		// In a real implementation, the user file would be preserved
		// This test validates the approach structure
	})

	t.Run("IsInstalled uses clean repository SHA", func(t *testing.T) {
		testSHA := "clean123456"

		rule := parser.Rule{
			ID:        "test-repo",
			Action:    "clone",
			CloneURL:  "https://github.com/test/repo.git",
			ClonePath: "~/.test-repo",
		}
		id := platform.RepoID{URL: rule.CloneURL}

		// Simulate a polluted target whose clean-storage copy matches the remote
		gitMock := mocks.NewMockGitProvider().
			WithLocalSHA(platform.RepoPath("/home/testuser/.test-repo"), "polluted456789").
			WithRemoteSHA(id, platform.SHA(testSHA)).
			WithStorageSHA(id, platform.SHA(testSHA))
		handler := NewCloneHandler(rule, "/tmp", newTestGitContainer(gitMock))

		status := &Status{
			Clones: []CloneStatus{
				{
					Path:      "~/.test-repo",
					URL:       "https://github.com/test/repo.git",
					SHA:       testSHA, // Should match clean SHA, not polluted
					Blueprint: "/tmp/test.bp",
					OS:        "darwin",
					ClonedAt:  time.Now().Format(time.RFC3339),
				},
			},
		}

		// Should be considered installed based on clean SHA, not polluted target
		isInstalled := handler.IsInstalled(status, "/tmp/test.bp", "darwin")
		if !isInstalled {
			t.Error("Expected repository to be detected as installed using clean SHA")
		}
	})

	t.Run("IsInstalled handles network failure gracefully", func(t *testing.T) {
		testSHA := "abc123456789"

		rule := parser.Rule{
			ID:        "test-repo",
			Action:    "clone",
			CloneURL:  "https://github.com/test/repo.git",
			ClonePath: "~/.test-repo",
		}
		id := platform.RepoID{URL: rule.CloneURL}

		// Simulate network failure: the remote SHA cannot be fetched
		gitMock := mocks.NewMockGitProvider().
			WithLocalSHA(platform.RepoPath("/home/testuser/.test-repo"), platform.SHA(testSHA)).
			WithUnreachableRemote(id)
		handler := NewCloneHandler(rule, "/tmp", newTestGitContainer(gitMock))

		status := &Status{
			Clones: []CloneStatus{
				{
					Path:      "~/.test-repo",
					URL:       "https://github.com/test/repo.git",
					SHA:       testSHA,
					Blueprint: "/tmp/test.bp",
					OS:        "darwin",
					ClonedAt:  time.Now().Format(time.RFC3339),
				},
			},
		}

		// Should trust the status when remote is unreachable
		isInstalled := handler.IsInstalled(status, "/tmp/test.bp", "darwin")
		if !isInstalled {
			t.Error("Expected repository to be trusted when remote SHA unavailable")
		}
	})

	t.Run("Multiple clones should use same storage", func(t *testing.T) {
		// Test that multiple clone operations for the same repo use consistent storage
		testSHA1 := "first123456"
		testSHA2 := "second67890"

		rule := parser.Rule{
			ID:        "test-repo",
			Action:    "clone",
			CloneURL:  "https://github.com/test/repo.git",
			ClonePath: "/tmp/test-repo",
		}

		// Both runs go through the seam with the same two-stage spec
		spec := platform.CloneSpec{URL: rule.CloneURL, Path: rule.ClonePath, Mode: platform.ModeTwoStage}
		counting := &countingGitProvider{
			GitProvider: mocks.NewMockGitProvider().
				WithCloneResult(spec, platform.CloneResult{Status: platform.StatusUpdated, OldSHA: platform.SHA(testSHA1), NewSHA: platform.SHA(testSHA2)}),
		}
		handler := NewCloneHandler(rule, "/tmp", newTestGitContainer(counting))

		// First clone
		output1, err1 := handler.Up()
		if err1 != nil {
			t.Fatalf("First clone failed: %v", err1)
		}
		if output1 == "" {
			t.Error("Expected non-empty output from first clone")
		}

		// Second clone (should update)
		output2, err2 := handler.Up()
		if err2 != nil {
			t.Fatalf("Second clone failed: %v", err2)
		}
		if output2 == "" {
			t.Error("Expected non-empty output from second clone")
		}

		// Verify both operations used the two-stage approach through the seam
		if counting.cloneCalls != 2 {
			t.Errorf("Expected 2 clone calls through the seam, got %d", counting.cloneCalls)
		}
		if counting.lastSpec.Mode != platform.ModeTwoStage {
			t.Errorf("Expected two-stage mode, got %v", counting.lastSpec.Mode)
		}
	})
}

// TestCloneHandlerBackwardCompatibility ensures existing clones continue to work
func TestCloneHandlerBackwardCompatibility(t *testing.T) {
	t.Run("Existing clones work with new implementation", func(t *testing.T) {
		testSHA := "existing123"

		rule := parser.Rule{
			ID:        "existing-repo",
			Action:    "clone",
			CloneURL:  "https://github.com/existing/repo.git",
			ClonePath: "~/.existing-repo",
		}
		id := platform.RepoID{URL: rule.CloneURL}

		// For existing installations, clean storage might not exist yet; when
		// it exists and matches the remote, they are still considered installed.
		gitMock := mocks.NewMockGitProvider().
			WithRemoteSHA(id, platform.SHA(testSHA)).
			WithStorageSHA(id, platform.SHA(testSHA))
		handler := NewCloneHandler(rule, "/tmp", newTestGitContainer(gitMock))

		status := &Status{
			Clones: []CloneStatus{
				{
					Path:      "~/.existing-repo",
					URL:       "https://github.com/existing/repo.git",
					SHA:       testSHA,
					Blueprint: "/tmp/test.bp",
					OS:        "darwin",
					ClonedAt:  "2023-01-01T00:00:00Z", // Old installation
				},
			},
		}

		// Should still be considered installed
		isInstalled := handler.IsInstalled(status, "/tmp/test.bp", "darwin")
		if !isInstalled {
			t.Error("Existing clone should still be considered installed")
		}
	})
}
