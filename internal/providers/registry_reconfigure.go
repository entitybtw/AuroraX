package providers

import (
	"context"
	"fmt"

	"aurora/internal/core"
)

// ReplaceProviders rebuilds the provider set from the supplied config while preserving
// model-list metadata, user-pricing overrides, and cache wiring.
func (r *ModelRegistry) ReplaceProviders(ctx context.Context, providerMap map[string]ProviderConfig, factory *ProviderFactory) (int, error) {
	if r == nil {
		return 0, fmt.Errorf("model registry is unavailable")
	}
	if factory == nil {
		return 0, fmt.Errorf("provider factory is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	r.mu.Lock()
	r.models = make(map[string]*ModelInfo)
	r.modelsByProvider = make(map[string]map[string]*ModelInfo)
	r.providers = nil
	r.providerTypes = make(map[core.Provider]string)
	r.providerNames = make(map[core.Provider]string)
	r.providerRuntime = make(map[string]providerRuntimeState)
	r.configMetadataOverrides = nil
	r.configuredProviderModels = nil
	r.initialized = false
	r.invalidateSortedCaches()
	r.mu.Unlock()

	count, err := initializeProviders(ctx, providerMap, factory, r)
	if err != nil {
		return count, err
	}
	if count == 0 {
		return 0, fmt.Errorf("no providers were successfully registered")
	}
	if err := r.Initialize(ctx); err != nil {
		return count, err
	}
	_ = r.ReloadUserOverrides()
	return count, nil
}
