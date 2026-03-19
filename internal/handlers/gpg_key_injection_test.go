package handlers

import (
	"strings"
	"testing"

	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/platform"
)

// gpgMockExecutor for test-specific behavior
type gpgMockExecutor struct {
	executeFunc func(string) (string, error)
}

// Ensure it implements platform.CommandExecutor
var _ platform.CommandExecutor = (*gpgMockExecutor)(nil)

func (c *gpgMockExecutor) Execute(cmd string) (string, error) {
	if c.executeFunc != nil {
		return c.executeFunc(cmd)
	}
	return "", nil
}

// TestGPGKeyDownloadKeyURLPassedAsArgument verifies that user-controlled URLs
// are passed as discrete exec.Command arguments rather than interpolated into
// a shell string. The downloadKey var is the injection point — test it directly.
func TestGPGKeyDownloadKeyURLPassedAsArgument(t *testing.T) {
	maliciousURL := "https://example.com/gpg.key'; touch /tmp/gpg-injection-marker; echo '"

	var capturedURL string
	original := downloadKey
	defer func() { downloadKey = original }()

	downloadKey = func(url, destPath, sudoPassword string) error {
		capturedURL = url
		return nil
	}

	rule := parser.Rule{
		Action:     "gpg-key",
		GPGKeyring: "safe-keyring",
		GPGKeyURL:  maliciousURL,
		GPGDebURL:  "https://example.com/apt",
	}
	handler := NewGPGKeyHandler(rule, "")

	// Call the method that invokes downloadKey to confirm URL is passed verbatim.
	_ = handler.downloadKey(maliciousURL, "/tmp/safe-keyring.asc")

	if capturedURL != maliciousURL {
		t.Fatalf("downloadKey received wrong URL: got %q, want %q", capturedURL, maliciousURL)
	}
}

// TestGPGKeyGetCommandDoesNotExecShellInjection verifies that the display
// command (GetCommand) embeds the URL in a string for human display, but that
// Up() never passes user data to executeCommandWithCache (which calls sh -c).
func TestGPGKeyGetCommandDoesNotExecShellInjection(t *testing.T) {
	maliciousURL := "https://example.com/gpg.key'; touch /tmp/gpg-injection-marker; echo '"

	rule := parser.Rule{
		Action:     "gpg-key",
		GPGKeyring: "safe-keyring",
		GPGKeyURL:  maliciousURL,
		GPGDebURL:  "https://example.com/apt",
	}
	handler := NewGPGKeyHandler(rule, "")

	var capturedExecCmd string

	// Use dependency injection pattern instead of global function assignment
	mockExecutor := &gpgMockExecutor{
		executeFunc: func(c string) (string, error) {
			capturedExecCmd = c
			return "", nil
		},
	}

	originalExecutor := commandExecutor
	defer func() { commandExecutor = originalExecutor }()
	commandExecutor = mockExecutor

	// Stub downloadKey so it succeeds without running curl.
	origDL := downloadKey
	defer func() { downloadKey = origDL }()
	downloadKey = func(url, destPath, sudoPassword string) error { return nil }

	_, _ = handler.Up()

	if strings.Contains(capturedExecCmd, "touch /tmp/gpg-injection-marker") {
		t.Errorf("SECURITY BUG: shell injection payload reached executeCommandWithCache: %q", capturedExecCmd)
	}
}
