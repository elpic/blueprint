package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

// GitURLParams holds parsed git URL information
type GitURLParams struct {
	URL    string
	Branch string
	Path   string
}

// IsGitURL checks if the given string is a git URL
func IsGitURL(input string) bool {
	// Remove branch/path specifiers to check base URL
	// Format: url[@branch][:path]

	// Split by @ first (branch specifier)
	beforeBranch := strings.Split(input, "@")[0]

	// For SSH URLs (git@...), check for : that's not part of protocol
	if strings.HasPrefix(beforeBranch, "git@") {
		return true
	}

	// For HTTP(S) URLs, we need to be more careful with colons
	if strings.HasPrefix(beforeBranch, "https://") || strings.HasPrefix(beforeBranch, "http://") {
		return true
	}

	// Check if ends with .git (even with branch/path)
	if strings.Contains(input, ".git") {
		return true
	}

	return false
}

// ParseGitURL parses a git URL with optional branch and path
// Format: repo.git[@branch][:path/to/file.bp]
// Examples:
//   https://github.com/user/repo.git
//   https://github.com/user/repo.git@main
//   https://github.com/user/repo.git:config/setup.bp
//   https://github.com/user/repo.git@dev:config/setup.bp
func ParseGitURL(input string) GitURLParams {
	params := GitURLParams{
		Path: "setup.bp", // Default path
	}

	// Split by @ to get branch
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
			params.Path = afterGit[1:] // Remove the leading :
			baseURL = baseURL[:gitIdx+4] // Keep everything up to and including .git
		}
	}

	params.URL = baseURL
	return params
}

// CloneRepository clones a git repository to a temporary directory
// and returns the path to the cloned repository
// Accepts URL with optional branch: url@branch or path: url:path or both: url@branch:path
func CloneRepository(input string) (string, string, error) {
	// Parse URL to extract branch and path
	params := ParseGitURL(input)

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "blueprint-*")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	fmt.Printf("Cloning repository: %s\n", params.URL)
	if params.Branch != "" {
		fmt.Printf("Branch: %s\n", params.Branch)
	}
	if params.Path != "setup.bp" {
		fmt.Printf("Setup file: %s\n", params.Path)
	}
	fmt.Printf("To: %s\n", tmpDir)

	// Try to clone with the original URL
	err = tryClone(tmpDir, params.URL, params.Branch)

	// If SSH fails on a public repo, try converting to HTTPS
	if err != nil && strings.HasPrefix(params.URL, "git@") {
		fmt.Printf("SSH failed, attempting HTTPS fallback...\n")
		httpsURL := convertSSHToHTTPS(params.URL)
		fmt.Printf("Trying: %s\n", httpsURL)
		err = tryClone(tmpDir, httpsURL, params.Branch)
	}

	if err != nil {
		// Clean up the temporary directory on error
		os.RemoveAll(tmpDir)
		return "", "", fmt.Errorf("failed to clone repository: %w", err)
	}

	fmt.Printf("Repository cloned successfully\n\n")
	return tmpDir, params.Path, nil
}

// tryClone attempts to clone a repository with the given URL and optional branch
func tryClone(tmpDir, url, branch string) error {
	// Prepare clone options
	cloneOpts := &git.CloneOptions{
		URL:      url,
		Progress: os.Stdout,
	}

	// Set branch if specified
	if branch != "" {
		cloneOpts.ReferenceName = plumbing.ReferenceName("refs/heads/" + branch)
		cloneOpts.SingleBranch = true
	}

	// Add authentication if credentials are provided
	if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		// Check for HTTPS credentials
		if username := os.Getenv("GITHUB_USER"); username != "" {
			if token := os.Getenv("GITHUB_TOKEN"); token != "" {
				cloneOpts.Auth = &http.BasicAuth{
					Username: username,
					Password: token,
				}
			}
		}
	} else if strings.HasPrefix(url, "git@") {
		// SSH authentication via SSH agent
		publicKeys, err := ssh.NewSSHAgentAuth("git")
		if err == nil {
			cloneOpts.Auth = publicKeys
		}
	}

	// Clone the repository
	_, err := git.PlainClone(tmpDir, false, cloneOpts)
	return err
}

// convertSSHToHTTPS converts an SSH git URL to HTTPS
// Example: git@github.com:user/repo.git -> https://github.com/user/repo.git
func convertSSHToHTTPS(sshURL string) string {
	// Remove git@ prefix
	sshURL = strings.TrimPrefix(sshURL, "git@")

	// Replace : with /
	httpsURL := strings.Replace(sshURL, ":", "/", 1)

	// Add https:// prefix
	return "https://" + httpsURL
}

// FindSetupFile looks for a setup file in the given directory
// If path is not provided, defaults to "setup.bp"
func FindSetupFile(dir, path string) (string, error) {
	if path == "" {
		path = "setup.bp"
	}

	setupPath := filepath.Join(dir, path)
	if _, err := os.Stat(setupPath); err == nil {
		return setupPath, nil
	}

	return "", fmt.Errorf("setup file not found: %s in %s", path, dir)
}

// CleanupRepository removes the temporary repository directory
func CleanupRepository(path string) error {
	if path == "" {
		return nil
	}
	return os.RemoveAll(path)
}
