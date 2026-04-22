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
	var lines []string

	for _, p := range rule.Packages {
		name := p.Name
		if p.PackageManager == "snap" {
			lines = append(lines,
				fmt.Sprintf("if ! snap list %s >/dev/null 2>&1; then", name),
				fmt.Sprintf("  sudo snap install %s", name),
				"fi",
			)
		} else if osName == "mac" {
			lines = append(lines,
				fmt.Sprintf("if ! brew list --versions %s >/dev/null 2>&1 && ! brew list --cask %s >/dev/null 2>&1; then", name, name),
				fmt.Sprintf("  brew install %s", name),
				"fi",
			)
		} else {
			installName := name
			if p.Version != "" && p.Version != "latest" {
				installName += "=" + p.Version
			}
			lines = append(lines,
				fmt.Sprintf("if ! dpkg -s %s >/dev/null 2>&1; then", name),
				fmt.Sprintf("  sudo apt-get install -y %s", installName),
				"fi",
			)
		}
	}

	return lines
}

func exportClone(rule parser.Rule, _, _ string) []string {
	path := shellHome(rule.ClonePath)
	branchFlag := ""
	if rule.Branch != "" {
		branchFlag = " -b " + shellQ(rule.Branch)
	}

	// Blueprint uses a two-stage approach: clone to a persistent cache dir,
	// compare SHAs, and only copy if the target is stale. The cache lives
	// under ~/.blueprint/repos/<hash> matching the engine's storage path.
	// Target directories don't have .git (contents-only copy), so we store
	// the last-copied SHA in a marker file next to the target.
	cacheKey := shellQ(rule.CloneURL)
	if rule.Branch != "" {
		cacheKey = shellQ(rule.CloneURL + "@" + rule.Branch)
	}

	// Determine the remote ref to reset to after fetch
	resetRef := "origin/HEAD"
	if rule.Branch != "" {
		resetRef = "origin/" + rule.Branch
	}

	return []string{
		fmt.Sprintf(`CLONE_CACHE="$HOME/.blueprint/repos/$(echo -n %s | shasum -a 256 | cut -c1-16)"`, cacheKey),
		fmt.Sprintf("if [ -d \"$CLONE_CACHE/.git\" ]; then"),
		fmt.Sprintf("  git -C \"$CLONE_CACHE\" fetch -q origin"),
		fmt.Sprintf("  git -C \"$CLONE_CACHE\" reset --hard %s -q 2>/dev/null || git -C \"$CLONE_CACHE\" reset --hard FETCH_HEAD -q", resetRef),
		fmt.Sprintf("else"),
		fmt.Sprintf("  rm -rf \"$CLONE_CACHE\""),
		fmt.Sprintf("  git clone%s %s \"$CLONE_CACHE\" -q", branchFlag, shellQ(rule.CloneURL)),
		fmt.Sprintf("fi"),
		`CLONE_SHA="$(git -C "$CLONE_CACHE" rev-parse HEAD)"`,
		fmt.Sprintf(`CLONE_SHA_FILE=%s.blueprint-sha`, path),
		`OLD_SHA=""`,
		`[ -f "$CLONE_SHA_FILE" ] && OLD_SHA="$(cat "$CLONE_SHA_FILE")"`,
		`if [ "$CLONE_SHA" != "$OLD_SHA" ]; then`,
		fmt.Sprintf("  mkdir -p %s", path),
		fmt.Sprintf(`  rsync -a --delete --exclude='.git' "$CLONE_CACHE/" %s/`, path),
		`  echo "$CLONE_SHA" > "$CLONE_SHA_FILE"`,
		`fi`,
	}
}

func exportHomebrew(rule parser.Rule, _, _ string) []string {
	var lines []string
	for _, f := range rule.HomebrewPackages {
		// Strip version for the check (brew list uses the base name)
		name := strings.Split(f, "@")[0]
		// Check both formula and cask — some packages (e.g. orbstack) are in
		// Caskroom even when specified as a plain formula.
		lines = append(lines,
			fmt.Sprintf("if ! brew list --versions %s >/dev/null 2>&1 && ! brew list --cask %s >/dev/null 2>&1; then", name, name),
			fmt.Sprintf("  brew install %s", f),
			"fi",
		)
	}
	for _, c := range rule.HomebrewCasks {
		lines = append(lines,
			fmt.Sprintf("if ! brew list --cask %s >/dev/null 2>&1; then", c),
			fmt.Sprintf("  brew install --cask %s", c),
			"fi",
		)
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
	skipCase := ".|..|.git|.github|README*|readme*|LICENSE*|license*"
	for _, s := range rule.DotfilesSkip {
		skipCase += "|" + s
	}

	resetRef := "origin/HEAD"
	if rule.DotfilesBranch != "" {
		resetRef = "origin/" + rule.DotfilesBranch
	}

	return []string{
		fmt.Sprintf("if [ ! -d %s ]; then", path),
		"  " + cloneCmd,
		"else",
		fmt.Sprintf("  git -C %s fetch -q origin", path),
		fmt.Sprintf("  git -C %s reset --hard %s -q 2>/dev/null || git -C %s reset --hard FETCH_HEAD -q", path, resetRef, path),
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
