package handlers

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/elpic/blueprint/internal/parser"
)

// shellHome returns a $HOME-based path for tilde paths, or the original path.
func shellHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		return `"$HOME/` + path[2:] + `"`
	}
	if path == "~" {
		return `"$HOME"`
	}
	return `"` + path + `"`
}

// shellQ quotes a string for shell safety.
func shellQ(s string) string {
	if s == "" {
		return `""`
	}
	// If it contains single quotes, use double quotes with escaping
	if strings.ContainsAny(s, `"$`+"`\\") {
		return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
	}
	return `"` + s + `"`
}

var exportRegistered bool

// RegisterExportFuncs registers shell export functions for all action types.
// Must be called after all action init() functions have run (i.e., not from init()).
func RegisterExportFuncs() {
	if exportRegistered {
		return
	}
	exportRegistered = true
	exports := map[string]ShellExportFunc{
		"install":         exportInstall,
		"clone":           exportClone,
		"homebrew":        exportHomebrew,
		"run":             exportRun,
		"run-sh":          exportRunSh,
		"download":        exportDownload,
		"mkdir":           exportMkdir,
		"known_hosts":     exportKnownHosts,
		"gpg_key":         exportGPGKey,
		"ollama":          exportOllama,
		"mise":            exportMise,
		"asdf":            exportAsdf,
		"sudoers":         exportSudoers,
		"shell":           exportShell,
		"schedule":        exportSchedule,
		"authorized_keys": exportAuthorizedKeys,
		"dotfiles":        exportDotfiles,
		"decrypt":         nil, // handled as skip in engine
	}

	for name, fn := range exports {
		def := GetAction(name)
		if def != nil {
			def.ShellExport = fn
		}
	}
}

func exportInstall(rule parser.Rule, format, osName string) []string {
	// Separate by package manager and build name lists
	var brewPkgs, aptPkgs, snapPkgs []string
	for _, p := range rule.Packages {
		name := p.Name
		if p.PackageManager == "snap" {
			snapPkgs = append(snapPkgs, name)
		} else {
			if p.Version != "" && p.Version != "latest" {
				name += "=" + p.Version
			}
			brewPkgs = append(brewPkgs, name)
			aptPkgs = append(aptPkgs, name)
		}
	}

	if osName == "mac" {
		return []string{"brew install " + strings.Join(brewPkgs, " ")}
	}

	var lines []string
	if len(aptPkgs) > 0 {
		lines = append(lines, "sudo apt-get install -y "+strings.Join(aptPkgs, " "))
	}
	if len(snapPkgs) > 0 {
		lines = append(lines, "sudo snap install "+strings.Join(snapPkgs, " "))
	}
	return lines
}

func exportClone(rule parser.Rule, _, _ string) []string {
	path := shellHome(rule.ClonePath)
	cloneCmd := "git clone"
	if rule.Branch != "" {
		cloneCmd += " -b " + shellQ(rule.Branch)
	}
	cloneCmd += " " + shellQ(rule.CloneURL) + " " + path

	return []string{
		fmt.Sprintf("if [ ! -d %s ]; then", path),
		"  " + cloneCmd,
		"else",
		fmt.Sprintf("  git -C %s pull", path),
		"fi",
	}
}

func exportHomebrew(rule parser.Rule, _, _ string) []string {
	var lines []string
	if len(rule.HomebrewPackages) > 0 {
		lines = append(lines, "brew install "+strings.Join(rule.HomebrewPackages, " "))
	}
	if len(rule.HomebrewCasks) > 0 {
		lines = append(lines, "brew install --cask "+strings.Join(rule.HomebrewCasks, " "))
	}
	return lines
}

func exportRun(rule parser.Rule, _, _ string) []string {
	cmd := rule.RunCommand
	if rule.RunSudo {
		cmd = "sudo " + cmd
	}

	if rule.RunUnless != "" {
		return []string{
			fmt.Sprintf("if ! (%s) >/dev/null 2>&1; then", rule.RunUnless),
			"  " + cmd,
			"fi",
		}
	}
	return []string{cmd}
}

func exportRunSh(rule parser.Rule, _, _ string) []string {
	cmd := fmt.Sprintf("curl -fsSL %s | sh", shellQ(rule.RunShURL))
	if rule.RunSudo {
		cmd = fmt.Sprintf("curl -fsSL %s | sudo sh", shellQ(rule.RunShURL))
	}

	if rule.RunUnless != "" {
		return []string{
			fmt.Sprintf("if ! (%s) >/dev/null 2>&1; then", rule.RunUnless),
			"  " + cmd,
			"fi",
		}
	}
	return []string{cmd}
}

func exportDownload(rule parser.Rule, _, _ string) []string {
	path := shellHome(rule.DownloadPath)
	dir := shellHome(filepath.Dir(strings.Replace(rule.DownloadPath, "~/", "", 1)))
	if strings.HasPrefix(rule.DownloadPath, "~/") {
		dir = `"$HOME/` + filepath.Dir(rule.DownloadPath[2:]) + `"`
	}

	var lines []string

	if !rule.DownloadOverwrite {
		lines = append(lines, fmt.Sprintf("if [ ! -f %s ]; then", path))
		lines = append(lines, fmt.Sprintf("  mkdir -p %s", dir))
		lines = append(lines, fmt.Sprintf("  curl -fsSL -o %s %s", path, shellQ(rule.DownloadURL)))
		if rule.DownloadPerms != "" {
			lines = append(lines, fmt.Sprintf("  chmod %s %s", rule.DownloadPerms, path))
		}
		lines = append(lines, "fi")
	} else {
		lines = append(lines, fmt.Sprintf("mkdir -p %s", dir))
		lines = append(lines, fmt.Sprintf("curl -fsSL -o %s %s", path, shellQ(rule.DownloadURL)))
		if rule.DownloadPerms != "" {
			lines = append(lines, fmt.Sprintf("chmod %s %s", rule.DownloadPerms, path))
		}
	}
	return lines
}

func exportMkdir(rule parser.Rule, _, _ string) []string {
	path := shellHome(rule.Mkdir)
	lines := []string{fmt.Sprintf("mkdir -p %s", path)}
	if rule.MkdirPerms != "" {
		lines = append(lines, fmt.Sprintf("chmod %s %s", rule.MkdirPerms, path))
	}
	return lines
}

func exportKnownHosts(rule parser.Rule, _, _ string) []string {
	keyType := rule.KnownHostsKey
	if keyType == "" {
		keyType = "ed25519"
	}
	return []string{
		`mkdir -p "$HOME/.ssh" && chmod 700 "$HOME/.ssh"`,
		fmt.Sprintf(`ssh-keyscan -t %s %s >> "$HOME/.ssh/known_hosts" 2>/dev/null`, keyType, rule.KnownHosts),
	}
}

func exportGPGKey(rule parser.Rule, _, _ string) []string {
	keyring := rule.GPGKeyring
	return []string{
		"sudo install -m 0755 -d /etc/apt/keyrings",
		fmt.Sprintf("curl -fsSL %s | sudo tee /etc/apt/keyrings/%s.asc > /dev/null", shellQ(rule.GPGKeyURL), keyring),
		fmt.Sprintf("sudo chmod go+r /etc/apt/keyrings/%s.asc", keyring),
		fmt.Sprintf(`echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/%s.asc] %s" | sudo tee /etc/apt/sources.list.d/%s.list > /dev/null`, keyring, rule.GPGDebURL, keyring),
		"sudo apt-get update",
	}
}

func exportOllama(rule parser.Rule, _, _ string) []string {
	var lines []string
	for _, model := range rule.OllamaModels {
		lines = append(lines, "ollama pull "+shellQ(model))
	}
	return lines
}

func exportMise(rule parser.Rule, _, _ string) []string {
	var lines []string
	for _, pkg := range rule.MisePackages {
		if rule.MisePath != "" {
			lines = append(lines, fmt.Sprintf("cd %s && mise use %s", shellQ(rule.MisePath), shellQ(pkg)))
		} else {
			lines = append(lines, "mise use -g "+shellQ(pkg))
		}
	}
	return lines
}

func exportAsdf(rule parser.Rule, _, _ string) []string {
	var lines []string
	for _, pkg := range rule.AsdfPackages {
		parts := strings.SplitN(pkg, "@", 2)
		plugin := parts[0]
		version := "latest"
		if len(parts) > 1 {
			version = parts[1]
		}
		lines = append(lines,
			fmt.Sprintf("asdf plugin add %s 2>/dev/null || true", shellQ(plugin)),
			fmt.Sprintf("asdf install %s %s", shellQ(plugin), shellQ(version)),
		)
	}
	return lines
}

func exportSudoers(rule parser.Rule, _, _ string) []string {
	user := rule.SudoersUser
	if user == "" {
		user = "$USER"
	}
	return []string{
		fmt.Sprintf(`echo "%s ALL=(ALL) NOPASSWD: ALL" | sudo tee /etc/sudoers.d/%s > /dev/null`, user, user),
		fmt.Sprintf(`sudo chmod 0440 /etc/sudoers.d/%s`, user),
		`sudo visudo -c -q`,
	}
}

func exportShell(rule parser.Rule, _, _ string) []string {
	shell := rule.ShellName
	// Common shell name to path resolution
	if !strings.HasPrefix(shell, "/") {
		return []string{
			fmt.Sprintf(`SHELL_PATH="$(command -v %s)"`, shellQ(shell)),
			`chsh -s "$SHELL_PATH"`,
		}
	}
	return []string{"chsh -s " + shellQ(shell)}
}

func exportSchedule(rule parser.Rule, _, _ string) []string {
	cron := rule.ScheduleCron
	if cron == "" {
		switch rule.SchedulePreset {
		case "daily":
			cron = "0 9 * * *"
		case "weekly":
			cron = "0 9 * * 1"
		case "hourly":
			cron = "0 * * * *"
		}
	}

	source := rule.ScheduleSource
	cronLine := fmt.Sprintf(`%s blueprint apply %s --skip-decrypt >> ~/.blueprint/schedule.log 2>&1`, cron, shellQ(source))

	return []string{
		fmt.Sprintf(`(crontab -l 2>/dev/null | grep -v %s; echo %s) | crontab -`, shellQ(source), shellQ(cronLine)),
	}
}

func exportAuthorizedKeys(rule parser.Rule, _, _ string) []string {
	lines := []string{
		`mkdir -p "$HOME/.ssh" && chmod 700 "$HOME/.ssh"`,
		`touch "$HOME/.ssh/authorized_keys" && chmod 600 "$HOME/.ssh/authorized_keys"`,
	}

	if rule.AuthorizedKeysEncrypted != "" {
		// Encrypted keys can't be exported — need blueprint decrypt
		return nil
	}

	if rule.AuthorizedKeysFile != "" {
		src := shellHome(rule.AuthorizedKeysFile)
		lines = append(lines, fmt.Sprintf(`cat %s >> "$HOME/.ssh/authorized_keys"`, src))
	}
	return lines
}

func exportDotfiles(rule parser.Rule, _, _ string) []string {
	clonePath := rule.DotfilesPath
	if clonePath == "" {
		// Derive from URL like the handler does
		parts := strings.Split(strings.TrimSuffix(rule.DotfilesURL, ".git"), "/")
		repoName := parts[len(parts)-1]
		clonePath = "~/.blueprint/dotfiles/" + repoName
	}
	path := shellHome(clonePath)

	cloneCmd := "git clone"
	if rule.DotfilesBranch != "" {
		cloneCmd += " -b " + shellQ(rule.DotfilesBranch)
	}
	cloneCmd += " " + shellQ(rule.DotfilesURL) + " " + path

	// Build skip list
	skipCase := ".git|.github|README*|readme*|LICENSE*|license*"
	for _, s := range rule.DotfilesSkip {
		skipCase += "|" + s
	}

	return []string{
		fmt.Sprintf("if [ ! -d %s ]; then", path),
		"  " + cloneCmd,
		"else",
		fmt.Sprintf("  git -C %s pull", path),
		"fi",
		"# Symlink dotfiles to home directory",
		fmt.Sprintf(`for f in %s/.* %s/*; do`, path, path),
		`  name="$(basename "$f")"`,
		`  case "$name" in`,
		`    ` + skipCase + `) continue ;;`,
		`  esac`,
		`  if [ -f "$f" ] || [ -L "$f" ]; then`,
		`    ln -sf "$f" "$HOME/$name"`,
		`  elif [ -d "$f" ]; then`,
		`    mkdir -p "$HOME/$name"`,
		`    for child in "$f"/*; do`,
		`      [ -e "$child" ] || continue`,
		`      ln -sf "$child" "$HOME/$name/$(basename "$child")"`,
		`    done`,
		`  fi`,
		`done`,
	}
}
