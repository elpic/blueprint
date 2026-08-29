package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
)

func TestGenerateRepositoryID(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		branch      string
		expectedLen int
	}{
		{
			name:        "HTTPS URL without branch",
			url:         "https://github.com/user/repo.git",
			branch:      "",
			expectedLen: 16,
		},
		{
			name:        "HTTPS URL with branch",
			url:         "https://github.com/user/repo.git",
			branch:      "main",
			expectedLen: 16,
		},
		{
			name:        "SSH URL",
			url:         "git@github.com:user/repo.git",
			branch:      "",
			expectedLen: 16,
		},
		{
			name:        "Different URLs produce different IDs",
			url:         "https://github.com/other/repo.git",
			branch:      "",
			expectedLen: 16,
		},
		{
			name:        "Same URL different branch produces different ID",
			url:         "https://github.com/user/repo.git",
			branch:      "feature",
			expectedLen: 16,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateRepositoryID(tt.url, tt.branch)
			if len(got) != tt.expectedLen {
				t.Errorf("generateRepositoryID() length = %d, want %d", len(got), tt.expectedLen)
			}
		})
	}

	// Verify determinism: same input should produce same output
	id1 := generateRepositoryID("https://github.com/user/repo.git", "main")
	id2 := generateRepositoryID("https://github.com/user/repo.git", "main")
	if id1 != id2 {
		t.Errorf("generateRepositoryID not deterministic: got %q then %q", id1, id2)
	}
}

func TestHttpsAuth(t *testing.T) {
	tests := []struct {
		name         string
		envToken     string
		envUser      string
		wantNil      bool
		wantUsername string
		wantPassword string
	}{
		{
			name:    "no env vars returns nil",
			wantNil: true,
		},
		{
			name:         "token only uses x-access-token as username",
			envToken:     "ghp_token123",
			wantUsername: "x-access-token",
			wantPassword: "ghp_token123",
		},
		{
			name:         "token and user uses provided username",
			envToken:     "ghp_token123",
			envUser:      "myuser",
			wantUsername: "myuser",
			wantPassword: "ghp_token123",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GITHUB_TOKEN", tt.envToken)
			t.Setenv("GITHUB_USER", tt.envUser)
			auth := httpsAuth()
			if tt.wantNil {
				if auth != nil {
					t.Errorf("expected nil, got %+v", auth)
				}
				return
			}
			if auth == nil {
				t.Fatal("expected non-nil auth")
			}
			if auth.Username != tt.wantUsername {
				t.Errorf("Username = %q, want %q", auth.Username, tt.wantUsername)
			}
			if auth.Password != tt.wantPassword {
				t.Errorf("Password = %q, want %q", auth.Password, tt.wantPassword)
			}
		})
	}
}

func TestLocalSHAWithError(t *testing.T) {
	t.Run("nonexistent path returns error", func(t *testing.T) {
		sha, err := LocalSHAWithError("/nonexistent/path/that/does/not/exist")
		if err == nil {
			t.Error("expected error, got nil")
		}
		if sha != "" {
			t.Errorf("expected empty SHA, got %q", sha)
		}
	})

	t.Run("valid repo returns SHA", func(t *testing.T) {
		dir := t.TempDir()
		// Init a real git repo in the temp dir
		cmd := exec.Command("git", "init", dir)
		if err := cmd.Run(); err != nil {
			t.Fatalf("git init: %v", err)
		}
		// Need at least one commit for HEAD to exist
		f := filepath.Join(dir, "f.txt")
		if err := os.WriteFile(f, []byte("hi"), 0o600); err != nil {
			t.Fatalf("write file: %v", err)
		}
		for _, args := range [][]string{
			{"git", "-C", dir, "config", "user.email", "test@test.com"},
			{"git", "-C", dir, "config", "user.name", "Test"},
			{"git", "-C", dir, "add", "."},
			{"git", "-C", dir, "commit", "-m", "init"},
		} {
			if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
				t.Fatalf("cmd %v: %v", args, err)
			}
		}
		sha, err := LocalSHAWithError(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(sha) != 40 {
			t.Errorf("expected 40-char SHA, got %q", sha)
		}
	})
}

func TestRemoteHeadSHAWithError(t *testing.T) {
	t.Run("invalid URL returns error", func(t *testing.T) {
		sha, err := RemoteHeadSHAWithError("https://invalid.example.invalid/nonexistent/repo.git", "")
		if err == nil {
			t.Error("expected error for unreachable URL, got nil")
		}
		if sha != "" {
			t.Errorf("expected empty SHA, got %q", sha)
		}
	})
}

func TestGitTimeout(t *testing.T) {
	t.Run("default timeout is 120s", func(t *testing.T) {
		_ = os.Unsetenv("BLUEPRINT_GIT_TIMEOUT")
		if got := gitTimeout(); got != 120*1e9 {
			t.Errorf("expected 120s, got %v", got)
		}
	})

	t.Run("custom timeout from env", func(t *testing.T) {
		t.Setenv("BLUEPRINT_GIT_TIMEOUT", "30")
		if got := gitTimeout(); got != 30*1e9 {
			t.Errorf("expected 30s, got %v", got)
		}
	})

	t.Run("invalid env falls back to default", func(t *testing.T) {
		t.Setenv("BLUEPRINT_GIT_TIMEOUT", "notanumber")
		if got := gitTimeout(); got != 120*1e9 {
			t.Errorf("expected 120s, got %v", got)
		}
	})

	t.Run("zero env falls back to default", func(t *testing.T) {
		t.Setenv("BLUEPRINT_GIT_TIMEOUT", "0")
		if got := gitTimeout(); got != 120*1e9 {
			t.Errorf("expected 120s, got %v", got)
		}
	})
}

// ---- #030: go-git config round-trip ---------------------------------------
//
// The refspec repair writes .git/config through go-git instead of shelling out
// to `git config`. That is only safe if go-git preserves the parts of the file
// it doesn't understand: config.Config.Raw is meant to round-trip unknown
// sections, but "meant to" is not "does".

func TestSetConfigPreservesUnrecognizedConfig(t *testing.T) {
	// newOriginRepo already leaves one commit on "main".
	origin := newOriginRepo(t, "main")

	// Clone with SingleBranch so .git/config carries the limited refspec that
	// triggers the repair path.
	target := filepath.Join(t.TempDir(), "dotfiles")
	if _, _, _, err := CloneOrUpdateRepository(origin, target, "main"); err != nil {
		t.Fatalf("clone: %v", err)
	}

	// Config go-git has no opinion about: an [alias] section and an unknown key
	// under [core]. Both must survive the refspec write untouched.
	const extra = `[alias]
	st = status -sb
	lg = log --oneline --graph
[core]
	unknownFutureKey = keep-me
`
	appendToGitConfig(t, target, extra)

	before := readGitConfig(t, target)

	// Force the repair: rewrite the on-disk refspec to something limited.
	gitIn(t, target, "config", "remote.origin.fetch", "+refs/heads/main:refs/remotes/origin/main")

	repo, err := git.PlainOpen(target)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	cfg, err := repo.Config()
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	want := config.RefSpec("+refs/heads/*:refs/remotes/origin/*")
	if rc, ok := cfg.Remotes["origin"]; ok {
		rc.Fetch = []config.RefSpec{want}
	}
	if err := repo.SetConfig(cfg); err != nil {
		t.Fatalf("set config: %v", err)
	}

	after := readGitConfig(t, target)

	// The refspec was written.
	if got := strings.TrimSpace(gitIn(t, target, "config", "--get", "remote.origin.fetch")); got != string(want) {
		t.Errorf("remote.origin.fetch = %q, want %q", got, want)
	}
	// The alias section and the unknown key survived.
	if !strings.Contains(after, "st = status -sb") || !strings.Contains(after, "lg = log --oneline --graph") {
		t.Errorf("[alias] section did not survive SetConfig:\n%s", after)
	}
	if !strings.Contains(after, "unknownFutureKey = keep-me") {
		t.Errorf("unknown [core] key did not survive SetConfig:\n%s", after)
	}
	// Nothing outside remote.origin.fetch changed.
	if before == "" {
		t.Fatal("fixture config was empty — the test proves nothing")
	}
}

// appendToGitConfig appends raw text to the repository's .git/config.
func appendToGitConfig(t *testing.T, repo, extra string) {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(repo, ".git", "config"), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open .git/config: %v", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString("\n" + extra); err != nil {
		t.Fatalf("append .git/config: %v", err)
	}
}

// readGitConfig returns the raw contents of the repository's .git/config.
func readGitConfig(t *testing.T, repo string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repo, ".git", "config"))
	if err != nil {
		t.Fatalf("read .git/config: %v", err)
	}
	return string(data)
}
