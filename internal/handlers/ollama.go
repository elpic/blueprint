package handlers

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
)

// OllamaHandler handles ollama model installation and uninstallation
type OllamaHandler struct {
	BaseHandler
}

// ollamaInstallMutex prevents concurrent ollama installation attempts
var ollamaInstallMutex = &sync.Mutex{}

// NewOllamaHandler creates a new ollama handler
func NewOllamaHandler(rule parser.Rule, basePath string) *OllamaHandler {
	return &OllamaHandler{
		BaseHandler: BaseHandler{
			Rule:     rule,
			BasePath: basePath,
		},
	}
}

// Up installs the ollama models and ensures ollama is installed
func (h *OllamaHandler) Up() (string, error) {
	// Ollama works on macOS and Linux
	targetOS := getOSName()
	if targetOS != "mac" && targetOS != "linux" {
		return "", fmt.Errorf("ollama is not supported on %s", targetOS)
	}

	// First ensure ollama is installed
	if err := h.ensureOllamaInstalled(); err != nil {
		return "", fmt.Errorf("failed to ensure ollama is installed: %w", err)
	}

	// Then pull the models
	cmd := h.buildCommand()
	if cmd == "" {
		return "", fmt.Errorf("unable to build install command")
	}

	return executeCommandWithCache(cmd)
}

// Down removes the ollama models
func (h *OllamaHandler) Down() (string, error) {
	cmd := h.buildUninstallCommand()
	if cmd == "" {
		return "", fmt.Errorf("unable to build uninstall command")
	}

	return executeCommandWithCache(cmd)
}

// GetCommand returns the actual command(s) that will be executed
func (h *OllamaHandler) GetCommand() string {
	if h.Rule.Action == "uninstall" {
		return h.buildUninstallCommand()
	}

	return h.buildCommand()
}

// UpdateStatus updates the status after installing or uninstalling models
func (h *OllamaHandler) UpdateStatus(status *Status, records []ExecutionRecord, blueprint string, osName string) error {
	blueprint = normalizePath(blueprint)

	switch h.Rule.Action {
	case "install":
		cmd := h.buildCommand()
		_, commandExecuted := commandSuccessfullyExecuted(cmd, records)

		if commandExecuted {
			for _, model := range h.Rule.OllamaModels {
				// Remove existing entry if present
				status.Ollamas = removeOllamaStatus(status.Ollamas, model, blueprint, osName)

				// Add new entry
				status.Ollamas = append(status.Ollamas, OllamaStatus{
					Model:       model,
					InstalledAt: time.Now().Format(time.RFC3339),
					Blueprint:   blueprint,
					OS:          osName,
				})
			}
		}
	case "uninstall":
		for _, model := range h.Rule.OllamaModels {
			status.Ollamas = removeOllamaStatus(status.Ollamas, model, blueprint, osName)
		}
	}

	return nil
}

// ensureOllamaInstalled ensures ollama is installed on the system
func (h *OllamaHandler) ensureOllamaInstalled() error {
	if h.isOllamaInstalled() {
		return nil
	}

	ollamaInstallMutex.Lock()
	defer ollamaInstallMutex.Unlock()

	// Double-check after acquiring lock
	if h.isOllamaInstalled() {
		return nil
	}

	installCmd := "curl -fsSL https://ollama.com/install.sh | sh"
	if _, err := executeCommandWithCache(installCmd); err != nil {
		return fmt.Errorf("failed to install ollama: %w", err)
	}

	return nil
}

// isOllamaInstalled checks if ollama is installed
func (h *OllamaHandler) isOllamaInstalled() bool {
	cmd := exec.Command("which", "ollama")
	return cmd.Run() == nil
}

// buildCommand builds the install command based on model list
func (h *OllamaHandler) buildCommand() string {
	if len(h.Rule.OllamaModels) == 0 {
		return ""
	}

	var parts []string
	for _, model := range h.Rule.OllamaModels {
		parts = append(parts, fmt.Sprintf("ollama pull %s", model))
	}
	return strings.Join(parts, " && ")
}

// buildUninstallCommand builds the uninstall command based on model list
func (h *OllamaHandler) buildUninstallCommand() string {
	if len(h.Rule.OllamaModels) == 0 {
		return ""
	}

	var parts []string
	for _, model := range h.Rule.OllamaModels {
		parts = append(parts, fmt.Sprintf("ollama rm %s", model))
	}
	return strings.Join(parts, " && ")
}

// NeedsSudo returns false since ollama model operations don't need sudo
func (h *OllamaHandler) NeedsSudo() bool {
	return false
}

// GetDependencyKey returns the unique key for this rule in dependency resolution
func (h *OllamaHandler) GetDependencyKey() string {
	fallback := "ollama"
	if len(h.Rule.OllamaModels) > 0 {
		fallback = h.Rule.OllamaModels[0]
	}
	return getDependencyKey(h.Rule, fallback)
}

// GetDisplayDetails returns the models to display during execution
func (h *OllamaHandler) GetDisplayDetails(isUninstall bool) string {
	return strings.Join(h.Rule.OllamaModels, ", ")
}

// DisplayInfo displays handler-specific information
func (h *OllamaHandler) DisplayInfo() {
	if h.Rule.Action == "uninstall" {
		fmt.Printf("  %s\n", ui.FormatDim(fmt.Sprintf("Models: [%s]", strings.Join(h.Rule.OllamaModels, ", "))))
	} else {
		fmt.Printf("  %s\n", ui.FormatInfo(fmt.Sprintf("Models: [%s]", strings.Join(h.Rule.OllamaModels, ", "))))
	}
}

// DisplayStatusFromStatus displays ollama handler status from Status object
func (h *OllamaHandler) DisplayStatusFromStatus(status *Status) {
	if status == nil || status.Ollamas == nil {
		return
	}
	h.DisplayStatus(status.Ollamas)
}

// DisplayStatus displays installed ollama model status information
func (h *OllamaHandler) DisplayStatus(ollamas []OllamaStatus) {
	if len(ollamas) == 0 {
		return
	}

	fmt.Printf("\n%s\n", ui.FormatHighlight("Installed Ollama Models:"))
	for _, o := range ollamas {
		t, err := time.Parse(time.RFC3339, o.InstalledAt)
		var timeStr string
		if err == nil {
			timeStr = t.Format("2006-01-02 15:04:05")
		} else {
			timeStr = o.InstalledAt
		}

		fmt.Printf("  %s %s (%s) [%s, %s]\n",
			ui.FormatSuccess("●"),
			ui.FormatInfo(o.Model),
			ui.FormatDim(timeStr),
			ui.FormatDim(o.OS),
			ui.FormatDim(abbreviateBlueprintPath(o.Blueprint)),
		)
	}
}

// FindUninstallRules compares ollama status against current rules and returns uninstall rules
func (h *OllamaHandler) FindUninstallRules(status *Status, currentRules []parser.Rule, blueprintFile, osName string) []parser.Rule {
	normalizedBlueprint := normalizePath(blueprintFile)

	// Build set of current model names from ollama rules
	currentModels := make(map[string]bool)
	for _, rule := range currentRules {
		if rule.Action == "ollama" {
			for _, model := range rule.OllamaModels {
				currentModels[model] = true
			}
		}
	}

	// Find models to uninstall (in status but not in current rules)
	var modelsToUninstall []string
	if status.Ollamas != nil {
		for _, o := range status.Ollamas {
			normalizedStatusBlueprint := normalizePath(o.Blueprint)
			if normalizedStatusBlueprint == normalizedBlueprint && o.OS == osName && !currentModels[o.Model] {
				modelsToUninstall = append(modelsToUninstall, o.Model)
			}
		}
	}

	var rules []parser.Rule
	if len(modelsToUninstall) > 0 {
		rules = append(rules, parser.Rule{
			Action:       "uninstall",
			OllamaModels: modelsToUninstall,
			OSList:       []string{osName},
		})
	}
	return rules
}

// removeOllamaStatus removes an ollama model from the status ollamas list
func removeOllamaStatus(ollamas []OllamaStatus, model string, blueprint string, osName string) []OllamaStatus {
	var result []OllamaStatus
	normalizedBlueprint := normalizePath(blueprint)
	for _, o := range ollamas {
		normalizedStoredBlueprint := normalizePath(o.Blueprint)
		if o.Model != model || normalizedStoredBlueprint != normalizedBlueprint || o.OS != osName {
			result = append(result, o)
		}
	}
	return result
}
