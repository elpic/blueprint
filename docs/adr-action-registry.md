## ADR: Action Registry -- Eliminate Hardcoded Action Lists

**Date:** 2026-03-24
**Status:** Proposed

---

### Context

Blueprint is a Go CLI tool that applies system configuration from `.bp` files. It currently supports 19 action types: `install`, `uninstall`, `clone`, `decrypt`, `mkdir`, `known_hosts`, `gpg-key`, `asdf`, `mise`, `sudoers`, `homebrew`, `ollama`, `download`, `run`, `run-sh`, `dotfiles`, `schedule`, `shell`, `authorized_keys`.

Adding a new action today requires touching **8 distinct locations** across the codebase, each with a hardcoded enumeration of action types. This is error-prone, violates the Open/Closed Principle, and creates a high coordination cost for contributors.

This ADR proposes replacing all hardcoded action enumerations with a single **Action Registry** that each handler self-registers into, so that adding a new action requires changes in exactly one place: the new handler file itself.

---

### Catalog of Hardcoded Action Enumerations

Below is an exhaustive inventory of every location where action types are enumerated. Each is a maintenance burden when adding a new action.

#### Site 1: Parser switch -- `parseContent()`
**File:** `internal/parser/parser.go`, lines 236-275
**Type:** `switch` / `case` chain with `strings.HasPrefix`
**Count:** 19 branches (one per action keyword)
```
case strings.HasPrefix(line, "install "):
case strings.HasPrefix(line, "clone "):
case strings.HasPrefix(line, "mise"):
...
```

#### Site 2: Handler factory -- `NewHandler()`
**File:** `internal/handlers/handlers.go`, lines 608-648
**Type:** `switch action` with 18 cases
```
switch action {
case "install":  return NewInstallHandler(...)
case "clone":    return NewCloneHandler(...)
...
}
```

#### Site 3: Status provider list -- `GetStatusProviderHandlers()`
**File:** `internal/handlers/handlers.go`, lines 653-674
**Type:** Hardcoded slice literal of 18 handler constructors
```
return []Handler{
    NewInstallHandlerLegacy(parser.Rule{}, ""),
    NewCloneHandlerLegacy(parser.Rule{}, ""),
    ...
}
```

#### Site 4: Rule key resolver -- `RuleKey()`
**File:** `internal/handlers/handlers.go`, lines 468-523
**Type:** `switch rule.Action` with 16 cases
```
switch rule.Action {
case "install", "uninstall": ...
case "clone": return rule.ClonePath
...
}
```

#### Site 5: Uninstall type detection -- `DetectRuleType()`
**File:** `internal/handlers/handlers.go`, lines 534-590
**Type:** 17-branch `if` chain checking rule fields
```
if len(rule.Packages) > 0 { return "install" }
if rule.CloneURL != "" { return "clone" }
...
```

#### Site 6: Display summary -- `Rule.DisplaySummary()`
**File:** `internal/parser/parser.go`, lines 102-150
**Type:** `switch r.Action` with 17 cases
```
switch r.Action {
case "install", "uninstall": ...
case "clone": return r.CloneURL + " -> " + r.ClonePath
...
}
```

#### Site 7: Orphan detection resource indexing -- `checkOrphansWithLoader()` / `rulesFor()`
**File:** `internal/engine/doctor.go`, lines 195-218
**Type:** `switch r.Action` with 10 cases for indexing resource keys
```
switch r.Action {
case "clone":     rs[r.ClonePath] = true
case "decrypt":   rs[r.DecryptPath] = true
...
}
```

#### Site 8: Status struct and entry iteration -- `AllEntries()` / `FilterEntries()`
**File:** `internal/handlers/handlers.go`, lines 272-292 (Status struct), 297-351 (AllEntries), 745-763 (FilterEntries)
**Type:** Hardcoded struct fields and iteration across 17 typed slices
```
type Status struct {
    Packages       []PackageStatus
    Clones         []CloneStatus
    ...
}
```

**Total: 8 major sites, each with 10-19 branches/entries that must be updated per new action.**

---

### Current Architecture

```
                    +---------------------+
                    |   parser/parser.go  |
                    |  parseContent()     |
                    |  19-branch switch   |
                    |  DisplaySummary()   |
                    |  17-branch switch   |
                    +--------+------------+
                             |
                             | []parser.Rule
                             v
                    +---------------------+
                    |   engine/engine.go  |
                    |   engine/command.go  |
                    +--------+------------+
                             |
                             | calls NewHandler()
                             v
                    +------------------------+
                    | handlers/handlers.go   |
                    |  NewHandler()           |
                    |   18-branch switch      |
                    |  GetStatusProviders()   |
                    |   18-item list          |
                    |  RuleKey()              |
                    |   16-branch switch      |
                    |  DetectRuleType()       |
                    |   17-branch if-chain    |
                    |  AllEntries()           |
                    |   17-slice iteration    |
                    |  FilterEntries()        |
                    |   17-slice filter       |
                    +------------------------+
                             |
                    +--------+--------+--------+--- ...
                    |        |        |        |
                  install  clone   decrypt   run   (18 handler files)
```

Each box containing a multi-branch switch/list is a maintenance choke point. Adding "action X" means touching all of them.

---

### Proposed Architecture

```
                    +---------------------+
                    |   parser/parser.go  |
                    |  parseContent()     |
                    |  registry.Parse()   |  <-- looks up parser func from registry
                    |  DisplaySummary()   |
                    |  registry.Summary() |  <-- looks up summary func from registry
                    +--------+------------+
                             |
                             | []parser.Rule
                             v
                    +---------------------+
                    |   engine/engine.go  |
                    |   engine/command.go  |
                    +--------+------------+
                             |
                             | registry.NewHandler()
                             v
                    +--------------------------+
                    | registry/registry.go     |
                    |  map[string]ActionDef     |  <-- THE single source of truth
                    |  Register()               |
                    |  NewHandler()              |
                    |  RuleKey()                 |
                    |  DetectRuleType()          |
                    |  AllStatusSliceNames()     |
                    +--------------------------+
                             ^
                    +--------+--------+--------+--- ...
                    |        |        |        |
                  install  clone   decrypt   run   (each calls Register in init())
```

The registry is the **single registration point**. Each handler file calls `Register()` in an `init()` function. No central file enumerates all action types.

---

### Decision

Introduce an `internal/registry` package with a global `ActionRegistry` that each handler registers into via `init()`. The registry entry (`ActionDef`) captures all the metadata currently spread across the 8 hardcoded sites.

---

### Detailed Design

#### 1. The `ActionDef` Interface and Registry

```go
// internal/registry/registry.go
package registry

import (
    "fmt"
    "sync"

    "github.com/elpic/blueprint/internal/parser"
)

// HandlerFactory creates a handler for a rule. basePath and passwordCache
// are passed through from the engine.
type HandlerFactory func(rule parser.Rule, basePath string, passwordCache map[string]string) Handler

// ParseFunc parses a single line (after stripping the action keyword prefix)
// and returns a Rule, or an error.
type ParseFunc func(line string) (*parser.Rule, error)

// RuleKeyFunc returns the dependency/dedup key for a rule of this action type
// when rule.ID is empty. Receives the full Rule.
type RuleKeyFunc func(rule parser.Rule) string

// DetectFunc returns true if the given rule (whose Action is "uninstall")
// was originally this action type, based on which fields are populated.
type DetectFunc func(rule parser.Rule) bool

// SummaryFunc returns a short human-readable summary for diff/plan output.
type SummaryFunc func(rule parser.Rule) string

// OrphanIndexFunc populates a rule-set map with the resource keys that
// a rule of this action type contributes to orphan detection.
// This replaces the per-action switch in doctor.go rulesFor().
type OrphanIndexFunc func(rule parser.Rule, index func(key string))

// ActionDef captures everything the system needs to know about an action type.
type ActionDef struct {
    // Name is the action keyword (e.g. "install", "clone", "dotfiles").
    Name string

    // Prefix is the string prefix used in .bp files to identify this action.
    // For most actions this equals Name (e.g. "install "), but some differ
    // (e.g. "gpg-key " for the "gpg-key" action). The parser uses this
    // to dispatch lines. Must include trailing space if the action requires
    // arguments (e.g. "install "), or be exact for bare keywords (e.g. "sudoers").
    Prefix string

    // PrefixMatchMode controls how the prefix is matched against a line.
    // "HasPrefix" (default): strings.HasPrefix(line, Prefix)
    // This handles both "install foo" and bare-keyword "sudoers" forms.
    PrefixMatchMode string

    // NewHandler creates a handler instance for a matched rule.
    NewHandler HandlerFactory

    // Parse parses a line (full line, prefix included) into a Rule.
    Parse ParseFunc

    // RuleKey returns the dedup/dependency key when rule.ID is empty.
    RuleKey RuleKeyFunc

    // Detect returns true if an "uninstall" rule was originally this action type.
    Detect DetectFunc

    // Summary returns a human-readable summary for display.
    Summary SummaryFunc

    // OrphanIndex populates orphan-detection keys for a rule of this type.
    // Optional -- if nil, only the RuleKey is indexed.
    OrphanIndex OrphanIndexFunc

    // ExcludeFromOrphanDetection, if true, means status entries of this type
    // are skipped during orphan checks (e.g. asdf, mise, sudoers).
    ExcludeFromOrphanDetection bool

    // StatusSliceAccessor provides access to the typed slice in Status for
    // AllEntries / FilterEntries. See "Status struct" section below.
    StatusSliceAccessor StatusSliceAccessor
}
```

#### 2. The Registry Itself

```go
// internal/registry/registry.go (continued)

var (
    mu       sync.RWMutex
    actions  = map[string]*ActionDef{}     // keyed by action name
    prefixes []*ActionDef                   // ordered for parse dispatch
)

// Register adds an action definition to the registry.
// Panics on duplicate names (catches wiring bugs at startup).
func Register(def ActionDef) {
    mu.Lock()
    defer mu.Unlock()
    if _, exists := actions[def.Name]; exists {
        panic(fmt.Sprintf("registry: duplicate action %q", def.Name))
    }
    actions[def.Name] = &def
    prefixes = append(prefixes, &def)
}

// Get returns the ActionDef for the given action name, or nil.
func Get(name string) *ActionDef {
    mu.RLock()
    defer mu.RUnlock()
    return actions[name]
}

// All returns all registered ActionDefs in registration order.
func All() []*ActionDef {
    mu.RLock()
    defer mu.RUnlock()
    out := make([]*ActionDef, len(prefixes))
    copy(out, prefixes)
    return out
}

// FindByPrefix returns the ActionDef whose Prefix matches the given line,
// or nil if no match. Longer prefixes are checked first to avoid ambiguity
// (e.g. "run-sh " before "run ").
func FindByPrefix(line string) *ActionDef {
    mu.RLock()
    defer mu.RUnlock()
    // Sort is done once at registration time; here we just scan.
    // With ~19 actions, linear scan is negligible.
    var best *ActionDef
    for _, def := range prefixes {
        if strings.HasPrefix(line, def.Prefix) {
            if best == nil || len(def.Prefix) > len(best.Prefix) {
                best = def
            }
        }
    }
    return best
}

// DetectRuleType determines the original action type for an "uninstall" rule
// by checking each registered action's Detect function.
func DetectRuleType(rule parser.Rule) string {
    mu.RLock()
    defer mu.RUnlock()
    for _, def := range prefixes {
        if def.Detect != nil && def.Detect(rule) {
            return def.Name
        }
    }
    return ""
}

// RuleKey returns the dependency/dedup key for a rule.
func RuleKey(rule parser.Rule) string {
    if rule.ID != "" {
        return rule.ID
    }
    action := rule.Action
    if action == "uninstall" {
        action = DetectRuleType(rule)
    }
    def := Get(action)
    if def != nil && def.RuleKey != nil {
        return def.RuleKey(rule)
    }
    return rule.Action
}

// NewHandler creates a handler for the given rule using the registry.
func NewHandler(rule parser.Rule, basePath string, passwordCache map[string]string) Handler {
    action := rule.Action
    if action == "uninstall" {
        action = DetectRuleType(rule)
        if action == "" {
            return nil
        }
    }
    def := Get(action)
    if def == nil {
        return nil
    }
    return def.NewHandler(rule, basePath, passwordCache)
}

// DisplaySummary returns a human-readable summary of the rule.
func DisplaySummary(rule parser.Rule) string {
    def := Get(rule.Action)
    if def != nil && def.Summary != nil {
        return def.Summary(rule)
    }
    return rule.Action
}
```

#### 3. Handler Interface (Unchanged)

The existing `Handler` interface and optional interfaces (`StatusProvider`, `DisplayProvider`, `StateProvider`, `KeyProvider`, `SudoAwareHandler`, `RecordAware`) remain exactly as they are. No handler implementation changes its method signatures.

```go
// These stay in internal/handlers/handlers.go -- no changes.
type Handler interface {
    Up() (string, error)
    Down() (string, error)
    UpdateStatus(status *Status, records []ExecutionRecord, blueprint string, osName string) error
    GetCommand() string
    DisplayInfo()
    IsInstalled(status *Status, blueprintFile, osName string) bool
}
```

The `Handler` type alias is re-exported from the registry package so the engine imports from one place:

```go
// internal/registry/registry.go
// Handler is re-exported from handlers for convenience.
type Handler = handlers.Handler
```

#### 4. Self-Registration via `init()`

Each handler file registers itself. Here is the concrete example for `install.go`:

```go
// internal/handlers/install.go

func init() {
    registry.Register(registry.ActionDef{
        Name:   "install",
        Prefix: "install ",

        NewHandler: func(rule parser.Rule, basePath string, _ map[string]string) registry.Handler {
            return NewInstallHandler(rule, basePath, platform.NewContainer())
        },

        Parse: func(line string) (*parser.Rule, error) {
            return parseInstallRule(line)
        },

        RuleKey: func(rule parser.Rule) string {
            if len(rule.Packages) > 0 {
                return rule.Packages[0].Name
            }
            return "install"
        },

        Detect: func(rule parser.Rule) bool {
            return len(rule.Packages) > 0
        },

        Summary: func(rule parser.Rule) string {
            names := make([]string, len(rule.Packages))
            for i, p := range rule.Packages {
                names[i] = p.Name
            }
            return strings.Join(names, ", ")
        },

        OrphanIndex: func(rule parser.Rule, index func(string)) {
            for _, pkg := range rule.Packages {
                index(pkg.Name)
            }
        },
    })
}
```

Here is a second example for `clone.go`:

```go
// internal/handlers/clone.go

func init() {
    registry.Register(registry.ActionDef{
        Name:   "clone",
        Prefix: "clone ",

        NewHandler: func(rule parser.Rule, basePath string, _ map[string]string) registry.Handler {
            return NewCloneHandler(rule, basePath, platform.NewContainer())
        },

        Parse: func(line string) (*parser.Rule, error) {
            return parseCloneRule(line)
        },

        RuleKey: func(rule parser.Rule) string {
            return rule.ClonePath
        },

        Detect: func(rule parser.Rule) bool {
            return rule.CloneURL != ""
        },

        Summary: func(rule parser.Rule) string {
            return rule.CloneURL + " -> " + rule.ClonePath
        },

        OrphanIndex: func(rule parser.Rule, index func(string)) {
            index(rule.ClonePath)
        },
    })
}
```

And for a bare-keyword action like `sudoers`:

```go
// internal/handlers/sudoers.go

func init() {
    registry.Register(registry.ActionDef{
        Name:   "sudoers",
        Prefix: "sudoers",  // no trailing space -- bare keyword allowed

        NewHandler: func(rule parser.Rule, basePath string, _ map[string]string) registry.Handler {
            return NewSudoersHandler(rule, basePath)
        },

        Parse: func(line string) (*parser.Rule, error) {
            return parseSudoersRule(line)
        },

        RuleKey: func(rule parser.Rule) string {
            return "sudoers"
        },

        Detect: func(rule parser.Rule) bool {
            return rule.SudoersUser != ""
        },

        Summary: func(rule parser.Rule) string {
            return rule.SudoersUser
        },

        ExcludeFromOrphanDetection: true,
    })
}
```

#### 5. How Each Call Site Changes

##### Site 1: Parser `parseContent()` -- Replace switch with registry lookup

**Before (19-branch switch):**
```go
switch {
case strings.HasPrefix(line, "install "):
    rule, err = parseInstallRule(line)
case strings.HasPrefix(line, "clone "):
    rule, err = parseCloneRule(line)
...
default:
    return nil, fmt.Errorf("line %d: unknown directive %q", lineNum+1, line)
}
```

**After (registry lookup):**
```go
def := registry.FindByPrefix(line)
if def == nil {
    return nil, fmt.Errorf("line %d: unknown directive %q", lineNum+1, line)
}
rule, err = def.Parse(line)
```

Note: The parse functions (`parseInstallRule`, `parseCloneRule`, etc.) stay in `parser/parser.go` or can be moved to each handler file. Since parser functions use unexported helpers like `parseFields()`, they initially stay in the parser package and are referenced by the `ActionDef.Parse` field. If desired, they can be moved later when `parseFields` is exported.

**Import consideration:** The parser package cannot import `internal/handlers` (it would create a cycle: `handlers` imports `parser`). The solution is that the registry package is a new leaf package (`internal/registry`) that both `parser` and `handlers` import. Parse functions stay in `parser/` initially; each handler's `init()` in `handlers/*.go` registers the parse function by importing parser. The parser's `parseContent()` calls `registry.FindByPrefix()` to dispatch.

Actually, to avoid the cycle entirely, the cleanest approach is:

```
internal/registry   -- defines ActionDef, Register(), lookup functions
internal/parser     -- imports registry, calls registry.FindByPrefix() in parseContent()
internal/handlers   -- imports registry AND parser, calls registry.Register() in init()
```

The parse functions are referenced by handlers' `init()` calls but live in `parser/`. This means `handlers` must import `parser` (already does) and the parse functions must be exported. Alternatively, each handler file can define its own parse function locally in the handlers package, duplicating nothing since each parse function is specific to one action. The recommended approach is to **move each parse function into its handler file** (e.g., `parseInstallRule` moves to `install.go`), since the parse function is inherently coupled to the handler's Rule fields. The shared `parseFields()` helper would need to be exported from the parser package or moved to a shared utility package.

##### Site 2: `NewHandler()` -- Delegate to registry

**Before:**
```go
func NewHandler(rule parser.Rule, basePath string, passwordCache map[string]string) Handler {
    switch action {
    case "install": return NewInstallHandler(...)
    ...
    }
}
```

**After:**
```go
func NewHandler(rule parser.Rule, basePath string, passwordCache map[string]string) Handler {
    return registry.NewHandler(rule, basePath, passwordCache)
}
```

Or call `registry.NewHandler()` directly from the engine without the wrapper.

##### Site 3: `GetStatusProviderHandlers()` -- Derive from registry

**Before:**
```go
func GetStatusProviderHandlers() []Handler {
    return []Handler{
        NewInstallHandlerLegacy(parser.Rule{}, ""),
        NewCloneHandlerLegacy(parser.Rule{}, ""),
        ...
    }
}
```

**After:**
```go
func GetStatusProviderHandlers() []Handler {
    var handlers []Handler
    for _, def := range registry.All() {
        h := def.NewHandler(parser.Rule{Action: def.Name}, "", nil)
        if h != nil {
            handlers = append(handlers, h)
        }
    }
    return handlers
}
```

##### Site 4: `RuleKey()` -- Delegate to registry

**Before:**
```go
func RuleKey(rule parser.Rule) string {
    if rule.ID != "" { return rule.ID }
    switch rule.Action {
    case "install", "uninstall": ...
    case "clone": return rule.ClonePath
    ...
    }
}
```

**After:**
```go
func RuleKey(rule parser.Rule) string {
    return registry.RuleKey(rule)
}
```

##### Site 5: `DetectRuleType()` -- Delegate to registry

**Before:**
```go
func DetectRuleType(rule parser.Rule) string {
    if len(rule.Packages) > 0 { return "install" }
    if rule.CloneURL != "" { return "clone" }
    ...
}
```

**After:**
```go
func DetectRuleType(rule parser.Rule) string {
    return registry.DetectRuleType(rule)
}
```

##### Site 6: `Rule.DisplaySummary()` -- Delegate to registry

**Before:**
```go
func (r Rule) DisplaySummary() string {
    switch r.Action {
    case "install", "uninstall": ...
    case "clone": return r.CloneURL + " -> " + r.ClonePath
    ...
    }
}
```

**After:**
```go
func (r Rule) DisplaySummary() string {
    return registry.DisplaySummary(r)
}
```

##### Site 7: Doctor orphan indexing -- Use `OrphanIndex` from registry

**Before (10-case switch in `rulesFor`):**
```go
switch r.Action {
case "clone":     rs[r.ClonePath] = true
case "decrypt":   rs[r.DecryptPath] = true
...
}
```

**After:**
```go
def := registry.Get(r.Action)
if def != nil && def.OrphanIndex != nil {
    def.OrphanIndex(r, func(key string) { rs[key] = true })
}
```

The per-package/formula/model indexing loops that follow the switch also move into each handler's `OrphanIndex` function:

```go
// In install.go's ActionDef:
OrphanIndex: func(rule parser.Rule, index func(string)) {
    for _, pkg := range rule.Packages {
        index(pkg.Name)
    }
},

// In homebrew.go's ActionDef:
OrphanIndex: func(rule parser.Rule, index func(string)) {
    for _, formula := range rule.HomebrewPackages {
        index(formula)
    }
},
```

##### Site 8: Status struct `AllEntries()` / `FilterEntries()` -- Incremental approach

The `Status` struct with its 17 typed slices is the hardest to generalize because its shape **is the `status.json` schema**, which must not change. The typed slices (`Packages []PackageStatus`, `Clones []CloneStatus`, etc.) and their JSON field names are a contract.

**Recommended approach: Leave the Status struct as-is.** The `AllEntries()` and `FilterEntries()` methods are already generic (using the `StatusEntry` interface) and only need updating when a new status slice is added -- which is inherent to adding a new status type. This is acceptable because:

1. It is one location (not 8).
2. The Status struct is a data schema, not dispatch logic.
3. Attempting to make it fully dynamic (e.g., `map[string][]StatusEntry`) would break the JSON contract.

However, we can add a **registration-time hook** so that each handler registers its status slice accessor, and `AllEntries()`/`FilterEntries()` iterate over those registered accessors instead of hardcoding them. This is a Phase 2 optimization described below.

**Phase 2 (optional): StatusSliceAccessor**

```go
// internal/registry/status_accessor.go

// StatusSliceAccessor provides generic access to a typed status slice.
type StatusSliceAccessor struct {
    // Entries returns all StatusEntry pointers from this slice in the given Status.
    Entries func(s *Status) []StatusEntry

    // Filter rebuilds the slice keeping only entries where keep returns true.
    Filter func(s *Status, keep func(StatusEntry) bool)
}
```

Each handler registers one:

```go
// In install.go's init():
StatusSliceAccessor: registry.StatusSliceAccessor{
    Entries: func(s *handlers.Status) []handlers.StatusEntry {
        entries := make([]handlers.StatusEntry, len(s.Packages))
        for i := range s.Packages {
            entries[i] = &s.Packages[i]
        }
        return entries
    },
    Filter: func(s *handlers.Status, keep func(handlers.StatusEntry) bool) {
        s.Packages = filterSlice[handlers.PackageStatus, *handlers.PackageStatus](s.Packages, keep)
    },
},
```

Then `AllEntries()` becomes:

```go
func (s *Status) AllEntries() []StatusEntry {
    var entries []StatusEntry
    for _, def := range registry.All() {
        if def.StatusSliceAccessor.Entries != nil {
            entries = append(entries, def.StatusSliceAccessor.Entries(s)...)
        }
    }
    return entries
}
```

This is a significant change with moderate risk. It can be deferred to Phase 2.

---

### Handling the Parser-Handlers Circular Dependency

The key architectural constraint is:

```
handlers imports parser  (for parser.Rule)
parser   imports ???     (to call registry.FindByPrefix)
```

If the registry lives in `handlers`, then `parser` cannot import it. The solution is a **separate `internal/registry` package** that depends only on `parser` (for `parser.Rule`):

```
Dependency graph (after):

    parser  <---  registry  <---  handlers
      |              ^               |
      |              |               |
      +--------------+               |
      (imports registry              |
       for FindByPrefix)             |
                                     |
    engine ---> registry (for NewHandler, RuleKey, etc.)
           ---> handlers (for Handler interface, Status types)
```

Wait -- `parser` importing `registry` while `registry` imports `parser` is also a cycle. The fix: **`registry` depends only on `parser` for `parser.Rule`**, and **`parser` depends on `registry` for dispatch**. This is a cycle.

**Resolution: Define `Rule` in a shared leaf package.**

Move `parser.Rule` (and `parser.Package`) to `internal/types/rule.go`. Then:

```
internal/types    -- Rule, Package (no dependencies)
internal/registry -- imports types (for Rule in ActionDef signatures)
internal/parser   -- imports types, registry
internal/handlers -- imports types, registry, parser (for parse helpers)
internal/engine   -- imports handlers, registry
```

Alternatively, to minimize disruption: **use a registration callback pattern where the parser does not import the registry at all.** Instead, the parser exposes a hook:

```go
// internal/parser/parser.go

// LineParser is a function that parses a .bp line into a Rule.
type LineParser func(line string) (*Rule, error)

// RegisteredParsers is populated by handler init() functions via the
// registry package. The parser uses this map for dispatch.
// Key: prefix string, Value: parse function.
var RegisteredParsers []struct {
    Prefix string
    Parse  LineParser
}

// RegisterParser is called during init() to register a line parser.
func RegisterParser(prefix string, parse LineParser) {
    RegisteredParsers = append(RegisteredParsers, struct {
        Prefix string
        Parse  LineParser
    }{Prefix: prefix, Parse: parse})
}
```

But then `handlers` needs to import `parser` to call `RegisterParser`, which it already does. And the parse functions are already in `parser`, so handlers just call `parser.RegisterParser("install ", parseInstallRule)` -- but `parseInstallRule` is unexported in parser. This needs the parse functions to be exported or moved.

**Recommended final approach: Move `Rule` to a shared package, put registry there.**

Create `internal/blueprint/types.go`:

```go
package blueprint

type Package struct { ... }  // same as current parser.Package
type Rule struct { ... }     // same as current parser.Rule
```

Then `parser`, `registry`, `handlers`, and `engine` all import `internal/blueprint` for the `Rule` type. The `parser` package keeps its parse functions but works with `blueprint.Rule`. The `registry` package defines `ActionDef` using `blueprint.Rule`. No cycles.

**However**, this is a large diff touching every file that references `parser.Rule`. A more pragmatic approach is:

**Pragmatic approach: Keep Rule in parser, put the registry dispatch table in parser, have handlers register into it.**

```go
// internal/parser/dispatch.go

package parser

// ActionParser defines how to parse a single action type.
type ActionParser struct {
    Prefix string
    Parse  func(line string) (*Rule, error)
}

var registeredParsers []ActionParser

// RegisterActionParser is called by handler init() functions to register
// their parse function. The parser uses this for dispatch in parseContent().
func RegisterActionParser(prefix string, parse func(line string) (*Rule, error)) {
    registeredParsers = append(registeredParsers, ActionParser{
        Prefix: prefix,
        Parse:  parse,
    })
}

// FindParser returns the parser for the given line, or nil.
func FindParser(line string) *ActionParser {
    var best *ActionParser
    for i := range registeredParsers {
        p := &registeredParsers[i]
        if strings.HasPrefix(line, p.Prefix) {
            if best == nil || len(p.Prefix) > len(best.Prefix) {
                best = p
            }
        }
    }
    return best
}
```

Since `handlers` already imports `parser`, handler init() functions can call `parser.RegisterActionParser(...)` and reference parse functions that are **exported from parser**:

```go
// internal/handlers/install.go
func init() {
    parser.RegisterActionParser("install ", parser.ParseInstallRule)
    // ... rest of registration
}
```

This requires exporting the parse functions (rename `parseInstallRule` to `ParseInstallRule`), which is a clean, low-risk change.

For the non-parser parts of the registry (handler factory, RuleKey, DetectRuleType, Summary, OrphanIndex), these can live in `internal/handlers/registry.go` since they already reference handler types. This avoids creating a new package entirely.

**Final recommended package layout:**

```
internal/parser/dispatch.go     -- RegisterActionParser, FindParser
                                   (parse dispatch only, no handler deps)

internal/handlers/registry.go   -- ActionDef (sans Parse), Register(),
                                   Get(), All(), NewHandler(),
                                   RuleKey(), DetectRuleType(),
                                   DisplaySummary(), OrphanIndex()
                                   (handler factory + metadata registry)

internal/handlers/install.go    -- init() calls both:
                                   parser.RegisterActionParser("install ", parser.ParseInstallRule)
                                   RegisterAction(ActionDef{Name: "install", ...})
```

This is the cleanest split: parser dispatch stays in `parser` (no cycle), handler metadata stays in `handlers` (no cycle), and each handler file registers both.

---

### Complete Registration Example

Here is what `install.go` looks like after the change (showing only the new `init()` and unchanged handler struct):

```go
package handlers

import (
    "strings"

    "github.com/elpic/blueprint/internal/parser"
    "github.com/elpic/blueprint/internal/platform"
)

func init() {
    // Register parser
    parser.RegisterActionParser("install ", parser.ParseInstallRule)

    // Register handler metadata
    RegisterAction(ActionDef{
        Name: "install",

        NewHandler: func(rule parser.Rule, basePath string, passwordCache map[string]string) Handler {
            return NewInstallHandler(rule, basePath, platform.NewContainer())
        },

        RuleKey: func(rule parser.Rule) string {
            if len(rule.Packages) > 0 {
                return rule.Packages[0].Name
            }
            return "install"
        },

        Detect: func(rule parser.Rule) bool {
            return len(rule.Packages) > 0
        },

        Summary: func(rule parser.Rule) string {
            names := make([]string, len(rule.Packages))
            for i, p := range rule.Packages {
                names[i] = p.Name
            }
            return strings.Join(names, ", ")
        },

        OrphanIndex: func(rule parser.Rule, index func(string)) {
            for _, pkg := range rule.Packages {
                index(pkg.Name)
            }
        },
    })
}

// ... rest of InstallHandler unchanged ...
```

---

### What "Adding a New Action" Looks Like

#### Before (current -- 8 files to touch):

1. Add parse function to `parser/parser.go` and add `case` to `parseContent()` switch
2. Add `case` to `DisplaySummary()` switch in `parser/parser.go`
3. Create handler file `handlers/newaction.go`
4. Add `case` to `NewHandler()` switch in `handlers/handlers.go`
5. Add entry to `GetStatusProviderHandlers()` list in `handlers/handlers.go`
6. Add `case` to `RuleKey()` switch in `handlers/handlers.go`
7. Add `case` to `DetectRuleType()` if-chain in `handlers/handlers.go`
8. Add `case` to `checkOrphansWithLoader()` switch in `engine/doctor.go`
9. (If new status type) Add field to `Status` struct, update `AllEntries()`, `FilterEntries()`

#### After (proposed -- 1-2 files to touch):

1. Create handler file `handlers/newaction.go` with:
   - Handler struct and methods (same as before)
   - `init()` function that calls `parser.RegisterActionParser(...)` and `RegisterAction(ActionDef{...})`
   - Parse function (either in this file or exported from parser)
2. (If new status type) Add field to `Status` struct, update `AllEntries()`, `FilterEntries()`
   - This is inherent to adding a new JSON schema field and cannot be eliminated without changing the status.json format

Step 2 is only needed for actions that track state in `status.json`. For actions like `run` that reuse an existing status type (`RunStatus`), only step 1 is needed.

---

### Migration Plan

The migration should be done in **4 phases**, each independently shippable and testable.

#### Phase 1: Create Registry Infrastructure (no behavior change)

**Files created:**
- `internal/parser/dispatch.go` -- `RegisterActionParser()`, `FindParser()`
- `internal/handlers/registry.go` -- `ActionDef`, `RegisterAction()`, `GetAction()`, `AllActions()`

**Files modified:**
- None yet. The registry exists but nothing uses it.

**Test:** Unit tests for `RegisterAction` and `FindParser`.

#### Phase 2: Register All Actions (no behavior change)

**Files modified (add `init()` to each):**
- `internal/handlers/install.go`
- `internal/handlers/clone.go`
- `internal/handlers/decrypt.go`
- `internal/handlers/mkdir.go`
- `internal/handlers/known_hosts.go`
- `internal/handlers/gpg_key.go`
- `internal/handlers/asdf.go`
- `internal/handlers/mise.go`
- `internal/handlers/sudoers.go`
- `internal/handlers/homebrew.go`
- `internal/handlers/ollama.go`
- `internal/handlers/download.go`
- `internal/handlers/run.go` (registers both `run` and `run-sh`)
- `internal/handlers/dotfiles.go`
- `internal/handlers/schedule.go`
- `internal/handlers/shell.go`
- `internal/handlers/authorized_keys.go`

**Files modified (export parse functions):**
- `internal/parser/parser.go` -- Rename `parseInstallRule` to `ParseInstallRule`, etc.

**Test:** Verify that `AllActions()` returns all 18/19 entries. Verify `FindParser()` finds the right parser for sample lines. No behavior changes yet; old switches still in use.

#### Phase 3: Switch Call Sites to Use Registry

**Files modified:**
- `internal/parser/parser.go` -- Replace 19-branch switch in `parseContent()` with `FindParser()` call. Replace `DisplaySummary()` switch with `GetAction().Summary()` call.
- `internal/handlers/handlers.go` -- Replace `NewHandler()` switch with `GetAction().NewHandler()`. Replace `GetStatusProviderHandlers()` with loop over `AllActions()`. Replace `RuleKey()` switch with `GetAction().RuleKey()`. Replace `DetectRuleType()` if-chain with loop over `AllActions()`.
- `internal/engine/doctor.go` -- Replace `rulesFor` switch with `GetAction().OrphanIndex()`.

**Test:** Full test suite must pass. This is the critical phase -- behavior must be identical.

#### Phase 4: Cleanup

- Delete the old switch/if-chain code that was replaced.
- Remove any now-unused helper functions.
- Update documentation.

---

### Options Considered

#### Option A: Central Registry with `init()` Self-Registration (Recommended)

- **Pros:**
  - Single registration point per action
  - Adding new action = one file
  - No central file to edit
  - `init()` is idiomatic Go (used by `database/sql`, `image`, etc.)
  - Compile-time safety: missing registration = parse error at runtime (caught by existing tests)
  - No reflection or code generation needed

- **Cons:**
  - `init()` ordering is per-package (but registration order does not matter for map-based lookup)
  - Parse function prefix matching must handle ambiguity (e.g., `run` vs `run-sh`) -- solved by longest-prefix-wins
  - Requires exporting parse functions from parser package
  - Two registration calls per handler (`RegisterActionParser` + `RegisterAction`) -- could be unified if we accept moving parse functions to handler files

#### Option B: Code Generation

- **Pros:**
  - Compile-time guarantees
  - No `init()` magic

- **Cons:**
  - Requires `go generate` step in the build
  - Adds tooling dependency
  - Harder for contributors to understand
  - Still needs a manifest file listing all actions

#### Option C: Interface-Only Approach (No Registry)

Each handler implements all needed methods (Parse, RuleKey, Detect, Summary, etc.) and a central list just instantiates them once.

- **Pros:**
  - Pure interface-driven, very Go-idiomatic
  - No global state

- **Cons:**
  - Still needs one central list of handlers somewhere (the "register" list)
  - The parse function cannot be a method on the handler because parsing creates the Rule which is needed to create the handler -- chicken-and-egg problem

---

### Consequences

**Positive:**
- Adding a new action type requires touching only 1-2 files instead of 8
- Each action is fully self-contained in its handler file
- Reduced risk of forgetting to update one of the 8 sites
- Easier to onboard new contributors
- Better separation of concerns

**Negative:**
- `init()` functions make registration implicit rather than explicit -- slightly harder to trace control flow
- Two separate registrations (parser + handler) per action, though they are co-located in the same `init()`
- Migration is a multi-phase effort touching many files

**Risks and mitigations:**
- **Risk:** Registration order affects prefix matching (e.g., `run` matching before `run-sh`). **Mitigation:** Longest-prefix-wins algorithm in `FindParser()`.
- **Risk:** `init()` in test files could interfere. **Mitigation:** Test helpers do not call `Register`; they use handlers directly.
- **Risk:** Parse functions exported from parser could be called directly, bypassing the registry. **Mitigation:** Document that `parseContent()` is the entry point; exported parse functions are for registration only.
- **Risk:** Forgetting to register a handler. **Mitigation:** Add a test that compares `AllActions()` count against a known expected value, and verifies each registered action has a working Parse + NewHandler.

---

### Technical Details

- **Packages affected:** `internal/parser`, `internal/handlers`, `internal/engine`
- **New files:** `internal/parser/dispatch.go`, `internal/handlers/registry.go`
- **APIs/Interfaces:** New `ActionDef` struct, `RegisterAction()`, `RegisterActionParser()` functions. Existing `Handler` interface unchanged.
- **Data models:** `Status` struct and `status.json` schema are **not changed**.
- **Estimated effort:** ~2-3 days for a developer familiar with the codebase. Phase 1-2 are low risk. Phase 3 is the critical switchover.
