package parser

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// repoRoot returns the absolute path to the repository root by walking up
// from this test file's location.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// internal/parser/consistency_test.go -> repo root is 3 levels up
	return filepath.Dir(filepath.Dir(filepath.Dir(filename)))
}

// extractGoModVersion parses go.mod and returns the Go version in the
// "go N.N.N" directive (e.g. "1.25.0").
func extractGoModVersion(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("reading go.mod: %v", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "go ") {
			// "go 1.25.0" -> "1.25.0"
			v := strings.Fields(line)
			if len(v) == 2 {
				return v[1]
			}
		}
	}
	t.Fatal("go version directive not found in go.mod")
	return ""
}

// extractBPGoVersion parses setup.bp and returns the GO_VERSION variable value.
func extractBPGoVersion(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "setup.bp"))
	if err != nil {
		t.Fatalf("reading setup.bp: %v", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "var GO_VERSION ") {
			// "var GO_VERSION 1.25.0" -> "1.25.0"
			parts := strings.Fields(trimmed)
			if len(parts) >= 3 {
				return parts[2]
			}
		}
	}
	t.Fatal("GO_VERSION variable not found in setup.bp")
	return ""
}

// extractMiseGoVersion parses mise.toml and returns the go tool version.
func extractMiseGoVersion(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "mise.toml"))
	if err != nil {
		t.Fatalf("reading mise.toml: %v", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "go = ") {
			// 'go = "1.25.0"' -> "1.25.0"
			start := strings.Index(trimmed, `"`)
			if start == -1 {
				continue
			}
			end := strings.LastIndex(trimmed, `"`)
			if end <= start {
				continue
			}
			return trimmed[start+1 : end]
		}
	}
	t.Fatal("go tool version not found in mise.toml")
	return ""
}

// TestGoVersionConsistency ensures the Go version declared across
// go.mod, setup.bp, and mise.toml all agree. A mismatch means
// local development (mise install), blueprint diff/status checks,
// and CI builds would use different Go versions.
//
// If you bump the Go version, update ALL three files:
//   - go.mod   (go directive)
//   - setup.bp (GO_VERSION variable)
//   - mise.toml ([tools] go)
func TestGoVersionConsistency(t *testing.T) {
	root := repoRoot(t)

	goModVer := extractGoModVersion(t, root)
	bpVer := extractBPGoVersion(t, root)
	miseVer := extractMiseGoVersion(t, root)

	if goModVer != bpVer {
		t.Errorf("Go version mismatch:\n  go.mod:   %s\n  setup.bp: %s\n",
			goModVer, bpVer)
	}
	if goModVer != miseVer {
		t.Errorf("Go version mismatch:\n  go.mod:  %s\n  mise:    %s\n",
			goModVer, miseVer)
	}
	if bpVer != miseVer {
		t.Errorf("Go version mismatch:\n  setup.bp: %s\n  mise:     %s\n",
			bpVer, miseVer)
	}
}
