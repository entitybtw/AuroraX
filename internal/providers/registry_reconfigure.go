package providers

import (
	"context"
	"fmt"
	"log/slog"
	"maps"

	"aurora/internal/core"
)

// registrySnapshot holds the inventory a rebuild destroys before it knows the
// replacement can finish. ReplaceProviders ends in a bounded model refresh, and
// a slow upstream makes that refresh fail — without a snapshot the gateway is
// left serving zero models (and, through Rebuild, zero pools) until somebody
// triggers a refresh by hand.
type registrySnapshot struct {
	providers        []core.Provider
	providerTypes    map[core.Provider]string
	providerNames    map[core.Provider]string
	providerRuntime  map[string]providerRuntimeState
	models           map[string]*ModelInfo
	modelsByProvider map[string]map[string]*ModelInfo
	initialized      bool
}

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

	snap := r.captureSnapshot()

	r.mu.Lock()
	r.models = make(map[string]*ModelInfo)
	r.modelsByProvider = make(map[string]map[string]*ModelInfo)
	r.providers = nil
	r.providerTypes = make(map[core.Provider]string)
	r.providerNames = make(map[core.Provider]string)
	r.providerRuntime = make(map[string]providerRuntimeState)
	r.configMetadataOverrides = nil
	r.configuredProviderModels = nil
	r.providerAutoFetchModels = nil
	r.providerAutoFetchFilters = nil
	r.initialized = false
	r.invalidateSortedCaches()
	r.mu.Unlock()

	count, err := initializeProviders(ctx, providerMap, factory, r)
	if err != nil {
		r.restoreSnapshot(snap)
		return count, err
	}
	if count == 0 {
		r.restoreSnapshot(snap)
		return 0, fmt.Errorf("no providers were successfully registered")
	}
	if err := r.Initialize(ctx); err != nil {
		// The freshly registered providers stay — they are the config the
		// caller asked for. Only the model inventory goes back, retargeted at
		// those instances, so a refresh that gave up waiting on a slow
		// upstream cannot take the gateway out of service.
		r.restoreModels(snap)
		return count, err
	}
	_ = r.ReloadUserOverrides()
	return count, nil
}

// captureSnapshot copies the registry fields a rebuild resets.
func (r *ModelRegistry) captureSnapshot() registrySnapshot {
	r.mu.RLock()
	snap := registrySnapshot{
		providers:        append([]core.Provider(nil), r.providers...),
		providerTypes:    maps.Clone(r.providerTypes),
		providerNames:    maps.Clone(r.providerNames),
		providerRuntime:  maps.Clone(r.providerRuntime),
		models:           maps.Clone(r.models),
		modelsByProvider: maps.Clone(r.modelsByProvider),
	}
	r.mu.RUnlock()
	r.initMu.Lock()
	snap.initialized = r.initialized
	r.initMu.Unlock()
	return snap
}

// restoreSnapshot puts every captured field back. Used when no replacement
// provider could be registered, so the registry is exactly as it was.
func (r *ModelRegistry) restoreSnapshot(snap registrySnapshot) {
	r.mu.Lock()
	r.providers = snap.providers
	r.providerTypes = snap.providerTypes
	r.providerNames = snap.providerNames
	r.providerRuntime = snap.providerRuntime
	r.models = snap.models
	r.modelsByProvider = snap.modelsByProvider
	r.invalidateSortedCaches()
	r.mu.Unlock()
	r.initMu.Lock()
	r.initialized = snap.initialized
	r.initMu.Unlock()
}

// restoreModels puts the captured model inventory back, rebinding every entry
// to the provider instances registered now. The instances the snapshot was
// taken from were dropped by the rebuild, so returning them verbatim would
// route requests at objects no longer in the provider set; entries whose
// provider no longer exists are dropped instead of being served stale.
func (r *ModelRegistry) restoreModels(snap registrySnapshot) {
	r.initMu.Lock()
	r.initialized = snap.initialized
	r.initMu.Unlock()
	if len(snap.models) == 0 && len(snap.modelsByProvider) == 0 {
		return
	}

	r.mu.Lock()
	current := r.providerInstancesLocked()

	models := make(map[string]*ModelInfo, len(snap.models))
	for modelID, info := range snap.models {
		bound, ok := rebindModelInfo(info, current)
		if !ok {
			continue
		}
		models[modelID] = bound
	}
	byProvider := make(map[string]map[string]*ModelInfo, len(snap.modelsByProvider))
	for providerName, entries := range snap.modelsByProvider {
		if _, ok := current[providerName]; !ok {
			continue
		}
		retained := make(map[string]*ModelInfo, len(entries))
		for modelID, info := range entries {
			bound, ok := rebindModelInfo(info, current)
			if !ok {
				continue
			}
			retained[modelID] = bound
		}
		if len(retained) == 0 {
			continue
		}
		byProvider[providerName] = retained
	}
	r.models = models
	r.modelsByProvider = byProvider
	r.invalidateSortedCaches()
	r.mu.Unlock()

	slog.Info("restored previous model inventory after refresh failure",
		"models", len(models),
		"providers", len(byProvider),
	)
}

// registeredProvider is one live provider instance keyed by the name its
// model entries are filed under.
type registeredProvider struct {
	provider core.Provider
	typeName string
}

// providerInstancesLocked indexes the registered providers by the same key the
// inventory uses: configured instance name, falling back to the provider type.
func (r *ModelRegistry) providerInstancesLocked() map[string]registeredProvider {
	out := make(map[string]registeredProvider, len(r.providers))
	for _, p := range r.providers {
		name := r.providerNames[p]
		typeName := r.providerTypes[p]
		if name == "" {
			name = typeName
		}
		if name == "" {
			continue
		}
		out[name] = registeredProvider{provider: p, typeName: typeName}
	}
	return out
}

// rebindModelInfo points a saved entry at the currently registered instance.
func rebindModelInfo(info *ModelInfo, current map[string]registeredProvider) (*ModelInfo, bool) {
	if info == nil {
		return nil, false
	}
	inst, ok := current[info.ProviderName]
	if !ok {
		return nil, false
	}
	return &ModelInfo{
		Model:        info.Model,
		Provider:     inst.provider,
		ProviderName: info.ProviderName,
		ProviderType: inst.typeName,
	}, true
}
