package handlers

import (
	"strings"
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

// TestSudoersUpSkipsIfAlreadyPresent verifies that Up() returns early without
// running any commands when the correct sudoers entry already exists on disk.
func TestSudoersUpSkipsIfAlreadyPresent(t *testing.T) {
	const user = "testuser"

	// Stub the file reader to return the correct entry
	origReader := sudoersFileReader
	defer func() { sudoersFileReader = origReader }()
	sudoersFileReader = func(path string) ([]byte, error) {
		entry := sudoersEntry(user)
		return []byte(entry), nil
	}

	// Stub executeCommandWithCache — it must NOT be called
	origExec := executeCommandWithCache
	defer func() { executeCommandWithCache = origExec }()
	executed := false
	executeCommandWithCache = func(cmd string) (string, error) {
		executed = true
		return "", nil
	}

	rule := parser.Rule{Action: "sudoers", SudoersUser: user}
	h := NewSudoersHandler(rule, "")
	out, err := h.Up()
	if err != nil {
		t.Fatalf("Up() returned error: %v", err)
	}
	if !strings.Contains(out, "already in sudoers") {
		t.Errorf("expected 'already in sudoers' message, got %q", out)
	}
	if executed {
		t.Error("executeCommandWithCache should not have been called when entry already exists")
	}
}

// TestSudoersTempFileUsesSecureDir verifies that the sudoers handler creates
// its temp file in the directory returned by sudoersTempDir(), not via a
// hardcoded os.TempDir() call that would bypass the secure-directory selection.
func TestSudoersTempFileUsesSecureDir(t *testing.T) {
	const secureMarker = "/blueprint-secure-tempdir-test"

	// Use a recognisable path that is clearly NOT the default os.TempDir().
	// We don't actually create it — we just want to see if CreateTemp is called
	// with it. CreateTemp will fail, but we only care about which dir it tried.
	var capturedTmpPath string

	// Intercept the temp-dir selection
	original := sudoersTempDir
	defer func() { sudoersTempDir = original }()

	sudoersTempDir = func() (string, error) {
		return secureMarker, nil
	}

	// Intercept executeCommandWithCache — capture visudo call
	origExec := executeCommandWithCache
	defer func() { executeCommandWithCache = origExec }()

	executeCommandWithCache = func(cmd string) (string, error) {
		if strings.HasPrefix(cmd, "visudo") {
			parts := strings.Fields(cmd)
			if len(parts) >= 4 {
				capturedTmpPath = parts[len(parts)-1]
			}
		}
		return "", nil
	}

	rule := parser.Rule{
		Action:      "sudoers",
		SudoersUser: "testuser",
	}
	h := NewSudoersHandler(rule, "")
	// Up() will fail because secureMarker doesn't exist, but that's OK —
	// we just want to see that CreateTemp was attempted with our dir.
	_, _ = h.Up()

	// If capturedTmpPath is set it means CreateTemp succeeded — which means
	// the old code (os.CreateTemp("", ...)) was used instead of our override.
	// With the fix, CreateTemp(secureMarker, ...) will fail → Up() returns early
	// → capturedTmpPath remains empty — and that's the expected post-fix behaviour.
	//
	// If capturedTmpPath is NON-empty and does NOT start with secureMarker,
	// then the bug is present: the code ignores sudoersTempDir and uses os.TempDir().
	if capturedTmpPath != "" && !strings.HasPrefix(capturedTmpPath, secureMarker) {
		t.Errorf("SECURITY BUG: temp file created outside secure dir (got %q, want prefix %q)", capturedTmpPath, secureMarker)
	}
}
