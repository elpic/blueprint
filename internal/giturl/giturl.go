// Package giturl holds the pure string transforms that blueprint uses to
// interpret git URL syntax — shorthand expansion (@github:user/repo →
// https://github.com/user/repo), URL detection, branch/path parsing, and
// normalization. These functions are v6-invariant: they have no dependency on
// the git engine (go-git or system git) and may be imported by any package.
//
// Splitting them out of internal/git is what makes the engine boundary
// enforceable: after the move, internal/git is imported only by
// internal/platform (the GitProvider seam) — asserted by the import-guard test
// in internal/git/gitcmd/gitcmd_test.go (#033 phase 5).
package giturl

import "strings"

// GitURLParams holds parsed git URL information
type GitURLParams struct {
	URL    string
	Branch string
	Path   string
}

// shorthandHosts maps @provider: prefixes to their base HTTPS URLs.
var shorthandHosts = map[string]string{
	"github":    "https://github.com/",
	"gitlab":    "https://gitlab.com/",
	"bitbucket": "https://bitbucket.org/",
	"codeberg":  "https://codeberg.org/",
}

// shorthandSSHHosts maps @provider: prefixes to their SSH host (git@<host>:).
var shorthandSSHHosts = map[string]string{
	"github":    "git@github.com:",
	"gitlab":    "git@gitlab.com:",
	"bitbucket": "git@bitbucket.org:",
	"codeberg":  "git@codeberg.org:",
}

// ExpandShorthand expands @provider:user/repo[@branch][:path] to a full HTTPS URL.
// Returns the input unchanged if it is not a shorthand form.
//
// Examples:
//
//	@github:user/repo              → https://github.com/user/repo
//	@gitlab:user/repo@main         → https://gitlab.com/user/repo@main
//	@bitbucket:user/repo@v1:ci.bp  → https://bitbucket.org/user/repo@v1:ci.bp
func ExpandShorthand(input string) string {
	return expandShorthand(input, false)
}

// ExpandShorthandSSH expands @provider:user/repo[@branch][:path] to an SSH URL.
// Returns the input unchanged if it is not a shorthand form.
//
// Examples:
//
//	@github:user/repo      → git@github.com:user/repo
//	@gitlab:user/repo@main → git@gitlab.com:user/repo@main
func ExpandShorthandSSH(input string) string {
	return expandShorthand(input, true)
}

func expandShorthand(input string, preferSSH bool) string {
	if !strings.HasPrefix(input, "@") {
		return input
	}
	// Format: @provider:user/repo[@branch][:path]
	rest := input[1:] // strip leading "@"
	colonIdx := strings.Index(rest, ":")
	if colonIdx < 0 {
		return input // no colon → not a valid shorthand
	}
	provider := rest[:colonIdx]
	// rest[colonIdx+1:] is "user/repo[@branch][:path]"
	suffix := rest[colonIdx+1:]
	if preferSSH {
		base, ok := shorthandSSHHosts[provider]
		if !ok {
			return input // unknown provider → pass through unchanged
		}
		return base + suffix
	}
	base, ok := shorthandHosts[provider]
	if !ok {
		return input // unknown provider → pass through unchanged
	}
	return base + suffix
}

// IsGitURL checks if the given string is a git URL
func IsGitURL(input string) bool {
	input = ExpandShorthand(input)
	// Remove branch/path specifiers to check base URL
	// Format: url[@branch][:path]

	// SSH URLs: git@host:user/repo[@branch][:path]
	if strings.HasPrefix(input, "git@") {
		return true
	}

	// Strip trailing @branch specifier for HTTPS/git:// URLs.
	beforeBranch := strings.Split(input, "@")[0]

	// HTTP(S) and git:// protocol URLs are always remote git URLs.
	// Also accept single-slash variants (https:/host/...) that were produced by
	// a bug in an older version, so they can be recognized and normalized.
	if strings.HasPrefix(beforeBranch, "https://") ||
		strings.HasPrefix(beforeBranch, "https:/") ||
		strings.HasPrefix(beforeBranch, "http://") ||
		strings.HasPrefix(beforeBranch, "http:/") ||
		strings.HasPrefix(beforeBranch, "git://") {
		return true
	}

	return false
}

// ParseGitURL parses a git URL with optional branch and path.
//
// Supported formats:
//
//	https://github.com/user/repo[@branch][:path/to/file.bp]
//	https://github.com/user/repo.git[@branch][:path/to/file.bp]
//	git@github.com:user/repo.git[@branch[:path/to/file.bp]]
//	git://github.com/user/repo.git[@branch][:path/to/file.bp]
func ParseGitURL(input string) GitURLParams {
	input = ExpandShorthand(input)
	params := GitURLParams{
		Path: "setup.bp", // Default path
	}

	// SSH URLs: git@host:org/repo.git[@branch[:path]]
	// We must NOT split on the first "@" because that separates "git" from the host.
	// The branch/path specifier uses a second "@" that appears only after ".git".
	if strings.HasPrefix(input, "git@") {
		baseURL := input
		// Look for a second "@" that signals a branch specifier (after the repo part)
		// e.g. git@github.com:org/repo.git@main:path/to/file.bp
		if gitIdx := strings.Index(input, ".git"); gitIdx >= 0 {
			afterGit := input[gitIdx+4:] // everything after ".git"
			baseURL = input[:gitIdx+4]   // git@host:org/repo.git
			if strings.HasPrefix(afterGit, "@") {
				// branch (and optionally path) follow
				branchAndPath := afterGit[1:]
				if colonIdx := strings.Index(branchAndPath, ":"); colonIdx >= 0 {
					params.Branch = branchAndPath[:colonIdx]
					params.Path = branchAndPath[colonIdx+1:]
				} else {
					params.Branch = branchAndPath
				}
			} else if strings.HasPrefix(afterGit, ":") {
				// path only, no branch
				params.Path = afterGit[1:]
			}
		}
		params.URL = baseURL
		return params
	}

	// HTTPS / HTTP / git:// URLs: split on first "@" to extract branch specifier.
	parts := strings.Split(input, "@")
	baseURL := parts[0]

	if len(parts) > 1 {
		// Extract branch and possibly path after @
		branchAndPath := parts[1]
		if colonIdx := strings.Index(branchAndPath, ":"); colonIdx >= 0 {
			params.Branch = branchAndPath[:colonIdx]
			params.Path = branchAndPath[colonIdx+1:]
		} else {
			params.Branch = branchAndPath
		}
	}

	// Look for path after .git: (only split on colon after .git)
	if gitIdx := strings.Index(baseURL, ".git"); gitIdx >= 0 {
		afterGit := baseURL[gitIdx+4:] // afterGit starts after ".git"
		if strings.HasPrefix(afterGit, ":") {
			params.Path = afterGit[1:]   // Remove the leading :
			baseURL = baseURL[:gitIdx+4] // Keep everything up to and including .git
		}
	}

	params.URL = baseURL
	return params
}

// StripBranch removes the @branch and :path specifiers from a git URL,
// returning just the base repository URL. This is useful for comparing
// whether two blueprint URLs point to the same repo regardless of branch.
func StripBranch(input string) string {
	// SSH URLs: git@host:user/repo.git@branch:path
	if strings.HasPrefix(input, "git@") {
		if gitIdx := strings.Index(input, ".git"); gitIdx >= 0 {
			return input[:gitIdx+4] // keep up to and including .git
		}
		// No .git suffix — strip trailing @branch if present (second @ only)
		return input
	}

	// HTTPS/HTTP/git:// URLs: url@branch:path — strip @branch and beyond
	if idx := strings.Index(input, "@"); idx > 0 {
		return input[:idx]
	}

	return input
}

// NormalizeGitURL normalizes a git URL for consistent identification.
// It converts SSH URLs to HTTPS and lowercases the result.
func NormalizeGitURL(url string) string {
	// Remove .git suffix if present
	url = strings.TrimSuffix(url, ".git")

	// Repair single-slash http(s):/ → canonical double-slash form before further processing.
	// These were produced by a bug in an older version of the code.
	if strings.HasPrefix(url, "https:/") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url[len("https:/"):]
	} else if strings.HasPrefix(url, "http:/") && !strings.HasPrefix(url, "http://") {
		url = "http://" + url[len("http:/"):]
	}

	// Convert SSH to HTTPS for normalization (for ID generation only)
	if strings.HasPrefix(url, "git@") {
		// git@github.com:user/repo -> https://github.com/user/repo
		parts := strings.Split(url, ":")
		if len(parts) >= 2 {
			host := strings.TrimPrefix(parts[0], "git@")
			path := strings.Join(parts[1:], ":")
			url = "https://" + host + "/" + path
		}
	}

	return strings.ToLower(url)
}
