// Package gitcmd is the only place in blueprint that executes the git binary.
//
// # Policy
//
// Internal git operations must use go-git (github.com/go-git/go-git/v5).
// System git is permitted only for:
//
//  1. the bare:/worktree feature — go-git v5 has no worktree support at all,
//     so `git worktree add` cannot be expressed with it; and
//  2. fallbacks when go-git fails (SSH agent, OS keychain, other auth cases
//     go-git handles poorly).
//
// Everything else — deleted-file detection and repair, config writes, remote
// ref resolution, clone/update — must go through go-git.
//
// # Enforcement
//
// This policy is enforced by gitcmd_test.go, which fails if any file outside
// this package imports os/exec, or if a `git` invocation appears at a
// call site that is not allowlisted with a written reason. Keeping every exec
// behind this one package is what makes the policy enforceable: a single
// chokepoint the compiler and the guard test can both see.
package gitcmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Run executes the git binary and returns its stdout.
//
// dir is passed as -C so the command operates on that repository; an empty dir
// runs git in the process working directory. Any stderr is folded into the
// returned error, since callers surface it directly to the user and an empty
// error message makes a failed git command very hard to diagnose.
//
// Callers are expected to pass a context carrying their own deadline; see
// Timeout for the standard one.
func Run(ctx context.Context, dir string, args ...string) (string, error) {
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	cmd := exec.CommandContext(ctx, "git", args...) // #nosec G204
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// Timeout returns the standard timeout for git network operations.
// It reads BLUEPRINT_GIT_TIMEOUT (seconds) and defaults to 120s.
func Timeout() time.Duration {
	if s := os.Getenv("BLUEPRINT_GIT_TIMEOUT"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return 120 * time.Second
}
