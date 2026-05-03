package renderer

import "strings"

// splitToolVersion splits "name@version" into ("name", "version", true).
// Returns ("", "", false) if no "@" separator is present.
func splitToolVersion(s string) (string, string, bool) {
	idx := strings.Index(s, "@")
	if idx < 0 {
		return "", "", false
	}
	return s[:idx], s[idx+1:], true
}
