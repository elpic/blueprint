package handlers

import (
	"strings"
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

// ---- cronExpression -------------------------------------------------------

func TestCronExpression(t *testing.T) {
	tests := []struct {
		preset string
		cron   string
		want   string
	}{
		{preset: "daily", want: "@daily"},
		{preset: "weekly", want: "@weekly"},
		{preset: "hourly", want: "@hourly"},
		{preset: "", cron: "0 9 * * 1-5", want: "0 9 * * 1-5"},
		{preset: "unknown", cron: "0 9 * * 1-5", want: "0 9 * * 1-5"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			h := NewScheduleHandler(parser.Rule{
				SchedulePreset: tt.preset,
				ScheduleCron:   tt.cron,
			}, "")
			if got := h.cronExpression(); got != tt.want {
				t.Errorf("cronExpression() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---- Bug: cronLine does not quote source paths with spaces ----------------
//
// A ScheduleSource like "~/my blueprints/setup.bp" produces a broken crontab
// entry because the space is not quoted, so cron passes two arguments to
// blueprint instead of one path.

func TestCronLineQuotesSourcePathWithSpaces(t *testing.T) {
	h := NewScheduleHandler(parser.Rule{
		SchedulePreset: "daily",
		ScheduleSource: "~/my blueprints/setup.bp",
	}, "")

	line := h.cronLine()

	if !strings.Contains(line, `"~/my blueprints/setup.bp"`) {
		t.Errorf("cronLine() does not quote source path with spaces\n  got:  %s\n  want: source path wrapped in double-quotes", line)
	}
}

func TestCronLineSimplePath(t *testing.T) {
	h := NewScheduleHandler(parser.Rule{
		SchedulePreset: "daily",
		ScheduleSource: "~/setup.bp",
	}, "")

	line := h.cronLine()

	if !strings.Contains(line, "~/setup.bp") {
		t.Errorf("cronLine() missing source path: %s", line)
	}
	if !strings.Contains(line, "@daily") {
		t.Errorf("cronLine() missing cron expression: %s", line)
	}
	if !strings.Contains(line, "--skip-decrypt") {
		t.Errorf("cronLine() missing --skip-decrypt flag: %s", line)
	}
}

// ---- Bug: UpWithStatus checks live sudo instead of Status.Sudoers ---------
//
// Even with a sudoers entry in Status, UpWithStatus still calls
// isUserInPasswordlessSudoers() which probes live "sudo -n true".
// On a first run, the sudoers file was just written but the process hasn't
// reloaded credentials, so this probe fails.
//
// Expected: when Status.Sudoers is non-empty, Up should succeed without any
// live sudo check.
//
// Expected: when Status.Sudoers is empty, Up should fail with a clear error.

func TestUpRequiresSudoersStatusEntry(t *testing.T) {
	h := NewScheduleHandler(parser.Rule{
		SchedulePreset: "daily",
		ScheduleSource: "~/setup.bp",
	}, "")

	_, err := h.UpWithStatus(&Status{}, nil, fakeReadCrontab(""), fakeWriteCrontab())
	if err == nil {
		t.Fatal("Up() should return an error when Status.Sudoers is empty")
	}
	if !strings.Contains(err.Error(), "sudoers") {
		t.Errorf("error should mention sudoers, got: %v", err)
	}
}

func TestUpSucceedsWhenSudoersStatusPresent(t *testing.T) {
	h := NewScheduleHandler(parser.Rule{
		SchedulePreset: "daily",
		ScheduleSource: "~/setup.bp",
	}, "")

	status := &Status{Sudoers: []SudoersStatus{{User: "testuser"}}}

	msg, err := h.UpWithStatus(status, nil, fakeReadCrontab(""), fakeWriteCrontab())
	if err != nil {
		t.Fatalf("Up() should succeed when sudoers entry exists, got: %v", err)
	}
	if !strings.Contains(msg, "Scheduled") {
		t.Errorf("expected 'Scheduled' in message, got: %q", msg)
	}
}

func TestUpIdempotentWhenAlreadyScheduled(t *testing.T) {
	h := NewScheduleHandler(parser.Rule{
		SchedulePreset: "daily",
		ScheduleSource: "~/setup.bp",
	}, "")

	status := &Status{Sudoers: []SudoersStatus{{User: "testuser"}}}
	existing := h.cronLine() + "\n"

	msg, err := h.UpWithStatus(status, nil, fakeReadCrontab(existing), fakeWriteCrontab())
	if err != nil {
		t.Fatalf("Up() idempotent check returned error: %v", err)
	}
	if !strings.Contains(msg, "already scheduled") {
		t.Errorf("expected 'already scheduled', got: %q", msg)
	}
}

// ---- Down -----------------------------------------------------------------

func TestDownRemovesLine(t *testing.T) {
	h := NewScheduleHandler(parser.Rule{
		SchedulePreset: "daily",
		ScheduleSource: "~/setup.bp",
	}, "")

	written := ""
	existing := "other line\n" + h.cronLine() + "\n"

	msg, err := h.DownWithCrontab(fakeReadCrontab(existing), func(c string) error {
		written = c
		return nil
	})
	if err != nil {
		t.Fatalf("Down() failed: %v", err)
	}
	if !strings.Contains(msg, "Removed schedule") {
		t.Errorf("expected 'Removed schedule', got: %q", msg)
	}
	if strings.Contains(written, h.cronLine()) {
		t.Errorf("Down() should have removed the crontab line:\n%s", written)
	}
	if !strings.Contains(written, "other line") {
		t.Errorf("Down() should have preserved other crontab lines:\n%s", written)
	}
}

func TestDownWhenEntryNotPresent(t *testing.T) {
	h := NewScheduleHandler(parser.Rule{
		SchedulePreset: "daily",
		ScheduleSource: "~/setup.bp",
	}, "")

	msg, err := h.DownWithCrontab(fakeReadCrontab("other line\n"), fakeWriteCrontab())
	if err != nil {
		t.Fatalf("Down() should not error when entry is absent: %v", err)
	}
	if !strings.Contains(msg, "already removed") {
		t.Errorf("expected 'already removed', got: %q", msg)
	}
}

// ---- FindUninstallRules ---------------------------------------------------

func TestFindUninstallRulesReturnsRemovedSchedules(t *testing.T) {
	h := NewScheduleHandler(parser.Rule{}, "")

	bp := "/home/user/setup.bp"
	status := &Status{
		Schedules: []ScheduleStatus{
			{CronExpr: "@daily", Source: "~/setup.bp", Blueprint: bp, OS: "linux"},
			{CronExpr: "@weekly", Source: "~/setup.bp", Blueprint: bp, OS: "linux"},
		},
	}
	currentRules := []parser.Rule{
		{Action: "schedule", SchedulePreset: "daily", ScheduleSource: "~/setup.bp"},
	}

	rules := h.FindUninstallRules(status, currentRules, bp, "linux")
	if len(rules) != 1 {
		t.Fatalf("expected 1 uninstall rule for removed schedule, got %d", len(rules))
	}
	if rules[0].ScheduleCron != "@weekly" {
		t.Errorf("expected @weekly uninstall rule, got %q", rules[0].ScheduleCron)
	}
}

func TestFindUninstallRulesIgnoresDifferentBlueprint(t *testing.T) {
	h := NewScheduleHandler(parser.Rule{}, "")

	status := &Status{
		Schedules: []ScheduleStatus{
			{CronExpr: "@daily", Source: "~/setup.bp", Blueprint: "/other/setup.bp", OS: "linux"},
		},
	}

	rules := h.FindUninstallRules(status, nil, "/home/user/setup.bp", "linux")
	if len(rules) != 0 {
		t.Errorf("expected 0 uninstall rules for a different blueprint, got %d", len(rules))
	}
}

// ---- Bug: schedule fails when sudoers runs in the same blueprint execution -
//
// When a sudoers rule runs at step N and schedule runs at step N+1 in the same
// blueprint apply, the status file on disk still has no sudoers entry (it is
// written only after all rules complete). UpWithStatus only checks Status.Sudoers
// from the file, so the schedule always fails on the first run even though
// sudoers succeeded moments before.
//
// Fix: UpWithStatus must also accept the current run's execution records and
// treat a successful sudoers record in those as equivalent to a status entry.

func TestUpSucceedsWhenSudoersRanInCurrentExecution(t *testing.T) {
	h := NewScheduleHandler(parser.Rule{
		SchedulePreset: "daily",
		ScheduleSource: "~/setup.bp",
	}, "")

	// Status file has no sudoers entry (first run, file not written yet).
	emptyStatus := &Status{}

	// But the sudoers handler ran and succeeded in this execution.
	sudoersHandler := NewSudoersHandler(parser.Rule{Action: "sudoers"}, "")
	currentRecords := []ExecutionRecord{
		{
			Command: sudoersHandler.GetCommand(),
			Status:  "success",
		},
	}

	msg, err := h.UpWithStatus(emptyStatus, currentRecords, fakeReadCrontab(""), fakeWriteCrontab())
	if err != nil {
		t.Fatalf("BUG: Up() should succeed when sudoers ran successfully in the current execution, got: %v", err)
	}
	if !strings.Contains(msg, "Scheduled") {
		t.Errorf("expected 'Scheduled' in message, got: %q", msg)
	}
}

// ---- Bug: Up() passes nil status to UpWithStatus causing a panic ----------
//
// Up() calls UpWithStatus(nil, ...) which immediately dereferences status.Sudoers
// → nil pointer dereference → panic.
// This test catches that: it calls Up() (which hits the real crontab, so it will
// fail on most machines), but it must NOT panic regardless of the outcome.

func TestUpDoesNotPanic(t *testing.T) {
	h := NewScheduleHandler(parser.Rule{
		SchedulePreset: "daily",
		ScheduleSource: "~/setup.bp",
	}, "")

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("BUG: Up() panicked with: %v", r)
		}
	}()

	// We don't care whether it returns an error (crontab may not be available
	// in CI, or sudoers check may fail) — the only requirement is no panic.
	h.Up() //nolint:errcheck
}

// ---- helpers --------------------------------------------------------------

func fakeReadCrontab(content string) func() (string, error) {
	return func() (string, error) { return content, nil }
}

func fakeWriteCrontab() func(string) error {
	return func(string) error { return nil }
}
