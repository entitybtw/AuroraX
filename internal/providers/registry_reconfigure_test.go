package providers

import (
	"context"
	"errors"
	"testing"

	"aurora/configuration"
	"aurora/internal/core"
	"aurora/internal/providers/pool"
)

// testProviderSet builds a factory whose provider fails once `fail` is set, so
// a rebuild can be driven into the "model refresh did not finish" path.
func testProviderSet(fail *bool, modelID string) *ProviderFactory {
	factory := NewProviderFactory()
	factory.Add(Registration{
		Type: "test-reconfigure",
		New: func(_ ProviderConfig, _ ProviderOptions) core.Provider {
			if *fail {
				return &registryMockProvider{err: errors.New("upstream unreachable")}
			}
			return &registryMockProvider{modelsResponse: &core.ModelsResponse{
				Object: "list",
				Data:   []core.Model{{ID: modelID, Object: "model", OwnedBy: "test"}},
			}}
		},
	})
	return factory
}

func TestReplaceProviders_RestoresInventoryWhenRefreshFails(t *testing.T) {
	fail := false
	factory := testProviderSet(&fail, "model-a")
	registry := NewModelRegistry()
	cfg := map[string]ProviderConfig{
		"alpha": {Type: "test-reconfigure", APIKey: "key", Name: "alpha"},
	}

	if _, err := registry.ReplaceProviders(context.Background(), cfg, factory); err != nil {
		t.Fatalf("seed rebuild failed: %v", err)
	}
	if registry.ModelCount() == 0 {
		t.Fatal("expected the seed rebuild to register models")
	}

	fail = true
	if _, err := registry.ReplaceProviders(context.Background(), cfg, factory); err == nil {
		t.Fatal("expected the refresh to fail")
	}

	if got := registry.ModelCount(); got == 0 {
		t.Fatal("model inventory was dropped by a refresh that did not finish")
	}
	if !registry.IsInitialized() {
		t.Fatal("registry still reports startup after the inventory was restored")
	}

	registry.mu.RLock()
	entries := registry.modelsByProvider["alpha"]
	var current []core.Provider
	current = append(current, registry.providers...)
	registry.mu.RUnlock()

	if len(entries) == 0 {
		t.Fatalf("restored inventory is not filed under provider %q", "alpha")
	}
	for modelID, info := range entries {
		if modelID != "model-a" {
			t.Errorf("unexpected restored model %q", modelID)
		}
		bound := false
		for _, p := range current {
			if p == info.Provider {
				bound = true
				break
			}
		}
		if !bound {
			t.Errorf("restored model %q points at a provider instance that is no longer registered", modelID)
		}
	}
}

func TestRebuild_InstallsPoolsWhenModelRefreshFails(t *testing.T) {
	fail := true
	factory := testProviderSet(&fail, "model-a")
	registry := NewModelRegistry()
	router, err := NewRouter(registry)
	if err != nil {
		t.Fatalf("router: %v", err)
	}

	result := &InitResult{
		Registry: registry,
		Router:   router,
		Factory:  factory,
		Pools:    pool.NewRegistry(),
	}

	rawProviders := map[string]config.RawProviderConfig{
		"alpha": {Type: "test-reconfigure", APIKey: "key"},
	}
	rawPools := map[string]config.RawPoolConfig{
		"fast": {Members: []string{"alpha"}},
	}

	if _, err := result.Rebuild(context.Background(), rawProviders, rawPools, &config.Config{}, factory); err == nil {
		t.Fatal("expected the rebuild to report the failed model refresh")
	}
	if result.Pools == nil {
		t.Fatal("pool registry is nil after a failed rebuild")
	}
	if !result.Pools.HasPool("fast") {
		t.Fatalf("dashboard pool overrides were not installed; pools=%v", result.Pools.Names())
	}
	if !router.Pools().HasPool("fast") {
		t.Fatal("router was not pointed at the rebuilt pool registry")
	}
}
