package handlers

import (
	"errors"
	"strings"
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

// ---- #7: known_hosts Down() returns success even when the command fails ----
//
// Down() runs a sed command and ignores its error, always returning nil.
// This leaves status out of sync when the removal actually fails.
// We test this by injecting a command runner that always fails.

func TestKnownHostsDownReturnsErrorOnCommandFailure(t *testing.T) {
	h := NewKnownHostsHandler(parser.Rule{
		Action:     "known_hosts",
		KnownHosts: "github.com",
	}, "")

	_, err := h.DownWithRunner(func(cmd string) error {
		return errors.New("sed: command not found")
	})

	if err == nil {
		t.Error("BUG: Down() should return an error when the removal command fails, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "could not remove") {
		t.Errorf("error message should mention removal failure, got: %v", err)
	}
}
