package handlers

import (
	"strings"
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

func TestExportInstall_Mac(t *testing.T) {
	rule := parser.Rule{
		Action:   "install",
		Packages: []parser.Package{{Name: "vim"}, {Name: "git"}},
	}
	lines := exportInstall(rule, "bash", "mac")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0] != "brew install vim git" {
		t.Errorf("got %q", lines[0])
	}
}

func TestExportInstall_Linux(t *testing.T) {
	rule := parser.Rule{
		Action:   "install",
		Packages: []parser.Package{{Name: "vim"}, {Name: "git"}},
	}
	lines := exportInstall(rule, "bash", "linux")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0] != "sudo apt-get install -y vim git" {
		t.Errorf("got %q", lines[0])
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
	lines := exportInstall(rule, "bash", "linux")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "apt-get") {
		t.Errorf("expected apt-get line, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "snap install") {
		t.Errorf("expected snap line, got %q", lines[1])
	}
}

func TestExportClone(t *testing.T) {
	rule := parser.Rule{
		Action:    "clone",
		CloneURL:  "https://github.com/user/repo",
		ClonePath: "~/projects/repo",
		Branch:    "main",
	}
	lines := exportClone(rule, "bash", "mac")
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
	if !strings.Contains(joined, "pull") {
		t.Error("expected pull fallback in else branch")
	}
}

func TestExportRun_WithUnless(t *testing.T) {
	rule := parser.Rule{
		Action:     "run",
		RunCommand: "echo hello",
		RunUnless:  "test -f /tmp/done",
	}
	lines := exportRun(rule, "bash", "mac")
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
	lines := exportRun(rule, "bash", "mac")
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
	lines := exportRun(rule, "bash", "linux")
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
	lines := exportDownload(rule, "bash", "mac")
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
	lines := exportDownload(rule, "bash", "mac")
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
	lines := exportMkdir(rule, "bash", "mac")
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
	lines := exportKnownHosts(rule, "bash", "mac")
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
	lines := exportHomebrew(rule, "bash", "mac")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0] != "brew install node git" {
		t.Errorf("got %q", lines[0])
	}
	if lines[1] != "brew install --cask docker" {
		t.Errorf("got %q", lines[1])
	}
}

func TestExportOllama(t *testing.T) {
	rule := parser.Rule{
		Action:       "ollama",
		OllamaModels: []string{"llama3", "codellama"},
	}
	lines := exportOllama(rule, "bash", "mac")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "ollama.com/install.sh") {
		t.Error("expected ollama install")
	}
	if !strings.Contains(joined, "ollama pull") {
		t.Error("expected ollama pull")
	}
}

func TestExportMise_Mac(t *testing.T) {
	rule := parser.Rule{
		Action:       "mise",
		MisePackages: []string{"node@20"},
	}
	lines := exportMise(rule, "bash", "mac")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "brew install mise") {
		t.Error("expected brew install on mac")
	}
	if !strings.Contains(joined, "mise use -g") {
		t.Error("expected mise use -g")
	}
}

func TestExportMise_Linux(t *testing.T) {
	rule := parser.Rule{
		Action:       "mise",
		MisePackages: []string{"node@20"},
	}
	lines := exportMise(rule, "bash", "linux")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "mise.run") {
		t.Error("expected curl mise.run on linux")
	}
}

func TestExportAsdf(t *testing.T) {
	rule := parser.Rule{
		Action:       "asdf",
		AsdfPackages: []string{"nodejs@21.4.0"},
	}
	lines := exportAsdf(rule, "bash", "mac")
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
	lines := exportSudoers(rule, "bash", "linux")
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
	lines := exportShell(rule, "bash", "mac")
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
	lines := exportDotfiles(rule, "bash", "mac")
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "git clone") {
		t.Error("expected git clone")
	}
	if !strings.Contains(joined, "ln -sf") {
		t.Error("expected symlink creation")
	}
	if !strings.Contains(joined, ".git|.github") {
		t.Error("expected skip patterns")
	}
}

func TestExportDecrypt_ReturnsNil(t *testing.T) {
	RegisterExportFuncs()
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
	lines := exportSchedule(rule, "bash", "mac")
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
	lines := exportAuthorizedKeys(rule, "bash", "mac")
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
	lines := exportAuthorizedKeys(rule, "bash", "mac")
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
