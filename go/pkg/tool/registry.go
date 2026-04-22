package tool

import (
	"maps"
	"slices"
	"sync"
)

// ============================================================================
// Registry — tool lookup and registration
// ============================================================================

// Registry stores and looks up tools by name or alias.
type Registry struct {
	mu     sync.RWMutex
	tools  map[string]Tool       // name → tool
	byName map[string]Tool       // canonical name → tool
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools:  make(map[string]Tool),
		byName: make(map[string]Tool),
	}
}

// Register adds a tool to the registry.
// If a tool with the same name already exists, it is replaced.
func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := t.Name()
	r.byName[name] = t
	r.tools[name] = t
}

// RegisterAlias adds a lookup alias for an existing tool.
// The tool must already be registered under its canonical name.
func (r *Registry) RegisterAlias(alias, canonicalName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if t, ok := r.byName[canonicalName]; ok {
		r.tools[alias] = t
	}
}

// Get returns the tool with the given name or alias.
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	t, ok := r.tools[name]
	return t, ok
}

// MustGet returns the tool with the given name, panicking if not found.
func (r *Registry) MustGet(name string) Tool {
	t, ok := r.Get(name)
	if !ok {
		panic("tool not found: " + name)
	}
	return t
}

// All returns all unique registered tools (deduplicated by canonical name).
func (r *Registry) All() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return slices.Collect(maps.Values(r.byName))
}

// Names returns all canonical tool names, sorted.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := slices.Collect(maps.Keys(r.byName))
	slices.Sort(names)
	return names
}

// Len returns the number of registered tools.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.byName)
}
