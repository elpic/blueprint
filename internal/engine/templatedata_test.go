package engine

import (
	"strings"
	"testing"
)

func TestPrintDiff_Changed(t *testing.T) {
	old := "FROM ruby:3.2-slim\nRUN apt-get install git"
	new := "FROM ruby:3.3-slim\nRUN apt-get install git"
	diff := printDiff(old, new, "Dockerfile")
	if !strings.Contains(diff, "-FROM ruby:3.2-slim") {
		t.Error("expected removal line in diff")
	}
	if !strings.Contains(diff, "+FROM ruby:3.3-slim") {
		t.Error("expected addition line in diff")
	}
}

func TestPrintDiff_Identical(t *testing.T) {
	content := "FROM python:3.13-slim"
	diff := printDiff(content, content, "Dockerfile")
	if strings.Contains(diff, "-") || strings.Contains(diff, "+") {
		// The header lines start with --- and +++ which is expected; only content lines
		// should not have +/- prefixes.
		lines := strings.Split(diff, "\n")
		for _, l := range lines[2:] { // skip the --- and +++ header lines
			if strings.HasPrefix(l, "-") || strings.HasPrefix(l, "+") {
				t.Errorf("unexpected diff line for identical content: %q", l)
			}
		}
	}
}

func TestPrintDiff_OnlyAdditions(t *testing.T) {
	old := "line1"
	new := "line1\nline2\nline3"
	diff := printDiff(old, new, "file")
	if !strings.Contains(diff, "+line2") {
		t.Error("expected added line2")
	}
	if !strings.Contains(diff, "+line3") {
		t.Error("expected added line3")
	}
}

func TestPrintDiff_OnlyDeletions(t *testing.T) {
	old := "line1\nline2\nline3"
	new := "line1"
	diff := printDiff(old, new, "file")
	if !strings.Contains(diff, "-line2") {
		t.Error("expected removed line2")
	}
	if !strings.Contains(diff, "-line3") {
		t.Error("expected removed line3")
	}
}

func TestPrintDiff_EmptyStrings(t *testing.T) {
	diff := printDiff("", "", "file")
	// Should not panic and should produce valid header
	if !strings.Contains(diff, "--- file") {
		t.Error("expected header in diff")
	}
}

func TestPrintDiff_LabelInHeader(t *testing.T) {
	diff := printDiff("a", "b", "my-special-file.txt")
	if !strings.Contains(diff, "my-special-file.txt") {
		t.Error("label should appear in diff header")
	}
	if !strings.Contains(diff, "(existing)") {
		t.Error("expected (existing) marker")
	}
	if !strings.Contains(diff, "(rendered)") {
		t.Error("expected (rendered) marker")
	}
}
