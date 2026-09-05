package orchestra

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

type routedBackend struct {
	base   ExecutionBackend
	routes map[string]ExecutionBackend
	name   string
}

// FreshExecutionBackend declares that every Execute call uses a fresh isolated execution.
type FreshExecutionBackend interface {
	ExecutionBackend
	FreshExecutionPerRequest() bool
}

// ProviderBackendResolver reports which backend a provider request would
// reach, so callers can attribute outcomes recorded before Execute returned
// (queued, parent-cancelled) to the same backend a completed run would name.
type ProviderBackendResolver interface {
	BackendNameFor(cfg ProviderConfig) string
}

// NewRoutedBackend routes requests with a configured backend key and delegates all others to base.
func NewRoutedBackend(base ExecutionBackend, routes map[string]ExecutionBackend) ExecutionBackend {
	ownedRoutes := make(map[string]ExecutionBackend, len(routes))
	keys := make([]string, 0, len(routes))
	for key, backend := range routes {
		ownedRoutes[key] = backend
		keys = append(keys, key)
	}
	slices.Sort(keys)

	name := base.Name()
	if len(keys) > 0 {
		name += "+" + strings.Join(keys, "+")
	}
	return &routedBackend{base: base, routes: ownedRoutes, name: name}
}

func (b *routedBackend) Execute(ctx context.Context, req ProviderRequest) (*ProviderResponse, error) {
	if backend, ok := b.routes[req.Config.Backend]; ok {
		return backend.Execute(ctx, req)
	}
	if req.Config.Backend != "" {
		// A provider that names a backend must never fall through to the
		// CLI base backend; that would spawn Config.Binary.
		return nil, fmt.Errorf("provider %s backend %q is not available in this execution path",
			req.Provider, req.Config.Backend)
	}
	return b.base.Execute(ctx, req)
}

func (b *routedBackend) freshExecutionPerRequest() bool {
	if !pipelineBackendHasFreshExecutionSemantics(b.base) {
		return false
	}
	for _, backend := range b.routes {
		fresh, ok := backend.(FreshExecutionBackend)
		if !ok || !fresh.FreshExecutionPerRequest() {
			return false
		}
	}
	return true
}

func (b *routedBackend) Name() string { return b.name }

// BackendNameFor names the route a provider resolves to: the registered
// backend, the provider's own (unregistered, fail-closed) key, or the base.
func (b *routedBackend) BackendNameFor(cfg ProviderConfig) string {
	if backend, ok := b.routes[cfg.Backend]; ok {
		return backend.Name()
	}
	if cfg.Backend != "" {
		return cfg.Backend
	}
	return b.base.Name()
}
