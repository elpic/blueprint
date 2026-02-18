package handlers

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/elpic/blueprint/internal/parser"
)

func TestOllamaHandlerGetCommand(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		expected string
	}{
		{
			name: "pull single model",
			rule: parser.Rule{
				Action:       "install",
				OllamaModels: []string{"llama3"},
			},
			expected: "ollama pull llama3",
		},
		{
			name: "pull multiple models",
			rule: parser.Rule{
				Action:       "install",
				OllamaModels: []string{"llama3", "codellama", "mistral"},
			},
			expected: "ollama pull llama3 && ollama pull codellama && ollama pull mistral",
		},
		{
			name: "uninstall single model",
			rule: parser.Rule{
				Action:       "uninstall",
				OllamaModels: []string{"llama3"},
			},
			expected: "ollama rm llama3",
		},
		{
			name: "uninstall multiple models",
			rule: parser.Rule{
				Action:       "uninstall",
				OllamaModels: []string{"llama3", "codellama"},
			},
			expected: "ollama rm llama3 && ollama rm codellama",
		},
		{
			name: "empty models returns empty",
			rule: parser.Rule{
				Action:       "install",
				OllamaModels: []string{},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewOllamaHandler(tt.rule, "")
			cmd := handler.GetCommand()
			if cmd != tt.expected {
				t.Errorf("GetCommand() = %q, want %q", cmd, tt.expected)
			}
		})
	}
}

func TestOllamaHandlerUpdateStatus(t *testing.T) {
	t.Run("install adds models to status", func(t *testing.T) {
		handler := NewOllamaHandler(parser.Rule{
			Action:       "install",
			OllamaModels: []string{"llama3", "codellama"},
		}, "")

		status := &Status{}
		cmd := handler.buildCommand()
		records := []ExecutionRecord{
			{Command: cmd, Status: "success"},
		}

		err := handler.UpdateStatus(status, records, "/tmp/test.bp", "mac")
		if err != nil {
			t.Fatalf("UpdateStatus() error = %v", err)
		}

		if len(status.Ollamas) != 2 {
			t.Fatalf("expected 2 ollama entries, got %d", len(status.Ollamas))
		}
		if status.Ollamas[0].Model != "llama3" {
			t.Errorf("first model = %q, want %q", status.Ollamas[0].Model, "llama3")
		}
		if status.Ollamas[1].Model != "codellama" {
			t.Errorf("second model = %q, want %q", status.Ollamas[1].Model, "codellama")
		}
	})

	t.Run("uninstall removes models from status", func(t *testing.T) {
		handler := NewOllamaHandler(parser.Rule{
			Action:       "uninstall",
			OllamaModels: []string{"llama3"},
		}, "")

		blueprint := normalizePath("/tmp/test.bp")
		status := &Status{
			Ollamas: []OllamaStatus{
				{Model: "llama3", Blueprint: blueprint, OS: "mac"},
				{Model: "codellama", Blueprint: blueprint, OS: "mac"},
			},
		}

		err := handler.UpdateStatus(status, nil, "/tmp/test.bp", "mac")
		if err != nil {
			t.Fatalf("UpdateStatus() error = %v", err)
		}

		if len(status.Ollamas) != 1 {
			t.Fatalf("expected 1 ollama entry, got %d", len(status.Ollamas))
		}
		if status.Ollamas[0].Model != "codellama" {
			t.Errorf("remaining model = %q, want %q", status.Ollamas[0].Model, "codellama")
		}
	})

	t.Run("install does not duplicate existing models", func(t *testing.T) {
		handler := NewOllamaHandler(parser.Rule{
			Action:       "install",
			OllamaModels: []string{"llama3"},
		}, "")

		blueprint := normalizePath("/tmp/test.bp")
		status := &Status{
			Ollamas: []OllamaStatus{
				{Model: "llama3", Blueprint: blueprint, OS: "mac", InstalledAt: "old-time"},
			},
		}

		cmd := handler.buildCommand()
		records := []ExecutionRecord{
			{Command: cmd, Status: "success"},
		}

		err := handler.UpdateStatus(status, records, "/tmp/test.bp", "mac")
		if err != nil {
			t.Fatalf("UpdateStatus() error = %v", err)
		}

		if len(status.Ollamas) != 1 {
			t.Fatalf("expected 1 ollama entry, got %d", len(status.Ollamas))
		}
		if status.Ollamas[0].InstalledAt == "old-time" {
			t.Error("expected InstalledAt to be updated, but it was not")
		}
	})
}

func TestOllamaHandlerGetDependencyKey(t *testing.T) {
	tests := []struct {
		name     string
		rule     parser.Rule
		expected string
	}{
		{
			name: "returns ID when present",
			rule: parser.Rule{
				ID:           "ai-models",
				OllamaModels: []string{"llama3"},
			},
			expected: "ai-models",
		},
		{
			name: "returns first model when no ID",
			rule: parser.Rule{
				OllamaModels: []string{"llama3", "codellama"},
			},
			expected: "llama3",
		},
		{
			name: "returns ollama when no ID and no models",
			rule: parser.Rule{},
			expected: "ollama",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewOllamaHandler(tt.rule, "")
			key := handler.GetDependencyKey()
			if key != tt.expected {
				t.Errorf("GetDependencyKey() = %q, want %q", key, tt.expected)
			}
		})
	}
}

func TestOllamaHandlerDisplayInfo(t *testing.T) {
	handler := NewOllamaHandler(parser.Rule{
		Action:       "install",
		OllamaModels: []string{"llama3", "codellama"},
	}, "")

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	handler.DisplayInfo()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if output == "" {
		t.Error("DisplayInfo() produced no output")
	}
}

func TestOllamaHandlerFindUninstallRules(t *testing.T) {
	blueprint := normalizePath("/tmp/test.bp")

	t.Run("finds models to uninstall", func(t *testing.T) {
		handler := NewOllamaHandler(parser.Rule{}, "")
		status := &Status{
			Ollamas: []OllamaStatus{
				{Model: "llama3", Blueprint: blueprint, OS: "mac"},
				{Model: "codellama", Blueprint: blueprint, OS: "mac"},
				{Model: "mistral", Blueprint: blueprint, OS: "mac"},
			},
		}

		currentRules := []parser.Rule{
			{Action: "ollama", OllamaModels: []string{"llama3"}},
		}

		rules := handler.FindUninstallRules(status, currentRules, "/tmp/test.bp", "mac")
		if len(rules) != 1 {
			t.Fatalf("expected 1 uninstall rule, got %d", len(rules))
		}
		if len(rules[0].OllamaModels) != 2 {
			t.Fatalf("expected 2 models to uninstall, got %d", len(rules[0].OllamaModels))
		}
	})

	t.Run("no uninstall when all models present", func(t *testing.T) {
		handler := NewOllamaHandler(parser.Rule{}, "")
		status := &Status{
			Ollamas: []OllamaStatus{
				{Model: "llama3", Blueprint: blueprint, OS: "mac"},
			},
		}

		currentRules := []parser.Rule{
			{Action: "ollama", OllamaModels: []string{"llama3"}},
		}

		rules := handler.FindUninstallRules(status, currentRules, "/tmp/test.bp", "mac")
		if len(rules) != 0 {
			t.Errorf("expected 0 uninstall rules, got %d", len(rules))
		}
	})

	t.Run("ignores models from different blueprint", func(t *testing.T) {
		handler := NewOllamaHandler(parser.Rule{}, "")
		otherBlueprint := normalizePath("/tmp/other.bp")
		status := &Status{
			Ollamas: []OllamaStatus{
				{Model: "llama3", Blueprint: blueprint, OS: "mac"},
				{Model: "codellama", Blueprint: otherBlueprint, OS: "mac"},
			},
		}

		currentRules := []parser.Rule{}

		rules := handler.FindUninstallRules(status, currentRules, "/tmp/test.bp", "mac")
		if len(rules) != 1 {
			t.Fatalf("expected 1 uninstall rule, got %d", len(rules))
		}
		if len(rules[0].OllamaModels) != 1 {
			t.Fatalf("expected 1 model to uninstall, got %d", len(rules[0].OllamaModels))
		}
		if rules[0].OllamaModels[0] != "llama3" {
			t.Errorf("expected model %q, got %q", "llama3", rules[0].OllamaModels[0])
		}
	})
}

func TestOllamaHandlerNeedsSudo(t *testing.T) {
	handler := NewOllamaHandler(parser.Rule{}, "")
	if handler.NeedsSudo() {
		t.Error("NeedsSudo() = true, want false")
	}
}
