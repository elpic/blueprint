package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/elpic/blueprint/internal"
	cryptopkg "github.com/elpic/blueprint/internal/crypto"
	"github.com/elpic/blueprint/internal/parser"
	"github.com/elpic/blueprint/internal/ui"
)

// AuthorizedKeysHandler manages ~/.ssh/authorized_keys entries
type AuthorizedKeysHandler struct {
	BaseHandler
	passwordCache map[string]string
}

// NewAuthorizedKeysHandler creates a new authorized_keys handler
func NewAuthorizedKeysHandler(rule parser.Rule, basePath string, passwordCache map[string]string) *AuthorizedKeysHandler {
	return &AuthorizedKeysHandler{
		BaseHandler: BaseHandler{
			Rule:     rule,
			BasePath: basePath,
		},
		passwordCache: passwordCache,
	}
}

func (h *AuthorizedKeysHandler) readKeyContent() (string, error) {
	if h.Rule.AuthorizedKeysEncrypted != "" {
		passwordID := h.Rule.AuthorizedKeysPasswordID
		if passwordID == "" {
			passwordID = "default"
		}
		password, ok := h.passwordCache[passwordID]
		if !ok {
			return "", fmt.Errorf("no password cached for password-id: %s", passwordID)
		}
		sourceFile := h.resolveFilePath(h.Rule.AuthorizedKeysEncrypted)
		encryptedData, err := os.ReadFile(sourceFile)
		if err != nil {
			return "", fmt.Errorf("failed to read encrypted file: %w", err)
		}
		decrypted, err := cryptopkg.DecryptFile(encryptedData, password)
		if err != nil {
			return "", fmt.Errorf("decryption failed: %w", err)
		}
		return string(decrypted), nil
	}

	filePath := expandPath(h.Rule.AuthorizedKeysFile)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read key file: %w", err)
	}
	return string(data), nil
}

func parseKeyLines(content string) []string {
	var keys []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		keys = append(keys, line)
	}
	return keys
}

func authorizedKeysFile(create bool) (string, error) {
	sshPath, err := sshDir(create)
	if err != nil {
		return "", err
	}
	authKeysPath := filepath.Join(sshPath, "authorized_keys")
	if create {
		if _, err := os.Stat(authKeysPath); os.IsNotExist(err) {
			if err := os.WriteFile(authKeysPath, []byte{}, internal.FilePermission); err != nil { // #nosec G703 -- path is always ~/.ssh/authorized_keys, constructed internally // #nosec G703 -- path is always ~/.ssh/authorized_keys
				return "", fmt.Errorf("failed to create authorized_keys file: %w", err)
			}
		}
	} else {
		if _, err := os.Stat(authKeysPath); os.IsNotExist(err) {
			return "", fmt.Errorf("authorized_keys file does not exist")
		}
	}
	return authKeysPath, nil
}

// Up adds key lines from the source to ~/.ssh/authorized_keys
func (h *AuthorizedKeysHandler) Up() (string, error) {
	content, err := h.readKeyContent()
	if err != nil {
		return "", err
	}

	authKeysPath, err := authorizedKeysFile(true)
	if err != nil {
		return "", err
	}

	existing, err := os.ReadFile(authKeysPath)
	if err != nil {
		return "", fmt.Errorf("failed to read authorized_keys: %w", err)
	}

	existingLines := make(map[string]bool)
	for _, line := range strings.Split(string(existing), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			existingLines[line] = true
		}
	}

	newKeys := parseKeyLines(content)
	added := 0

	f, err := os.OpenFile(authKeysPath, os.O_APPEND|os.O_WRONLY, internal.FilePermission) // #nosec G703 -- path is always ~/.ssh/authorized_keys, constructed internally
	if err != nil {
		return "", fmt.Errorf("failed to open authorized_keys for writing: %w", err)
	}
	defer f.Close() //nolint:errcheck

	for _, key := range newKeys {
		if !existingLines[key] {
			if _, err := fmt.Fprintf(f, "%s\n", key); err != nil {
				return "", fmt.Errorf("failed to write key to authorized_keys: %w", err)
			}
			added++
		}
	}

	return fmt.Sprintf("Added %d key(s) to authorized_keys", added), nil
}

// Down removes key lines from ~/.ssh/authorized_keys that came from this rule
func (h *AuthorizedKeysHandler) Down() (string, error) {
	content, err := h.readKeyContent()
	if err != nil {
		return "", fmt.Errorf("failed to read key source for removal: %w", err)
	}

	authKeysPath, err := authorizedKeysFile(false)
	if err != nil {
		return "", err
	}

	keysToRemove := make(map[string]bool)
	for _, key := range parseKeyLines(content) {
		keysToRemove[key] = true
	}

	existing, err := os.ReadFile(authKeysPath)
	if err != nil {
		return "", fmt.Errorf("failed to read authorized_keys: %w", err)
	}

	var kept []string
	removed := 0
	for _, line := range strings.Split(string(existing), "\n") {
		trimmed := strings.TrimSpace(line)
		if keysToRemove[trimmed] {
			removed++
		} else {
			kept = append(kept, line)
		}
	}

	newContent := strings.Join(kept, "\n")
	if err := os.WriteFile(authKeysPath, []byte(newContent), internal.FilePermission); err != nil { // #nosec G703 -- authKeysPath is always ~/.ssh/authorized_keys, constructed internally
		return "", fmt.Errorf("failed to rewrite authorized_keys: %w", err)
	}

	return fmt.Sprintf("Removed %d key(s) from authorized_keys", removed), nil
}

// GetCommand returns a description of what will be executed
func (h *AuthorizedKeysHandler) GetCommand() string {
	if h.Rule.AuthorizedKeysEncrypted != "" {
		return fmt.Sprintf("decrypt %s >> ~/.ssh/authorized_keys", h.Rule.AuthorizedKeysEncrypted)
	}
	return fmt.Sprintf("cat %s >> ~/.ssh/authorized_keys", h.Rule.AuthorizedKeysFile)
}

// UpdateStatus updates the status after adding or removing authorized keys
func (h *AuthorizedKeysHandler) UpdateStatus(status *Status, records []ExecutionRecord, blueprint string, osName string) error {
	blueprint = normalizeBlueprint(blueprint)

	source := h.Rule.AuthorizedKeysFile
	if h.Rule.AuthorizedKeysEncrypted != "" {
		source = h.Rule.AuthorizedKeysEncrypted
	}

	if h.Rule.Action == "authorized_keys" {
		cmd := h.GetCommand()
		_, commandExecuted := commandSuccessfullyExecuted(cmd, records)
		if commandExecuted {
			status.AuthorizedKeys = removeAuthorizedKeysStatus(status.AuthorizedKeys, source, blueprint, osName)
			status.AuthorizedKeys = append(status.AuthorizedKeys, AuthorizedKeysStatus{
				Source:    source,
				AddedAt:   time.Now().Format(time.RFC3339),
				Blueprint: blueprint,
				OS:        osName,
			})
		}
	} else if h.Rule.Action == "uninstall" && DetectRuleType(h.Rule) == "authorized_keys" {
		status.AuthorizedKeys = removeAuthorizedKeysStatus(status.AuthorizedKeys, source, blueprint, osName)
	}

	return nil
}

// DisplayInfo displays handler-specific information
func (h *AuthorizedKeysHandler) DisplayInfo() {
	formatFunc := ui.FormatInfo
	if h.Rule.Action == "uninstall" {
		formatFunc = ui.FormatDim
	}

	if h.Rule.AuthorizedKeysEncrypted != "" {
		fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Encrypted: %s", h.Rule.AuthorizedKeysEncrypted)))
		if h.Rule.AuthorizedKeysPasswordID != "" {
			fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("Password ID: %s", h.Rule.AuthorizedKeysPasswordID)))
		}
	} else {
		fmt.Printf("  %s\n", formatFunc(fmt.Sprintf("File: %s", h.Rule.AuthorizedKeysFile)))
	}
}

// GetDependencyKey returns the unique key for this rule in dependency resolution
func (h *AuthorizedKeysHandler) GetDependencyKey() string {
	source := h.Rule.AuthorizedKeysFile
	if h.Rule.AuthorizedKeysEncrypted != "" {
		source = h.Rule.AuthorizedKeysEncrypted
	}
	return getDependencyKey(h.Rule, source)
}

// GetDisplayDetails returns the source path to display during execution
func (h *AuthorizedKeysHandler) GetDisplayDetails(isUninstall bool) string {
	if h.Rule.AuthorizedKeysEncrypted != "" {
		return h.Rule.AuthorizedKeysEncrypted
	}
	return h.Rule.AuthorizedKeysFile
}

// GetState returns handler-specific state as key-value pairs
func (h *AuthorizedKeysHandler) GetState(isUninstall bool) map[string]string {
	return map[string]string{
		"summary": h.GetDisplayDetails(isUninstall),
		"source":  h.GetDisplayDetails(isUninstall),
	}
}

// FindUninstallRules compares authorized_keys status against current rules and returns uninstall rules
func (h *AuthorizedKeysHandler) FindUninstallRules(status *Status, currentRules []parser.Rule, blueprintFile, osName string) []parser.Rule {
	normalizedBlueprint := normalizeBlueprint(blueprintFile)

	currentSources := make(map[string]bool)
	for _, rule := range currentRules {
		if rule.Action == "authorized_keys" {
			source := rule.AuthorizedKeysFile
			if rule.AuthorizedKeysEncrypted != "" {
				source = rule.AuthorizedKeysEncrypted
			}
			if source != "" {
				currentSources[source] = true
			}
		}
	}

	var rules []parser.Rule
	if status.AuthorizedKeys != nil {
		for _, ak := range status.AuthorizedKeys {
			normalizedStatusBlueprint := normalizeBlueprint(ak.Blueprint)
			if normalizedStatusBlueprint == normalizedBlueprint && ak.OS == osName && !currentSources[ak.Source] {
				rules = append(rules, parser.Rule{
					Action:                  "uninstall",
					AuthorizedKeysFile:      ak.Source,
					AuthorizedKeysEncrypted: "",
					OSList:                  []string{osName},
				})
			}
		}
	}

	return rules
}

// IsInstalled returns true if the source is recorded in status AND each key line is present in authorized_keys
func (h *AuthorizedKeysHandler) IsInstalled(status *Status, blueprintFile, osName string) bool {
	normalizedBlueprint := normalizeBlueprint(blueprintFile)

	source := h.Rule.AuthorizedKeysFile
	if h.Rule.AuthorizedKeysEncrypted != "" {
		source = h.Rule.AuthorizedKeysEncrypted
	}

	inStatus := false
	for _, ak := range status.AuthorizedKeys {
		if ak.Source == source && normalizeBlueprint(ak.Blueprint) == normalizedBlueprint && ak.OS == osName {
			inStatus = true
			break
		}
	}
	if !inStatus {
		return false
	}

	content, err := h.readKeyContent()
	if err != nil {
		return false
	}

	authKeysPath, err := authorizedKeysFile(false)
	if err != nil {
		return false
	}

	existing, err := os.ReadFile(authKeysPath)
	if err != nil {
		return false
	}

	existingLines := make(map[string]bool)
	for _, line := range strings.Split(string(existing), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			existingLines[line] = true
		}
	}

	for _, key := range parseKeyLines(content) {
		if !existingLines[key] {
			return false
		}
	}

	return true
}

// resolveFilePath resolves the file path, checking multiple locations
func (h *AuthorizedKeysHandler) resolveFilePath(file string) string {
	if filepath.IsAbs(file) {
		return expandPath(file)
	}

	basePath := h.BasePath
	if basePath != "" {
		candidate := filepath.Join(basePath, file)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	if _, err := os.Stat(file); err == nil {
		return file
	}

	if len(file) > 0 && file[0] == '~' {
		return expandPath(file)
	}

	return file
}

// DisplayStatusFromStatus displays authorized_keys status from Status object
func (h *AuthorizedKeysHandler) DisplayStatusFromStatus(status *Status) {
	if status == nil || len(status.AuthorizedKeys) == 0 {
		return
	}

	fmt.Printf("\n%s\n", ui.FormatHighlight("Authorized Keys:"))
	for _, ak := range status.AuthorizedKeys {
		t, err := time.Parse(time.RFC3339, ak.AddedAt)
		var timeStr string
		if err == nil {
			timeStr = t.Format("2006-01-02 15:04:05")
		} else {
			timeStr = ak.AddedAt
		}

		fmt.Printf("  %s %s (%s) [%s, %s]\n",
			ui.FormatSuccess("●"),
			ui.FormatInfo(ak.Source),
			ui.FormatDim(timeStr),
			ui.FormatDim(ak.OS),
			ui.FormatDim(abbreviateBlueprintPath(ak.Blueprint)),
		)
	}
}
