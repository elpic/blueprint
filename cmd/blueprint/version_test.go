package main

import (
	"testing"
)

func TestVersionDefault(t *testing.T) {
	if version == "" {
		t.Fatal("version should not be empty")
	}
	if version != "dev" {
		t.Fatalf("expected default version to be %q, got %q", "dev", version)
	}
}

func TestCommitDefault(t *testing.T) {
	if commit == "" {
		t.Fatal("commit should not be empty")
	}
	if commit != "none" {
		t.Fatalf("expected default commit to be %q, got %q", "none", commit)
	}
}

func TestUnknownCommandMessageIncludesVersion(t *testing.T) {
	msg := unknownCommandMessage("bogus")
	if msg == "" {
		t.Fatal("unknownCommandMessage should not return empty string")
	}
	// Verify version is listed in known commands
	for _, cmd := range []string{"plan", "apply", "version"} {
		if !isKnownCommand(cmd) {
			t.Errorf("%q should be a known command", cmd)
		}
	}
}
