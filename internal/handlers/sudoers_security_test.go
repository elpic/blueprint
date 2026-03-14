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

	// Stub sudoRun — it must NOT be called when already present
	origSudoRun := sudoRun
	defer func() { sudoRun = origSudoRun }()
	sudoRunCalled := false
	sudoRun = func(args ...string) (string, error) {
		sudoRunCalled = true
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
	if sudoRunCalled {
		t.Error("sudoRun should not have been called when entry already exists")
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

	// Stub sudoersFileReader so the skip-check doesn't short-circuit
	origReader := sudoersFileReader
	defer func() { sudoersFileReader = origReader }()
	sudoersFileReader = func(path string) ([]byte, error) {
		return nil, nil // empty → not already present
	}

	// Intercept sudoRun — capture visudo call to verify temp path
	origSudoRun := sudoRun
	defer func() { sudoRun = origSudoRun }()
	sudoRun = func(args ...string) (string, error) {
		// args: ["visudo", "-c", "-f", "<tmpPath>"]
		if len(args) > 0 && args[0] == "visudo" {
			capturedTmpPath = args[len(args)-1]
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
