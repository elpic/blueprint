package git

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
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

	// Try go-git first; fall back to system git if go-git fails (e.g. SSH agent/passphrase issues).
	if err = tryClone(tmpDir, params.URL, params.Branch, verbose); err != nil {
		_ = os.RemoveAll(tmpDir) // clean up any partial clone before retrying
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

// LocalSHA reads the HEAD SHA of a local repository at the given path using go-git.
// Returns empty string if the path is not a valid git repository.
func LocalSHA(path string) string {
	return gitSHA(path)
}

// RemoteHeadSHA returns the SHA of the remote HEAD (or branch tip) for the given URL and branch.
// Returns empty string if the check fails (network unavailable, auth issue, etc.).
func RemoteHeadSHA(url, branch string) string {
	ref := "HEAD"
	if branch != "" {
		ref = "refs/heads/" + branch
	}
	return remoteRef(url, ref)
}

// gitSHA reads the HEAD SHA of a repository at the given path using go-git.
func gitSHA(path string) string {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return ""
	}
	ref, err := repo.Head()
	if err != nil {
		return ""
	}
	return ref.Hash().String()
}

// repoAuth returns the appropriate go-git auth method for the given URL.
func repoAuth(url string) (transport.AuthMethod, error) {
	if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		if username := os.Getenv("GITHUB_USER"); username != "" {
			if token := os.Getenv("GITHUB_TOKEN"); token != "" {
				return &http.BasicAuth{Username: username, Password: token}, nil
			}
		}
		return nil, fmt.Errorf("no HTTPS credentials available")
	}
	if strings.HasPrefix(url, "git@") {
		return sshAuth()
	}
	return nil, fmt.Errorf("unsupported URL scheme")
}

// remoteRef returns the SHA for the given ref on the remote using go-git (no git binary required).
// Returns empty string if the check fails (network unavailable, auth issue, etc.) —
// callers should fall back to a full fetch in that case.
func remoteRef(url, ref string) string {
	remote := git.NewRemote(nil, &config.RemoteConfig{
		Name: "origin",
		URLs: []string{url},
	})
	refs, err := remote.List(&git.ListOptions{})
	if err != nil {
		return ""
	}
	// Build a map of ref name → SHA for easy lookup
	refMap := make(map[string]string, len(refs))
	for _, r := range refs {
		if !r.Hash().IsZero() {
			refMap[r.Name().String()] = r.Hash().String()
		}
	}

	// Direct match (e.g. refs/heads/main)
	if sha, ok := refMap[ref]; ok {
		return sha
	}

	// For "HEAD", resolve through the symbolic ref target
	if ref == "HEAD" {
		for _, r := range refs {
			if r.Name().String() == "HEAD" && r.Type() == plumbing.SymbolicReference {
				return refMap[r.Target().String()]
			}
		}
	}

	return ""
}

// CloneOrUpdateRepository clones a repository to a specific path or updates it if already exists.
// Tries go-git first; falls back to the system git binary if go-git fails (e.g. SSH agent issues).
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

		// Check remote HEAD without fetching to avoid unnecessary network traffic
		ref := "HEAD"
		if branch != "" {
			ref = "refs/heads/" + branch
		}
		remoteSHA := remoteRef(url, ref)

		// If local HEAD already matches remote, skip fetch entirely
		if remoteSHA != "" && oldSHA == remoteSHA {
			return oldSHA, oldSHA, "Already up to date", nil
		}

		// Try go-git fetch first; fall back to system git on failure.
		fetched := false
		{
			goRepo, openErr := git.PlainOpen(path)
			if openErr == nil {
				fetchOpts := &git.FetchOptions{
					RemoteName: "origin",
					Progress:   io.Discard,
				}
				if auth, authErr := repoAuth(url); authErr == nil {
					fetchOpts.Auth = auth
				}
				fetchErr := goRepo.Fetch(fetchOpts)
				if fetchErr == nil || fetchErr == git.NoErrAlreadyUpToDate {
					fetched = true
				}
			}
		}
		if !fetched {
			// go-git failed (e.g. SSH agent/passphrase issues) — use system git
			fetchArgs := []string{"-C", path, "fetch", "--quiet", "origin"}
			fetchCmd := exec.Command("git", fetchArgs...) // #nosec G204
			fetchCmd.Stdout = io.Discard
			fetchCmd.Stderr = os.Stderr
			if fetchErr := fetchCmd.Run(); fetchErr != nil {
				return oldSHA, "", "", fmt.Errorf("failed to fetch: %w", fetchErr)
			}
		}

		// Open the repo for local ref resolution and reset (no auth needed — local only)
		repo, err := git.PlainOpen(path)
		if err != nil {
			return oldSHA, "", "", fmt.Errorf("failed to open repository: %w", err)
		}

		// Resolve the target ref to reset to
		var targetHash plumbing.Hash
		if branch != "" {
			ref, refErr := repo.Reference(plumbing.NewRemoteReferenceName("origin", branch), true)
			if refErr != nil {
				return oldSHA, "", "", fmt.Errorf("failed to resolve origin/%s: %w", branch, refErr)
			}
			targetHash = ref.Hash()
		} else {
			ref, refErr := repo.Reference(plumbing.NewRemoteReferenceName("origin", "HEAD"), true)
			if refErr != nil {
				// Fall back to FETCH_HEAD
				ref, refErr = repo.Reference("FETCH_HEAD", true)
				if refErr != nil {
					return oldSHA, "", "", fmt.Errorf("failed to resolve fetch target: %w", refErr)
				}
			}
			targetHash = ref.Hash()
		}

		worktree, err := repo.Worktree()
		if err != nil {
			return oldSHA, "", "", fmt.Errorf("failed to get worktree: %w", err)
		}
		if err := worktree.Reset(&git.ResetOptions{
			Commit: targetHash,
			Mode:   git.HardReset,
		}); err != nil {
			return oldSHA, "", "", fmt.Errorf("failed to reset: %w", err)
		}

		newSHA := targetHash.String()
		if oldSHA != newSHA {
			return oldSHA, newSHA, "Updated", nil
		}
		return oldSHA, newSHA, "Already up to date", nil
	}

	// Try go-git clone first; fall back to system git on failure.
	if cloneErr := tryClone(path, url, branch, false); cloneErr != nil {
		// go-git failed (e.g. SSH agent/passphrase issues) — use system git
		_ = os.RemoveAll(path) // clean up any partial clone
		args := []string{"clone", "--quiet"}
		if branch != "" {
			args = append(args, "--branch", branch)
		}
		args = append(args, url, path)
		cloneCmd := exec.Command("git", args...) // #nosec G204
		cloneCmd.Stdout = io.Discard
		cloneCmd.Stderr = os.Stderr
		if err := cloneCmd.Run(); err != nil {
			return "", "", "", fmt.Errorf("failed to clone: %w", err)
		}
	}

	newSHA := gitSHA(path)
	return "", newSHA, "Cloned", nil
}

// generateRepositoryID creates a unique ID for a repository based on URL and branch
func generateRepositoryID(url, branch string) string {
	normalizedURL := normalizeGitURL(url)
	key := normalizedURL
	if branch != "" {
		key += "@" + branch
	}

	hasher := sha256.New()
	hasher.Write([]byte(key))
	return fmt.Sprintf("%x", hasher.Sum(nil))[:16] // Use first 16 chars of hash
}

// normalizeGitURL normalizes a git URL for consistent identification
func normalizeGitURL(url string) string {
	// Remove .git suffix if present
	if strings.HasSuffix(url, ".git") {
		url = url[:len(url)-4]
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

// getRepositoryStoragePath returns the path where a repository should be stored
func getRepositoryStoragePath(url, branch string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	repoID := generateRepositoryID(url, branch)
	return filepath.Join(homeDir, ".blueprint", "repos", repoID), nil
}

// copyRepositoryContents copies the contents of a repository (excluding .git) from source to destination
func copyRepositoryContents(sourcePath, destPath string) error {
	// Ensure destination parent directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0750); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Create destination directory if it doesn't exist
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Walk the source directory and copy all files except .git
	return filepath.Walk(sourcePath, func(srcPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip .git directory entirely
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		// Calculate relative path from source
		relPath, err := filepath.Rel(sourcePath, srcPath)
		if err != nil {
			return err
		}

		// Skip the source root directory itself
		if relPath == "." {
			return nil
		}

		// Calculate destination path
		destItemPath := filepath.Join(destPath, relPath)

		if info.IsDir() {
			// Create directory
			return os.MkdirAll(destItemPath, info.Mode())
		} else {
			// Copy file
			return copyFile(srcPath, destItemPath, info.Mode())
		}
	})
}

// copyFile copies a file from source to destination with the specified permissions
func copyFile(src, dest string, mode os.FileMode) error {
	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Create destination file
	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	// Copy contents
	if _, err := io.Copy(destFile, srcFile); err != nil {
		return err
	}

	// Set file permissions
	return os.Chmod(dest, mode)
}

// GetCleanRepositorySHA returns the SHA of a repository from clean storage
// Made as a variable for test stubbing
var GetCleanRepositorySHA = func(url, branch string) string {
	storagePath, err := getRepositoryStoragePath(url, branch)
	if err != nil {
		return ""
	}
	return gitSHA(storagePath)
}

// CloneOrUpdateRepositoryTwoStage variable for test stubbing
var CloneOrUpdateRepositoryTwoStage = cloneOrUpdateRepositoryTwoStageImpl

// cloneOrUpdateRepositoryTwoStageImpl implements the actual two-stage clone logic
func cloneOrUpdateRepositoryTwoStageImpl(url, targetPath, branch string) (string, string, string, error) {
	// Get storage path for clean repository
	storagePath, err := getRepositoryStoragePath(url, branch)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get storage path: %w", err)
	}

	// Expand target path
	expandedTargetPath := targetPath
	if strings.HasPrefix(targetPath, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", "", "", fmt.Errorf("failed to get home directory: %w", err)
		}
		expandedTargetPath = filepath.Join(homeDir, targetPath[2:])
	}

	// Check if target exists and get its current state
	targetExists := false
	if info, err := os.Stat(expandedTargetPath); err == nil && info.IsDir() {
		targetExists = true
	}

	// Get old SHA from clean storage (not from potentially polluted target)
	oldStorageSHA := ""
	if storageInfo, err := os.Stat(storagePath); err == nil && storageInfo.IsDir() {
		oldStorageSHA = gitSHA(storagePath)
	}

	// Clone/update the clean repository in storage
	_, newStorageSHA, storageStatus, err := CloneOrUpdateRepository(url, storagePath, branch)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to clone/update to storage: %w", err)
	}

	// Determine if we need to update the target
	needsUpdate := false
	switch storageStatus {
	case "Cloned":
		needsUpdate = true
	case "Updated":
		needsUpdate = true
	case "Already up to date":
		// Check if target exists and has the right content
		if !targetExists {
			needsUpdate = true
		} else {
			// Target exists, but we need to check if it has the right content
			// Since we can't reliably check SHA of potentially polluted target,
			// we'll rely on the storage being up to date and assume target needs sync
			// This could be optimized with content comparison in the future
			needsUpdate = false
		}
	}

	// Copy from storage to target if needed
	if needsUpdate {
		if err := copyRepositoryContents(storagePath, expandedTargetPath); err != nil {
			return oldStorageSHA, newStorageSHA, "", fmt.Errorf("failed to copy to target: %w", err)
		}
	}

	// Determine the appropriate status message
	var status string
	if !targetExists {
		status = "Cloned"
	} else if needsUpdate {
		if oldStorageSHA != newStorageSHA {
			status = "Updated"
		} else {
			status = "Synced" // Content was resynced but repo SHA didn't change
		}
	} else {
		status = "Already up to date"
	}

	return oldStorageSHA, newStorageSHA, status, nil
}
