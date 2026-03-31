package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// testBinary holds the path to the compiled binary built once for all integration tests.
var testBinary string

// TestMain builds the CLI binary once and runs all tests.
// Non-integration tests (unit tests in main_test.go / version_test.go) are unaffected
// because they do not rely on testBinary.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "blueprint-integration-*")
	if err != nil {
		panic("failed to create temp dir: " + err.Error())
	}
	defer os.RemoveAll(dir)

	testBinary = filepath.Join(dir, "blueprint")
	cmd := exec.Command("go", "build", "-o", testBinary, ".")
	// The test binary is built from the same package directory where tests run.
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		panic("build failed: " + string(out))
	}

	os.Exit(m.Run())
}

// runBinary executes the compiled binary with the given arguments and returns
// stdout, stderr, and the process exit code.
func runBinary(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()

	var outBuf, errBuf bytes.Buffer
	cmd := exec.Command(testBinary, args...)
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			// Unexpected execution error — fail the test immediately.
			t.Fatalf("unexpected error running binary: %v", err)
		}
	}
	return
}

// sampleBP returns the absolute path to the shared test fixture.
// Note: this file contains directives (known-hosts) that fail strict validation.
func sampleBP(t *testing.T) string {
	t.Helper()
	// This test file lives in cmd/blueprint/; the fixture is two levels up.
	p, err := filepath.Abs("../../internal/testdata/sample-blueprint.bp")
	if err != nil {
		t.Fatalf("could not resolve sample blueprint path: %v", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("sample blueprint not found at %s: %v", p, err)
	}
	return p
}

// validBP returns the absolute path to a blueprint fixture that passes strict validation.
func validBP(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs("../../internal/engine/testdata/elpic-setup-runs.bp")
	if err != nil {
		t.Fatalf("could not resolve valid blueprint path: %v", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("valid blueprint not found at %s: %v", p, err)
	}
	return p
}

// ---------------------------------------------------------------------------
// No-args
// ---------------------------------------------------------------------------

func TestIntegration_NoArgs_PrintsUsageExits1(t *testing.T) {
	stdout, _, exitCode := runBinary(t)
	if exitCode != 1 {
		t.Errorf("expected exit 1, got %d", exitCode)
	}
	if !strings.Contains(stdout, "Usage:") {
		t.Errorf("expected usage message in stdout, got: %q", stdout)
	}
}

// ---------------------------------------------------------------------------
// version subcommand
// ---------------------------------------------------------------------------

func TestIntegration_Version_PrintsVersionAndCommit(t *testing.T) {
	stdout, _, exitCode := runBinary(t, "version")
	if exitCode != 0 {
		t.Errorf("expected exit 0, got %d", exitCode)
	}
	if !strings.Contains(stdout, "Version:") {
		t.Errorf("expected 'Version:' in output, got: %q", stdout)
	}
	if !strings.Contains(stdout, "Commit:") {
		t.Errorf("expected 'Commit:' in output, got: %q", stdout)
	}
}

func TestIntegration_Version_Commit_PrintsOnlyCommit(t *testing.T) {
	stdout, _, exitCode := runBinary(t, "version", "--commit")
	if exitCode != 0 {
		t.Errorf("expected exit 0, got %d", exitCode)
	}
	// Output should be a single trimmed token (the commit hash), no "Version:" label.
	trimmed := strings.TrimSpace(stdout)
	if trimmed == "" {
		t.Error("expected non-empty commit output")
	}
	if strings.Contains(stdout, "Version:") {
		t.Errorf("--commit flag should not print 'Version:', got: %q", stdout)
	}
}

func TestIntegration_Version_Short_PrintsOnlyVersion(t *testing.T) {
	stdout, _, exitCode := runBinary(t, "version", "--short")
	if exitCode != 0 {
		t.Errorf("expected exit 0, got %d", exitCode)
	}
	trimmed := strings.TrimSpace(stdout)
	if trimmed == "" {
		t.Error("expected non-empty version output")
	}
	if strings.Contains(stdout, "Commit:") {
		t.Errorf("--short flag should not print 'Commit:', got: %q", stdout)
	}
}

// ---------------------------------------------------------------------------
// Subcommands that require a file argument — missing arg → usage + exit 1
// ---------------------------------------------------------------------------

func TestIntegration_MissingFileArg(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantMsg string
	}{
		{"plan", []string{"plan"}, "Usage:"},
		{"apply", []string{"apply"}, "Usage:"},
		{"encrypt", []string{"encrypt"}, "Usage:"},
		{"diff", []string{"diff"}, "Usage:"},
		{"validate", []string{"validate"}, "Usage:"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			stdout, _, exitCode := runBinary(t, tc.args...)
			if exitCode != 1 {
				t.Errorf("%s: expected exit 1, got %d", tc.name, exitCode)
			}
			if !strings.Contains(stdout, tc.wantMsg) {
				t.Errorf("%s: expected %q in stdout, got: %q", tc.name, tc.wantMsg, stdout)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Commands that run without file arguments
// ---------------------------------------------------------------------------

func TestIntegration_Status_ExitsZero(t *testing.T) {
	_, _, exitCode := runBinary(t, "status")
	if exitCode != 0 {
		t.Errorf("expected exit 0, got %d", exitCode)
	}
}

func TestIntegration_History_ExitsZero(t *testing.T) {
	_, _, exitCode := runBinary(t, "history")
	if exitCode != 0 {
		t.Errorf("expected exit 0, got %d", exitCode)
	}
}

func TestIntegration_PS_ExitsZero(t *testing.T) {
	_, _, exitCode := runBinary(t, "ps")
	if exitCode != 0 {
		t.Errorf("expected exit 0, got %d", exitCode)
	}
}

func TestIntegration_Slow_ExitsZero(t *testing.T) {
	_, _, exitCode := runBinary(t, "slow")
	if exitCode != 0 {
		t.Errorf("expected exit 0, got %d", exitCode)
	}
}

func TestIntegration_Doctor_ExitsZero(t *testing.T) {
	_, _, exitCode := runBinary(t, "doctor")
	if exitCode != 0 {
		t.Errorf("expected exit 0, got %d", exitCode)
	}
}

func TestIntegration_DoctorFix_ExitsZero(t *testing.T) {
	_, _, exitCode := runBinary(t, "doctor", "--fix")
	if exitCode != 0 {
		t.Errorf("expected exit 0, got %d", exitCode)
	}
}

// ---------------------------------------------------------------------------
// slow --top variations
// ---------------------------------------------------------------------------

func TestIntegration_Slow_Top5_ExitsZero(t *testing.T) {
	_, _, exitCode := runBinary(t, "slow", "--top", "5")
	if exitCode != 0 {
		t.Errorf("expected exit 0, got %d", exitCode)
	}
}

func TestIntegration_Slow_TopZero_Exits1(t *testing.T) {
	_, stderr, exitCode := runBinary(t, "slow", "--top", "0")
	if exitCode != 1 {
		t.Errorf("expected exit 1 for --top 0, got %d", exitCode)
	}
	if !strings.Contains(stderr, "positive integer") {
		t.Errorf("expected error about positive integer in stderr, got: %q", stderr)
	}
}

func TestIntegration_Slow_TopInvalid_Exits1(t *testing.T) {
	_, stderr, exitCode := runBinary(t, "slow", "--top", "abc")
	if exitCode != 1 {
		t.Errorf("expected exit 1 for --top abc, got %d", exitCode)
	}
	if !strings.Contains(stderr, "valid integer") {
		t.Errorf("expected error about valid integer in stderr, got: %q", stderr)
	}
}

// ---------------------------------------------------------------------------
// history with run_number argument
// ---------------------------------------------------------------------------

func TestIntegration_History_InvalidRunNumber_Exits1(t *testing.T) {
	_, stderr, exitCode := runBinary(t, "history", "abc")
	if exitCode != 1 {
		t.Errorf("expected exit 1 for non-integer run_number, got %d", exitCode)
	}
	if !strings.Contains(stderr, "valid integer") {
		t.Errorf("expected error about valid integer in stderr, got: %q", stderr)
	}
}

func TestIntegration_History_NegativeRunNumber_Exits1(t *testing.T) {
	_, stderr, exitCode := runBinary(t, "history", "-1")
	if exitCode != 1 {
		t.Errorf("expected exit 1 for negative run_number, got %d", exitCode)
	}
	if !strings.Contains(stderr, "non-negative") {
		t.Errorf("expected error about non-negative in stderr, got: %q", stderr)
	}
}

// ---------------------------------------------------------------------------
// plan with valid .bp file (dry-run — safe to run)
// ---------------------------------------------------------------------------

func TestIntegration_Plan_ValidFile_ExitsZero(t *testing.T) {
	bp := sampleBP(t)
	_, _, exitCode := runBinary(t, "plan", bp)
	if exitCode != 0 {
		t.Errorf("expected exit 0 for plan with valid file, got %d", exitCode)
	}
}

// ---------------------------------------------------------------------------
// validate subcommand
// ---------------------------------------------------------------------------

// TestIntegration_Validate_ValidFile_ExitsZero uses a fixture that contains
// only directives the parser recognises, so Validate reports no issues.
func TestIntegration_Validate_ValidFile_ExitsZero(t *testing.T) {
	bp := validBP(t)
	stdout, _, exitCode := runBinary(t, "validate", bp)
	if exitCode != 0 {
		t.Errorf("expected exit 0 for validate with valid file, got %d", exitCode)
	}
	if !strings.Contains(stdout, "No issues") {
		t.Errorf("expected 'No issues' in output, got: %q", stdout)
	}
}

// TestIntegration_Validate_SampleFile_Exits1 verifies that validate exits 1
// when the blueprint file contains unrecognised directives (known-hosts).
func TestIntegration_Validate_SampleFile_Exits1(t *testing.T) {
	bp := sampleBP(t)
	stdout, _, exitCode := runBinary(t, "validate", bp)
	if exitCode != 1 {
		t.Errorf("expected exit 1 for file with unknown directives, got %d", exitCode)
	}
	if !strings.Contains(stdout, "failed") {
		t.Errorf("expected 'failed' in output for invalid file, got: %q", stdout)
	}
}

// ---------------------------------------------------------------------------
// diff with valid .bp file
// ---------------------------------------------------------------------------

func TestIntegration_Diff_ValidFile_ExitsZero(t *testing.T) {
	bp := validBP(t)
	_, _, exitCode := runBinary(t, "diff", bp)
	if exitCode != 0 {
		t.Errorf("expected exit 0 for diff with valid file, got %d", exitCode)
	}
}

// ---------------------------------------------------------------------------
// Error cases
// ---------------------------------------------------------------------------

func TestIntegration_UnknownCommand_PrintsErrorExits1(t *testing.T) {
	_, stderr, exitCode := runBinary(t, "unknown-command")
	if exitCode != 1 {
		t.Errorf("expected exit 1 for unknown command, got %d", exitCode)
	}
	if !strings.Contains(stderr, "unknown command") {
		t.Errorf("expected 'unknown command' in stderr, got: %q", stderr)
	}
}

func TestIntegration_NonExistentFilePath_PrintsErrorExits1(t *testing.T) {
	_, stderr, exitCode := runBinary(t, "/nonexistent/path/to/file.bp")
	if exitCode != 1 {
		t.Errorf("expected exit 1 for nonexistent file path, got %d", exitCode)
	}
	if !strings.Contains(stderr, "unknown command") {
		t.Errorf("expected 'unknown command' in stderr, got: %q", stderr)
	}
}
