package providers

import (
	"fmt"
	"log/slog"
	"strings"

	"aurora/configuration"
	"aurora/internal/providers/pool"
)

// providerTypeCapabilities maps a provider's type string to the pool
// capabilities it supports. This is used at pool build time to annotate
// pool members so that the router can filter by capability during dispatch.
var providerTypeCapabilities = map[string][]pool.Capability{
	"reranker": {pool.CapRerank},
	"openai":   {pool.CapChat, pool.CapEmbedding, pool.CapResponses, pool.CapFiles, pool.CapBatches},
	"groq":     {pool.CapChat, pool.CapEmbedding},
}

// buildPoolRegistry validates the raw pool configs against the configured
// provider map and returns a pool registry usable by the Router.
//
// Validation rules:
//   - Pool names must not be empty.
//   - Pool names must NOT collide with a configured provider instance name
//     (the model registry already uses that namespace for routing).
//   - Each member must reference an existing configured provider.
//   - All members must share the same provider type (so they can serve the
//     same models). A mixed-type pool is a configuration error.
//   - Pool names must be unique.
//   - Filtered-out providers (missing credentials, etc.) are reported as
//     warnings rather than errors so partial deploys still boot, but a
//     pool with zero remaining members is rejected.
func buildPoolRegistry(rawPools map[string]config.RawPoolConfig, providerMap map[string]ProviderConfig, registry *ModelRegistry) (*pool.Registry, error) {
	pools := pool.NewRegistry()
	if len(rawPools) == 0 {
		return pools, nil
	}

	for name, raw := range rawPools {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, fmt.Errorf("pool entry has empty name")
		}
		if _, collides := providerMap[name]; collides {
			return nil, fmt.Errorf("pool %q: name collides with a configured provider instance — pick a distinct pool name", name)
		}

		strategy, err := pool.ParseStrategy(raw.Strategy)
		if err != nil {
			return nil, fmt.Errorf("pool %q: %w", name, err)
		}

		if len(raw.Members) == 0 {
			return nil, fmt.Errorf("pool %q: members list is empty", name)
		}

		members := make([]pool.MemberConfig, 0, len(raw.Members))
		var sharedType string
		for _, memberName := range raw.Members {
			memberName = strings.TrimSpace(memberName)
			if memberName == "" {
				return nil, fmt.Errorf("pool %q: empty member name in members list", name)
			}
			pCfg, ok := providerMap[memberName]
			if !ok {
				// Skip with a warning rather than fail: missing credentials
				// are a common deploy state and we want partial boots to work.
				continue
			}
			if sharedType == "" {
				sharedType = pCfg.Type
			} else if sharedType != pCfg.Type {
				return nil, fmt.Errorf("pool %q: mixed provider types (%q and %q) — all members must share a type", name, sharedType, pCfg.Type)
			}
			caps := inferCapabilities(pCfg.Type)
			members = append(members, pool.MemberConfig{ProviderName: memberName, Capabilities: caps})
		}

		if len(members) == 0 {
			slog.Warn("pool disabled because no members survived credential filtering", "pool", name)
			continue
		}

		health := poolHealthChecker(registry, raw.HealthAware)
		p, err := pool.NewPool(pool.Config{
			Name:     name,
			Strategy: strategy,
			Members:  members,
			Health:   health,
		})
		if err != nil {
			return nil, err
		}
		if err := pools.Register(p); err != nil {
			return nil, err
		}
	}

	return pools, nil
}

// inferCapabilities returns the pool capabilities for a given provider type.
// Unknown types default to chat+embedding+responses (the most common case).
func inferCapabilities(providerType string) []pool.Capability {
	if caps, ok := providerTypeCapabilities[strings.ToLower(strings.TrimSpace(providerType))]; ok {
		return caps
	}
	return []pool.Capability{pool.CapChat, pool.CapEmbedding, pool.CapResponses}
}

// poolHealthChecker returns a HealthChecker backed by the model registry's
// per-provider availability state. When healthAware is non-nil and false, an
// always-healthy checker is returned (useful for testing / forced routing).
func poolHealthChecker(registry *ModelRegistry, healthAware *bool) pool.HealthChecker {
	if healthAware != nil && !*healthAware {
		return passThroughHealth{}
	}
	return registryHealth{registry: registry}
}

type passThroughHealth struct{}

func (passThroughHealth) IsProviderHealthy(string) bool { return true }

// registryHealth treats a configured provider as healthy unless the model
// registry's most recent availability probe failed AND no successful probe
// has happened since. This mirrors the existing dashboard semantics.
type registryHealth struct {
	registry *ModelRegistry
}

func (r registryHealth) IsProviderHealthy(providerName string) bool {
	if r.registry == nil {
		return true
	}
	for _, snap := range r.registry.ProviderRuntimeSnapshots() {
		if snap.Name != providerName {
			continue
		}
		// Healthy if a recent successful availability probe exists OR no probe
		// has been recorded yet (provider just registered, async init pending).
		if snap.LastAvailabilityError == "" {
			return true
		}
		if snap.LastAvailabilityOKAt != nil && snap.LastAvailabilityCheckAt != nil &&
			!snap.LastAvailabilityOKAt.Before(*snap.LastAvailabilityCheckAt) {
			return true
		}
		return false
	}
	// Unknown provider — defer to the router which will fail with a clear error.
	return true
}
