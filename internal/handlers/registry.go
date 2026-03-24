package handlers

import (
	"strings"
	"sync"

	"github.com/elpic/blueprint/internal/parser"
)

// HandlerFactory creates a Handler for a given rule.
type HandlerFactory func(rule parser.Rule, basePath string, passwordCache map[string]string) Handler

// RuleKeyFunc returns the dedup/dependency key for a rule when rule.ID is empty.
type RuleKeyFunc func(rule parser.Rule) string

// DetectFunc returns true if an uninstall rule was originally this action type.
type DetectFunc func(rule parser.Rule) bool

// SummaryFunc returns a human-readable summary for display/diff output.
type SummaryFunc func(rule parser.Rule) string

// OrphanIndexFunc indexes resource keys from a rule for orphan detection.
// Call index(key) for each key this rule contributes.
type OrphanIndexFunc func(rule parser.Rule, index func(key string))

// ActionDef captures everything the system needs to know about one action type.
type ActionDef struct {
	Name                       string
	Prefix                     string // e.g. "install ", "clone ", "sudoers"
	NewHandler                 HandlerFactory
	RuleKey                    RuleKeyFunc
	Detect                     DetectFunc
	Summary                    SummaryFunc
	OrphanIndex                OrphanIndexFunc
	ExcludeFromOrphanDetection bool
	// IsAlias marks this entry as an alias for another action. Aliases are
	// excluded from GetStatusProviderHandlers to avoid duplicate status checks.
	IsAlias bool
}

var (
	registryMu      sync.RWMutex
	registryByName  = map[string]*ActionDef{}
	registryOrdered []*ActionDef
)

// RegisterAction adds an ActionDef to the global registry.
// Panics on duplicate names to catch wiring bugs at startup.
func RegisterAction(def ActionDef) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, exists := registryByName[def.Name]; exists {
		panic("registry: duplicate action: " + def.Name)
	}
	d := def
	registryByName[def.Name] = &d
	registryOrdered = append(registryOrdered, &d)
}

// GetAction returns the ActionDef for the given action name, or nil.
func GetAction(name string) *ActionDef {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return registryByName[name]
}

// AllActions returns all registered ActionDefs in registration order.
func AllActions() []*ActionDef {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]*ActionDef, len(registryOrdered))
	copy(out, registryOrdered)
	return out
}

// findActionByPrefix returns the ActionDef whose Prefix matches line (longest match wins).
func findActionByPrefix(line string) *ActionDef {
	registryMu.RLock()
	defer registryMu.RUnlock()
	var best *ActionDef
	for _, def := range registryOrdered {
		if def.Prefix != "" && strings.HasPrefix(line, def.Prefix) {
			if best == nil || len(def.Prefix) > len(best.Prefix) {
				best = def
			}
		}
	}
	return best
}
