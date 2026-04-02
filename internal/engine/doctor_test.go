package engine

import (
	"encoding/json"
	"os"
	"testing"

	handlerskg "github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/parser"
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

func TestCheckBlueprintURLs_MangledAbsPath(t *testing.T) {
	// The exact form found on the remote machine: normalizePath() prepended
	// the home directory to the single-slash URL, producing an absolute path.
	status := &handlerskg.Status{
		Packages: []handlerskg.PackageStatus{
			{Name: "vim", Blueprint: "/home/elpic/https:/github.com/elpic/setup.git", OS: "linux"},
		},
	}

	issues := checkBlueprintURLs(status)
	if len(issues) == 0 {
		t.Fatal("expected issues for mangled absolute path blueprint, got none")
	}
	if issues[0].examples[0] != "/home/elpic/https:/github.com/elpic/setup.git → https://github.com/elpic/setup" {
		t.Errorf("unexpected example: %s", issues[0].examples[0])
	}
}

func TestCheckDuplicates_MangledAbsPath(t *testing.T) {
	// Mangled path + correct URL should be detected as duplicates.
	status := &handlerskg.Status{
		Packages: []handlerskg.PackageStatus{
			{Name: "vim", Blueprint: "/home/elpic/https:/github.com/elpic/setup.git", OS: "linux"},
			{Name: "vim", Blueprint: "https://github.com/elpic/setup", OS: "linux"},
		},
	}

	issues := checkDuplicates(status)
	if len(issues) == 0 {
		t.Fatal("expected duplicate issues for mangled+correct blueprint pair, got none")
	}
	if issues[0].count != 1 {
		t.Errorf("expected count 1, got %d", issues[0].count)
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

func TestDeduplicateAfterMigrate_ReducesCount(t *testing.T) {
	// Simulates the full fix path: MigrateStatus then DeduplicateStatus.
	// Before fix: 4 packages (2 pairs of duplicates under different URL forms).
	// After fix: 2 packages (one per unique name+os+blueprint).
	status := handlerskg.Status{
		Packages: []handlerskg.PackageStatus{
			{Name: "vim", Blueprint: "https:/github.com/elpic/setup.git", OS: "linux"},
			{Name: "git", Blueprint: "https:/github.com/elpic/setup.git", OS: "linux"},
			{Name: "vim", Blueprint: "https://github.com/elpic/setup", OS: "linux"},
			{Name: "git", Blueprint: "https://github.com/elpic/setup", OS: "linux"},
		},
	}

	handlerskg.MigrateStatus(&status)
	handlerskg.DeduplicateStatus(&status)

	if len(status.Packages) != 2 {
		t.Errorf("expected 2 packages after dedup, got %d", len(status.Packages))
	}
	// The newer entries (originally from https://github.com/elpic/setup) should be kept.
	for _, p := range status.Packages {
		if p.Blueprint != "https://github.com/elpic/setup" {
			t.Errorf("expected blueprint https://github.com/elpic/setup, got %q", p.Blueprint)
		}
	}
}

func TestDeduplicateAfterMigrate_RunsAndDownloads(t *testing.T) {
	// Simulates the exact run/download entries from the remote status output.
	// Entry 1: old command (asdf which nvim) — genuinely different, NOT a duplicate
	// Entry 2: calibre installer with old blueprint — duplicate of entry 4
	// Entry 3: nvim link with old blueprint — duplicate of entry 5
	// Entry 4: calibre installer with new blueprint
	// Entry 5: nvim link with new blueprint
	status := handlerskg.Status{
		Downloads: []handlerskg.DownloadStatus{
			{Path: "~/.oh-my-zsh/antigen.zsh", Blueprint: "https:/github.com/elpic/setup.git", OS: "linux"},
			{Path: "~/.oh-my-zsh/antigen.zsh", Blueprint: "https://github.com/elpic/setup", OS: "linux"},
		},
		Runs: []handlerskg.RunStatus{
			{Command: "rm -rf ~/.local/bin/vim && ln -s $(asdf which nvim) ~/.local/bin/vim", Blueprint: "https:/github.com/elpic/setup.git", OS: "linux"},
			{Command: "https://download.calibre-ebook.com/linux-installer.sh", Blueprint: "https:/github.com/elpic/setup.git", OS: "linux"},
			{Command: "rm -rf ~/.local/bin/vim && ln -s $(which nvim) ~/.local/bin/vim", Blueprint: "https:/github.com/elpic/setup.git", OS: "linux"},
			{Command: "https://download.calibre-ebook.com/linux-installer.sh", Blueprint: "https://github.com/elpic/setup", OS: "linux"},
			{Command: "rm -rf ~/.local/bin/vim && ln -s $(which nvim) ~/.local/bin/vim", Blueprint: "https://github.com/elpic/setup", OS: "linux"},
		},
	}

	// checkDuplicates should find: 1 download dup + 2 run dups = 3
	issues := checkDuplicates(&status)
	if len(issues) == 0 {
		t.Fatal("expected duplicate issues, got none")
	}
	if issues[0].count != 3 {
		t.Errorf("expected count 3, got %d", issues[0].count)
	}

	// After migrate+dedup: 1 download, 3 runs (asdf-nvim kept as unique, calibre+nvim-link deduplicated to newest)
	handlerskg.MigrateStatus(&status)
	handlerskg.DeduplicateStatus(&status)

	if len(status.Downloads) != 1 {
		t.Errorf("expected 1 download after dedup, got %d", len(status.Downloads))
	}
	if len(status.Runs) != 3 {
		t.Errorf("expected 3 runs after dedup (asdf-nvim + calibre + nvim-link), got %d", len(status.Runs))
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

func TestDoctorFixture_DoctorDuplicatesStatus(t *testing.T) {
	// Loads testdata/doctor-duplicates-status.json — mirrors what was found on the
	// remote machine: mangled absolute-path blueprint URLs paired with their
	// correct forms, producing both stale-URL and duplicate issues.
	data, err := os.ReadFile("testdata/doctor-duplicates-status.json")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	var status handlerskg.Status
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatalf("failed to parse fixture: %v", err)
	}

	urlIssues := checkBlueprintURLs(&status)
	if len(urlIssues) == 0 {
		t.Fatal("expected stale URL issues, got none")
	}
	// 2 packages + 1 download + 3 runs (calibre, asdf-nvim, nvim-link) all have mangled blueprints = 6 stale entries.
	if urlIssues[0].count != 6 {
		t.Errorf("expected 6 stale URL entries, got %d", urlIssues[0].count)
	}

	dupIssues := checkDuplicates(&status)
	if len(dupIssues) == 0 {
		t.Fatal("expected duplicate issues, got none")
	}
	// 2 package dups + 1 download dup + 2 run dups (calibre + nvim-link) = 5.
	if dupIssues[0].count != 5 {
		t.Errorf("expected 5 duplicate entries, got %d", dupIssues[0].count)
	}

	// After fix: migrate then dedup — verify counts are clean.
	handlerskg.MigrateStatus(&status)
	handlerskg.DeduplicateStatus(&status)

	if len(checkBlueprintURLs(&status)) != 0 {
		t.Error("expected no stale URL issues after fix")
	}
	if len(checkDuplicates(&status)) != 0 {
		t.Error("expected no duplicate issues after fix")
	}
	// 5 unique packages (vim, git, curl), 1 download, 3 runs (calibre, asdf-nvim, nvim-link).
	if len(status.Packages) != 3 {
		t.Errorf("expected 3 packages after dedup, got %d", len(status.Packages))
	}
	if len(status.Downloads) != 1 {
		t.Errorf("expected 1 download after dedup, got %d", len(status.Downloads))
	}
	if len(status.Runs) != 3 {
		t.Errorf("expected 3 runs after dedup, got %d", len(status.Runs))
	}
}

// loaderFromFile parses testdata/<filename> and returns a loader func that
// serves those rules for any blueprint URL. Used to inject fixture blueprints
// into checkOrphansWithLoader without touching the filesystem cache.
func loaderFromFile(t *testing.T, filename string) func(string) []parser.Rule {
	t.Helper()
	rules, err := parser.ParseFile("testdata/" + filename)
	if err != nil {
		t.Fatalf("failed to parse fixture blueprint %s: %v", filename, err)
	}
	return func(_ string) []parser.Rule { return rules }
}

func TestCheckOrphans_DetectsRemovedRunCommand(t *testing.T) {
	// doctor-orphan-setup.bp contains two run commands:
	//   run: rm -rf ~/.local/bin/vim && ln -s $(which nvim) ~/.local/bin/vim
	//   run: https://download.calibre-ebook.com/linux-installer.sh
	//
	// Status has three run entries — the third (asdf which nvim) is orphaned.
	status := &handlerskg.Status{
		Runs: []handlerskg.RunStatus{
			{Command: "rm -rf ~/.local/bin/vim && ln -s $(which nvim) ~/.local/bin/vim", Blueprint: "https://github.com/elpic/setup", OS: "linux"},
			{Command: "https://download.calibre-ebook.com/linux-installer.sh", Blueprint: "https://github.com/elpic/setup", OS: "linux"},
			{Command: "rm -rf ~/.local/bin/vim && ln -s $(asdf which nvim) ~/.local/bin/vim", Blueprint: "https://github.com/elpic/setup", OS: "linux"},
		},
	}

	issues := checkOrphansWithLoader(status, loaderFromFile(t, "doctor-orphan-setup.bp"))
	if len(issues) == 0 {
		t.Fatal("expected orphan issues, got none")
	}
	if issues[0].count != 1 {
		t.Errorf("expected 1 orphaned entry, got %d", issues[0].count)
	}
	if len(issues[0].examples) == 0 || issues[0].examples[0] == "" {
		t.Error("expected at least one example in orphan issue")
	}
}

func TestCheckOrphans_NoOrphans(t *testing.T) {
	// All run commands exist in the blueprint — no orphans expected.
	status := &handlerskg.Status{
		Runs: []handlerskg.RunStatus{
			{Command: "rm -rf ~/.local/bin/vim && ln -s $(which nvim) ~/.local/bin/vim", Blueprint: "https://github.com/elpic/setup", OS: "linux"},
			{Command: "https://download.calibre-ebook.com/linux-installer.sh", Blueprint: "https://github.com/elpic/setup", OS: "linux"},
		},
	}

	issues := checkOrphansWithLoader(status, loaderFromFile(t, "doctor-orphan-setup.bp"))
	if len(issues) != 0 {
		t.Errorf("expected no orphan issues, got %d: %+v", len(issues), issues)
	}
}

func TestCheckOrphans_SkipsUncachedBlueprint(t *testing.T) {
	// loader returns nil (blueprint not cached) — orphan check must be skipped.
	status := &handlerskg.Status{
		Runs: []handlerskg.RunStatus{
			{Command: "some-old-command", Blueprint: "https://github.com/elpic/setup", OS: "linux"},
		},
	}
	loader := func(_ string) []parser.Rule { return nil }
	issues := checkOrphansWithLoader(status, loader)
	if len(issues) != 0 {
		t.Errorf("expected no issues when blueprint not cached, got %d", len(issues))
	}
}

func TestCheckOrphans_ElpicSetupRealStatus(t *testing.T) {
	// Mirrors the exact runs from the real https://github.com/elpic/setup status:
	//
	//   ✓ rm -rf ~/.local/bin/vim && ln -s $(asdf which nvim) ~/.local/bin/vim  ← ORPHANED
	//   ✓ https://download.calibre-ebook.com/linux-installer.sh                  ← still present (run-sh)
	//   ✓ rm -rf ~/.local/bin/vim && ln -s $(which nvim) ~/.local/bin/vim        ← still present
	//
	// The fixture blueprint (elpic-setup-runs.bp) has the two current commands
	// but not the old "asdf which nvim" one, so it should be flagged as orphaned.
	status := &handlerskg.Status{
		Runs: []handlerskg.RunStatus{
			{
				Action:    "run",
				Command:   "rm -rf ~/.local/bin/vim && ln -s $(asdf which nvim) ~/.local/bin/vim",
				Blueprint: "https://github.com/elpic/setup",
				OS:        "linux",
			},
			{
				Action:    "run-sh",
				Command:   "https://download.calibre-ebook.com/linux-installer.sh",
				Blueprint: "https://github.com/elpic/setup",
				OS:        "linux",
			},
			{
				Action:    "run",
				Command:   "rm -rf ~/.local/bin/vim && ln -s $(which nvim) ~/.local/bin/vim",
				Blueprint: "https://github.com/elpic/setup",
				OS:        "linux",
			},
		},
	}

	issues := checkOrphansWithLoader(status, loaderFromFile(t, "elpic-setup-runs.bp"))
	if len(issues) == 0 {
		t.Fatal("expected orphan issue for removed 'asdf which nvim' command, got none")
	}
	if issues[0].count != 1 {
		t.Errorf("expected 1 orphaned entry, got %d", issues[0].count)
	}
	if len(issues[0].examples) == 0 {
		t.Fatal("expected at least one example in orphan issue")
	}
	want := "rm -rf ~/.local/bin/vim && ln -s $(asdf which nvim) ~/.local/bin/vim"
	if issues[0].examples[0] != want+" (blueprint: https://github.com/elpic/setup)" {
		t.Errorf("unexpected orphan example:\n  got:  %s\n  want: %s (blueprint: https://github.com/elpic/setup)", issues[0].examples[0], want)
	}
}

func TestCheckOrphans_FixRemovesOrphanedEntry(t *testing.T) {
	// Verifies that the fix func attached to the orphan issue actually removes
	// the orphaned run entry from status, leaving the two valid entries intact.
	status := &handlerskg.Status{
		Runs: []handlerskg.RunStatus{
			{
				Action:    "run",
				Command:   "rm -rf ~/.local/bin/vim && ln -s $(asdf which nvim) ~/.local/bin/vim",
				Blueprint: "https://github.com/elpic/setup",
				OS:        "linux",
			},
			{
				Action:    "run-sh",
				Command:   "https://download.calibre-ebook.com/linux-installer.sh",
				Blueprint: "https://github.com/elpic/setup",
				OS:        "linux",
			},
			{
				Action:    "run",
				Command:   "rm -rf ~/.local/bin/vim && ln -s $(which nvim) ~/.local/bin/vim",
				Blueprint: "https://github.com/elpic/setup",
				OS:        "linux",
			},
		},
	}

	issues := checkOrphansWithLoader(status, loaderFromFile(t, "elpic-setup-runs.bp"))
	if len(issues) == 0 || issues[0].fix == nil {
		t.Fatal("expected orphan issue with fix func")
	}

	issues[0].fix()

	if len(status.Runs) != 2 {
		t.Errorf("expected 2 runs after fix, got %d", len(status.Runs))
	}
	for _, r := range status.Runs {
		if r.Command == "rm -rf ~/.local/bin/vim && ln -s $(asdf which nvim) ~/.local/bin/vim" {
			t.Error("orphaned 'asdf which nvim' run entry was not removed")
		}
	}
}

func TestCheckStaleSymlinks_NoLinks(t *testing.T) {
	status := &handlerskg.Status{}
	issues := checkStaleSymlinks(status)
	if len(issues) != 0 {
		t.Errorf("expected 0 issues for empty status, got %d", len(issues))
	}
}

func TestCheckStaleSymlinks_AllValid(t *testing.T) {
	// Create a real symlink pointing to a real file.
	dir := t.TempDir()
	target := dir + "/target.txt"
	link := dir + "/link"
	if err := os.WriteFile(target, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	status := &handlerskg.Status{
		Dotfiles: []handlerskg.DotfilesStatus{
			{URL: "https://github.com/u/dots", Path: dir, Links: []string{link}, OS: "linux", Blueprint: "https://github.com/u/setup"},
		},
	}
	issues := checkStaleSymlinks(status)
	if len(issues) != 0 {
		t.Errorf("expected 0 issues for valid symlink, got %d", len(issues))
	}
}

func TestCheckStaleSymlinks_BrokenSymlink(t *testing.T) {
	dir := t.TempDir()
	link := dir + "/broken"
	// Create symlink pointing to a non-existent target.
	if err := os.Symlink(dir+"/nonexistent", link); err != nil {
		t.Fatal(err)
	}

	status := &handlerskg.Status{
		Dotfiles: []handlerskg.DotfilesStatus{
			{URL: "https://github.com/u/dots", Path: dir, Links: []string{link}, OS: "linux", Blueprint: "https://github.com/u/setup"},
		},
	}
	issues := checkStaleSymlinks(status)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for broken symlink, got %d", len(issues))
	}
	if issues[0].count != 1 {
		t.Errorf("expected count 1, got %d", issues[0].count)
	}
}

func TestCheckStaleSymlinks_MissingPath(t *testing.T) {
	// Link path does not exist at all.
	status := &handlerskg.Status{
		Dotfiles: []handlerskg.DotfilesStatus{
			{URL: "https://github.com/u/dots", Path: "/tmp/dots", Links: []string{"/tmp/nonexistent_link_xyz"}, OS: "linux", Blueprint: "https://github.com/u/setup"},
		},
	}
	issues := checkStaleSymlinks(status)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for missing path, got %d", len(issues))
	}
}

func TestCheckStaleSymlinks_Fix(t *testing.T) {
	dir := t.TempDir()
	link := dir + "/broken"
	if err := os.Symlink(dir+"/nonexistent", link); err != nil {
		t.Fatal(err)
	}

	status := &handlerskg.Status{
		Dotfiles: []handlerskg.DotfilesStatus{
			{URL: "https://github.com/u/dots", Path: dir, Links: []string{link}, OS: "linux", Blueprint: "https://github.com/u/setup"},
		},
	}
	issues := checkStaleSymlinks(status)
	if len(issues) == 0 || issues[0].fix == nil {
		t.Fatal("expected issue with fix func")
	}
	issues[0].fix()

	// Symlink should be removed from disk.
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Error("expected broken symlink to be removed from disk")
	}
	// Entry with no remaining links should be removed from status.
	if len(status.Dotfiles) != 0 {
		t.Errorf("expected dotfiles entry to be removed, got %d entries", len(status.Dotfiles))
	}
}

func TestCheckMissingCloneDirs_NoDirs(t *testing.T) {
	status := &handlerskg.Status{}
	issues := checkMissingCloneDirs(status)
	if len(issues) != 0 {
		t.Errorf("expected 0 issues for empty status, got %d", len(issues))
	}
}

func TestCheckMissingCloneDirs_AllPresent(t *testing.T) {
	dir := t.TempDir()
	status := &handlerskg.Status{
		Clones: []handlerskg.CloneStatus{
			{URL: "https://github.com/u/repo", Path: dir, Blueprint: "https://github.com/u/setup", OS: "linux"},
		},
	}
	issues := checkMissingCloneDirs(status)
	if len(issues) != 0 {
		t.Errorf("expected 0 issues for existing dir, got %d", len(issues))
	}
}

func TestCheckMissingCloneDirs_Missing(t *testing.T) {
	status := &handlerskg.Status{
		Clones: []handlerskg.CloneStatus{
			{URL: "https://github.com/u/repo", Path: "/tmp/does_not_exist_xyz_123", Blueprint: "https://github.com/u/setup", OS: "linux"},
		},
	}
	issues := checkMissingCloneDirs(status)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for missing dir, got %d", len(issues))
	}
	if issues[0].count != 1 {
		t.Errorf("expected count 1, got %d", issues[0].count)
	}
}

func TestCheckMissingCloneDirs_Fix(t *testing.T) {
	status := &handlerskg.Status{
		Clones: []handlerskg.CloneStatus{
			{URL: "https://github.com/u/repo", Path: "/tmp/does_not_exist_xyz_123", Blueprint: "https://github.com/u/setup", OS: "linux"},
		},
	}
	issues := checkMissingCloneDirs(status)
	if len(issues) == 0 || issues[0].fix == nil {
		t.Fatal("expected issue with fix func")
	}
	issues[0].fix()

	if len(status.Clones) != 0 {
		t.Errorf("expected clone entry to be removed, got %d entries", len(status.Clones))
	}
}

func TestExpandHomedir(t *testing.T) {
	home, _ := os.UserHomeDir()
	tests := []struct {
		input string
		want  string
	}{
		{"~/foo/bar", home + "/foo/bar"},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"", ""},
	}
	for _, tt := range tests {
		got := expandHomedir(tt.input)
		if got != tt.want {
			t.Errorf("expandHomedir(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCheckMissingDownloadFiles_NoneTracked(t *testing.T) {
	status := &handlerskg.Status{}
	issues := checkMissingDownloadFiles(status)
	if len(issues) != 0 {
		t.Errorf("expected 0 issues for empty status, got %d", len(issues))
	}
}

func TestCheckMissingDownloadFiles_AllPresent(t *testing.T) {
	f := t.TempDir() + "/file.sh"
	if err := os.WriteFile(f, []byte("#!/bin/sh"), 0600); err != nil {
		t.Fatal(err)
	}
	status := &handlerskg.Status{
		Downloads: []handlerskg.DownloadStatus{
			{URL: "https://example.com/file.sh", Path: f, Blueprint: "https://github.com/u/setup", OS: "mac"},
		},
	}
	issues := checkMissingDownloadFiles(status)
	if len(issues) != 0 {
		t.Errorf("expected 0 issues for existing file, got %d", len(issues))
	}
}

func TestCheckMissingDownloadFiles_Missing(t *testing.T) {
	status := &handlerskg.Status{
		Downloads: []handlerskg.DownloadStatus{
			{URL: "https://example.com/file.sh", Path: "/tmp/does_not_exist_dl_xyz", Blueprint: "https://github.com/u/setup", OS: "mac"},
		},
	}
	issues := checkMissingDownloadFiles(status)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for missing file, got %d", len(issues))
	}
	if issues[0].count != 1 {
		t.Errorf("expected count 1, got %d", issues[0].count)
	}
}

func TestCheckMissingDownloadFiles_Fix(t *testing.T) {
	status := &handlerskg.Status{
		Downloads: []handlerskg.DownloadStatus{
			{URL: "https://example.com/file.sh", Path: "/tmp/does_not_exist_dl_xyz", Blueprint: "https://github.com/u/setup", OS: "mac"},
		},
	}
	issues := checkMissingDownloadFiles(status)
	if len(issues) == 0 || issues[0].fix == nil {
		t.Fatal("expected issue with fix func")
	}
	issues[0].fix()

	if len(status.Downloads) != 0 {
		t.Errorf("expected download entry to be removed, got %d entries", len(status.Downloads))
	}
}
