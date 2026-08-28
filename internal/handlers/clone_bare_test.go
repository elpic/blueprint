package handlers

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

// newLocalOrigin creates a git repository with one commit on branch and
// returns its path. Local paths work as clone sources for both go-git and
// system git, so these tests need no network.
func newLocalOrigin(t *testing.T, branch string) string {
	t.Helper()

	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...) // #nosec G204 -- test fixture
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
	}

	run("git", "init", "-q", "-b", branch, ".")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# fixture\n"), 0o600); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}
	run("git", "add", ".")
	run("git", "commit", "-qm", "initial commit")
	return dir
}

func TestCloneHandlerGetCommandBare(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		expected string
	}{
		{
			name: "bare clone without branch",
			rule: parser.Rule{
				Action:    "clone",
				CloneURL:  "https://github.com/user/repo.git",
				ClonePath: "~/projects/repo",
				CloneBare: true,
			},
			expected: "git clone --bare https://github.com/user/repo.git ~/projects/repo/.git",
		},
		{
			name: "bare clone with branch",
			rule: parser.Rule{
				Action:    "clone",
				CloneURL:  "https://github.com/user/repo.git",
				ClonePath: "~/projects/repo",
				Branch:    "develop",
				CloneBare: true,
			},
			expected: "git clone --bare -b develop https://github.com/user/repo.git ~/projects/repo/.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewCloneHandlerLegacy(tt.rule, "")
			cmd := handler.GetCommand()
			if cmd != tt.expected {
				t.Errorf("GetCommand() = %q, want %q", cmd, tt.expected)
			}
		})
	}
}

func TestCloneHandlerUpBare(t *testing.T) {
	origin := newLocalOrigin(t, "main")
	target := filepath.Join(t.TempDir(), "project")

	handler := NewCloneHandlerLegacy(parser.Rule{
		Action:    "clone",
		CloneURL:  origin,
		ClonePath: target,
		CloneBare: true,
	}, "")

	output, err := handler.Up()
	if err != nil {
		t.Fatalf("Up(): %v", err)
	}
	if !strings.HasPrefix(output, "Cloned (SHA: ") {
		t.Errorf("Up() = %q, want a %q message with a SHA", output, "Cloned")
	}

	// The bare repository holds no working tree; the branch is a worktree.
	gitDir := filepath.Join(target, ".git")
	worktree := filepath.Join(target, "main")

	if out, err := exec.Command("git", "-C", gitDir, "rev-parse", "--is-bare-repository").Output(); err != nil {
		t.Fatalf("git rev-parse: %v", err)
	} else if strings.TrimSpace(string(out)) != "true" {
		t.Errorf("%s is not a bare repository", gitDir)
	}

	info, err := os.Stat(filepath.Join(worktree, ".git"))
	if err != nil {
		t.Fatalf("worktree %s has no .git: %v", worktree, err)
	}
	if info.IsDir() {
		t.Errorf("worktree .git should be a gitfile, not a directory")
	}
	if _, err := os.Stat(filepath.Join(worktree, "README.md")); err != nil {
		t.Errorf("worktree is not checked out: %v", err)
	}
}

func TestCloneHandlerUpBareDoesNotTouchWorktrees(t *testing.T) {
	origin := newLocalOrigin(t, "main")
	target := filepath.Join(t.TempDir(), "project")

	handler := NewCloneHandlerLegacy(parser.Rule{
		Action:    "clone",
		CloneURL:  origin,
		ClonePath: target,
		CloneBare: true,
	}, "")

	if _, err := handler.Up(); err != nil {
		t.Fatalf("first Up(): %v", err)
	}

	// Uncommitted work an agent left in the worktree survives a re-run.
	wip := filepath.Join(target, "main", "WIP.md")
	if err := os.WriteFile(wip, []byte("work in progress\n"), 0o600); err != nil {
		t.Fatalf("write WIP file: %v", err)
	}

	output, err := handler.Up()
	if err != nil {
		t.Fatalf("second Up(): %v", err)
	}
	if !strings.HasPrefix(output, "Already up to date") {
		t.Errorf("second Up() = %q, want %q", output, "Already up to date")
	}
	if _, err := os.Stat(wip); err != nil {
		t.Errorf("uncommitted work was not preserved: %v", err)
	}
}

// Uninstall rules are rebuilt from status, so they carry no bare flag. The
// layout has to be recognised on disk, otherwise removal is skipped as "not
// installed" and the bare repository is left behind.
func TestCloneHandlerUninstallBare(t *testing.T) {
	origin := newLocalOrigin(t, "main")
	target := filepath.Join(t.TempDir(), "project")

	clone := parser.Rule{Action: "clone", CloneURL: origin, ClonePath: target, CloneBare: true}
	if _, err := NewCloneHandlerLegacy(clone, "").Up(); err != nil {
		t.Fatalf("Up(): %v", err)
	}

	status := Status{Clones: []CloneStatus{{
		URL:       origin,
		Path:      target,
		ClonedAt:  "2026-01-01T00:00:00Z",
		Blueprint: "setup.bp",
		OS:        "mac",
	}}}

	uninstall := parser.Rule{Action: "uninstall", CloneURL: origin, ClonePath: target}
	handler := NewCloneHandlerLegacy(uninstall, "")

	if !handler.IsInstalled(&status, "setup.bp", "mac") {
		t.Fatal("IsInstalled() = false for a bare clone on disk — uninstall would be skipped")
	}

	output, err := handler.Down()
	if err != nil {
		t.Fatalf("Down(): %v", err)
	}
	if !strings.Contains(output, "Removed cloned repository") {
		t.Errorf("Down() = %q, want it to report the removal", output)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("bare clone still on disk after Down(): %v", err)
	}
}

func TestCloneHandlerGetStateBare(t *testing.T) {
	rule := parser.Rule{
		Action:    "clone",
		CloneURL:  "https://github.com/user/repo.git",
		ClonePath: "~/projects/repo",
		CloneBare: true,
	}
	handler := NewCloneHandlerLegacy(rule, "")

	state := handler.GetState(false)
	if state["layout"] != "bare" {
		t.Errorf("GetState()[layout] = %q, want %q", state["layout"], "bare")
	}

	nonBare := NewCloneHandlerLegacy(parser.Rule{Action: "clone", CloneURL: rule.CloneURL, ClonePath: rule.ClonePath}, "")
	if _, ok := nonBare.GetState(false)["layout"]; ok {
		t.Error("layout should only be reported for bare clones")
	}
}
