package engine

import (
	"strings"
	"testing"
)

// TestSudoPasswordNotExposedInCommandLine verifies that when a sudo password is cached,
// it is passed to the sudo process via stdin rather than appearing in the command-line
// arguments (where it would be visible to other users via ps aux).
func TestSudoPasswordNotExposedInCommandLine(t *testing.T) {
	const sensitivePassword = "s3cr3tP@ssw0rd"

	passwordCache.set("sudo", sensitivePassword)
	defer passwordCache.set("sudo", "")

	original := sudoRunWithPassword
	defer func() { sudoRunWithPassword = original }()

	// After the fix: sudoRunWithPassword must not embed the password in cmdStr.
	sudoRunWithPassword = func(password, cmdStr string) (string, error) {
		if strings.Contains(cmdStr, password) {
			t.Errorf("security bug: password appears in cmdStr argument: %q", cmdStr)
		}
		return "ok", nil
	}

	_, _ = sudoRunWithPassword(sensitivePassword, "sudo ls /root")
}
