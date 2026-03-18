package unit

import (
	"testing"
	"time"

	"github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/platform/testutils"
)

// BenchmarkPureUnitTests demonstrates the speed difference between
// pure unit tests with mocks vs integration tests with real dependencies.
func BenchmarkPureUnitTests(b *testing.B) {
	// Pre-create test data to exclude setup time from benchmark
	rule := testutils.NewRule().
		WithAction("install").
		WithPackages("git", "curl", "wget").
		Build()

	container := testutils.NewMockContainer().
		WithOS("linux").
		WithUser("testuser", "1001", "1001", "/home/testuser").
		Build()

	handler := handlers.NewInstallHandler(rule, "/test", container)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Test pure logic operations
		_ = handler.GetDependencyKey()
		_ = handler.GetDisplayDetails(false)
		_ = handler.GetState(false)
	}
}

// TestSpeedComparison demonstrates the execution speed of different test types.
func TestSpeedComparison(t *testing.T) {
	rule := testutils.NewRule().
		WithAction("clone").
		WithClone("https://github.com/user/repo.git", "~/projects/repo").
		Build()

	container := testutils.NewMockContainer().
		WithOS("linux").
		WithUser("testuser", "1001", "1001", "/home/testuser").
		Build()

	handler := handlers.NewCloneHandler(rule, "/test", container)

	// Measure pure logic operations
	start := time.Now()
	for i := 0; i < 1000; i++ {
		_ = handler.GetDependencyKey()
		_ = handler.GetDisplayDetails(false)
		_ = handler.GetState(false)
	}
	pureLogicDuration := time.Since(start)

	// Measure mock filesystem operations
	start = time.Now()
	for i := 0; i < 1000; i++ {
		_ = container.SystemProvider().Filesystem().ExpandPath("~/test/path")
		_ = container.SystemProvider().OS().Name()
	}
	mockOperationsDuration := time.Since(start)

	t.Logf("1000 pure logic operations took: %v (avg: %v per operation)",
		pureLogicDuration, pureLogicDuration/1000)
	t.Logf("1000 mock operations took: %v (avg: %v per operation)",
		mockOperationsDuration, mockOperationsDuration/1000)

	// Pure logic should be extremely fast (< 1ms total for 1000 operations)
	if pureLogicDuration > time.Millisecond {
		t.Errorf("Pure logic operations too slow: %v, expected < 1ms for 1000 operations",
			pureLogicDuration)
	}

	// Mock operations should be fast (< 10ms total for 1000 operations)
	if mockOperationsDuration > 10*time.Millisecond {
		t.Errorf("Mock operations too slow: %v, expected < 10ms for 1000 operations",
			mockOperationsDuration)
	}
}

// TestIntegrationVsUnitSpeed compares the speed of integration vs unit tests.
func TestIntegrationVsUnitSpeed(t *testing.T) {
	// This test documents the speed improvement we achieve with unit tests

	// Unit test with mocks (what we want)
	rule := testutils.NewRule().
		WithAction("clone").
		WithClone("https://github.com/user/repo.git", "~/projects/repo").
		Build()

	mockContainer := testutils.NewMockContainer().
		WithOS("linux").
		WithUser("testuser", "1001", "1001", "/home/testuser").
		Build()

	start := time.Now()
	handler := handlers.NewCloneHandler(rule, "/test", mockContainer)
	_ = handler.GetCommand()
	_ = handler.GetDependencyKey()
	_ = handler.GetDisplayDetails(false)
	unitTestDuration := time.Since(start)

	// Integration test with real dependencies (what we're moving away from)
	start = time.Now()
	legacyHandler := handlers.NewCloneHandlerLegacy(rule, "/test")
	_ = legacyHandler.GetCommand()
	_ = legacyHandler.GetDependencyKey()
	_ = legacyHandler.GetDisplayDetails(false)
	// Note: We can't call Up() here as it would try to clone a real repo
	integrationTestDuration := time.Since(start)

	t.Logf("Unit test with mocks took: %v", unitTestDuration)
	t.Logf("Integration test took: %v", integrationTestDuration)

	// Unit tests should be significantly faster than integration tests
	if unitTestDuration > time.Millisecond {
		t.Logf("Note: Unit test took %v, ideally should be < 1ms", unitTestDuration)
	}

	// This demonstrates the testing transformation goals:
	// 1. Faster execution
	// 2. No external dependencies
	// 3. Predictable results
	// 4. Easy to mock different scenarios
}

// TestMockVersusRealBehavior demonstrates how mocks give us control
// over test scenarios that would be difficult with real dependencies.
func TestMockVersusRealBehavior(t *testing.T) {
	testCases := []struct {
		name     string
		os       string
		homeDir  string
		expected string
	}{
		{
			name:     "Linux user",
			os:       "linux",
			homeDir:  "/home/testuser",
			expected: "/home/testuser/projects/repo",
		},
		{
			name:     "macOS user",
			os:       "mac",
			homeDir:  "/Users/macuser",
			expected: "/Users/macuser/projects/repo",
		},
		{
			name:     "custom home directory",
			os:       "linux",
			homeDir:  "/custom/home/dir",
			expected: "/custom/home/dir/projects/repo",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			container := testutils.NewMockContainer().
				WithOS(tc.os).
				WithUser("user", "1000", "1000", tc.homeDir).
				Build()

			expanded := container.SystemProvider().Filesystem().ExpandPath("~/projects/repo")

			if expanded != tc.expected {
				t.Errorf("ExpandPath() = %q, want %q", expanded, tc.expected)
			}
		})
	}

	// Note: With real filesystem operations, we couldn't easily test
	// different home directories or OS configurations without complex
	// test environment setup.
}
