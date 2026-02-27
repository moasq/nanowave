package integrations

import (
	"fmt"
	"sort"
	"sync"
)

// Registry holds registered integration providers.
// Pattern: database/sql Register() + sync.RWMutex-protected map.
// At 3-5 providers, explicit registration is more traceable than init() magic.
type Registry struct {
	mu        sync.RWMutex
	providers map[ProviderID]Provider
}

// NewRegistry creates an empty provider registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[ProviderID]Provider),
	}
}

// Register adds a provider to the registry.
// Panics if a provider with the same ID is already registered (like sql.Register).
func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := p.ID()
	if _, exists := r.providers[id]; exists {
		panic(fmt.Sprintf("integrations: provider %q already registered", id))
	}
	r.providers[id] = p
}

// Get returns a provider by ID, or nil and false if not found.
func (r *Registry) Get(id ProviderID) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.providers[id]
	return p, ok
}

// All returns all registered providers in stable ID-sorted order.
func (r *Registry) All() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.providers))
	for id := range r.providers {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)

	result := make([]Provider, 0, len(ids))
	for _, id := range ids {
		result = append(result, r.providers[ProviderID(id)])
	}
	return result
}
