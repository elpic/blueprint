package engine

import (
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

func TestSemanticCheck_Clean(t *testing.T) {
	rules := []parser.Rule{
		{ID: "a", Action: "install", Packages: []parser.Package{{Name: "git"}}, OSList: []string{"mac"}},
		{ID: "b", Action: "clone", CloneURL: "https://github.com/user/repo", After: []string{"a"}},
	}
	issues := semanticCheck(rules)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %d: %v", len(issues), issues)
	}
}


func TestCheckAfterReferences_Valid(t *testing.T) {
	rules := []parser.Rule{
		{ID: "base", Action: "install", Packages: []parser.Package{{Name: "git"}}},
		{Action: "clone", CloneURL: "https://github.com/user/repo", After: []string{"base"}},
	}
	if issues := checkAfterReferences(rules); len(issues) != 0 {
		t.Errorf("expected no issues, got %v", issues)
	}
}

func TestCheckAfterReferences_ByResourceKey(t *testing.T) {
	// after: can reference the primary resource key (e.g. package name) even without an id:
	rules := []parser.Rule{
		{Action: "install", Packages: []parser.Package{{Name: "git"}}},
		{Action: "clone", CloneURL: "https://github.com/user/repo", After: []string{"git"}},
	}
	if issues := checkAfterReferences(rules); len(issues) != 0 {
		t.Errorf("expected no issues, got %v", issues)
	}
}

func TestCheckAfterReferences_Missing(t *testing.T) {
	rules := []parser.Rule{
		{Action: "clone", CloneURL: "https://github.com/user/repo", After: []string{"nonexistent"}},
	}
	issues := checkAfterReferences(rules)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].line != 1 {
		t.Errorf("expected issue at rule 1, got %d", issues[0].line)
	}
}

func TestCheckOSFilters_Valid(t *testing.T) {
	rules := []parser.Rule{
		{Action: "install", Packages: []parser.Package{{Name: "git"}}, OSList: []string{"mac", "linux"}},
	}
	if issues := checkOSFilters(rules); len(issues) != 0 {
		t.Errorf("expected no issues, got %v", issues)
	}
}

func TestCheckOSFilters_Invalid(t *testing.T) {
	rules := []parser.Rule{
		{Action: "install", Packages: []parser.Package{{Name: "git"}}, OSList: []string{"darwin"}},
	}
	issues := checkOSFilters(rules)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].line != 1 {
		t.Errorf("expected issue at rule 1, got %d", issues[0].line)
	}
}

func TestCheckOSFilters_NoFilter(t *testing.T) {
	// No OSList means applies to all OSes — not an error.
	rules := []parser.Rule{
		{Action: "install", Packages: []parser.Package{{Name: "git"}}},
	}
	if issues := checkOSFilters(rules); len(issues) != 0 {
		t.Errorf("expected no issues, got %v", issues)
	}
}

func TestRuleLabel(t *testing.T) {
	tests := []struct {
		rule parser.Rule
		want string
	}{
		{parser.Rule{ID: "myid", Action: "install"}, "myid"},
		{parser.Rule{Action: "install", Packages: []parser.Package{{Name: "git"}}}, "install git"},
		{parser.Rule{Action: "clone", ClonePath: "~/projects"}, "clone ~/projects"},
		{parser.Rule{Action: "run", RunCommand: "echo hi"}, "run echo hi"},
		{parser.Rule{Action: "mkdir", Mkdir: "/tmp/foo"}, "mkdir /tmp/foo"},
	}
	for _, tt := range tests {
		got := ruleLabel(tt.rule)
		if got != tt.want {
			t.Errorf("ruleLabel(%v) = %q, want %q", tt.rule.Action, got, tt.want)
		}
	}
}

func TestValidateIssueString(t *testing.T) {
	issue := validateIssue{line: 3, summary: "install git", message: "some problem"}
	want := "rule 3 (install git): some problem"
	if got := issue.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	fileLevelIssue := validateIssue{line: 0, message: "file-level problem"}
	if got := fileLevelIssue.String(); got != "file-level problem" {
		t.Errorf("got %q, want %q", got, "file-level problem")
	}
}
