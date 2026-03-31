package unit

import (
	"testing"

	"github.com/elpic/blueprint/internal/handlers"
)

// TestStatusEntry_GettersSetters covers the StatusEntry interface methods on all status types.
func TestStatusEntry_PackageStatus(t *testing.T) {
	ps := handlers.PackageStatus{
		Name:      "curl",
		Blueprint: "git@github.com:user/repo.git",
		OS:        "linux",
	}

	if ps.GetBlueprint() != "git@github.com:user/repo.git" {
		t.Errorf("GetBlueprint() = %q, want %q", ps.GetBlueprint(), "git@github.com:user/repo.git")
	}
	if ps.GetResourceKey() != "curl" {
		t.Errorf("GetResourceKey() = %q, want %q", ps.GetResourceKey(), "curl")
	}
	if ps.GetOS() != "linux" {
		t.Errorf("GetOS() = %q, want %q", ps.GetOS(), "linux")
	}
	if ps.GetAction() != "install" {
		t.Errorf("GetAction() = %q, want %q", ps.GetAction(), "install")
	}
	ps.SetBlueprint("https://github.com/user/repo")
	if ps.GetBlueprint() != "https://github.com/user/repo" {
		t.Errorf("after SetBlueprint, GetBlueprint() = %q, want %q", ps.GetBlueprint(), "https://github.com/user/repo")
	}
}

func TestStatusEntry_CloneStatus(t *testing.T) {
	cs := handlers.CloneStatus{
		URL:       "https://github.com/user/repo",
		Path:      "/home/user/repo",
		Blueprint: "/home/user/blueprint.yml",
		OS:        "mac",
	}

	if cs.GetBlueprint() != "/home/user/blueprint.yml" {
		t.Errorf("GetBlueprint() = %q", cs.GetBlueprint())
	}
	if cs.GetResourceKey() != "/home/user/repo" {
		t.Errorf("GetResourceKey() = %q, want path", cs.GetResourceKey())
	}
	if cs.GetOS() != "mac" {
		t.Errorf("GetOS() = %q", cs.GetOS())
	}
	if cs.GetAction() != "clone" {
		t.Errorf("GetAction() = %q, want clone", cs.GetAction())
	}
	cs.SetBlueprint("new-blueprint")
	if cs.Blueprint != "new-blueprint" {
		t.Errorf("SetBlueprint did not update field")
	}
}

func TestStatusEntry_MkdirStatus(t *testing.T) {
	ms := handlers.MkdirStatus{
		Path:      "/home/user/workspace",
		Blueprint: "/etc/blueprint.yml",
		OS:        "linux",
	}

	if ms.GetBlueprint() != "/etc/blueprint.yml" {
		t.Errorf("GetBlueprint() = %q", ms.GetBlueprint())
	}
	if ms.GetResourceKey() != "/home/user/workspace" {
		t.Errorf("GetResourceKey() = %q", ms.GetResourceKey())
	}
	if ms.GetOS() != "linux" {
		t.Errorf("GetOS() = %q", ms.GetOS())
	}
	if ms.GetAction() != "mkdir" {
		t.Errorf("GetAction() = %q, want mkdir", ms.GetAction())
	}
	ms.SetBlueprint("updated")
	if ms.Blueprint != "updated" {
		t.Errorf("SetBlueprint did not update field")
	}
}

func TestStatusEntry_DecryptStatus(t *testing.T) {
	ds := handlers.DecryptStatus{
		SourceFile: "secrets.enc",
		DestPath:   "/home/user/.secrets",
		Blueprint:  "/tmp/blueprint.yml",
		OS:         "linux",
	}

	if ds.GetResourceKey() != "/home/user/.secrets" {
		t.Errorf("GetResourceKey() = %q, want dest path", ds.GetResourceKey())
	}
	if ds.GetAction() != "decrypt" {
		t.Errorf("GetAction() = %q, want decrypt", ds.GetAction())
	}
	ds.SetBlueprint("new")
	if ds.Blueprint != "new" {
		t.Errorf("SetBlueprint did not update field")
	}
}

func TestStatusEntry_KnownHostsStatus(t *testing.T) {
	khs := handlers.KnownHostsStatus{
		Host:      "github.com",
		KeyType:   "ecdsa",
		Blueprint: "/tmp/bp.yml",
		OS:        "linux",
	}

	if khs.GetResourceKey() != "github.com" {
		t.Errorf("GetResourceKey() = %q, want host", khs.GetResourceKey())
	}
	if khs.GetAction() != "known_hosts" {
		t.Errorf("GetAction() = %q, want known_hosts", khs.GetAction())
	}
	khs.SetBlueprint("x")
	if khs.Blueprint != "x" {
		t.Errorf("SetBlueprint did not update")
	}
}

func TestStatusEntry_GPGKeyStatus(t *testing.T) {
	gks := handlers.GPGKeyStatus{
		Keyring:   "/usr/share/keyrings/docker.gpg",
		Blueprint: "/tmp/bp.yml",
		OS:        "linux",
	}

	if gks.GetResourceKey() != "/usr/share/keyrings/docker.gpg" {
		t.Errorf("GetResourceKey() = %q, want keyring", gks.GetResourceKey())
	}
	if gks.GetAction() != "gpg_key" {
		t.Errorf("GetAction() = %q, want gpg_key", gks.GetAction())
	}
	gks.SetBlueprint("x")
	if gks.Blueprint != "x" {
		t.Errorf("SetBlueprint did not update")
	}
}

func TestStatusEntry_AsdfStatus(t *testing.T) {
	as := handlers.AsdfStatus{
		Plugin:    "nodejs",
		Version:   "18.0.0",
		Blueprint: "/tmp/bp.yml",
		OS:        "linux",
	}

	if as.GetResourceKey() != "nodejs\x0018.0.0" {
		t.Errorf("GetResourceKey() = %q, want plugin+version key", as.GetResourceKey())
	}
	if as.GetAction() != "asdf" {
		t.Errorf("GetAction() = %q, want asdf", as.GetAction())
	}
	as.SetBlueprint("y")
	if as.Blueprint != "y" {
		t.Errorf("SetBlueprint did not update")
	}
}

func TestStatusEntry_MiseStatus(t *testing.T) {
	ms := handlers.MiseStatus{
		Tool:      "node",
		Version:   "18.0.0",
		Blueprint: "/tmp/bp.yml",
		OS:        "linux",
	}

	if ms.GetResourceKey() != "node\x0018.0.0" {
		t.Errorf("GetResourceKey() = %q, want tool+version key", ms.GetResourceKey())
	}
	if ms.GetAction() != "mise" {
		t.Errorf("GetAction() = %q, want mise", ms.GetAction())
	}
	ms.SetBlueprint("z")
	if ms.Blueprint != "z" {
		t.Errorf("SetBlueprint did not update")
	}
}

func TestStatusEntry_SudoersStatus(t *testing.T) {
	ss := handlers.SudoersStatus{
		User:      "alice",
		Blueprint: "/tmp/bp.yml",
		OS:        "linux",
	}

	if ss.GetResourceKey() != "alice" {
		t.Errorf("GetResourceKey() = %q, want user", ss.GetResourceKey())
	}
	if ss.GetAction() != "sudoers" {
		t.Errorf("GetAction() = %q, want sudoers", ss.GetAction())
	}
	ss.SetBlueprint("updated")
	if ss.Blueprint != "updated" {
		t.Errorf("SetBlueprint did not update")
	}
}

func TestStatusEntry_HomebrewStatus(t *testing.T) {
	hs := handlers.HomebrewStatus{
		Formula:   "git",
		Blueprint: "/tmp/bp.yml",
		OS:        "mac",
	}

	if hs.GetResourceKey() != "git" {
		t.Errorf("GetResourceKey() = %q, want formula", hs.GetResourceKey())
	}
	if hs.GetAction() != "homebrew" {
		t.Errorf("GetAction() = %q, want homebrew", hs.GetAction())
	}
	hs.SetBlueprint("x")
	if hs.Blueprint != "x" {
		t.Errorf("SetBlueprint did not update")
	}
}

func TestStatusEntry_OllamaStatus(t *testing.T) {
	os := handlers.OllamaStatus{
		Model:     "llama2",
		Blueprint: "/tmp/bp.yml",
		OS:        "mac",
	}

	if os.GetResourceKey() != "llama2" {
		t.Errorf("GetResourceKey() = %q, want model", os.GetResourceKey())
	}
	if os.GetAction() != "ollama" {
		t.Errorf("GetAction() = %q, want ollama", os.GetAction())
	}
	os.SetBlueprint("x")
	if os.Blueprint != "x" {
		t.Errorf("SetBlueprint did not update")
	}
}

func TestStatusEntry_DownloadStatus(t *testing.T) {
	ds := handlers.DownloadStatus{
		URL:       "https://example.com/file.tar.gz",
		Path:      "/usr/local/bin/tool",
		Blueprint: "/tmp/bp.yml",
		OS:        "linux",
	}

	if ds.GetResourceKey() != "/usr/local/bin/tool" {
		t.Errorf("GetResourceKey() = %q, want path", ds.GetResourceKey())
	}
	if ds.GetAction() != "download" {
		t.Errorf("GetAction() = %q, want download", ds.GetAction())
	}
	ds.SetBlueprint("new")
	if ds.Blueprint != "new" {
		t.Errorf("SetBlueprint did not update")
	}
}

func TestStatusEntry_RunStatus(t *testing.T) {
	rs := handlers.RunStatus{
		Action:    "run",
		Command:   "echo hello",
		Blueprint: "/tmp/bp.yml",
		OS:        "linux",
	}

	if rs.GetResourceKey() != "echo hello" {
		t.Errorf("GetResourceKey() = %q, want command", rs.GetResourceKey())
	}
	if rs.GetAction() != "run" {
		t.Errorf("GetAction() = %q, want run", rs.GetAction())
	}
	rs.SetBlueprint("x")
	if rs.Blueprint != "x" {
		t.Errorf("SetBlueprint did not update")
	}
}

func TestStatusEntry_DotfilesStatus(t *testing.T) {
	ds := handlers.DotfilesStatus{
		URL:       "https://github.com/user/dotfiles",
		Path:      "/home/user/.dotfiles",
		Blueprint: "/tmp/bp.yml",
		OS:        "linux",
	}

	if ds.GetResourceKey() != "https://github.com/user/dotfiles" {
		t.Errorf("GetResourceKey() = %q, want URL", ds.GetResourceKey())
	}
	if ds.GetAction() != "dotfiles" {
		t.Errorf("GetAction() = %q, want dotfiles", ds.GetAction())
	}
	ds.SetBlueprint("new")
	if ds.Blueprint != "new" {
		t.Errorf("SetBlueprint did not update")
	}
}

func TestStatusEntry_ScheduleStatus(t *testing.T) {
	ss := handlers.ScheduleStatus{
		CronExpr:  "0 * * * *",
		Source:    "http://example.com/blueprint.yml",
		Blueprint: "/tmp/bp.yml",
		OS:        "linux",
	}

	if ss.GetResourceKey() != "http://example.com/blueprint.yml" {
		t.Errorf("GetResourceKey() = %q, want source", ss.GetResourceKey())
	}
	if ss.GetAction() != "schedule" {
		t.Errorf("GetAction() = %q, want schedule", ss.GetAction())
	}
	ss.SetBlueprint("x")
	if ss.Blueprint != "x" {
		t.Errorf("SetBlueprint did not update")
	}
}

func TestStatusEntry_AuthorizedKeysStatus(t *testing.T) {
	aks := handlers.AuthorizedKeysStatus{
		Source:    "https://github.com/user.keys",
		Blueprint: "/tmp/bp.yml",
		OS:        "linux",
	}

	if aks.GetResourceKey() != "https://github.com/user.keys" {
		t.Errorf("GetResourceKey() = %q, want source", aks.GetResourceKey())
	}
	if aks.GetAction() != "authorized_keys" {
		t.Errorf("GetAction() = %q, want authorized_keys", aks.GetAction())
	}
	aks.SetBlueprint("x")
	if aks.Blueprint != "x" {
		t.Errorf("SetBlueprint did not update")
	}
}

func TestStatusEntry_ShellStatus(t *testing.T) {
	ss := handlers.ShellStatus{
		Shell:     "/bin/zsh",
		User:      "alice",
		Blueprint: "/tmp/bp.yml",
		OS:        "linux",
	}

	if ss.GetResourceKey() != "alice" {
		t.Errorf("GetResourceKey() = %q, want user", ss.GetResourceKey())
	}
	if ss.GetAction() != "shell" {
		t.Errorf("GetAction() = %q, want shell", ss.GetAction())
	}
	ss.SetBlueprint("x")
	if ss.Blueprint != "x" {
		t.Errorf("SetBlueprint did not update")
	}
}

// TestNormalizeBlueprint covers the exported NormalizeBlueprint function with all path types.
func TestNormalizeBlueprint(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "SSH git URL normalizes to HTTPS",
			input: "git@github.com:user/repo.git",
			want:  "https://github.com/user/repo",
		},
		{
			name:  "HTTPS git URL strips .git suffix",
			input: "https://github.com/user/repo.git",
			want:  "https://github.com/user/repo",
		},
		{
			name:  "HTTPS git URL already normalized",
			input: "https://github.com/user/repo",
			want:  "https://github.com/user/repo",
		},
		{
			name:  "mangled path with embedded https URL",
			input: "/home/user/https:/github.com/user/repo.git",
			want:  "https://github.com/user/repo",
		},
		{
			name:  "local absolute path returned as-is (normalized)",
			input: "/home/user/blueprint.yml",
			want:  "/home/user/blueprint.yml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := handlers.NormalizeBlueprint(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeBlueprint(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestMigrateStatus verifies blueprint fields are normalized across all entry types.
func TestMigrateStatus(t *testing.T) {
	status := &handlers.Status{
		Packages: []handlers.PackageStatus{
			{Name: "curl", Blueprint: "git@github.com:user/repo.git", OS: "linux"},
		},
		Clones: []handlers.CloneStatus{
			{URL: "https://github.com/user/repo", Path: "/tmp/repo", Blueprint: "https://github.com/user/repo.git", OS: "linux"},
		},
		Mkdirs: []handlers.MkdirStatus{
			{Path: "/tmp/dir", Blueprint: "/abs/path/blueprint.yml", OS: "linux"},
		},
	}

	handlers.MigrateStatus(status)

	if status.Packages[0].Blueprint != "https://github.com/user/repo" {
		t.Errorf("Packages[0].Blueprint = %q, want normalized HTTPS URL", status.Packages[0].Blueprint)
	}
	if status.Clones[0].Blueprint != "https://github.com/user/repo" {
		t.Errorf("Clones[0].Blueprint = %q, want normalized HTTPS URL", status.Clones[0].Blueprint)
	}
	// Local absolute path stays as absolute path
	if status.Mkdirs[0].Blueprint != "/abs/path/blueprint.yml" {
		t.Errorf("Mkdirs[0].Blueprint = %q, want absolute path unchanged", status.Mkdirs[0].Blueprint)
	}
}

// TestDeduplicateStatus verifies that duplicate entries (same resource, OS, blueprint) are deduplicated.
func TestDeduplicateStatus(t *testing.T) {
	status := &handlers.Status{
		Packages: []handlers.PackageStatus{
			{Name: "curl", Blueprint: "/test/bp.yml", OS: "linux", InstalledAt: "2024-01-01T00:00:00Z"},
			{Name: "git", Blueprint: "/test/bp.yml", OS: "linux", InstalledAt: "2024-01-01T00:00:00Z"},
			// Duplicate of curl — same resource+os+blueprint but different timestamp
			{Name: "curl", Blueprint: "/test/bp.yml", OS: "linux", InstalledAt: "2024-06-01T00:00:00Z"},
		},
		Clones: []handlers.CloneStatus{
			{Path: "/tmp/repo", Blueprint: "/test/bp.yml", OS: "linux", ClonedAt: "2024-01-01T00:00:00Z"},
			// Duplicate of above clone
			{Path: "/tmp/repo", Blueprint: "/test/bp.yml", OS: "linux", ClonedAt: "2024-06-01T00:00:00Z"},
		},
	}

	handlers.DeduplicateStatus(status)

	// After dedup, curl should appear only once
	curlCount := 0
	for _, p := range status.Packages {
		if p.Name == "curl" {
			curlCount++
		}
	}
	if curlCount != 1 {
		t.Errorf("expected 1 curl entry after dedup, got %d", curlCount)
	}

	// git is unique and should remain
	gitCount := 0
	for _, p := range status.Packages {
		if p.Name == "git" {
			gitCount++
		}
	}
	if gitCount != 1 {
		t.Errorf("expected 1 git entry after dedup, got %d", gitCount)
	}

	// Clone should be deduplicated to 1
	if len(status.Clones) != 1 {
		t.Errorf("expected 1 clone entry after dedup, got %d", len(status.Clones))
	}
}

// TestDeduplicateStatus_DifferentOSKept verifies entries with different OS are kept.
func TestDeduplicateStatus_DifferentOSKept(t *testing.T) {
	status := &handlers.Status{
		Packages: []handlers.PackageStatus{
			{Name: "curl", Blueprint: "/bp.yml", OS: "linux"},
			{Name: "curl", Blueprint: "/bp.yml", OS: "mac"},
		},
	}

	handlers.DeduplicateStatus(status)

	if len(status.Packages) != 2 {
		t.Errorf("expected 2 entries (different OS), got %d", len(status.Packages))
	}
}

// TestDeduplicateStatus_NormalizesBlueprintForComparison verifies that blueprint normalization
// is applied when comparing for duplicates (SSH URL == HTTPS URL).
func TestDeduplicateStatus_NormalizesBlueprintForComparison(t *testing.T) {
	status := &handlers.Status{
		Packages: []handlers.PackageStatus{
			{Name: "curl", Blueprint: "git@github.com:user/repo.git", OS: "linux"},
			// Same blueprint expressed as HTTPS — should be considered duplicate
			{Name: "curl", Blueprint: "https://github.com/user/repo", OS: "linux"},
		},
	}

	handlers.DeduplicateStatus(status)

	if len(status.Packages) != 1 {
		t.Errorf("expected 1 entry after dedup (same blueprint, different URL form), got %d", len(status.Packages))
	}
}
