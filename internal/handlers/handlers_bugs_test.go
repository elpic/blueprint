package handlers

import "testing"

// ---- #4: commandSuccessfullyExecuted returns correct record ----------------
//
// Note: Go 1.22+ fixed loop-variable capture so &record in a range loop is
// now safe. This test documents the expected contract and guards against
// any future regression (e.g. if the loop is rewritten without the fix).

func TestCommandSuccessfullyExecutedReturnsCorrectRecord(t *testing.T) {
	records := []ExecutionRecord{
		{Command: "cmd-a", Status: "success", Output: "output-a"},
		{Command: "cmd-b", Status: "success", Output: "output-b"},
		{Command: "cmd-c", Status: "success", Output: "output-c"},
	}

	// Match the FIRST record, not the last — any capture bug would return "output-c".
	record, ok := commandSuccessfullyExecuted("cmd-a", records)
	if !ok {
		t.Fatal("commandSuccessfullyExecuted() returned false for a present command")
	}
	if record == nil {
		t.Fatal("commandSuccessfullyExecuted() returned nil record")
	}
	if record.Output != "output-a" {
		t.Errorf("returned wrong record: Output=%q, want %q", record.Output, "output-a")
	}
}

func TestCommandSuccessfullyExecutedNotFound(t *testing.T) {
	records := []ExecutionRecord{
		{Command: "cmd-a", Status: "success", Output: "output-a"},
		{Command: "cmd-b", Status: "error", Output: "output-b"},
	}

	_, ok := commandSuccessfullyExecuted("cmd-b", records)
	if ok {
		t.Error("commandSuccessfullyExecuted() should return false for a failed command")
	}

	_, ok = commandSuccessfullyExecuted("missing", records)
	if ok {
		t.Error("commandSuccessfullyExecuted() should return false for an absent command")
	}
}
