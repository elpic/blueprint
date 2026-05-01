package engine

import (
	"strings"
	"testing"
)

func TestPrintDiff(t *testing.T) {
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
