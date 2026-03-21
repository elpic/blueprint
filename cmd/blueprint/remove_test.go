package main

import (
	"testing"
)

func TestParseFlagsAutoConfirm(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantConfirm bool
		wantSkipGrp string
		wantSkipID  string
	}{
		{
			name:        "no flags",
			args:        []string{},
			wantConfirm: false,
		},
		{
			name:        "--yes flag",
			args:        []string{"--yes"},
			wantConfirm: true,
		},
		{
			name:        "-y flag",
			args:        []string{"-y"},
			wantConfirm: true,
		},
		{
			name:        "--yes with skip flags",
			args:        []string{"--skip-group", "dev", "--skip-id", "foo", "--yes"},
			wantConfirm: true,
			wantSkipGrp: "dev",
			wantSkipID:  "foo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skipGroup, skipID, _, _, autoConfirm := parseFlags(tt.args)
			if autoConfirm != tt.wantConfirm {
				t.Errorf("autoConfirm = %v, want %v", autoConfirm, tt.wantConfirm)
			}
			if skipGroup != tt.wantSkipGrp {
				t.Errorf("skipGroup = %q, want %q", skipGroup, tt.wantSkipGrp)
			}
			if skipID != tt.wantSkipID {
				t.Errorf("skipID = %q, want %q", skipID, tt.wantSkipID)
			}
		})
	}
}

func TestRemoveIsKnownCommand(t *testing.T) {
	if !isKnownCommand("remove") {
		t.Error("remove should be a known command")
	}
}

func TestUnknownCommandMessageIncludesRemove(t *testing.T) {
	msg := unknownCommandMessage("bogus")
	if msg == "" {
		t.Fatal("unknownCommandMessage should not return empty string")
	}
	// Check that "remove" appears in the usage message
	if !containsSubstring(msg, "remove") {
		t.Error("unknownCommandMessage should include 'remove' in usage")
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
