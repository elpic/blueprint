package unit

import (
	"strings"
	"testing"
	"time"

	"github.com/elpic/blueprint/internal/handlers"
	"github.com/elpic/blueprint/internal/parser"
)

// TestOllamaHandler_GetCommand_Pure tests command generation - pure function, no I/O.
func TestOllamaHandler_GetCommand_Pure(t *testing.T) {
	tests := []struct {
		name        string
		models      []string
		isUninstall bool
		expected    string
	}{
		{
			name:        "single model install",
			models:      []string{"llama3"},
			isUninstall: false,
			expected:    "ollama pull llama3",
		},
		{
			name:        "multiple models install",
			models:      []string{"llama3", "codellama", "mistral"},
			isUninstall: false,
			expected:    "ollama pull llama3 && ollama pull codellama && ollama pull mistral",
		},
		{
			name:        "single model uninstall",
			models:      []string{"llama3"},
			isUninstall: true,
			expected:    "ollama rm llama3",
		},
		{
			name:        "multiple models uninstall",
			models:      []string{"llama3", "codellama"},
			isUninstall: true,
			expected:    "ollama rm llama3 && ollama rm codellama",
		},
		{
			name:        "no models returns empty",
			models:      []string{},
			isUninstall: false,
			expected:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Create rule manually
			rule := parser.Rule{
				Action:       "ollama",
				OllamaModels: tt.models,
			}

			if tt.isUninstall {
				rule.Action = "uninstall"
			}

			// Create handler
			handler := handlers.NewOllamaHandler(rule, "/test/path")

			// Test command generation (pure function - no I/O)
			cmd := handler.GetCommand()

			duration := time.Since(start)

			// Verify command generation
			if cmd != tt.expected {
				t.Errorf("GetCommand() = %q, want %q", cmd, tt.expected)
			}

			// Verify that this is a fast unit test (< 10ms)
			if duration > 10*time.Millisecond {
				t.Errorf("Test took %v, expected < 10ms for pure unit test", duration)
			}
		})
	}
}

// TestOllamaHandler_NeedsSudo_Pure tests sudo requirement - pure function.
func TestOllamaHandler_NeedsSudo_Pure(t *testing.T) {
	rule := parser.Rule{
		Action:       "ollama",
		OllamaModels: []string{"llama3"},
	}

	handler := handlers.NewOllamaHandler(rule, "/test")

	start := time.Now()
	needsSudo := handler.NeedsSudo()
	duration := time.Since(start)

	// Ollama should never need sudo
	if needsSudo {
		t.Errorf("NeedsSudo() = true, want false (ollama operations don't need sudo)")
	}

	if duration > 10*time.Millisecond {
		t.Errorf("Test took %v, expected < 10μs for trivial function", duration)
	}
}

// TestOllamaHandler_GetDependencyKey_Pure tests dependency key generation
// without any I/O operations. Executes in microseconds.
func TestOllamaHandler_GetDependencyKey_Pure(t *testing.T) {
	tests := []struct {
		name     string
		ruleID   string
		models   []string
		expected string
	}{
		{
			name:     "uses rule ID when present",
			ruleID:   "custom-ollama-id",
			models:   []string{"llama3"},
			expected: "custom-ollama-id",
		},
		{
			name:     "falls back to first model",
			ruleID:   "",
			models:   []string{"codellama", "mistral"},
			expected: "codellama",
		},
		{
			name:     "falls back to 'ollama' when no models",
			ruleID:   "",
			models:   []string{},
			expected: "ollama",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			// Build rule manually
			rule := parser.Rule{
				ID:           tt.ruleID,
				Action:       "ollama",
				OllamaModels: tt.models,
			}

			// Test dependency key generation
			handler := handlers.NewOllamaHandler(rule, "/test")
			key := handler.GetDependencyKey()

			duration := time.Since(start)

			if key != tt.expected {
				t.Errorf("GetDependencyKey() = %q, want %q", key, tt.expected)
			}

			// This should be extremely fast (microseconds)
			if duration > 10*time.Millisecond {
				t.Errorf("Test took %v, expected < 10ms for pure logic test", duration)
			}
		})
	}
}

// TestOllamaHandler_GetDisplayDetails_Pure tests display information generation.
func TestOllamaHandler_GetDisplayDetails_Pure(t *testing.T) {
	tests := []struct {
		name     string
		models   []string
		expected string
	}{
		{
			name:     "single model",
			models:   []string{"llama3"},
			expected: "llama3",
		},
		{
			name:     "multiple models",
			models:   []string{"llama3", "codellama", "mistral"},
			expected: "llama3, codellama, mistral",
		},
		{
			name:     "no models",
			models:   []string{},
			expected: "",
		},
		{
			name:     "models with versions",
			models:   []string{"llama3:8b", "llama3:70b"},
			expected: "llama3:8b, llama3:70b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			rule := parser.Rule{
				Action:       "ollama",
				OllamaModels: tt.models,
			}

			handler := handlers.NewOllamaHandler(rule, "/test")

			details := handler.GetDisplayDetails(false)
			duration := time.Since(start)

			if details != tt.expected {
				t.Errorf("GetDisplayDetails() = %q, want %q", details, tt.expected)
			}

			if duration > 10*time.Millisecond {
				t.Errorf("Test took %v, expected < 10ms for pure logic test", duration)
			}
		})
	}
}

// TestOllamaHandler_GetState_Pure tests state generation for the "blueprint ps" command.
func TestOllamaHandler_GetState_Pure(t *testing.T) {
	tests := []struct {
		name   string
		models []string
	}{
		{
			name:   "single model state",
			models: []string{"llama3"},
		},
		{
			name:   "multiple models state",
			models: []string{"llama3", "codellama"},
		},
		{
			name:   "empty models state",
			models: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			rule := parser.Rule{
				Action:       "ollama",
				OllamaModels: tt.models,
			}

			handler := handlers.NewOllamaHandler(rule, "/test")
			state := handler.GetState(false)

			duration := time.Since(start)

			expectedModels := ""
			if len(tt.models) > 0 {
				expectedModels = strings.Join(tt.models, ", ")
			}

			// Verify required keys
			if state["summary"] != expectedModels {
				t.Errorf("state[summary] = %q, want %q", state["summary"], expectedModels)
			}
			if state["models"] != expectedModels {
				t.Errorf("state[models] = %q, want %q", state["models"], expectedModels)
			}

			if duration > 10*time.Millisecond {
				t.Errorf("Test took %v, expected < 10ms for pure logic test", duration)
			}
		})
	}
}
