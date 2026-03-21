package engine

import (
	"strings"
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

func TestRemoveDisplaySummary(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		contains string // just check the output is non-empty and contains key info
	}{
		{
			name:     "install rule contains package names",
			rule:     parser.Rule{Action: "install", Packages: []parser.Package{{Name: "git"}, {Name: "curl"}}},
			contains: "git",
		},
		{
			name:     "clone rule contains URL",
			rule:     parser.Rule{Action: "clone", CloneURL: "https://github.com/user/repo.git"},
			contains: "github.com/user/repo.git",
		},
		{
			name:     "mise rule contains package name",
			rule:     parser.Rule{Action: "mise", MisePackages: []string{"node@20"}},
			contains: "node@20",
		},
		{
			name:     "mkdir rule contains path",
			rule:     parser.Rule{Action: "mkdir", Mkdir: "/tmp/config"},
			contains: "/tmp/config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.rule.DisplaySummary()
			if !strings.Contains(result, tt.contains) {
				t.Errorf("expected DisplaySummary to contain %q, got %q", tt.contains, result)
			}
		})
	}
}
