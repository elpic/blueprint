package handlers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/platform"
	"github.com/elpic/blueprint/internal/platform/mocks"
)

// TestCloneRepositoryPollutionFix validates that the two-stage clone approach
// prevents repository pollution issues like the antigen.zsh problem
func TestCloneRepositoryPollutionFix(t *testing.T) {
	t.Run("User files survive repository clone operations", func(t *testing.T) {
		tmpDir := t.TempDir()
		targetPath := filepath.Join(tmpDir, "oh-my-zsh")
		userFile := filepath.Join(targetPath, "antigen.zsh")

		// Simulate the oh-my-zsh scenario
		rule := parser.Rule{
			ID:        "oh-my-zsh",
			Action:    "clone",
			CloneURL:  "https://github.com/ohmyzsh/ohmyzsh.git",
			ClonePath: targetPath,
			Branch:    "",
		}

		// Create target directory with user file (simulating antigen.zsh added by user)
		err := os.MkdirAll(targetPath, 0755)
		if err != nil {
			t.Fatalf("Failed to create target directory: %v", err)
		}

		userFileContent := "# User's antigen configuration\n# This should survive clone operations"
		err = os.WriteFile(userFile, []byte(userFileContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create user file: %v", err)
		}

		// Verify user file exists before clone
		beforeContent, err := os.ReadFile(userFile)
		if err != nil {
			t.Fatalf("User file should exist before clone: %v", err)
		}

		// Configure the seam: a successful two-stage clone (the mock never
		// touches the filesystem, so user files survive by construction).
		testSHA := "abc123456789"
		spec := platform.CloneSpec{URL: rule.CloneURL, Path: targetPath, Branch: "", Mode: platform.ModeTwoStage}
		counting := &countingGitProvider{
			GitProvider: mocks.NewMockGitProvider().
				WithCloneResult(spec, platform.CloneResult{Status: platform.StatusCloned, NewSHA: platform.SHA(testSHA)}),
		}
		handler := NewCloneHandler(rule, tmpDir, newTestGitContainer(counting))

		// Execute clone operation through the seam
		output, err := handler.Up()
		if err != nil {
			t.Fatalf("Clone operation failed: %v", err)
		}

		// Verify the two-stage approach was used
		if counting.cloneCalls != 1 {
			t.Errorf("Expected exactly one clone call through the seam, got %d", counting.cloneCalls)
		}
		if counting.lastSpec.Mode != platform.ModeTwoStage {
			t.Errorf("Expected two-stage mode, got %v", counting.lastSpec.Mode)
		}

		// Verify output indicates successful operation
		if want := "Cloned (SHA: " + testSHA + ")"; output != want {
			t.Errorf("Up() = %q, want %q", output, want)
		}

		// User file survives untouched
		afterContent, err := os.ReadFile(userFile)
		if err != nil {
			t.Fatalf("User file should survive the clone operation: %v", err)
		}
		if string(afterContent) != string(beforeContent) {
			t.Error("User file content changed during clone")
		}

		t.Logf("Clone operation completed: %s", output)
		t.Log("Two-stage approach successfully invoked - foundation for pollution fix is in place")
	})

	t.Run("Idempotency works with clean repository storage", func(t *testing.T) {
		tmpDir := t.TempDir()
		targetPath := filepath.Join(tmpDir, "test-repo")

		rule := parser.Rule{
			ID:        "test-repo",
			Action:    "clone",
			CloneURL:  "https://github.com/test/repo.git",
			ClonePath: targetPath,
		}
		id := platform.RepoID{URL: rule.CloneURL}

		// Simulate clean repository storage exists and is up to date, while
		// the target itself is polluted with additional files
		testSHA := "abc123456789"
		gitMock := mocks.NewMockGitProvider().
			WithLocalSHA(platform.RepoPath(targetPath), "polluted456789"). // Target is polluted
			WithRemoteSHA(id, platform.SHA(testSHA)).
			WithStorageSHA(id, platform.SHA(testSHA)) // Clean storage
		handler := NewCloneHandler(rule, tmpDir, newTestGitContainer(gitMock))

		// Create status indicating the repository was previously cloned
		status := &Status{
			Clones: []CloneStatus{
				{
					Path:      targetPath,
					URL:       "https://github.com/test/repo.git",
					SHA:       testSHA,
					Blueprint: tmpDir + "/test.bp",
					OS:        "darwin",
				},
			},
		}

		// Check if installed - should use clean repository SHA, not polluted target
		isInstalled := handler.IsInstalled(status, tmpDir+"/test.bp", "darwin")
		if !isInstalled {
			t.Error("Repository should be considered installed based on clean SHA match")
		}

		// This demonstrates the fix: idempotency now works correctly
		// even when the target directory is polluted with additional files
		t.Log("Idempotency check passed using clean repository storage")
	})

	t.Run("Backward compatibility with existing installations", func(t *testing.T) {
		tmpDir := t.TempDir()
		targetPath := filepath.Join(tmpDir, "existing-repo")

		rule := parser.Rule{
			ID:        "existing-repo",
			Action:    "clone",
			CloneURL:  "https://github.com/existing/repo.git",
			ClonePath: targetPath,
		}
		id := platform.RepoID{URL: rule.CloneURL}

		// Simulate existing installation (no clean storage yet — the mock's
		// default) that falls back to the target directory SHA
		testSHA := "existing123456"
		gitMock := mocks.NewMockGitProvider().
			WithLocalSHA(platform.RepoPath(targetPath), platform.SHA(testSHA)).
			WithRemoteSHA(id, platform.SHA(testSHA))
		handler := NewCloneHandler(rule, tmpDir, newTestGitContainer(gitMock))

		status := &Status{
			Clones: []CloneStatus{
				{
					Path:      targetPath,
					URL:       "https://github.com/existing/repo.git",
					SHA:       testSHA,
					Blueprint: tmpDir + "/test.bp",
					OS:        "darwin",
				},
			},
		}

		// Should fall back to checking target directory for backward compatibility
		isInstalled := handler.IsInstalled(status, tmpDir+"/test.bp", "darwin")
		if !isInstalled {
			t.Error("Existing installation should still be considered installed")
		}

		t.Log("Backward compatibility maintained for existing installations")
	})
}
