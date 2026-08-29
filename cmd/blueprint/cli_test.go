package main

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// ansiRegexp matches ANSI escape sequences (e.g. the lipgloss styling the
// engine/ui packages emit unconditionally) so assertions can match on plain
// substrings of captured output.
var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripANSI removes ANSI escape sequences from s.
func stripANSI(s string) string {
	return ansiRegexp.ReplaceAllString(s, "")
}

// runCLI sets HOME to a temp dir, swaps os.Stdout/os.Stderr, runs
// ExecuteCommand, restores the streams, and returns (exitCode, stdout, stderr)
// with ANSI codes stripped from the captured output.
//
// The temp HOME redirects all ~/.blueprint writes (status.json, history, etc.)
// so tests never touch the real user directory. The os.Stdout/os.Stderr swap
// makes these tests not parallel-safe by design.
func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	oldStdout, oldStderr := os.Stdout, os.Stderr
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = stdoutW
	os.Stderr = stderrW

	code := ExecuteCommand(args)

	os.Stdout = oldStdout
	os.Stderr = oldStderr
	if err := stdoutW.Close(); err != nil {
		t.Fatal(err)
	}
	if err := stderrW.Close(); err != nil {
		t.Fatal(err)
	}

	stdout, err := io.ReadAll(stdoutR)
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := io.ReadAll(stderrR)
	if err != nil {
		t.Fatal(err)
	}
	return code, stripANSI(string(stdout)), stripANSI(string(stderr))
}

// writeMockBlueprint writes a minimal valid blueprint file and returns its path.
// The run rule has no `on:` filter so it applies on all OSes; the dry-run,
// status, and diff paths never execute it.
func writeMockBlueprint(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "mock.bp")
	if err := os.WriteFile(path, []byte("run echo hello\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

// knownCommandNames returns the sorted list of subcommand names so table-driven
// tests run in a deterministic order.
func knownCommandNames() []string {
	names := make([]string, 0, len(knownCommands))
	for cmd := range knownCommands {
		names = append(names, cmd)
	}
	sort.Strings(names)
	return names
}

// ---------------------------------------------------------------------------
// Arg validation & error cases (exit before reaching the engine — SAFE)
// ---------------------------------------------------------------------------

func TestExecuteCommand_MissingFileArg(t *testing.T) {
	commands := []string{"plan", "apply", "validate", "diff", "export", "encrypt", "render", "check", "template", "get"}
	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			code, stdout, stderr := runCLI(t, cmd)
			if code != 1 {
				t.Errorf("exit code: want 1, got %d", code)
			}
			if out := stdout + stderr; !strings.Contains(out, "Usage:") {
				t.Errorf("output should contain \"Usage:\", got:\n%s", out)
			}
		})
	}
}

func TestExecuteCommand_GetTooFewArgs(t *testing.T) {
	// get needs <file> <action> <key>; only file+action is provided.
	mock := writeMockBlueprint(t, t.TempDir())
	code, stdout, stderr := runCLI(t, "get", mock, "mise")
	if code != 1 {
		t.Errorf("exit code: want 1, got %d", code)
	}
	if out := stdout + stderr; !strings.Contains(out, "Usage:") {
		t.Errorf("output should contain \"Usage:\", got:\n%s", out)
	}
}

func TestExecuteCommand_HistoryInvalidRunNumber(t *testing.T) {
	code, _, stderr := runCLI(t, "history", "abc")
	if code != 1 {
		t.Errorf("exit code: want 1, got %d", code)
	}
	if !strings.Contains(stderr, "valid integer") {
		t.Errorf("stderr should contain \"valid integer\", got: %s", stderr)
	}
}

func TestExecuteCommand_SlowInvalidTop(t *testing.T) {
	// --top 0 is numeric but below 1, so parsePositiveInt rejects it.
	code, _, stderr := runCLI(t, "slow", "--top", "0")
	if code != 1 {
		t.Errorf("exit code: want 1, got %d", code)
	}
	if !strings.Contains(stderr, "positive integer") {
		t.Errorf("stderr should contain \"positive integer\", got: %s", stderr)
	}
}

func TestExecuteCommand_ExportInvalidFormat(t *testing.T) {
	mock := writeMockBlueprint(t, t.TempDir())
	code, _, stderr := runCLI(t, "export", mock, "--format", "bogus")
	if code != 1 {
		t.Errorf("exit code: want 1, got %d", code)
	}
	if !strings.Contains(stderr, `must be "bash" or "sh"`) {
		t.Errorf("stderr should contain the format error, got: %s", stderr)
	}
}

func TestExecuteCommand_PlanMalformedVar(t *testing.T) {
	// --var without "=" is rejected before the engine is reached.
	code, _, stderr := runCLI(t, "plan", "x.bp", "--var", "BADVAR")
	if code != 1 {
		t.Errorf("exit code: want 1, got %d", code)
	}
	if !strings.Contains(stderr, "KEY=VALUE") {
		t.Errorf("stderr should contain \"KEY=VALUE\", got: %s", stderr)
	}
}

func TestExecuteCommand_RenderMissingTemplate(t *testing.T) {
	code, _, stderr := runCLI(t, "render", "x.bp")
	if code != 1 {
		t.Errorf("exit code: want 1, got %d", code)
	}
	if !strings.Contains(stderr, "--template") {
		t.Errorf("stderr should contain \"--template\", got: %s", stderr)
	}
}

func TestExecuteCommand_CheckMissingTemplate(t *testing.T) {
	code, _, stderr := runCLI(t, "check", "x.bp")
	if code != 1 {
		t.Errorf("exit code: want 1, got %d", code)
	}
	if !strings.Contains(stderr, "--template") {
		t.Errorf("stderr should contain \"--template\", got: %s", stderr)
	}
}

func TestExecuteCommand_TemplateMissingOutput(t *testing.T) {
	code, _, stderr := runCLI(t, "template", "x")
	if code != 1 {
		t.Errorf("exit code: want 1, got %d", code)
	}
	if !strings.Contains(stderr, "--output") {
		t.Errorf("stderr should contain \"--output\", got: %s", stderr)
	}
}

func TestExecuteCommand_UnknownCommand(t *testing.T) {
	code, _, stderr := runCLI(t, "bogus")
	if code != 1 {
		t.Errorf("exit code: want 1, got %d", code)
	}
	if !strings.Contains(stderr, "unknown command") {
		t.Errorf("stderr should contain \"unknown command\", got: %s", stderr)
	}
}

// ---------------------------------------------------------------------------
// --help / bare invocation
// ---------------------------------------------------------------------------

func TestExecuteCommand_HelpForAllCommands(t *testing.T) {
	for _, cmd := range knownCommandNames() {
		t.Run(cmd, func(t *testing.T) {
			code, stdout, stderr := runCLI(t, cmd, "--help")
			if code != 0 {
				t.Errorf("exit code: want 0, got %d", code)
			}
			if out := stdout + stderr; !strings.Contains(out, "Usage:") {
				t.Errorf("output should contain \"Usage:\", got:\n%s", out)
			}
		})
	}
}

func TestExecuteCommand_GlobalHelpFlags(t *testing.T) {
	for _, flag := range []string{"--help", "-h"} {
		code, stdout, stderr := runCLI(t, flag)
		if code != 0 {
			t.Errorf("%s exit code: want 0, got %d", flag, code)
		}
		if out := stdout + stderr; !strings.Contains(out, "Usage:") {
			t.Errorf("%s output should contain \"Usage:\", got:\n%s", flag, out)
		}
	}
}

func TestExecuteCommand_NoArgsShowsHelpExitOne(t *testing.T) {
	code, stdout, stderr := runCLI(t)
	if code != 1 {
		t.Errorf("bare invocation exit code: want 1, got %d", code)
	}
	if out := stdout + stderr; !strings.Contains(out, "Usage:") {
		t.Errorf("output should contain \"Usage:\", got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Safe success paths (engine prints & returns normally — NO real system changes)
// ---------------------------------------------------------------------------

func TestExecuteCommand_Version(t *testing.T) {
	code, stdout, _ := runCLI(t, "version")
	if code != 0 {
		t.Errorf("exit code: want 0, got %d", code)
	}
	if !strings.Contains(stdout, "Version:") {
		t.Errorf("stdout should contain \"Version:\", got: %s", stdout)
	}
	if !strings.Contains(stdout, "Commit:") {
		t.Errorf("stdout should contain \"Commit:\", got: %s", stdout)
	}
}

func TestExecuteCommand_VersionShort(t *testing.T) {
	code, stdout, _ := runCLI(t, "version", "--short")
	if code != 0 {
		t.Errorf("exit code: want 0, got %d", code)
	}
	if !strings.Contains(stdout, "dev") {
		t.Errorf("stdout should contain \"dev\" (default version), got: %s", stdout)
	}
}

func TestExecuteCommand_VersionCommit(t *testing.T) {
	code, stdout, _ := runCLI(t, "version", "--commit")
	if code != 0 {
		t.Errorf("exit code: want 0, got %d", code)
	}
	if !strings.Contains(stdout, "none") {
		t.Errorf("stdout should contain \"none\" (default commit), got: %s", stdout)
	}
}

func TestExecuteCommand_PlanDryRun(t *testing.T) {
	// plan is apply in dry-run mode and never executes rules.
	mock := writeMockBlueprint(t, t.TempDir())
	code, stdout, stderr := runCLI(t, "plan", mock)
	if code != 0 {
		t.Errorf("exit code: want 0, got %d", code)
	}
	if out := stdout + stderr; !strings.Contains(out, "PLAN MODE - DRY RUN") {
		t.Errorf("output should contain \"PLAN MODE - DRY RUN\", got:\n%s", out)
	}
}

func TestExecuteCommand_StatusNoStatusFile(t *testing.T) {
	code, stdout, stderr := runCLI(t, "status")
	if code != 0 {
		t.Errorf("exit code: want 0, got %d", code)
	}
	if out := stdout + stderr; !strings.Contains(out, "No status file found") {
		t.Errorf("output should contain \"No status file found\", got:\n%s", out)
	}
}

func TestExecuteCommand_Diff(t *testing.T) {
	// PrintDiff parses the blueprint and compares against an empty status;
	// RunHandler.IsInstalled only checks status records, so no commands run.
	mock := writeMockBlueprint(t, t.TempDir())
	code, stdout, stderr := runCLI(t, "diff", mock)
	if code != 0 {
		t.Errorf("exit code: want 0, got %d", code)
	}
	if out := stdout + stderr; !strings.Contains(out, "Blueprint Diff") {
		t.Errorf("output should contain \"Blueprint Diff\", got:\n%s", out)
	}
}

func TestExecuteCommand_DoctorNoStatusFile(t *testing.T) {
	code, stdout, stderr := runCLI(t, "doctor")
	if code != 0 {
		t.Errorf("exit code: want 0, got %d", code)
	}
	if out := stdout + stderr; !strings.Contains(out, "No issues found") {
		t.Errorf("output should contain \"No issues found\", got:\n%s", out)
	}
}

func TestExecuteCommand_ValidateValidBlueprint(t *testing.T) {
	// Only the success path is tested here: engine.Validate calls os.Exit(1)
	// on errors, which would kill the test process.
	mock := writeMockBlueprint(t, t.TempDir())
	code, stdout, stderr := runCLI(t, "validate", mock)
	if code != 0 {
		t.Errorf("exit code: want 0, got %d", code)
	}
	if out := stdout + stderr; !strings.Contains(out, "No issues found.") {
		t.Errorf("output should contain \"No issues found.\", got:\n%s", out)
	}
}
