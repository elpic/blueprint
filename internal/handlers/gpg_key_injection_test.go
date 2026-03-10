package handlers

import (
	"strings"
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

// TestGPGKeyUpCommandDoesNotInterpolateUserDataIntoShellString verifies that
// user-controlled fields (GPGKeyURL, GPGKeyring, GPGDebURL) are NOT embedded
// as raw strings inside a shell command string where shell metacharacters would
// be interpreted. A malicious value like `https://evil.com/key; rm -rf /` must
// not end up executable as a shell command.
func TestGPGKeyUpCommandDoesNotInterpolateUserDataIntoShellString(t *testing.T) {
	maliciousURL := "https://example.com/gpg.key'; touch /tmp/gpg-injection-marker; echo '"

	rule := parser.Rule{
		Action:     "gpg-key",
		GPGKeyring: "safe-keyring",
		GPGKeyURL:  maliciousURL,
		GPGDebURL:  "https://example.com/apt",
	}

	handler := NewGPGKeyHandler(rule, "")
	gpgKeyURL, keyringPath, tmpFile, sourcesListPath := handler.buildUpCommand()

	// Simulate what the old (buggy) code does: interpolate URL directly into shell string
	buggyCmd := "sh -c 'curl -fsSL " + gpgKeyURL + " | sudo gpg --yes --dearmor -o " + keyringPath +
		" 2>/dev/null || true && sudo cp " + tmpFile + " " + sourcesListPath + " && sudo apt update 2>/dev/null || true'"

	// The bug: the malicious payload is present in the shell string, ready to execute
	if !strings.Contains(buggyCmd, "touch /tmp/gpg-injection-marker") {
		t.Fatal("test setup broken: injection payload not found in buggy command")
	}

	// The fix must not produce a string with the payload executable.
	// With the fixed implementation the URL is passed as a discrete argument
	// to exec.Command — it never appears as part of a shell-interpreted string.
	// We verify this by checking that buildUpCommand returns the raw URL
	// (which the fixed Up() then passes as an argument, not embedded in a shell string).
	if gpgKeyURL != maliciousURL {
		t.Fatalf("buildUpCommand() returned wrong URL: %q", gpgKeyURL)
	}

	// After the fix: the Up() method must not build a shell string containing gpgKeyURL.
	// We capture what executeCommandWithCache is called with.
	var capturedCmd string
	original := executeCommandWithCache
	defer func() { executeCommandWithCache = original }()

	executeCommandWithCache = func(cmd string) (string, error) {
		capturedCmd = cmd
		return "", nil
	}

	// Call Up() — it writes a tmp sources file and calls executeCommandWithCache
	// We ignore errors from os.WriteFile since we just want to capture the cmd
	_, _ = handler.Up()

	// After the fix: the shell command passed to executeCommandWithCache must NOT
	// contain the raw malicious URL as an unquoted shell token.
	if strings.Contains(capturedCmd, "touch /tmp/gpg-injection-marker") {
		t.Errorf("SECURITY BUG: shell injection payload is present and executable in the command string: %q", capturedCmd)
	}
}
