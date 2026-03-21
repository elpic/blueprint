package engine

import (
	"testing"

	handlerskg "github.com/elpic/blueprint/internal/handlers"
)

func TestCheckBlueprintURLs_CleanStatus(t *testing.T) {
	// All blueprint fields already normalized — expect no issues.
	status := &handlerskg.Status{
		Packages: []handlerskg.PackageStatus{
			{Name: "vim", Blueprint: "https://github.com/user/setup", OS: "linux"},
		},
		Clones: []handlerskg.CloneStatus{
			{URL: "https://github.com/user/dotfiles", Path: "/tmp/dotfiles", Blueprint: "https://github.com/user/setup", OS: "linux"},
		},
	}

	issues := checkBlueprintURLs(status)
	if len(issues) != 0 {
		t.Errorf("expected 0 issues for clean status, got %d: %+v", len(issues), issues)
	}
}

func TestCheckBlueprintURLs_StaleSSHURLs(t *testing.T) {
	// Blueprint fields stored with SSH + .git suffix — should be flagged.
	staleBlueprint := "git@github.com:user/setup.git"
	status := &handlerskg.Status{
		Packages: []handlerskg.PackageStatus{
			{Name: "vim", Blueprint: staleBlueprint, OS: "linux"},
			{Name: "git", Blueprint: staleBlueprint, OS: "linux"},
		},
		Clones: []handlerskg.CloneStatus{
			{URL: "https://github.com/user/repo", Path: "/tmp/repo", Blueprint: staleBlueprint, OS: "linux"},
		},
	}

	issues := checkBlueprintURLs(status)
	if len(issues) == 0 {
		t.Fatal("expected issues for stale SSH blueprint URLs, got none")
	}
	if issues[0].count != 3 {
		t.Errorf("expected count 3 (3 stale entries), got %d", issues[0].count)
	}
	if len(issues[0].examples) == 0 {
		t.Error("expected at least one example in issue, got none")
	}
}

func TestCheckBlueprintURLs_MixedForms(t *testing.T) {
	// Mix of stale and normalized blueprint fields.
	staleBlueprint := "git@github.com:elpic/setup.git"
	normalBlueprint := "https://github.com/elpic/setup"

	status := &handlerskg.Status{
		Packages: []handlerskg.PackageStatus{
			{Name: "vim", Blueprint: staleBlueprint, OS: "linux"},
			{Name: "git", Blueprint: normalBlueprint, OS: "linux"},
		},
		Dotfiles: []handlerskg.DotfilesStatus{
			{URL: "https://github.com/elpic/dotfiles", Path: "/tmp/dots", Blueprint: normalBlueprint, OS: "linux"},
		},
	}

	issues := checkBlueprintURLs(status)
	if len(issues) == 0 {
		t.Fatal("expected issues for mixed blueprint URLs, got none")
	}
	// Only the one stale entry should be counted.
	if issues[0].count != 1 {
		t.Errorf("expected count 1 (only 1 stale entry), got %d", issues[0].count)
	}
}

func TestCheckBlueprintURLs_EmptyStatus(t *testing.T) {
	status := &handlerskg.Status{}
	issues := checkBlueprintURLs(status)
	if len(issues) != 0 {
		t.Errorf("expected 0 issues for empty status, got %d", len(issues))
	}
}

func TestCheckDuplicates_SingleSlashAndNormal(t *testing.T) {
	// Simulates the exact scenario from the remote machine:
	// same packages installed twice — once under "https:/host/repo.git" (old, malformed)
	// and once under "https://host/repo" (correct).
	status := &handlerskg.Status{
		Packages: []handlerskg.PackageStatus{
			{Name: "vim", Blueprint: "https:/github.com/elpic/setup.git", OS: "linux"},
			{Name: "vim", Blueprint: "https://github.com/elpic/setup", OS: "linux"},
			{Name: "git", Blueprint: "https:/github.com/elpic/setup.git", OS: "linux"},
			{Name: "git", Blueprint: "https://github.com/elpic/setup", OS: "linux"},
		},
	}

	issues := checkDuplicates(status)
	if len(issues) == 0 {
		t.Fatal("expected duplicate issues, got none")
	}
	if issues[0].count != 2 {
		t.Errorf("expected count 2 (2 duplicate packages), got %d", issues[0].count)
	}
}

func TestCheckDuplicates_NoDuplicates(t *testing.T) {
	status := &handlerskg.Status{
		Packages: []handlerskg.PackageStatus{
			{Name: "vim", Blueprint: "https://github.com/elpic/setup", OS: "linux"},
			{Name: "git", Blueprint: "https://github.com/elpic/setup", OS: "linux"},
		},
	}

	issues := checkDuplicates(status)
	if len(issues) != 0 {
		t.Errorf("expected 0 duplicate issues, got %d: %+v", len(issues), issues)
	}
}

func TestCheckBlueprintURLs_SingleSlash(t *testing.T) {
	// The single-slash form "https:/host/repo.git" should be detected as stale.
	status := &handlerskg.Status{
		Packages: []handlerskg.PackageStatus{
			{Name: "vim", Blueprint: "https:/github.com/elpic/setup.git", OS: "linux"},
		},
	}

	issues := checkBlueprintURLs(status)
	if len(issues) == 0 {
		t.Fatal("expected issues for single-slash blueprint URL, got none")
	}
	if issues[0].examples[0] != "https:/github.com/elpic/setup.git → https://github.com/elpic/setup" {
		t.Errorf("unexpected example: %s", issues[0].examples[0])
	}
}

func TestCheckBlueprintURLs_AllStatusTypes(t *testing.T) {
	// Verify that all status slice types are checked, not just Packages.
	staleBlueprint := "git@github.com:corp/blueprint.git"
	status := &handlerskg.Status{
		Decrypts:       []handlerskg.DecryptStatus{{SourceFile: "foo", DestPath: "/tmp/foo", Blueprint: staleBlueprint, OS: "darwin"}},
		Mkdirs:         []handlerskg.MkdirStatus{{Path: "/tmp/dir", Blueprint: staleBlueprint, OS: "darwin"}},
		KnownHosts:     []handlerskg.KnownHostsStatus{{Host: "github.com", Blueprint: staleBlueprint, OS: "darwin"}},
		GPGKeys:        []handlerskg.GPGKeyStatus{{Keyring: "/tmp/key.gpg", Blueprint: staleBlueprint, OS: "darwin"}},
		Asdfs:          []handlerskg.AsdfStatus{{Plugin: "nodejs", Version: "18.0.0", Blueprint: staleBlueprint, OS: "darwin"}},
		Mises:          []handlerskg.MiseStatus{{Tool: "node", Version: "18", Blueprint: staleBlueprint, OS: "darwin"}},
		Sudoers:        []handlerskg.SudoersStatus{{User: "alice", Blueprint: staleBlueprint, OS: "darwin"}},
		Brews:          []handlerskg.HomebrewStatus{{Formula: "jq", Blueprint: staleBlueprint, OS: "darwin"}},
		Ollamas:        []handlerskg.OllamaStatus{{Model: "llama3", Blueprint: staleBlueprint, OS: "darwin"}},
		Downloads:      []handlerskg.DownloadStatus{{URL: "https://example.com/file", Path: "/tmp/file", Blueprint: staleBlueprint, OS: "darwin"}},
		Runs:           []handlerskg.RunStatus{{Action: "run", Command: "echo hi", Blueprint: staleBlueprint, OS: "darwin"}},
		Dotfiles:       []handlerskg.DotfilesStatus{{URL: "https://github.com/user/dots", Path: "/tmp/dots", Blueprint: staleBlueprint, OS: "darwin"}},
		Schedules:      []handlerskg.ScheduleStatus{{CronExpr: "0 * * * *", Blueprint: staleBlueprint, OS: "darwin"}},
		AuthorizedKeys: []handlerskg.AuthorizedKeysStatus{{Source: "key.pub", Blueprint: staleBlueprint, OS: "darwin"}},
	}

	issues := checkBlueprintURLs(status)
	if len(issues) == 0 {
		t.Fatal("expected issues for stale blueprint URLs across all status types, got none")
	}
	// 14 slices each with 1 entry = 14 stale entries.
	if issues[0].count != 14 {
		t.Errorf("expected count 14, got %d", issues[0].count)
	}
}
