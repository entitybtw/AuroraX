package providers

import (
	"context"
	"fmt"
	"maps"

	"aurora/configuration"
)

// Rebuild replaces provider instances, model inventory, router pools, and public
// configured-provider snapshots from the supplied raw provider/pool config.
func (r *InitResult) Rebuild(ctx context.Context, rawProviders map[string]config.RawProviderConfig, rawPools map[string]config.RawPoolConfig, cfg *config.Config, factory *ProviderFactory) (int, error) {
	if r == nil || r.Registry == nil || r.Router == nil {
		return 0, fmt.Errorf("provider runtime is unavailable")
	}
	if cfg == nil {
		return 0, fmt.Errorf("config is required")
	}
	if factory == nil {
		factory = r.Factory
	}
	if factory == nil {
		return 0, fmt.Errorf("provider factory is required")
	}

	providerMap, credentialResolved := resolveProviders(rawProviders, cfg.Resilience, factory.discoveryConfigsSnapshot())
	pools, err := buildPoolRegistry(rawPools, providerMap, r.Registry)
	if err != nil {
		return 0, err
	}
	count, err := r.Registry.ReplaceProviders(ctx, providerMap, factory)
	if err != nil {
		return count, err
	}
	if r.Pools == nil {
		r.Pools = pools
	} else {
		r.Pools.Replace(pools)
	}
	r.Router.SetPools(r.Pools)
	r.ConfiguredProviders = SanitizeProviderConfigs(providerMap)
	r.CredentialResolvedProviders = maps.Clone(credentialResolved)
	return count, nil
}
