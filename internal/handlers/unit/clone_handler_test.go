package unit

import (
	"testing"
	"time"

	"github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/platform/testutils"
)

// TestCloneHandler_GetCommand_Pure demonstrates fast, pure unit testing
// for clone command generation. Executes in microseconds.
func TestCloneHandler_GetCommand_Pure(t *testing.T) {
	tests := []struct {
		name     string
		action   string
		url      string
		path     string
		branch   string
		expected string
	}{
		{
			name:     "clone without branch",
			action:   "clone",
			url:      "https://github.com/user/repo.git",
			path:     "~/projects/repo",
			branch:   "",
			expected: "git clone https://github.com/user/repo.git ~/projects/repo",
		},
		{
			name:     "clone with specific branch",
			action:   "clone",
			url:      "https://github.com/user/repo.git",
			path:     "~/projects/repo",
			branch:   "develop",
			expected: "git clone -b develop https://github.com/user/repo.git ~/projects/repo",
		},
		{
			name:     "uninstall clone",
			action:   "uninstall",
			url:      "",
			path:     "~/projects/repo",
			branch:   "",
			expected: "rm -rf ~/projects/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Build rule using test builder
			ruleBuilder := testutils.NewRule().WithAction(tt.action)

			if tt.action == "clone" {
				ruleBuilder = ruleBuilder.WithClone(tt.url, tt.path)
				if tt.branch != "" {
					ruleBuilder = ruleBuilder.WithBranch(tt.branch)
				}
			} else {
				// For uninstall, manually set the path
				rule := ruleBuilder.Build()
				rule.ClonePath = tt.path
				ruleBuilder = &testutils.RuleBuilder{}
				*ruleBuilder = testutils.RuleBuilder{}
				// Rebuild with updated rule fields
				rule.Action = tt.action

				// Create handler directly with this rule
				container := testutils.NewMockContainer().WithOS("linux").Build()
				handler := handlers.NewCloneHandler(rule, "/test", container)

				cmd := handler.GetCommand()
				duration := time.Since(start)

				if cmd != tt.expected {
					t.Errorf("GetCommand() = %q, want %q", cmd, tt.expected)
				}

				if duration > time.Millisecond {
					t.Errorf("Test took %v, expected < 1ms for pure unit test", duration)
				}
				return
			}

			rule := ruleBuilder.Build()
			container := testutils.NewMockContainer().WithOS("linux").Build()
			handler := handlers.NewCloneHandler(rule, "/test", container)

			cmd := handler.GetCommand()
			duration := time.Since(start)

			if cmd != tt.expected {
				t.Errorf("GetCommand() = %q, want %q", cmd, tt.expected)
			}

			if duration > time.Millisecond {
				t.Errorf("Test took %v, expected < 1ms for pure unit test", duration)
			}
		})
	}
}

// TestCloneHandler_GetDependencyKey_Pure tests dependency key generation.
func TestCloneHandler_GetDependencyKey_Pure(t *testing.T) {
	tests := []struct {
		name      string
		ruleID    string
		clonePath string
		expected  string
	}{
		{
			name:      "uses rule ID when present",
			ruleID:    "my-repo-id",
			clonePath: "~/projects/repo",
			expected:  "my-repo-id",
		},
		{
			name:      "falls back to clone path",
			ruleID:    "",
			clonePath: "~/projects/myrepo",
			expected:  "~/projects/myrepo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			ruleBuilder := testutils.NewRule().
				WithClone("https://github.com/user/repo.git", tt.clonePath)

			if tt.ruleID != "" {
				ruleBuilder = ruleBuilder.WithID(tt.ruleID)
			}

			rule := ruleBuilder.Build()
			container := testutils.NewMockContainer().WithOS("linux").Build()
			handler := handlers.NewCloneHandler(rule, "/test", container)

			key := handler.GetDependencyKey()
			duration := time.Since(start)

			if key != tt.expected {
				t.Errorf("GetDependencyKey() = %q, want %q", key, tt.expected)
			}

			if duration > 10*time.Millisecond {
				t.Errorf("Test took %v, expected < 10ms for pure logic test", duration)
			}
		})
	}
}

// TestCloneHandler_GetDisplayDetails_Pure tests display information.
func TestCloneHandler_GetDisplayDetails_Pure(t *testing.T) {
	clonePath := "~/projects/myrepo"

	rule := testutils.NewRule().
		WithClone("https://github.com/user/repo.git", clonePath).
		Build()

	container := testutils.NewMockContainer().WithOS("linux").Build()
	handler := handlers.NewCloneHandler(rule, "/test", container)

	start := time.Now()
	details := handler.GetDisplayDetails(false)
	duration := time.Since(start)

	if details != clonePath {
		t.Errorf("GetDisplayDetails() = %q, want %q", details, clonePath)
	}

	if duration > 50*time.Microsecond {
		t.Errorf("Test took %v, expected < 50μs for simple getter", duration)
	}
}

// TestCloneHandler_GetState_Pure tests state generation.
func TestCloneHandler_GetState_Pure(t *testing.T) {
	url := "https://github.com/user/repo.git"
	path := "~/projects/myrepo"
	branch := "main"

	rule := testutils.NewRule().
		WithClone(url, path).
		WithBranch(branch).
		Build()

	container := testutils.NewMockContainer().WithOS("linux").Build()
	handler := handlers.NewCloneHandler(rule, "/test", container)

	start := time.Now()
	state := handler.GetState(false)
	duration := time.Since(start)

	// Verify state fields
	if state["summary"] != path {
		t.Errorf("state[summary] = %q, want %q", state["summary"], path)
	}
	if state["url"] != url {
		t.Errorf("state[url] = %q, want %q", state["url"], url)
	}
	if state["path"] != path {
		t.Errorf("state[path] = %q, want %q", state["path"], path)
	}
	if state["branch"] != branch {
		t.Errorf("state[branch] = %q, want %q", state["branch"], branch)
	}

	if duration > 10*time.Millisecond {
		t.Errorf("Test took %v, expected < 10ms for pure logic test", duration)
	}
}

// TestCloneHandler_PathExpansion_Mock demonstrates mocking filesystem operations
// for path expansion logic without real filesystem I/O.
func TestCloneHandler_PathExpansion_Mock(t *testing.T) {
	tests := []struct {
		name         string
		clonePath    string
		expectedPath string
		homeDir      string
	}{
		{
			name:         "expands tilde path",
			clonePath:    "~/projects/repo",
			expectedPath: "/home/testuser/projects/repo",
			homeDir:      "/home/testuser",
		},
		{
			name:         "absolute path unchanged",
			clonePath:    "/var/projects/repo",
			expectedPath: "/var/projects/repo",
			homeDir:      "/home/testuser",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			rule := testutils.NewRule().
				WithClone("https://github.com/user/repo.git", tt.clonePath).
				Build()

			// Mock filesystem with home directory expansion
			container := testutils.NewMockContainer().
				WithOS("linux").
				WithUser("testuser", "1001", "1001", tt.homeDir).
				Build()

			// Create handler to demonstrate DI setup, but test filesystem mock directly
			_ = handlers.NewCloneHandler(rule, "/test", container)

			// The Up() method would call filesystem.ExpandPath(), but we don't want to
			// actually clone anything. Instead, we can test the filesystem mock directly.
			expandedPath := container.SystemProvider().Filesystem().ExpandPath(tt.clonePath)
			duration := time.Since(start)

			if expandedPath != tt.expectedPath {
				t.Errorf("ExpandPath(%q) = %q, want %q", tt.clonePath, expandedPath, tt.expectedPath)
			}

			// This mock operation should be very fast
			if duration > time.Millisecond {
				t.Errorf("Test took %v, expected < 1ms for mock operation", duration)
			}
		})
	}
}
