package handlers

import (
	"strings"
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
