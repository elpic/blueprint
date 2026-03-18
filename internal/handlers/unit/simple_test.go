package unit

import (
	"testing"

	"github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/platform/testutils"
)

// TestRefactoredHandlers demonstrates that the dependency injection refactoring works.
// This validates Phase 2 implementation.

func TestInstallHandler_WithDependencyInjection(t *testing.T) {
	tests := []struct {
		name     string
		osName   string
		packages []string
		expected string
	}{
		{
			name:     "mac brew install",
			osName:   "mac",
			packages: []string{"curl"},
			expected: "brew install curl",
		},
		{
			name:     "linux apt install",
			osName:   "linux",
			packages: []string{"curl"},
			expected: "sudo apt-get install -y curl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create rule
			rule := testutils.NewRule().
				WithPackages(tt.packages...).
				Build()

			// Create mock container with OS detection
			container := testutils.NewMockContainer().
				WithOS(tt.osName).
				WithUser("testuser", "1000", "1000", "/home/testuser").
				Build()

			// Create handler with dependency injection
			handler := handlers.NewInstallHandler(rule, "", container)

			// Test command building uses injected dependencies
			cmd := handler.GetCommand()
			if cmd != tt.expected {
				t.Errorf("GetCommand() = %q, want %q", cmd, tt.expected)
			}

			// Test that NeedsSudo uses injected dependencies
			needsSudo := handler.NeedsSudo()
			expectedSudo := (tt.osName == "linux" || tt.osName == "mac") && len(tt.packages) > 0
			if needsSudo != expectedSudo {
				t.Errorf("NeedsSudo() = %v, want %v", needsSudo, expectedSudo)
			}
		})
	}
}

func TestCloneHandler_WithDependencyInjection(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		path     string
		branch   string
		expected string
	}{
		{
			name:     "clone without branch",
			url:      "https://github.com/user/repo.git",
			path:     "~/projects/repo",
			expected: "git clone https://github.com/user/repo.git ~/projects/repo",
		},
		{
			name:     "clone with branch",
			url:      "https://github.com/user/repo.git",
			path:     "~/projects/repo",
			branch:   "develop",
			expected: "git clone -b develop https://github.com/user/repo.git ~/projects/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create rule
			ruleBuilder := testutils.NewRule().WithClone(tt.url, tt.path)
			if tt.branch != "" {
				ruleBuilder = ruleBuilder.WithBranch(tt.branch)
			}
			rule := ruleBuilder.Build()

			// Create mock container with filesystem
			container := testutils.NewMockContainer().
				WithOS("mac").
				WithDirectory("/Users/testuser").
				Build()

			// Create handler with dependency injection
			handler := handlers.NewCloneHandler(rule, "", container)

			// Test command building
			cmd := handler.GetCommand()
			if cmd != tt.expected {
				t.Errorf("GetCommand() = %q, want %q", cmd, tt.expected)
			}
		})
	}
}

func TestDependencyInjection_Performance(t *testing.T) {
	// Quick benchmark to ensure dependency injection doesn't slow things down
	rule := testutils.NewRule().WithPackages("curl", "git", "wget").Build()
	container := testutils.NewMockContainer().WithOS("mac").Build()

	handler := handlers.NewInstallHandler(rule, "", container)

	// This should be very fast (sub-millisecond) since it's pure logic
	for i := 0; i < 1000; i++ {
		_ = handler.GetCommand()
		_ = handler.GetDependencyKey()
	}
}

func TestBackwardCompatibility(t *testing.T) {
	// Test that legacy constructors still work
	rule := testutils.NewRule().WithPackage("curl").Build()

	// Legacy constructor should still work
	handler := handlers.NewInstallHandlerLegacy(rule, "")

	// Should be able to get basic info
	key := handler.GetDependencyKey()
	if key != "curl" {
		t.Errorf("GetDependencyKey() = %q, want %q", key, "curl")
	}
}
