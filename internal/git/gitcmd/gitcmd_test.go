package gitcmd

// This file enforces the policy in the package doc comment: internal git
// operations use go-git, and system git is confined to the bare/worktree
// feature plus clone/fetch fallbacks.
//
// Two layers:
//
//   (a) Primary — no file in the git subsystem other than this package may
//       import os/exec. Checked on the AST rather than by grepping, so it
//       catches `exec.Command(bin, ...)` where bin is a variable — a regex
//       over call sites never would. (Blueprint is a provisioning tool and
//       shells out to brew, asdf, mise, gpg and friends all over, so a
//       module-wide ban on os/exec would be nothing but false positives.
//       This layer is scoped to internal/git, where git operations live.)
//   (b) Secondary — module-wide, every exec.Command / exec.CommandContext
//       whose first string-literal argument is "git" must live in this
//       package or be listed in approvedSites with a written reason. This
//       layer is self-documenting: it makes the surviving exceptions, and
//       why they survive, legible in one place.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// approvedSites records the system-git call sites permitted outside gitcmd,
// keyed "path/from/module/root:line". A site with an empty reason fails: an
// unexamined exception is not an exception, it is a leak.
//
// Entries marked "TODO: migrate" are the #030 work queue — each is deleted as
// its site is ported to go-git, until the map holds only the bare/worktree
// feature and the clone/fetch fallbacks.
var approvedSites = map[string]string{
	// --- Permanent: the bare:/worktree feature -------------------------------
	// go-git v5 has no worktree support at all, so `git worktree add` cannot be
	// expressed with it. The whole bare path runs on system git by design.
	"internal/git/git.go:1443": "bare/worktree feature: second refspec-repair site (ensureFetchRefspec); go-git cannot express `git worktree`",

	// --- Permanent: clone/fetch fallbacks ------------------------------------
	// Each of these arms runs only after go-git has already failed. They cover
	// SSH agent, OS keychain and other auth cases go-git handles poorly.
	"internal/git/git.go:234": "clone fallback: system git clone after go-git tryClone fails (SSH agent/keychain auth)",
	"internal/git/git.go:688": "clone fallback: system git clone after go-git PlainClone fails (SSH agent/keychain auth)",
	"internal/git/git.go:597": "fetch fallback: system git fetch after go-git Fetch fails (SSH agent/keychain auth)",

	// --- Permanent: ls-remote fallback ---------------------------------------
	// remoteRefWithError already tries go-git remote.List (and resolves
	// symrefs) first; this is the arm reached only when that fails.
	"internal/git/git.go:496": "ls-remote fallback: reached only after go-git remote.List fails; symref resolution already handled in go-git",

	// --- TODO: migrate (#030) ------------------------------------------------
	"internal/git/git.go:608": "TODO: migrate (#030) remote.origin.fetch refspec repair -> repo.SetConfig",
	"internal/git/git.go:965": "TODO: migrate (#030) restore in repairDeletedFiles -> go-git blob write",
	"internal/git/git.go:939": "TODO: migrate (#030) diff-index deleted-path detection -> go-git HEAD-tree walk + os.Lstat",
}

// execAllowedDuringMigration grants a file-level exemption from the primary
// invariant while #030 is in flight, each with a mandatory reason. It exists
// so the invariant can be enforced from day one without leaving the tree red:
// a *new* os/exec import anywhere in the git subsystem fails immediately,
// while the known stragglers are named and tracked.
//
// Every entry is deleted when its file's last exec site is ported, at which
// point layer (a) becomes absolute.
var execAllowedDuringMigration = map[string]string{
	"internal/git/git.go": "TODO: migrate (#030) — still imports os/exec for 3 sites (608 refspec repair; 939/965 the shell reference impl, deleted with the differential test); delete this entry when the last is ported to go-git",
}

// gitSubsystem is the directory tree layer (a) governs: where blueprint's own
// git operations live, and therefore where a stray exec would violate policy.
const gitSubsystem = "internal/git"

// moduleRoot walks up from the test's directory until it finds go.mod, so the
// guard works regardless of how the test is invoked.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate module root (no go.mod above the test directory)")
		}
		dir = parent
	}
}

// goFiles returns every .go file under root. skipTests drops _test.go files:
// tests legitimately run git to build fixtures, and neither layer governs them.
func goFiles(t *testing.T, root string, skipTests bool) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip vendored and hidden directories (.git, .cache, ...).
			if d.Name() == "vendor" || (len(d.Name()) > 1 && d.Name()[0] == '.') {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if skipTests && strings.HasSuffix(path, "_test.go") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return files
}

// relPath renders path relative to the module root with forward slashes, so
// approvedSites keys read the same on every platform.
func relPath(t *testing.T, root, path string) string {
	t.Helper()
	rel, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatalf("rel %s: %v", path, err)
	}
	return filepath.ToSlash(rel)
}

// TestGitSubsystemDoesNotExec is the primary invariant: within the git
// subsystem, gitcmd is the sole exec boundary. This is the layer that catches
// `exec.Command(bin, ...)` with a variable binary name.
func TestGitSubsystemDoesNotExec(t *testing.T) {
	root := moduleRoot(t)

	for _, path := range goFiles(t, root, true) {
		rel := relPath(t, root, path)
		dir := filepath.ToSlash(filepath.Dir(rel))
		// Only the git subsystem, and never gitcmd itself.
		if dir != gitSubsystem {
			continue
		}

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, imp := range file.Imports {
			if imp.Path.Value != `"os/exec"` {
				continue
			}
			reason, exempt := execAllowedDuringMigration[rel]
			switch {
			case !exempt:
				t.Errorf("%s imports os/exec — internal git operations must use go-git; "+
					"route the call through %s/gitcmd.Run (the only exec boundary)",
					rel, gitSubsystem)
			case strings.TrimSpace(reason) == "":
				t.Errorf("%s is exempt from the exec ban with an empty reason — "+
					"document why, or finish the migration and delete the entry", rel)
			}
		}
	}
}

// TestGitCallSitesAreApproved is the secondary, self-documenting layer: every
// `git` invocation outside gitcmd needs an allowlisted reason.
func TestGitCallSitesAreApproved(t *testing.T) {
	root := moduleRoot(t)

	for _, path := range goFiles(t, root, true) {
		rel := relPath(t, root, path)
		if filepath.ToSlash(filepath.Dir(rel)) == gitSubsystem+"/gitcmd" {
			continue // this package is the boundary
		}

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isExecCall(call) || !firstStringArgIs(call, "git") {
				return true
			}
			key := rel + ":" + strconv.Itoa(fset.Position(call.Pos()).Line)
			reason, approved := approvedSites[key]
			switch {
			case !approved:
				t.Errorf("git exec at %s not allowlisted — move it behind gitcmd.Run, "+
					"or add to approvedSites with a reason.", key)
			case strings.TrimSpace(reason) == "":
				t.Errorf("git exec at %s is allowlisted with an empty reason — "+
					"document why system git is required here, or migrate it to go-git.", key)
			}
			return true
		})
	}
}

// isExecCall reports whether call is exec.Command or exec.CommandContext.
// Restricting to these two avoids matching unrelated calls that merely take a
// "git" argument — go-git's ssh.NewSSHAgentAuth("git") passes the SSH *user*,
// not the binary.
func isExecCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok || ident.Name != "exec" {
		return false
	}
	return sel.Sel.Name == "Command" || sel.Sel.Name == "CommandContext"
}

// firstStringArgIs reports whether the first string-literal argument of call
// equals want. For exec.CommandContext the context comes first, so non-literal
// arguments are skipped rather than treated as a mismatch.
func firstStringArgIs(call *ast.CallExpr, want string) bool {
	for _, arg := range call.Args {
		lit, ok := arg.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			continue
		}
		return lit.Value == `"`+want+`"`
	}
	return false
}

// TestApprovedSitesAreDocumented keeps every entry honest: a reason is
// mandatory and a stale entry is a lie about where system git is used.
func TestApprovedSitesAreDocumented(t *testing.T) {
	for key, reason := range approvedSites {
		if strings.TrimSpace(reason) == "" {
			t.Errorf("approvedSites[%q] has an empty reason — every exception must say why", key)
		}
		if !strings.Contains(key, ":") {
			t.Errorf("approvedSites key %q must be of the form file:line", key)
		}
	}
}
