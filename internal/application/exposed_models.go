package app

import (
	"sort"
	"strings"

	"aurora/configuration"
	"aurora/internal/core"
	"aurora/internal/gateway"
	"aurora/internal/model_aliases"
	"aurora/internal/model_combinations"
	"aurora/internal/server"
)

func requestModelResolver(aliasService *aliases.Service, comboService *combos.Service) gateway.ModelResolver {
	if comboService != nil {
		return comboService
	}
	return aliasService
}

func exposedModelLister(aliasService *aliases.Service, comboService *combos.Service, fallbackProvider func() config.FallbackConfig) server.ExposedModelLister {
	listers := make([]server.ExposedModelLister, 0, 3)
	if aliasService != nil {
		listers = append(listers, aliasService)
	}
	if comboService != nil {
		listers = append(listers, comboService)
	}
	if chainLister := fallbackChainModelLister(fallbackProvider); chainLister != nil {
		listers = append(listers, chainLister)
	}
	return compositeExposedModelLister{listers: listers}
}

// fallbackChainModelLister surfaces each manual fallback chain name (source) as
// a selectable model in GET /v1/models so clients can request a chain by name.
// The provider function is re-evaluated on every call so chain edits made via
// the dashboard appear immediately without a restart.
func fallbackChainModelLister(fallbackProvider func() config.FallbackConfig) server.ExposedModelLister {
	if fallbackProvider == nil {
		return nil
	}
	return dynamicFallbackChainLister{fallbackProvider: fallbackProvider}
}

type dynamicFallbackChainLister struct {
	fallbackProvider func() config.FallbackConfig
}

func (l dynamicFallbackChainLister) ExposedModels() []core.Model {
	fallback := l.fallbackProvider()
	names := make([]string, 0, len(fallback.Manual))
	for source := range fallback.Manual {
		source = strings.TrimSpace(source)
		if source == "" || strings.Contains(source, "/") {
			// Skip provider-qualified keys (provider/model) and empty names;
			// they are already real models, not chain aliases.
			continue
		}
		names = append(names, source)
	}
	if len(names) == 0 {
		return nil
	}
	sort.Strings(names)

	models := make([]core.Model, 0, len(names))
	for _, name := range names {
		models = append(models, core.Model{
			ID:      name,
			Object:  "model",
			OwnedBy: "fallback-chain",
			Metadata: &core.ModelMetadata{
				DisplayName: name + " (fallback chain)",
				Description: "Fallback chain: attempts ordered providers until one succeeds.",
				Modes:       []string{"chat"},
				Categories:  []core.ModelCategory{core.CategoryTextGeneration},
			},
		})
	}
	return models
}

type compositeExposedModelLister struct {
	listers []server.ExposedModelLister
}

func (l compositeExposedModelLister) ExposedModels() []core.Model {
	out := make([]core.Model, 0)
	for _, lister := range l.listers {
		if lister != nil {
			out = append(out, lister.ExposedModels()...)
		}
	}
	return out
}
