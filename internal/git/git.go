package git

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
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

	// SSH URLs: git@host:user/repo[@branch][:path]
	if strings.HasPrefix(input, "git@") {
		return true
	}

	// Strip trailing @branch specifier for HTTPS/git:// URLs.
	beforeBranch := strings.Split(input, "@")[0]

	// HTTP(S) and git:// protocol URLs are always remote git URLs.
	if strings.HasPrefix(beforeBranch, "https://") ||
		strings.HasPrefix(beforeBranch, "http://") ||
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

// CloneRepository clones a git repository to a temporary directory
// and returns the path to the cloned repository
// Accepts URL with optional branch: url@branch or path: url:path or both: url@branch:path
// verbose: if true, shows clone progress; if false, hides progress output
func CloneRepository(input string, verbose bool) (string, string, error) {
	// Parse URL to extract branch and path
	params := ParseGitURL(input)

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "blueprint-*")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	if verbose {
		fmt.Printf("Cloning repository: %s\n", params.URL)
		if params.Branch != "" {
			fmt.Printf("Branch: %s\n", params.Branch)
		}
		if params.Path != "setup.bp" {
			fmt.Printf("Setup file: %s\n", params.Path)
		}
		fmt.Printf("To: %s\n", tmpDir)
	}

	// For SSH URLs use the system git binary so that ~/.ssh/config, the SSH
	// agent, and known_hosts work exactly as they do when running git manually.
	// go-git's SSH transport cannot talk to the SSH agent reliably.
	if strings.HasPrefix(params.URL, "git@") {
		args := []string{"clone"}
		if !verbose {
			args = append(args, "--quiet")
		}
		if params.Branch != "" {
			args = append(args, "--branch", params.Branch)
		}
		args = append(args, params.URL, tmpDir)
		cloneCmd := exec.Command("git", args...) // #nosec G204
		cloneCmd.Stdout = os.Stdout
		cloneCmd.Stderr = os.Stderr
		if err = cloneCmd.Run(); err != nil {
			_ = os.RemoveAll(tmpDir)
			return "", "", fmt.Errorf("failed to clone repository: %w", err)
		}
	} else {
		err = tryClone(tmpDir, params.URL, params.Branch, verbose)
		if err != nil {
			_ = os.RemoveAll(tmpDir)
			return "", "", fmt.Errorf("failed to clone repository: %w", err)
		}
	}

	if verbose {
		fmt.Printf("Repository cloned successfully\n\n")
	}
	return tmpDir, params.Path, nil
}

// tryClone attempts to clone a repository with the given URL and optional branch
func tryClone(tmpDir, url, branch string, verbose bool) error {
	// Prepare clone options
	var progress io.Writer
	if verbose {
		progress = os.Stdout
	} else {
		progress = io.Discard
	}

	cloneOpts := &git.CloneOptions{
		URL:      url,
		Progress: progress,
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
		if auth, authErr := sshAuth(); authErr == nil {
			cloneOpts.Auth = auth
		}
	}

	// Clone the repository
	_, err := git.PlainClone(tmpDir, false, cloneOpts)
	return err
}

// hostKeyCallback returns a HostKeyCallback backed by ~/.ssh/known_hosts.
// If the file doesn't exist or can't be parsed, returns InsecureIgnoreHostKey with a warning.
func hostKeyCallback() gossh.HostKeyCallback {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return gossh.InsecureIgnoreHostKey() // #nosec G106
	}
	knownHostsPath := filepath.Join(homeDir, ".ssh", "known_hosts")
	if _, statErr := os.Stat(knownHostsPath); statErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: ~/.ssh/known_hosts not found, skipping host key verification\n")
		return gossh.InsecureIgnoreHostKey() // #nosec G106
	}
	cb, parseErr := knownhosts.New(knownHostsPath)
	if parseErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not parse ~/.ssh/known_hosts (%v), skipping host key verification\n", parseErr)
		return gossh.InsecureIgnoreHostKey() // #nosec G106
	}
	return cb
}

// sshAuth returns SSH auth, trying the SSH agent first then falling back to key files.
// Candidate key files are tried in order: id_ed25519, id_ecdsa, id_rsa.
// All auth methods use the user's ~/.ssh/known_hosts for host key verification.
func sshAuth() (ssh.AuthMethod, error) {
	hkc := hostKeyCallback()

	// Try SSH agent first
	agentAuth, err := ssh.NewSSHAgentAuth("git")
	if err == nil {
		agentAuth.HostKeyCallback = hkc
		return agentAuth, nil
	}

	// Fall back to key files
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("SSH agent unavailable and could not determine home directory: %w", err)
	}

	candidates := []string{
		filepath.Join(homeDir, ".ssh", "id_ed25519"),
		filepath.Join(homeDir, ".ssh", "id_ecdsa"),
		filepath.Join(homeDir, ".ssh", "id_rsa"),
	}

	for _, keyPath := range candidates {
		if _, statErr := os.Stat(keyPath); statErr != nil {
			continue
		}
		publicKeys, keyErr := ssh.NewPublicKeysFromFile("git", keyPath, "")
		if keyErr == nil {
			publicKeys.HostKeyCallback = hkc
			return publicKeys, nil
		}
	}

	return nil, fmt.Errorf("no usable SSH authentication: SSH agent unavailable and no key files found in ~/.ssh")
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

// gitSHA reads the HEAD SHA of a repository at the given path using the system git binary.
func gitSHA(path string) string {
	out, err := exec.Command("git", "-C", path, "rev-parse", "HEAD").Output() // #nosec G204
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// CloneOrUpdateRepository clones a repository to a specific path or updates it if already exists.
// Uses the system git binary so that ~/.ssh/config, SSH agent, and host key verification work
// exactly as they do when running git manually.
// Returns: (oldSHA, newSHA, status_message, error)
// status_message can be: "Cloned", "Updated", "Already up to date"
func CloneOrUpdateRepository(url, path, branch string) (string, string, string, error) {
	// Expand home directory
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", "", "", fmt.Errorf("failed to get home directory: %w", err)
		}
		path = filepath.Join(homeDir, path[2:])
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return "", "", "", fmt.Errorf("failed to create parent directory: %w", err)
	}

	info, err := os.Stat(path)
	pathExists := err == nil && info.IsDir()

	if pathExists {
		oldSHA := gitSHA(path)

		// Fetch and reset to origin HEAD
		var stderr bytes.Buffer
		fetchCmd := exec.Command("git", "-C", path, "fetch", "--quiet", "origin") // #nosec G204
		fetchCmd.Stderr = &stderr
		if err := fetchCmd.Run(); err != nil {
			return oldSHA, "", "", fmt.Errorf("failed to fetch: %w\n%s", err, stderr.String())
		}

		// Determine the remote tracking branch to reset to
		resetTarget := "FETCH_HEAD"
		if branch != "" {
			resetTarget = "origin/" + branch
		}
		stderr.Reset()
		resetCmd := exec.Command("git", "-C", path, "reset", "--hard", resetTarget) // #nosec G204
		resetCmd.Stderr = &stderr
		if err := resetCmd.Run(); err != nil {
			return oldSHA, "", "", fmt.Errorf("failed to reset to %s: %w\n%s", resetTarget, err, stderr.String())
		}

		newSHA := gitSHA(path)
		if oldSHA != newSHA {
			return oldSHA, newSHA, "Updated", nil
		}
		return oldSHA, newSHA, "Already up to date", nil
	}

	// Clone
	args := []string{"clone", "--quiet"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, url, path)

	var stderr bytes.Buffer
	cloneCmd := exec.Command("git", args...) // #nosec G204
	cloneCmd.Stderr = &stderr
	if err := cloneCmd.Run(); err != nil {
		return "", "", "", fmt.Errorf("failed to clone: %w\n%s", err, stderr.String())
	}

	newSHA := gitSHA(path)
	return "", newSHA, "Cloned", nil
}
