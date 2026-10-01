package app

import (
	"os"
	"path/filepath"
	"testing"

	"aurora/configuration"
	"aurora/internal/admin"
)

func writeOverrideStore(t *testing.T, entries string) *admin.ProviderOverrideStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "provider-overrides.json")
	if err := os.WriteFile(path, []byte(entries), 0o600); err != nil {
		t.Fatalf("write override store: %v", err)
	}
	t.Setenv("AURORA_PROVIDER_OVERRIDES_PATH", path)
	return admin.NewProviderOverrideStore()
}

// The startup merge folds persisted overrides into RawProviders so the first
// Init registers them. Those names must not stay in the static base, or a
// provider deleted from the dashboard keeps running until the process restarts.
func TestDropStartupMergedOverridesRemovesPhantomProviders(t *testing.T) {
	store := writeOverrideStore(t, `[
		{"name":"vllm-zen","type":"vllm","base_url":"https://opencode.ai/zen/v1"}
	]`)

	a := &App{
		rawProviders: map[string]config.RawProviderConfig{
			// Folded in at startup, deleted from the dashboard afterwards.
			"vllm-zen-backup": {Type: "vllm", BaseURL: "https://opencode.ai/zen/v1"},
			// Folded in at startup, still managed by the store.
			"vllm-zen": {Type: "vllm", BaseURL: "https://opencode.ai/zen/v1"},
			// Genuine static config entry, never touched by the merge.
			"static-provider": {Type: "vllm", BaseURL: "https://static.example/v1"},
		},
		providerOverrides: store,
	}

	a.dropStartupMergedOverrides([]string{"vllm-zen-backup", "vllm-zen"})

	if _, ok := a.rawProviders["vllm-zen-backup"]; ok {
		t.Fatal("deleted provider survived in the static provider base")
	}
	if _, ok := a.rawProviders["vllm-zen"]; ok {
		t.Fatal("override-owned provider must not be kept in the static base")
	}
	if _, ok := a.rawProviders["static-provider"]; !ok {
		t.Fatal("static provider must stay in the base")
	}

	runtime := a.runtimeRawProviders()
	if _, ok := runtime["vllm-zen-backup"]; ok {
		t.Fatalf("rebuild would resurrect the deleted provider: %+v", runtime)
	}
	if _, ok := runtime["vllm-zen"]; !ok {
		t.Fatalf("override-owned provider must be re-applied to the rebuild set: %+v", runtime)
	}
	if len(runtime) != 2 {
		t.Fatalf("expected 2 providers in the rebuild set, got %d: %+v", len(runtime), runtime)
	}
}

// A base that never went through the merge is left alone.
func TestDropStartupMergedOverridesWithoutMergeNames(t *testing.T) {
	a := &App{
		rawProviders: map[string]config.RawProviderConfig{
			"static-provider": {Type: "vllm"},
		},
	}
	a.dropStartupMergedOverrides(nil)
	if _, ok := a.rawProviders["static-provider"]; !ok {
		t.Fatal("static provider must stay in the base")
	}
}
