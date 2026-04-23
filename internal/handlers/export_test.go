package handlers

import (
	"strings"
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

// shellExport is a test helper that calls the ShellExport function for an action.
func shellExport(t *testing.T, action string, rule parser.Rule, format, osName string) []string {
	t.Helper()
	def := GetAction(action)
	if def == nil {
		t.Fatalf("action %q not registered", action)
	}
	if def.ShellExport == nil {
		return nil
	}
	return def.ShellExport(rule, format, osName)
}

func TestExportInstall_Mac(t *testing.T) {
	rule := parser.Rule{
		Action:   "install",
		Packages: []parser.Package{{Name: "vim"}, {Name: "git"}},
	}
	lines := shellExport(t, "install", rule, "bash", "mac")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "brew list --versions vim") || !strings.Contains(joined, "brew list --cask vim") {
		t.Error("expected both formula and cask check for vim")
	}
	if !strings.Contains(joined, "brew install vim") {
		t.Error("expected brew install vim")
	}
	if !strings.Contains(joined, "brew list --versions git") {
		t.Error("expected brew list check for git")
	}
}

func TestExportInstall_Linux(t *testing.T) {
	rule := parser.Rule{
		Action:   "install",
		Packages: []parser.Package{{Name: "vim"}, {Name: "git"}},
	}
	lines := shellExport(t, "install", rule, "bash", "linux")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "dpkg -s vim") {
		t.Error("expected dpkg check for vim")
	}
	if !strings.Contains(joined, "sudo apt-get install -y vim") {
		t.Error("expected apt-get install for vim")
	}
}

func TestExportInstall_SnapPackages(t *testing.T) {
	rule := parser.Rule{
		Action: "install",
		Packages: []parser.Package{
			{Name: "vim"},
			{Name: "code", PackageManager: "snap"},
		},
	}
	lines := shellExport(t, "install", rule, "bash", "linux")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "dpkg -s vim") {
		t.Error("expected dpkg check for vim")
	}
	if !strings.Contains(joined, "snap list code") {
		t.Error("expected snap list check for code")
	}
	if !strings.Contains(joined, "sudo snap install code") {
		t.Error("expected snap install for code")
	}
}

func TestExportClone(t *testing.T) {
	rule := parser.Rule{
		Action:    "clone",
		CloneURL:  "https://github.com/user/repo",
		ClonePath: "~/projects/repo",
		Branch:    "main",
	}
	lines := shellExport(t, "clone", rule, "bash", "mac")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "git clone") {
		t.Error("expected git clone")
	}
	if !strings.Contains(joined, "-b") {
		t.Error("expected branch flag")
	}
	if !strings.Contains(joined, "$HOME/projects/repo") {
		t.Errorf("expected $HOME path, got:\n%s", joined)
	}
	if !strings.Contains(joined, ".blueprint/repos") {
		t.Error("expected persistent cache dir under ~/.blueprint/repos")
	}
	if !strings.Contains(joined, "shasum") {
		t.Error("expected SHA-based cache key")
	}
	if !strings.Contains(joined, "rev-parse HEAD") {
		t.Error("expected SHA comparison via rev-parse")
	}
	if !strings.Contains(joined, "rsync") {
		t.Error("expected rsync copy excluding .git")
	}
	if !strings.Contains(joined, ".blueprint-sha") {
		t.Error("expected SHA marker file")
	}
}

func TestExportClone_NoBranch(t *testing.T) {
	rule := parser.Rule{
		Action:    "clone",
		CloneURL:  "https://github.com/user/repo",
		ClonePath: "~/projects/repo",
	}
	lines := shellExport(t, "clone", rule, "bash", "mac")
	joined := strings.Join(lines, "\n")
	if strings.Contains(joined, "-b") {
		t.Error("should not have branch flag when no branch specified")
	}
}

func TestExportClone_SkipsCopyWhenUpToDate(t *testing.T) {
	rule := parser.Rule{
		Action:    "clone",
		CloneURL:  "https://github.com/user/repo",
		ClonePath: "~/projects/repo",
	}
	lines := shellExport(t, "clone", rule, "bash", "mac")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, `"$CLONE_SHA" != "$OLD_SHA"`) {
		t.Error("expected SHA comparison to skip copy when up to date")
	}
}

func TestExportRun_WithUnless(t *testing.T) {
	rule := parser.Rule{
		Action:     "run",
		RunCommand: "echo hello",
		RunUnless:  "test -f /tmp/done",
	}
	lines := shellExport(t, "run", rule, "bash", "mac")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "if !") {
		t.Error("expected unless check")
	}
	if !strings.Contains(joined, "echo hello") {
		t.Error("expected command")
	}
}

func TestExportRun_Simple(t *testing.T) {
	rule := parser.Rule{
		Action:     "run",
		RunCommand: "echo hello",
	}
	lines := shellExport(t, "run", rule, "bash", "mac")
	if len(lines) != 1 || lines[0] != "echo hello" {
		t.Errorf("expected simple command, got %v", lines)
	}
}

func TestExportRun_Sudo(t *testing.T) {
	rule := parser.Rule{
		Action:     "run",
		RunCommand: "systemctl enable foo",
		RunSudo:    true,
	}
	lines := shellExport(t, "run", rule, "bash", "linux")
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "sudo ") {
		t.Errorf("expected sudo prefix, got %v", lines)
	}
}

func TestExportDownload(t *testing.T) {
	rule := parser.Rule{
		Action:        "download",
		DownloadURL:   "https://example.com/tool",
		DownloadPath:  "~/.local/bin/tool",
		DownloadPerms: "0755",
	}
	lines := shellExport(t, "download", rule, "bash", "mac")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "curl") {
		t.Error("expected curl")
	}
	if !strings.Contains(joined, "chmod 0755") {
		t.Error("expected chmod")
	}
	if !strings.Contains(joined, "if [ ! -f") {
		t.Error("expected existence check for non-overwrite mode")
	}
}

func TestExportDownload_Overwrite(t *testing.T) {
	rule := parser.Rule{
		Action:            "download",
		DownloadURL:       "https://example.com/tool",
		DownloadPath:      "~/.local/bin/tool",
		DownloadOverwrite: true,
	}
	lines := shellExport(t, "download", rule, "bash", "mac")
	joined := strings.Join(lines, "\n")
	if strings.Contains(joined, "if [") {
		t.Error("overwrite mode should not have existence check")
	}
}

func TestExportMkdir(t *testing.T) {
	rule := parser.Rule{
		Action:     "mkdir",
		Mkdir:      "~/projects",
		MkdirPerms: "755",
	}
	lines := shellExport(t, "mkdir", rule, "bash", "mac")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "mkdir -p") {
		t.Error("expected mkdir -p")
	}
	if !strings.Contains(lines[1], "chmod 755") {
		t.Error("expected chmod")
	}
}

func TestExportKnownHosts(t *testing.T) {
	rule := parser.Rule{
		Action:     "known_hosts",
		KnownHosts: "github.com",
	}
	lines := shellExport(t, "known_hosts", rule, "bash", "mac")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "ssh-keyscan") {
		t.Error("expected ssh-keyscan")
	}
	if !strings.Contains(joined, "ed25519") {
		t.Error("expected default ed25519 key type")
	}
}

func TestExportHomebrew(t *testing.T) {
	rule := parser.Rule{
		Action:           "homebrew",
		HomebrewPackages: []string{"node", "git"},
		HomebrewCasks:    []string{"docker"},
	}
	lines := shellExport(t, "homebrew", rule, "bash", "mac")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "brew list --versions node") || !strings.Contains(joined, "brew list --cask node") {
		t.Error("expected both formula and cask check for node")
	}
	if !strings.Contains(joined, "brew install node") {
		t.Error("expected brew install node")
	}
	if !strings.Contains(joined, "brew list --cask docker") {
		t.Error("expected brew list --cask check for docker")
	}
	if !strings.Contains(joined, "brew install --cask docker") {
		t.Error("expected brew install --cask docker")
	}
}

func TestExportHomebrew_VersionedFormula(t *testing.T) {
	rule := parser.Rule{
		Action:           "homebrew",
		HomebrewPackages: []string{"node@20"},
	}
	lines := shellExport(t, "homebrew", rule, "bash", "mac")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "brew list --versions node") || !strings.Contains(joined, "brew list --cask node") {
		t.Error("expected both formula and cask check using base name")
	}
	if !strings.Contains(joined, "brew install node@20") {
		t.Error("expected brew install with version")
	}
}

func TestExportOllama(t *testing.T) {
	rule := parser.Rule{
		Action:       "ollama",
		OllamaModels: []string{"llama3", "codellama"},
	}
	lines := shellExport(t, "ollama", rule, "bash", "mac")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, `ollama pull "llama3"`) {
		t.Error("expected ollama pull for llama3")
	}
	if !strings.Contains(joined, `ollama pull "codellama"`) {
		t.Error("expected ollama pull for codellama")
	}
}

func TestExportMise(t *testing.T) {
	rule := parser.Rule{
		Action:       "mise",
		MisePackages: []string{"node@20", "python@3.11"},
	}
	lines := shellExport(t, "mise", rule, "bash", "mac")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "mise use -g \"node@20\"") {
		t.Errorf("expected mise use -g for node, got:\n%s", joined)
	}
	if !strings.Contains(joined, "mise use -g \"python@3.11\"") {
		t.Error("expected mise use -g for python")
	}
}

func TestExportMise_ProjectLocal(t *testing.T) {
	rule := parser.Rule{
		Action:       "mise",
		MisePackages: []string{"node@20"},
		MisePath:     "/tmp/project",
	}
	lines := shellExport(t, "mise", rule, "bash", "mac")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "cd") || !strings.Contains(joined, "mise use") {
		t.Errorf("expected cd and mise use for project-local, got:\n%s", joined)
	}
}

func TestExportAsdf(t *testing.T) {
	rule := parser.Rule{
		Action:       "asdf",
		AsdfPackages: []string{"nodejs@21.4.0"},
	}
	lines := shellExport(t, "asdf", rule, "bash", "mac")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "asdf plugin add") {
		t.Error("expected plugin add")
	}
	if !strings.Contains(joined, "asdf install") {
		t.Error("expected asdf install")
	}
}

func TestExportSudoers(t *testing.T) {
	rule := parser.Rule{
		Action:      "sudoers",
		SudoersUser: "deploy",
	}
	lines := shellExport(t, "sudoers", rule, "bash", "linux")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "deploy ALL=(ALL) NOPASSWD: ALL") {
		t.Error("expected sudoers entry")
	}
	if !strings.Contains(joined, "visudo -c") {
		t.Error("expected visudo check")
	}
}

func TestExportShell(t *testing.T) {
	rule := parser.Rule{
		Action:    "shell",
		ShellName: "zsh",
	}
	lines := shellExport(t, "shell", rule, "bash", "mac")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "chsh") {
		t.Error("expected chsh")
	}
}

func TestExportDotfiles(t *testing.T) {
	rule := parser.Rule{
		Action:      "dotfiles",
		DotfilesURL: "https://github.com/user/dotfiles",
	}
	lines := shellExport(t, "dotfiles", rule, "bash", "mac")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "git clone") {
		t.Error("expected git clone")
	}
	if !strings.Contains(joined, "fetch -q origin") {
		t.Error("expected git fetch instead of git pull")
	}
	if !strings.Contains(joined, "reset --hard") {
		t.Error("expected git reset --hard for dirty repo safety")
	}
	if !strings.Contains(joined, "ln -sf") {
		t.Error("expected symlink creation")
	}
	if !strings.Contains(joined, ".git|.github") {
		t.Error("expected skip patterns")
	}
}

func TestExportDecrypt_ReturnsNil(t *testing.T) {
	def := GetAction("decrypt")
	if def == nil {
		t.Fatal("decrypt action not found")
	}
	if def.ShellExport != nil {
		t.Error("decrypt should have nil ShellExport")
	}
}

func TestExportSchedule(t *testing.T) {
	rule := parser.Rule{
		Action:         "schedule",
		SchedulePreset: "daily",
		ScheduleSource: "setup.bp",
	}
	lines := shellExport(t, "schedule", rule, "bash", "mac")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "crontab") {
		t.Error("expected crontab")
	}
	if !strings.Contains(joined, "0 9 * * *") {
		t.Error("expected daily cron expression")
	}
}

func TestExportAuthorizedKeys(t *testing.T) {
	rule := parser.Rule{
		Action:             "authorized_keys",
		AuthorizedKeysFile: "~/.ssh/id_rsa.pub",
	}
	lines := shellExport(t, "authorized_keys", rule, "bash", "mac")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "authorized_keys") {
		t.Error("expected authorized_keys path")
	}
}

func TestExportAuthorizedKeys_Encrypted_ReturnsNil(t *testing.T) {
	rule := parser.Rule{
		Action:                  "authorized_keys",
		AuthorizedKeysEncrypted: "keys.enc",
	}
	lines := shellExport(t, "authorized_keys", rule, "bash", "mac")
	if lines != nil {
		t.Error("encrypted authorized_keys should return nil")
	}
}

func TestShellHome(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"~/projects", `"$HOME/projects"`},
		{"~", `"$HOME"`},
		{"/usr/local/bin", `"/usr/local/bin"`},
	}
	for _, tt := range tests {
		got := shellHome(tt.input)
		if got != tt.want {
			t.Errorf("shellHome(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
