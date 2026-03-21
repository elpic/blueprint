package engine

import (
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

func TestBuildRemoveSummary(t *testing.T) {
	tests := []struct {
		name     string
		rules    []parser.Rule
		expected []string
	}{
		{
			name: "install rule shows package names",
			rules: []parser.Rule{
				{Action: "install", Packages: []parser.Package{{Name: "git"}, {Name: "curl"}}},
			},
			expected: []string{"install: git, curl"},
		},
		{
			name: "clone rule shows URL",
			rules: []parser.Rule{
				{Action: "clone", CloneURL: "https://github.com/user/repo.git"},
			},
			expected: []string{"clone: https://github.com/user/repo.git"},
		},
		{
			name: "dotfiles rule shows URL",
			rules: []parser.Rule{
				{Action: "dotfiles", CloneURL: "git@github.com:user/dotfiles.git"},
			},
			expected: []string{"dotfiles: git@github.com:user/dotfiles.git"},
		},
		{
			name: "mise rule shows packages",
			rules: []parser.Rule{
				{Action: "mise", MisePackages: []string{"node@20", "python@3.11"}},
			},
			expected: []string{"mise: node@20, python@3.11"},
		},
		{
			name: "homebrew rule shows package names",
			rules: []parser.Rule{
				{Action: "homebrew", Packages: []parser.Package{{Name: "wget"}}},
			},
			expected: []string{"homebrew: wget"},
		},
		{
			name: "shell rule with ID",
			rules: []parser.Rule{
				{Action: "shell", ID: "setup-env"},
			},
			expected: []string{"shell: setup-env"},
		},
		{
			name: "unknown action with ID uses action: id format",
			rules: []parser.Rule{
				{Action: "mkdir", ID: "config-dir"},
			},
			expected: []string{"mkdir: config-dir"},
		},
		{
			name: "action without ID shows just action",
			rules: []parser.Rule{
				{Action: "mkdir"},
			},
			expected: []string{"mkdir"},
		},
		{
			name: "multiple rules",
			rules: []parser.Rule{
				{Action: "install", Packages: []parser.Package{{Name: "git"}}},
				{Action: "clone", CloneURL: "https://github.com/user/repo.git"},
			},
			expected: []string{"install: git", "clone: https://github.com/user/repo.git"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildRemoveSummary(tt.rules)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d lines, got %d: %v", len(tt.expected), len(result), result)
			}
			for i, line := range result {
				if line != tt.expected[i] {
					t.Errorf("line %d: expected %q, got %q", i, tt.expected[i], line)
				}
			}
		})
	}
}
