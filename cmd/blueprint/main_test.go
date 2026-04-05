package main

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// parseNonNegativeInt
// ---------------------------------------------------------------------------

func TestParseNonNegativeInt_ValidZero(t *testing.T) {
	n, ok := parseNonNegativeInt("0", "run_number")
	if !ok {
		t.Fatal("expected ok=true for \"0\"")
	}
	if n != 0 {
		t.Fatalf("expected 0, got %d", n)
	}
}

func TestParseNonNegativeInt_ValidPositive(t *testing.T) {
	n, ok := parseNonNegativeInt("42", "run_number")
	if !ok {
		t.Fatal("expected ok=true for \"42\"")
	}
	if n != 42 {
		t.Fatalf("expected 42, got %d", n)
	}
}

func TestParseNonNegativeInt_NegativeRejected(t *testing.T) {
	_, ok := parseNonNegativeInt("-1", "run_number")
	if ok {
		t.Fatal("expected ok=false for \"-1\"")
	}
}

func TestParseNonNegativeInt_AlphaRejected(t *testing.T) {
	_, ok := parseNonNegativeInt("abc", "run_number")
	if ok {
		t.Fatal("expected ok=false for \"abc\"")
	}
}

func TestParseNonNegativeInt_EmptyRejected(t *testing.T) {
	_, ok := parseNonNegativeInt("", "run_number")
	if ok {
		t.Fatal("expected ok=false for empty string")
	}
}

func TestParseNonNegativeInt_FloatRejected(t *testing.T) {
	_, ok := parseNonNegativeInt("3.14", "run_number")
	if ok {
		t.Fatal("expected ok=false for \"3.14\"")
	}
}

func TestParseNonNegativeInt_StepNumberZeroAllowed(t *testing.T) {
	// step_number == 0 is a valid explicit step index
	n, ok := parseNonNegativeInt("0", "step_number")
	if !ok {
		t.Fatal("expected ok=true for step_number=0")
	}
	if n != 0 {
		t.Fatalf("expected 0, got %d", n)
	}
}

// ---------------------------------------------------------------------------
// parsePositiveInt
// ---------------------------------------------------------------------------

func TestParsePositiveInt_ValidOne(t *testing.T) {
	n, ok := parsePositiveInt("1", "--top")
	if !ok {
		t.Fatal("expected ok=true for \"1\"")
	}
	if n != 1 {
		t.Fatalf("expected 1, got %d", n)
	}
}

func TestParsePositiveInt_ValidLarge(t *testing.T) {
	n, ok := parsePositiveInt("100", "--top")
	if !ok {
		t.Fatal("expected ok=true for \"100\"")
	}
	if n != 100 {
		t.Fatalf("expected 100, got %d", n)
	}
}

func TestParsePositiveInt_ZeroRejected(t *testing.T) {
	_, ok := parsePositiveInt("0", "--top")
	if ok {
		t.Fatal("expected ok=false for \"0\"")
	}
}

func TestParsePositiveInt_NegativeRejected(t *testing.T) {
	_, ok := parsePositiveInt("-5", "--top")
	if ok {
		t.Fatal("expected ok=false for \"-5\"")
	}
}

func TestParsePositiveInt_AlphaRejected(t *testing.T) {
	_, ok := parsePositiveInt("abc", "--top")
	if ok {
		t.Fatal("expected ok=false for \"abc\"")
	}
}

func TestParsePositiveInt_EmptyRejected(t *testing.T) {
	_, ok := parsePositiveInt("", "--top")
	if ok {
		t.Fatal("expected ok=false for empty string")
	}
}

// ---------------------------------------------------------------------------
// parseFlags
// ---------------------------------------------------------------------------

func TestParseFlags_Empty(t *testing.T) {
	skipGroup, skipID, onlyID, skipDecrypt, preferSSH, noStatus := parseFlags([]string{})
	if skipGroup != "" || skipID != "" || onlyID != "" || skipDecrypt || preferSSH || noStatus {
		t.Fatal("expected all zero values for empty args")
	}
}

func TestParseFlags_SkipGroup(t *testing.T) {
	skipGroup, _, _, _, _, _ := parseFlags([]string{"--skip-group", "mygroup"})
	if skipGroup != "mygroup" {
		t.Fatalf("expected skipGroup=%q, got %q", "mygroup", skipGroup)
	}
}

func TestParseFlags_SkipID(t *testing.T) {
	_, skipID, _, _, _, _ := parseFlags([]string{"--skip-id", "myid"})
	if skipID != "myid" {
		t.Fatalf("expected skipID=%q, got %q", "myid", skipID)
	}
}

func TestParseFlags_Only(t *testing.T) {
	_, _, onlyID, _, _, _ := parseFlags([]string{"--only", "step-42"})
	if onlyID != "step-42" {
		t.Fatalf("expected onlyID=%q, got %q", "step-42", onlyID)
	}
}

func TestParseFlags_SkipDecrypt(t *testing.T) {
	_, _, _, skipDecrypt, _, _ := parseFlags([]string{"--skip-decrypt"})
	if !skipDecrypt {
		t.Fatal("expected skipDecrypt=true")
	}
}

func TestParseFlags_PreferSSH(t *testing.T) {
	_, _, _, _, preferSSH, _ := parseFlags([]string{"--prefer-ssh"})
	if !preferSSH {
		t.Fatal("expected preferSSH=true")
	}
}

func TestParseFlags_Combined(t *testing.T) {
	skipGroup, skipID, onlyID, skipDecrypt, preferSSH, noStatus := parseFlags([]string{
		"--skip-group", "grp",
		"--skip-id", "sid",
		"--only", "oid",
		"--skip-decrypt",
		"--prefer-ssh",
		"--no-status",
	})
	if skipGroup != "grp" {
		t.Errorf("skipGroup: want %q got %q", "grp", skipGroup)
	}
	if skipID != "sid" {
		t.Errorf("skipID: want %q got %q", "sid", skipID)
	}
	if onlyID != "oid" {
		t.Errorf("onlyID: want %q got %q", "oid", onlyID)
	}
	if !skipDecrypt {
		t.Error("expected skipDecrypt=true")
	}
	if !preferSSH {
		t.Error("expected preferSSH=true")
	}
	if !noStatus {
		t.Error("expected noStatus=true")
	}
}

func TestParseFlags_NoStatus(t *testing.T) {
	_, _, _, _, _, noStatus := parseFlags([]string{"--no-status"})
	if !noStatus {
		t.Fatal("expected noStatus=true")
	}
}

func TestParseFlags_MissingValueIgnored(t *testing.T) {
	// Flag present but no following argument — should not panic, value stays empty.
	skipGroup, _, _, _, _, _ := parseFlags([]string{"--skip-group"})
	if skipGroup != "" {
		t.Fatalf("expected skipGroup=%q when value is missing, got %q", "", skipGroup)
	}
}

// ---------------------------------------------------------------------------
// isKnownCommand / unknownCommandMessage
// ---------------------------------------------------------------------------

func TestIsKnownCommand_AllKnown(t *testing.T) {
	known := []string{"plan", "apply", "encrypt", "status", "history", "ps", "slow", "diff", "version", "doctor", "validate"}
	for _, cmd := range known {
		if !isKnownCommand(cmd) {
			t.Errorf("%q should be a known command", cmd)
		}
	}
}

func TestIsKnownCommand_Unknown(t *testing.T) {
	if isKnownCommand("bogus") {
		t.Error("\"bogus\" should not be a known command")
	}
}

func TestUnknownCommandMessage_ContainsCommand(t *testing.T) {
	msg := unknownCommandMessage("foobar")
	if !strings.Contains(msg, "foobar") {
		t.Errorf("message should contain the unknown command name, got: %s", msg)
	}
}

func TestUnknownCommandMessage_ContainsUsage(t *testing.T) {
	msg := unknownCommandMessage("x")
	if !strings.Contains(msg, "Usage:") {
		t.Errorf("message should contain usage hint, got: %s", msg)
	}
}
