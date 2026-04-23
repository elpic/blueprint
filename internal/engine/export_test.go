package engine

import (
	"strings"
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

func TestDetectToolNeeds_None(t *testing.T) {
	rules := []parser.Rule{
		{Action: "run", RunCommand: "echo hi"},
		{Action: "mkdir", Mkdir: "~/tmp"},
	}
	t1 := detectToolNeeds(rules, "linux")
	if t1.brew || t1.mise || t1.asdf || t1.ollama {
		t.Errorf("expected no tools needed, got %+v", t1)
	}
}

func TestDetectToolNeeds_InstallOnMac(t *testing.T) {
	rules := []parser.Rule{
		{Action: "install", Packages: []parser.Package{{Name: "vim"}}},
	}
	t1 := detectToolNeeds(rules, "mac")
	if !t1.brew {
		t.Error("expected brew needed on mac for install action")
	}
	t2 := detectToolNeeds(rules, "linux")
	if t2.brew {
		t.Error("did not expect brew needed on linux for install action")
	}
}

func TestDetectToolNeeds_Homebrew(t *testing.T) {
	rules := []parser.Rule{
		{Action: "homebrew", HomebrewPackages: []string{"node"}},
	}
	t1 := detectToolNeeds(rules, "mac")
	if !t1.brew {
		t.Error("expected brew needed for homebrew action")
	}
}

func TestDetectToolNeeds_Mise(t *testing.T) {
	rules := []parser.Rule{
		{Action: "mise", MisePackages: []string{"node@20"}},
	}
	t1 := detectToolNeeds(rules, "mac")
	if !t1.mise {
		t.Error("expected mise needed")
	}
	if !t1.brew {
		t.Error("expected brew needed on mac for mise (installed via brew)")
	}
	t2 := detectToolNeeds(rules, "linux")
	if !t2.mise {
		t.Error("expected mise needed on linux")
	}
	if t2.brew {
		t.Error("did not expect brew needed on linux for mise")
	}
}

func TestDetectToolNeeds_All(t *testing.T) {
	rules := []parser.Rule{
		{Action: "homebrew", HomebrewPackages: []string{"node"}},
		{Action: "mise", MisePackages: []string{"node@20"}},
		{Action: "asdf", AsdfPackages: []string{"nodejs@21"}},
		{Action: "ollama", OllamaModels: []string{"llama3"}},
	}
	t1 := detectToolNeeds(rules, "mac")
	if !t1.brew || !t1.mise || !t1.asdf || !t1.ollama {
		t.Errorf("expected all tools needed, got %+v", t1)
	}
}

func TestWritePrerequisites_EmitsBrewOnce(t *testing.T) {
	var b strings.Builder
	rules := []parser.Rule{
		{Action: "homebrew", HomebrewPackages: []string{"node"}},
		{Action: "install", Packages: []parser.Package{{Name: "vim"}}},
	}
	writePrerequisites(&b, rules, "mac")
	output := b.String()
	count := strings.Count(output, "command_exists brew")
	if count != 1 {
		t.Errorf("expected brew check exactly once, got %d", count)
	}
}

func TestWritePrerequisites_NoneNeeded(t *testing.T) {
	var b strings.Builder
	rules := []parser.Rule{
		{Action: "run", RunCommand: "echo hi"},
	}
	writePrerequisites(&b, rules, "linux")
	if b.Len() != 0 {
		t.Errorf("expected empty output when no tools needed, got: %s", b.String())
	}
}
