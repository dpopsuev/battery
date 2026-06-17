package translate

import (
	"encoding/json"
	"fmt"
	"sync"
)

// ErrNoTranslator is returned when no translator is registered for an event type.
var ErrNoTranslator = fmt.Errorf("no translator registered")

// Registry maps source event types to their Translators.
type Registry struct {
	mu          sync.RWMutex
	translators map[string]Translator
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{translators: make(map[string]Translator)}
}

// Register adds a translator for the given source event type.
func (r *Registry) Register(sourceType string, t Translator) {
	r.mu.Lock()
	r.translators[sourceType] = t
	r.mu.Unlock()
}

// Translate converts source data using the registered translator.
func (r *Registry) Translate(sourceType string, data json.RawMessage) (Result, error) {
	r.mu.RLock()
	t, ok := r.translators[sourceType]
	r.mu.RUnlock()
	if !ok {
		return Result{}, fmt.Errorf("%w for %q", ErrNoTranslator, sourceType)
	}
	return t(sourceType, data)
}

// Has returns true if a translator is registered for the given type.
func (r *Registry) Has(sourceType string) bool {
	r.mu.RLock()
	_, ok := r.translators[sourceType]
	r.mu.RUnlock()
	return ok
}

// Types returns all registered source event types.
func (r *Registry) Types() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	types := make([]string, 0, len(r.translators))
	for k := range r.translators {
		types = append(types, k)
	}
	return types
}
