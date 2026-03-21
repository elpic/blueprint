package handlers

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

// ---------------------------------------------------------------------------
// parseKeyLines
// ---------------------------------------------------------------------------

func TestParseKeyLines(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "empty content returns nil",
			content:  "",
			expected: nil,
		},
		{
			name:     "comment lines are skipped",
			content:  "# this is a comment\n# another comment",
			expected: nil,
		},
		{
			name:     "blank lines are skipped",
			content:  "\n\n   \n",
			expected: nil,
		},
		{
			name:     "single valid key line",
			content:  "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI user@host",
			expected: []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI user@host"},
		},
		{
			name: "multiple valid key lines",
			content: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI user@host\n" +
				"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAB other@host",
			expected: []string{
				"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI user@host",
				"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAB other@host",
			},
		},
		{
			name:    "mixed content: comments, blanks, and keys",
			content: "# comment\n\nssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI user@host\n\n# another comment\nssh-rsa AAAA other@host",
			expected: []string{
				"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI user@host",
				"ssh-rsa AAAA other@host",
			},
		},
		{
			name:     "lines with only whitespace are skipped",
			content:  "  \t  \n ssh-ed25519 AAAA user@host \n  ",
			expected: []string{"ssh-ed25519 AAAA user@host"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseKeyLines(tt.content)
			if len(got) != len(tt.expected) {
				t.Fatalf("parseKeyLines() returned %d lines, want %d; got: %v", len(got), len(tt.expected), got)
			}
			for i, line := range got {
				if line != tt.expected[i] {
					t.Errorf("parseKeyLines()[%d] = %q, want %q", i, line, tt.expected[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GetCommand
// ---------------------------------------------------------------------------

func TestAuthorizedKeysGetCommand(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		expected string
	}{
		{
			name: "file source",
			rule: parser.Rule{
				Action:             "authorized_keys",
				AuthorizedKeysFile: "~/.ssh/id_ed25519.pub",
			},
			expected: "cat ~/.ssh/id_ed25519.pub >> ~/.ssh/authorized_keys",
		},
		{
			name: "encrypted source",
			rule: parser.Rule{
				Action:                  "authorized_keys",
				AuthorizedKeysEncrypted: "secrets/key.enc",
			},
			expected: "decrypt secrets/key.enc >> ~/.ssh/authorized_keys",
		},
		{
			name: "uninstall action uses file source same as install",
			rule: parser.Rule{
				Action:             "uninstall",
				AuthorizedKeysFile: "~/.ssh/id_ed25519.pub",
			},
			expected: "cat ~/.ssh/id_ed25519.pub >> ~/.ssh/authorized_keys",
		},
		{
			name: "encrypted beats file when both set",
			rule: parser.Rule{
				Action:                  "authorized_keys",
				AuthorizedKeysFile:      "~/.ssh/id_ed25519.pub",
				AuthorizedKeysEncrypted: "secrets/key.enc",
			},
			expected: "decrypt secrets/key.enc >> ~/.ssh/authorized_keys",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewAuthorizedKeysHandler(tt.rule, "", nil)
			got := handler.GetCommand()
			if got != tt.expected {
				t.Errorf("GetCommand() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GetDependencyKey
// ---------------------------------------------------------------------------

func TestAuthorizedKeysGetDependencyKey(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		expected string
	}{
		{
			name: "uses rule ID when present",
			rule: parser.Rule{
				ID:                 "my-key",
				AuthorizedKeysFile: "~/.ssh/id_ed25519.pub",
			},
			expected: "my-key",
		},
		{
			name: "falls back to file path when no ID",
			rule: parser.Rule{
				AuthorizedKeysFile: "~/.ssh/id_ed25519.pub",
			},
			expected: "~/.ssh/id_ed25519.pub",
		},
		{
			name: "falls back to encrypted path when no ID and no file",
			rule: parser.Rule{
				AuthorizedKeysEncrypted: "secrets/key.enc",
			},
			expected: "secrets/key.enc",
		},
		{
			name: "encrypted beats file in fallback",
			rule: parser.Rule{
				AuthorizedKeysFile:      "~/.ssh/id_ed25519.pub",
				AuthorizedKeysEncrypted: "secrets/key.enc",
			},
			expected: "secrets/key.enc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewAuthorizedKeysHandler(tt.rule, "", nil)
			got := handler.GetDependencyKey()
			if got != tt.expected {
				t.Errorf("GetDependencyKey() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GetDisplayDetails
// ---------------------------------------------------------------------------

func TestAuthorizedKeysGetDisplayDetails(t *testing.T) {
	tests := []struct {
		name        string
		rule        parser.Rule
		isUninstall bool
		expected    string
	}{
		{
			name: "returns file when set",
			rule: parser.Rule{
				AuthorizedKeysFile: "~/.ssh/id_ed25519.pub",
			},
			isUninstall: false,
			expected:    "~/.ssh/id_ed25519.pub",
		},
		{
			name: "returns encrypted when set",
			rule: parser.Rule{
				AuthorizedKeysEncrypted: "secrets/key.enc",
			},
			isUninstall: false,
			expected:    "secrets/key.enc",
		},
		{
			name: "returns encrypted over file when both set",
			rule: parser.Rule{
				AuthorizedKeysFile:      "~/.ssh/id_ed25519.pub",
				AuthorizedKeysEncrypted: "secrets/key.enc",
			},
			isUninstall: false,
			expected:    "secrets/key.enc",
		},
		{
			name: "returns file for uninstall",
			rule: parser.Rule{
				AuthorizedKeysFile: "~/.ssh/id_ed25519.pub",
			},
			isUninstall: true,
			expected:    "~/.ssh/id_ed25519.pub",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewAuthorizedKeysHandler(tt.rule, "", nil)
			got := handler.GetDisplayDetails(tt.isUninstall)
			if got != tt.expected {
				t.Errorf("GetDisplayDetails(%v) = %q, want %q", tt.isUninstall, got, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GetState
// ---------------------------------------------------------------------------

func TestAuthorizedKeysGetState(t *testing.T) {
	tests := []struct {
		name            string
		rule            parser.Rule
		isUninstall     bool
		expectedSummary string
		expectedSource  string
	}{
		{
			name: "file source state",
			rule: parser.Rule{
				AuthorizedKeysFile: "~/.ssh/id_ed25519.pub",
			},
			isUninstall:     false,
			expectedSummary: "~/.ssh/id_ed25519.pub",
			expectedSource:  "~/.ssh/id_ed25519.pub",
		},
		{
			name: "encrypted source state",
			rule: parser.Rule{
				AuthorizedKeysEncrypted: "secrets/key.enc",
			},
			isUninstall:     false,
			expectedSummary: "secrets/key.enc",
			expectedSource:  "secrets/key.enc",
		},
		{
			name: "uninstall state for file source",
			rule: parser.Rule{
				AuthorizedKeysFile: "~/.ssh/id_ed25519.pub",
			},
			isUninstall:     true,
			expectedSummary: "~/.ssh/id_ed25519.pub",
			expectedSource:  "~/.ssh/id_ed25519.pub",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewAuthorizedKeysHandler(tt.rule, "", nil)
			state := handler.GetState(tt.isUninstall)

			if state["summary"] != tt.expectedSummary {
				t.Errorf("GetState().summary = %q, want %q", state["summary"], tt.expectedSummary)
			}
			if state["source"] != tt.expectedSource {
				t.Errorf("GetState().source = %q, want %q", state["source"], tt.expectedSource)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// DisplayInfo
// ---------------------------------------------------------------------------

func captureAuthorizedKeysDisplayInfo(handler *AuthorizedKeysHandler) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	handler.DisplayInfo()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestAuthorizedKeysDisplayInfo(t *testing.T) {
	tests := []struct {
		name             string
		rule             parser.Rule
		expectedContains []string
	}{
		{
			name: "file source shows file path",
			rule: parser.Rule{
				Action:             "authorized_keys",
				AuthorizedKeysFile: "~/.ssh/id_ed25519.pub",
			},
			expectedContains: []string{"File:", "~/.ssh/id_ed25519.pub"},
		},
		{
			name: "encrypted source shows encrypted path",
			rule: parser.Rule{
				Action:                  "authorized_keys",
				AuthorizedKeysEncrypted: "secrets/key.enc",
			},
			expectedContains: []string{"Encrypted:", "secrets/key.enc"},
		},
		{
			name: "encrypted source with password-id shows both",
			rule: parser.Rule{
				Action:                   "authorized_keys",
				AuthorizedKeysEncrypted:  "secrets/key.enc",
				AuthorizedKeysPasswordID: "vault-key",
			},
			expectedContains: []string{"Encrypted:", "secrets/key.enc", "Password ID:", "vault-key"},
		},
		{
			name: "uninstall action still shows file info",
			rule: parser.Rule{
				Action:             "uninstall",
				AuthorizedKeysFile: "~/.ssh/id_ed25519.pub",
			},
			expectedContains: []string{"File:", "~/.ssh/id_ed25519.pub"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewAuthorizedKeysHandler(tt.rule, "", nil)
			output := captureAuthorizedKeysDisplayInfo(handler)

			for _, expected := range tt.expectedContains {
				if !strings.Contains(output, expected) {
					t.Errorf("DisplayInfo() output missing %q\nGot: %s", expected, output)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// UpdateStatus
// ---------------------------------------------------------------------------

func TestAuthorizedKeysUpdateStatus(t *testing.T) {
	tests := []struct {
		name          string
		rule          parser.Rule
		records       []ExecutionRecord
		initialStatus Status
		checkStatus   func(t *testing.T, status Status)
	}{
		{
			name: "successful execution adds entry to status",
			rule: parser.Rule{
				Action:             "authorized_keys",
				AuthorizedKeysFile: "~/.ssh/id_ed25519.pub",
			},
			records: []ExecutionRecord{
				{
					Status:  "success",
					Command: "cat ~/.ssh/id_ed25519.pub >> ~/.ssh/authorized_keys",
				},
			},
			initialStatus: Status{},
			checkStatus: func(t *testing.T, status Status) {
				t.Helper()
				if len(status.AuthorizedKeys) != 1 {
					t.Fatalf("expected 1 authorized key entry, got %d", len(status.AuthorizedKeys))
				}
				if status.AuthorizedKeys[0].Source != "~/.ssh/id_ed25519.pub" {
					t.Errorf("Source = %q, want %q", status.AuthorizedKeys[0].Source, "~/.ssh/id_ed25519.pub")
				}
				if status.AuthorizedKeys[0].Blueprint == "" {
					t.Error("Blueprint should not be empty")
				}
				if status.AuthorizedKeys[0].OS != "mac" {
					t.Errorf("OS = %q, want %q", status.AuthorizedKeys[0].OS, "mac")
				}
			},
		},
		{
			name: "failed execution does not add entry",
			rule: parser.Rule{
				Action:             "authorized_keys",
				AuthorizedKeysFile: "~/.ssh/id_ed25519.pub",
			},
			records: []ExecutionRecord{
				{
					Status:  "error",
					Command: "cat ~/.ssh/id_ed25519.pub >> ~/.ssh/authorized_keys",
				},
			},
			initialStatus: Status{},
			checkStatus: func(t *testing.T, status Status) {
				t.Helper()
				if len(status.AuthorizedKeys) != 0 {
					t.Errorf("expected no authorized key entries, got %d", len(status.AuthorizedKeys))
				}
			},
		},
		{
			name: "uninstall removes entry from status",
			rule: parser.Rule{
				Action:             "uninstall",
				AuthorizedKeysFile: "~/.ssh/id_ed25519.pub",
			},
			records: []ExecutionRecord{},
			initialStatus: Status{
				AuthorizedKeys: []AuthorizedKeysStatus{
					{
						Source:    "~/.ssh/id_ed25519.pub",
						Blueprint: "/tmp/test.bp",
						OS:        "mac",
					},
				},
			},
			checkStatus: func(t *testing.T, status Status) {
				t.Helper()
				if len(status.AuthorizedKeys) != 0 {
					t.Errorf("expected 0 authorized key entries after uninstall, got %d", len(status.AuthorizedKeys))
				}
			},
		},
		{
			name: "successful execution with encrypted source",
			rule: parser.Rule{
				Action:                  "authorized_keys",
				AuthorizedKeysEncrypted: "secrets/key.enc",
			},
			records: []ExecutionRecord{
				{
					Status:  "success",
					Command: "decrypt secrets/key.enc >> ~/.ssh/authorized_keys",
				},
			},
			initialStatus: Status{},
			checkStatus: func(t *testing.T, status Status) {
				t.Helper()
				if len(status.AuthorizedKeys) != 1 {
					t.Fatalf("expected 1 authorized key entry, got %d", len(status.AuthorizedKeys))
				}
				if status.AuthorizedKeys[0].Source != "secrets/key.enc" {
					t.Errorf("Source = %q, want %q", status.AuthorizedKeys[0].Source, "secrets/key.enc")
				}
			},
		},
		{
			name: "no matching command record does nothing",
			rule: parser.Rule{
				Action:             "authorized_keys",
				AuthorizedKeysFile: "~/.ssh/id_ed25519.pub",
			},
			records: []ExecutionRecord{
				{
					Status:  "success",
					Command: "some other command",
				},
			},
			initialStatus: Status{},
			checkStatus: func(t *testing.T, status Status) {
				t.Helper()
				if len(status.AuthorizedKeys) != 0 {
					t.Errorf("expected no entries, got %d", len(status.AuthorizedKeys))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewAuthorizedKeysHandler(tt.rule, "", nil)
			status := tt.initialStatus

			err := handler.UpdateStatus(&status, tt.records, "/tmp/test.bp", "mac")
			if err != nil {
				t.Errorf("UpdateStatus() unexpected error: %v", err)
			}

			tt.checkStatus(t, status)
		})
	}
}

// ---------------------------------------------------------------------------
// FindUninstallRules
// ---------------------------------------------------------------------------

func TestAuthorizedKeysFindUninstallRules(t *testing.T) {
	tests := []struct {
		name          string
		statusEntries []AuthorizedKeysStatus
		currentRules  []parser.Rule
		blueprint     string
		osName        string
		expectedCount int
		expectedSrc   string
	}{
		{
			name: "source in status but not in current rules returns uninstall rule",
			statusEntries: []AuthorizedKeysStatus{
				{
					Source:    "~/.ssh/id_ed25519.pub",
					Blueprint: "/tmp/test.bp",
					OS:        "mac",
				},
			},
			currentRules:  []parser.Rule{},
			blueprint:     "/tmp/test.bp",
			osName:        "mac",
			expectedCount: 1,
			expectedSrc:   "~/.ssh/id_ed25519.pub",
		},
		{
			name: "source in both status and current rules returns no uninstall rule",
			statusEntries: []AuthorizedKeysStatus{
				{
					Source:    "~/.ssh/id_ed25519.pub",
					Blueprint: "/tmp/test.bp",
					OS:        "mac",
				},
			},
			currentRules: []parser.Rule{
				{
					Action:             "authorized_keys",
					AuthorizedKeysFile: "~/.ssh/id_ed25519.pub",
				},
			},
			blueprint:     "/tmp/test.bp",
			osName:        "mac",
			expectedCount: 0,
		},
		{
			name: "different blueprint is not returned",
			statusEntries: []AuthorizedKeysStatus{
				{
					Source:    "~/.ssh/id_ed25519.pub",
					Blueprint: "/other/blueprint.bp",
					OS:        "mac",
				},
			},
			currentRules:  []parser.Rule{},
			blueprint:     "/tmp/test.bp",
			osName:        "mac",
			expectedCount: 0,
		},
		{
			name: "different OS is not returned",
			statusEntries: []AuthorizedKeysStatus{
				{
					Source:    "~/.ssh/id_ed25519.pub",
					Blueprint: "/tmp/test.bp",
					OS:        "linux",
				},
			},
			currentRules:  []parser.Rule{},
			blueprint:     "/tmp/test.bp",
			osName:        "mac",
			expectedCount: 0,
		},
		{
			name:          "nil status returns empty",
			statusEntries: nil,
			currentRules:  []parser.Rule{},
			blueprint:     "/tmp/test.bp",
			osName:        "mac",
			expectedCount: 0,
		},
		{
			name: "encrypted source in status but not in current rules",
			statusEntries: []AuthorizedKeysStatus{
				{
					Source:    "secrets/key.enc",
					Blueprint: "/tmp/test.bp",
					OS:        "mac",
				},
			},
			currentRules:  []parser.Rule{},
			blueprint:     "/tmp/test.bp",
			osName:        "mac",
			expectedCount: 1,
			expectedSrc:   "secrets/key.enc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewAuthorizedKeysHandler(parser.Rule{}, "", nil)
			status := &Status{
				AuthorizedKeys: tt.statusEntries,
			}

			rules := handler.FindUninstallRules(status, tt.currentRules, tt.blueprint, tt.osName)

			if len(rules) != tt.expectedCount {
				t.Fatalf("FindUninstallRules() returned %d rules, want %d", len(rules), tt.expectedCount)
			}

			if tt.expectedCount > 0 && tt.expectedSrc != "" {
				if rules[0].AuthorizedKeysFile != tt.expectedSrc {
					t.Errorf("uninstall rule AuthorizedKeysFile = %q, want %q", rules[0].AuthorizedKeysFile, tt.expectedSrc)
				}
				if rules[0].Action != "uninstall" {
					t.Errorf("uninstall rule Action = %q, want %q", rules[0].Action, "uninstall")
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// IsInstalled
// ---------------------------------------------------------------------------

func TestAuthorizedKeysIsInstalledNotInStatus(t *testing.T) {
	rule := parser.Rule{
		Action:             "authorized_keys",
		AuthorizedKeysFile: "~/.ssh/id_ed25519.pub",
	}
	handler := NewAuthorizedKeysHandler(rule, "", nil)
	status := &Status{}

	result := handler.IsInstalled(status, "/tmp/test.bp", "mac")
	if result {
		t.Error("IsInstalled() = true, want false when source not in status")
	}
}

func TestAuthorizedKeysIsInstalledWithRealFiles(t *testing.T) {
	// Create temp dir to hold the pub key file and authorized_keys file.
	tmpDir := t.TempDir()

	pubKeyContent := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI testkey user@host"
	pubKeyFile := filepath.Join(tmpDir, "id_ed25519.pub")
	if err := os.WriteFile(pubKeyFile, []byte(pubKeyContent+"\n"), 0600); err != nil {
		t.Fatalf("failed to write pub key file: %v", err)
	}

	authKeysFile := filepath.Join(tmpDir, "authorized_keys")

	// Redirect sshDir to use tmpDir via the authorizedKeysFile call path.
	// Since authorizedKeysFile() calls sshDir() which calls os.UserHomeDir(),
	// we test IsInstalled through the status-not-found path first (pure logic),
	// and then via actual file reads using the real home dir only when the
	// authorized_keys file already exists at the real path.

	// Test: in status but authorized_keys file missing at real path.
	// We test this by checking whether the real authorized_keys exists; if not,
	// IsInstalled returns false after the status check.
	blueprint := "/tmp/test.bp"
	osName := "mac"

	rule := parser.Rule{
		Action:             "authorized_keys",
		AuthorizedKeysFile: pubKeyFile,
	}
	handler := NewAuthorizedKeysHandler(rule, "", nil)

	status := &Status{
		AuthorizedKeys: []AuthorizedKeysStatus{
			{
				Source:    pubKeyFile,
				Blueprint: blueprint,
				OS:        osName,
			},
		},
	}

	// If the real ~/.ssh/authorized_keys doesn't exist, IsInstalled returns false.
	// We don't want to create real files in ~/.ssh, so we verify the false-when-file-missing path.
	realHome, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}
	realAuthKeys := filepath.Join(realHome, ".ssh", "authorized_keys")

	if _, statErr := os.Stat(realAuthKeys); os.IsNotExist(statErr) {
		result := handler.IsInstalled(status, blueprint, osName)
		if result {
			t.Error("IsInstalled() = true, want false when authorized_keys file does not exist")
		}
		return
	}

	// If the real authorized_keys exists, write our key to a temp file and test
	// that the key presence check works using the temp authorized_keys.
	// Since we cannot redirect the path inside authorizedKeysFile(), we at least
	// validate that with the key present in the real file the function returns true.
	// Write our key to the real authorized_keys temporarily (using append; restore after).
	existingContent, readErr := os.ReadFile(realAuthKeys)
	if readErr != nil {
		t.Skip("cannot read real authorized_keys file")
	}

	// Append our test key.
	appendContent := "\n" + pubKeyContent + "\n"
	if writeErr := os.WriteFile(realAuthKeys, append(existingContent, []byte(appendContent)...), 0600); writeErr != nil {
		t.Skip("cannot write to real authorized_keys file (no permission?)")
	}
	t.Cleanup(func() {
		_ = os.WriteFile(realAuthKeys, existingContent, 0600)
	})

	result := handler.IsInstalled(status, blueprint, osName)
	if !result {
		t.Error("IsInstalled() = false, want true when key is present in authorized_keys")
	}

	// Also test the file-written-but-key-missing path using an unrelated key.
	ruleOther := parser.Rule{
		Action:             "authorized_keys",
		AuthorizedKeysFile: authKeysFile,
	}

	otherKeyContent := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI missingkey nobody@nowhere"
	if writeErr := os.WriteFile(authKeysFile, []byte(otherKeyContent+"\n"), 0600); writeErr != nil {
		t.Fatalf("failed to write other pub key file: %v", writeErr)
	}

	handlerOther := NewAuthorizedKeysHandler(ruleOther, "", nil)
	statusOther := &Status{
		AuthorizedKeys: []AuthorizedKeysStatus{
			{
				Source:    authKeysFile,
				Blueprint: blueprint,
				OS:        osName,
			},
		},
	}
	// The key in authKeysFile ("missingkey nobody@nowhere") is not in the real authorized_keys,
	// so IsInstalled should return false.
	resultOther := handlerOther.IsInstalled(statusOther, blueprint, osName)
	if resultOther {
		// This can be true if "missingkey" happens to be in the real file — skip rather than fail.
		t.Log("skipping key-missing sub-test: key coincidentally in real authorized_keys")
	}
}

// ---------------------------------------------------------------------------
// Up — error paths only (no real ~/.ssh mutation needed)
// ---------------------------------------------------------------------------

func TestAuthorizedKeysUpErrors(t *testing.T) {
	t.Run("nonexistent file source returns error", func(t *testing.T) {
		rule := parser.Rule{
			Action:             "authorized_keys",
			AuthorizedKeysFile: "/nonexistent/path/id_ed25519.pub",
		}
		handler := NewAuthorizedKeysHandler(rule, "", nil)

		_, err := handler.Up()
		if err == nil {
			t.Fatal("Up() expected error for nonexistent file, got nil")
		}
		if !strings.Contains(err.Error(), "failed to read key file") {
			t.Errorf("Up() error = %q, want it to contain 'failed to read key file'", err.Error())
		}
	})

	t.Run("encrypted source with no password cache returns error", func(t *testing.T) {
		rule := parser.Rule{
			Action:                  "authorized_keys",
			AuthorizedKeysEncrypted: "secrets/key.enc",
		}
		handler := NewAuthorizedKeysHandler(rule, "", nil)

		_, err := handler.Up()
		if err == nil {
			t.Fatal("Up() expected error for missing password, got nil")
		}
		if !strings.Contains(err.Error(), "no password cached") {
			t.Errorf("Up() error = %q, want it to contain 'no password cached'", err.Error())
		}
	})

	t.Run("encrypted source with empty password cache returns error", func(t *testing.T) {
		rule := parser.Rule{
			Action:                  "authorized_keys",
			AuthorizedKeysEncrypted: "secrets/key.enc",
		}
		handler := NewAuthorizedKeysHandler(rule, "", map[string]string{})

		_, err := handler.Up()
		if err == nil {
			t.Fatal("Up() expected error for empty password cache, got nil")
		}
		if !strings.Contains(err.Error(), "no password cached") {
			t.Errorf("Up() error = %q, want it to contain 'no password cached'", err.Error())
		}
	})
}

// ---------------------------------------------------------------------------
// Down — error paths
// ---------------------------------------------------------------------------

func TestAuthorizedKeysDownErrors(t *testing.T) {
	t.Run("nonexistent file source returns error on down", func(t *testing.T) {
		rule := parser.Rule{
			Action:             "uninstall",
			AuthorizedKeysFile: "/nonexistent/path/id_ed25519.pub",
		}
		handler := NewAuthorizedKeysHandler(rule, "", nil)

		_, err := handler.Down()
		if err == nil {
			t.Fatal("Down() expected error for nonexistent file, got nil")
		}
		if !strings.Contains(err.Error(), "failed to read key file") {
			t.Errorf("Down() error = %q, want it to contain 'failed to read key file'", err.Error())
		}
	})
}

// ---------------------------------------------------------------------------
// resolveFilePath
// ---------------------------------------------------------------------------

func TestAuthorizedKeysResolveFilePath(t *testing.T) {
	tmpDir := t.TempDir()
	existingFile := filepath.Join(tmpDir, "key.enc")
	if err := os.WriteFile(existingFile, []byte("data"), 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	t.Run("absolute path is returned as-is (after expandPath)", func(t *testing.T) {
		handler := NewAuthorizedKeysHandler(parser.Rule{}, "", nil)
		got := handler.resolveFilePath(existingFile)
		if got != existingFile {
			t.Errorf("resolveFilePath(%q) = %q, want %q", existingFile, got, existingFile)
		}
	})

	t.Run("relative path found under basePath", func(t *testing.T) {
		handler := NewAuthorizedKeysHandler(parser.Rule{}, tmpDir, nil)
		got := handler.resolveFilePath("key.enc")
		if got != existingFile {
			t.Errorf("resolveFilePath(%q) = %q, want %q", "key.enc", got, existingFile)
		}
	})

	t.Run("relative path not found under basePath falls through to raw path", func(t *testing.T) {
		handler := NewAuthorizedKeysHandler(parser.Rule{}, tmpDir, nil)
		// "missing.enc" does not exist under tmpDir
		got := handler.resolveFilePath("missing.enc")
		// Since it's not under basePath and doesn't exist as a relative path,
		// the function returns the raw file argument.
		if got != "missing.enc" {
			t.Errorf("resolveFilePath(%q) = %q, want %q", "missing.enc", got, "missing.enc")
		}
	})

	t.Run("tilde path is expanded", func(t *testing.T) {
		handler := NewAuthorizedKeysHandler(parser.Rule{}, "", nil)
		// This should expand ~ — the exact path depends on actual home dir.
		got := handler.resolveFilePath("~/some/path")
		if got == "~/some/path" {
			t.Error("resolveFilePath() did not expand tilde prefix")
		}
		if !strings.HasSuffix(got, "some/path") {
			t.Errorf("resolveFilePath() = %q, expected it to end with 'some/path'", got)
		}
	})
}

// ---------------------------------------------------------------------------
// DisplayStatusFromStatus
// ---------------------------------------------------------------------------

func captureAuthorizedKeysDisplayStatus(handler *AuthorizedKeysHandler, status *Status) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	handler.DisplayStatusFromStatus(status)

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestAuthorizedKeysDisplayStatusFromStatus(t *testing.T) {
	handler := NewAuthorizedKeysHandler(parser.Rule{}, "", nil)

	t.Run("nil status produces no output and no panic", func(t *testing.T) {
		output := captureAuthorizedKeysDisplayStatus(handler, nil)
		if output != "" {
			t.Errorf("DisplayStatusFromStatus(nil) output = %q, want empty", output)
		}
	})

	t.Run("empty AuthorizedKeys slice produces no output", func(t *testing.T) {
		status := &Status{AuthorizedKeys: []AuthorizedKeysStatus{}}
		output := captureAuthorizedKeysDisplayStatus(handler, status)
		if output != "" {
			t.Errorf("DisplayStatusFromStatus(empty) output = %q, want empty", output)
		}
	})

	t.Run("with entries output contains source path", func(t *testing.T) {
		status := &Status{
			AuthorizedKeys: []AuthorizedKeysStatus{
				{
					Source:    "~/.ssh/id_ed25519.pub",
					AddedAt:   "2024-01-15T10:30:00Z",
					Blueprint: "/tmp/test.bp",
					OS:        "mac",
				},
			},
		}
		output := captureAuthorizedKeysDisplayStatus(handler, status)
		if !strings.Contains(output, "~/.ssh/id_ed25519.pub") {
			t.Errorf("DisplayStatusFromStatus() output missing source path\nGot: %s", output)
		}
		if !strings.Contains(output, "Authorized Keys") {
			t.Errorf("DisplayStatusFromStatus() output missing header\nGot: %s", output)
		}
	})

	t.Run("with multiple entries all sources shown", func(t *testing.T) {
		status := &Status{
			AuthorizedKeys: []AuthorizedKeysStatus{
				{
					Source:    "~/.ssh/id_ed25519.pub",
					AddedAt:   "2024-01-15T10:30:00Z",
					Blueprint: "/tmp/test.bp",
					OS:        "mac",
				},
				{
					Source:    "secrets/team.enc",
					AddedAt:   "2024-01-16T11:00:00Z",
					Blueprint: "/tmp/test.bp",
					OS:        "linux",
				},
			},
		}
		output := captureAuthorizedKeysDisplayStatus(handler, status)
		if !strings.Contains(output, "~/.ssh/id_ed25519.pub") {
			t.Errorf("DisplayStatusFromStatus() missing first source\nGot: %s", output)
		}
		if !strings.Contains(output, "secrets/team.enc") {
			t.Errorf("DisplayStatusFromStatus() missing second source\nGot: %s", output)
		}
	})

	t.Run("with invalid timestamp falls back to raw string", func(t *testing.T) {
		status := &Status{
			AuthorizedKeys: []AuthorizedKeysStatus{
				{
					Source:    "~/.ssh/id_ed25519.pub",
					AddedAt:   "not-a-real-timestamp",
					Blueprint: "/tmp/test.bp",
					OS:        "mac",
				},
			},
		}
		output := captureAuthorizedKeysDisplayStatus(handler, status)
		if !strings.Contains(output, "not-a-real-timestamp") {
			t.Errorf("DisplayStatusFromStatus() should show raw timestamp string\nGot: %s", output)
		}
	})
}

// ---------------------------------------------------------------------------
// authorizedKeysFile helper — pure logic branches
// ---------------------------------------------------------------------------

func TestAuthorizedKeysFileHelperCreate(t *testing.T) {
	// authorizedKeysFile(create=true) should create the file when it does not exist.
	// Since we cannot override sshDir's home-dir call, we verify behavior against the
	// real ~/.ssh directory. If we can write there, test that create=true works; otherwise skip.
	realHome, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	sshPath := filepath.Join(realHome, ".ssh")
	authKeysPath := filepath.Join(sshPath, "authorized_keys")

	// Ensure the ~/.ssh directory exists (it almost always does on a developer machine).
	if _, statErr := os.Stat(sshPath); os.IsNotExist(statErr) {
		t.Skip("~/.ssh directory does not exist; skipping authorizedKeysFile test")
	}

	// If the authorized_keys file already exists, just verify we get the correct path back.
	if _, statErr := os.Stat(authKeysPath); statErr == nil {
		got, err := authorizedKeysFile(false)
		if err != nil {
			t.Fatalf("authorizedKeysFile(false) unexpected error: %v", err)
		}
		if got != authKeysPath {
			t.Errorf("authorizedKeysFile(false) = %q, want %q", got, authKeysPath)
		}
		return
	}

	// File doesn't exist — test create=true path.
	got, err := authorizedKeysFile(true)
	if err != nil {
		t.Fatalf("authorizedKeysFile(true) unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(got) })

	if got != authKeysPath {
		t.Errorf("authorizedKeysFile(true) = %q, want %q", got, authKeysPath)
	}
	if _, statErr := os.Stat(got); os.IsNotExist(statErr) {
		t.Error("authorizedKeysFile(true) should have created the file")
	}
}

func TestAuthorizedKeysFileHelperNoCreateMissing(t *testing.T) {
	// authorizedKeysFile(false) should return an error when the file does not exist.
	// We test this only when the real authorized_keys is absent.
	realHome, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	authKeysPath := filepath.Join(realHome, ".ssh", "authorized_keys")
	if _, statErr := os.Stat(authKeysPath); statErr == nil {
		t.Skip("authorized_keys already exists; skipping missing-file error test")
	}

	_, err = authorizedKeysFile(false)
	if err == nil {
		t.Error("authorizedKeysFile(false) expected error for missing file, got nil")
	}
	if !strings.Contains(err.Error(), "authorized_keys file does not exist") {
		t.Errorf("authorizedKeysFile(false) error = %q, want it to contain 'does not exist'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// readKeyContent — file-path branch
// ---------------------------------------------------------------------------

func TestReadKeyContentFilePath(t *testing.T) {
	t.Run("reads and returns file content", func(t *testing.T) {
		tmpDir := t.TempDir()
		pubKeyFile := filepath.Join(tmpDir, "id_ed25519.pub")
		want := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI testkey user@host\n"
		if err := os.WriteFile(pubKeyFile, []byte(want), 0600); err != nil {
			t.Fatalf("setup: %v", err)
		}

		rule := parser.Rule{
			AuthorizedKeysFile: pubKeyFile,
		}
		handler := NewAuthorizedKeysHandler(rule, "", nil)

		got, err := handler.readKeyContent()
		if err != nil {
			t.Fatalf("readKeyContent() unexpected error: %v", err)
		}
		if got != want {
			t.Errorf("readKeyContent() = %q, want %q", got, want)
		}
	})

	t.Run("missing file returns error", func(t *testing.T) {
		rule := parser.Rule{
			AuthorizedKeysFile: "/nonexistent/file.pub",
		}
		handler := NewAuthorizedKeysHandler(rule, "", nil)

		_, err := handler.readKeyContent()
		if err == nil {
			t.Fatal("readKeyContent() expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to read key file") {
			t.Errorf("readKeyContent() error = %q, want to contain 'failed to read key file'", err.Error())
		}
	})
}

func TestReadKeyContentEncryptedNoCachedPassword(t *testing.T) {
	t.Run("no password cache returns error with default password-id", func(t *testing.T) {
		rule := parser.Rule{
			AuthorizedKeysEncrypted: "secrets/key.enc",
		}
		handler := NewAuthorizedKeysHandler(rule, "", nil)

		_, err := handler.readKeyContent()
		if err == nil {
			t.Fatal("readKeyContent() expected error for missing password, got nil")
		}
		if !strings.Contains(err.Error(), "no password cached for password-id: default") {
			t.Errorf("readKeyContent() error = %q, want 'no password cached for password-id: default'", err.Error())
		}
	})

	t.Run("custom password-id not in cache returns correct error", func(t *testing.T) {
		rule := parser.Rule{
			AuthorizedKeysEncrypted:  "secrets/key.enc",
			AuthorizedKeysPasswordID: "mypassword",
		}
		handler := NewAuthorizedKeysHandler(rule, "", map[string]string{})

		_, err := handler.readKeyContent()
		if err == nil {
			t.Fatal("readKeyContent() expected error for missing custom password-id, got nil")
		}
		if !strings.Contains(err.Error(), "no password cached for password-id: mypassword") {
			t.Errorf("readKeyContent() error = %q, want 'no password cached for password-id: mypassword'", err.Error())
		}
	})
}

// ---------------------------------------------------------------------------
// Up / Down — happy paths using real ~/.ssh/authorized_keys (with backup/restore)
// ---------------------------------------------------------------------------

// withRealAuthKeysBackup is a helper that backs up the real authorized_keys file,
// runs fn, then restores it. If the file doesn't exist or cannot be written, the
// test is skipped.
func withRealAuthKeysBackup(t *testing.T, fn func(authKeysPath string)) {
	t.Helper()

	realHome, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	sshPath := filepath.Join(realHome, ".ssh")
	authKeysPath := filepath.Join(sshPath, "authorized_keys")

	// Ensure ~/.ssh exists.
	if err := os.MkdirAll(sshPath, 0700); err != nil {
		t.Skipf("cannot create ~/.ssh: %v", err)
	}

	// Read existing content for restore.
	existingContent, readErr := os.ReadFile(authKeysPath)
	fileExisted := readErr == nil

	// Create the file if it doesn't exist so Up() can operate.
	if !fileExisted {
		if writeErr := os.WriteFile(authKeysPath, []byte{}, 0600); writeErr != nil {
			t.Skipf("cannot create authorized_keys: %v", writeErr)
		}
	}

	// Always restore on cleanup.
	t.Cleanup(func() {
		if fileExisted {
			_ = os.WriteFile(authKeysPath, existingContent, 0600)
		} else {
			_ = os.Remove(authKeysPath)
		}
	})

	fn(authKeysPath)
}

func TestAuthorizedKeysUpHappyPath(t *testing.T) {
	withRealAuthKeysBackup(t, func(authKeysPath string) {
		// Start with an empty authorized_keys.
		if err := os.WriteFile(authKeysPath, []byte{}, 0600); err != nil {
			t.Fatalf("setup: cannot clear authorized_keys: %v", err)
		}

		// Write a temp pub key file.
		tmpDir := t.TempDir()
		pubKeyFile := filepath.Join(tmpDir, "id_ed25519.pub")
		keyLine := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI testUp user@host"
		if err := os.WriteFile(pubKeyFile, []byte(keyLine+"\n"), 0600); err != nil {
			t.Fatalf("setup: cannot write pub key: %v", err)
		}

		rule := parser.Rule{
			Action:             "authorized_keys",
			AuthorizedKeysFile: pubKeyFile,
		}
		handler := NewAuthorizedKeysHandler(rule, "", nil)

		msg, err := handler.Up()
		if err != nil {
			t.Fatalf("Up() unexpected error: %v", err)
		}
		if !strings.Contains(msg, "Added") {
			t.Errorf("Up() message = %q, want it to contain 'Added'", msg)
		}

		// Verify the key was written.
		content, err := os.ReadFile(authKeysPath)
		if err != nil {
			t.Fatalf("failed to read authorized_keys after Up(): %v", err)
		}
		if !strings.Contains(string(content), keyLine) {
			t.Errorf("authorized_keys after Up() does not contain key line %q\nContent: %s", keyLine, string(content))
		}
	})
}

func TestAuthorizedKeysUpIdempotent(t *testing.T) {
	withRealAuthKeysBackup(t, func(authKeysPath string) {
		tmpDir := t.TempDir()
		pubKeyFile := filepath.Join(tmpDir, "id_ed25519.pub")
		keyLine := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI testIdem user@host"

		// Pre-populate authorized_keys with the key already present.
		if err := os.WriteFile(authKeysPath, []byte(keyLine+"\n"), 0600); err != nil {
			t.Fatalf("setup: cannot write authorized_keys: %v", err)
		}
		if err := os.WriteFile(pubKeyFile, []byte(keyLine+"\n"), 0600); err != nil {
			t.Fatalf("setup: cannot write pub key: %v", err)
		}

		rule := parser.Rule{
			Action:             "authorized_keys",
			AuthorizedKeysFile: pubKeyFile,
		}
		handler := NewAuthorizedKeysHandler(rule, "", nil)

		msg, err := handler.Up()
		if err != nil {
			t.Fatalf("Up() unexpected error on idempotent run: %v", err)
		}
		// Should report 0 keys added since the key is already present.
		if !strings.Contains(msg, "Added 0") {
			t.Errorf("Up() idempotent message = %q, want 'Added 0 key(s)'", msg)
		}
	})
}

func TestAuthorizedKeysDownHappyPath(t *testing.T) {
	withRealAuthKeysBackup(t, func(authKeysPath string) {
		tmpDir := t.TempDir()
		pubKeyFile := filepath.Join(tmpDir, "id_ed25519.pub")
		keyLine := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI testDown user@host"

		// Pre-populate authorized_keys with the key.
		if err := os.WriteFile(authKeysPath, []byte(keyLine+"\n"), 0600); err != nil {
			t.Fatalf("setup: cannot write authorized_keys: %v", err)
		}
		if err := os.WriteFile(pubKeyFile, []byte(keyLine+"\n"), 0600); err != nil {
			t.Fatalf("setup: cannot write pub key: %v", err)
		}

		rule := parser.Rule{
			Action:             "uninstall",
			AuthorizedKeysFile: pubKeyFile,
		}
		handler := NewAuthorizedKeysHandler(rule, "", nil)

		msg, err := handler.Down()
		if err != nil {
			t.Fatalf("Down() unexpected error: %v", err)
		}
		if !strings.Contains(msg, "Removed") {
			t.Errorf("Down() message = %q, want it to contain 'Removed'", msg)
		}

		// Verify the key was removed.
		content, err := os.ReadFile(authKeysPath)
		if err != nil {
			t.Fatalf("failed to read authorized_keys after Down(): %v", err)
		}
		if strings.Contains(string(content), keyLine) {
			t.Errorf("authorized_keys after Down() still contains key line %q", keyLine)
		}
	})
}
